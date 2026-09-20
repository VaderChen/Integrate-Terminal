package transport

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/jlaffaye/ftp"
)

// ValidateRootPath is an optional capability for the MCP remote mount. Every
// component is inspected without following links, including ancestors of root.
// It is a preflight check; a different remote writer can still change the tree
// between this check and the following operation.
func (c *SFTPClient) ValidateRootPath(root, target string, allowMissingLeaf bool) error {
	if c.sftpClient == nil {
		return fmt.Errorf("sftp client not connected")
	}
	_, _, err := inspectRemoteRootPath(root, target, allowMissingLeaf, c.mcpLstat)
	return err
}

func (c *SFTPClient) mcpLstat(remotePath string) (os.FileMode, error) {
	info, err := c.sftpClient.Lstat(remotePath)
	if err != nil {
		return 0, err
	}
	return info.Mode(), nil
}

// CommitFile never removes the old target before rename. Replacing an existing
// SFTP file requires the server's POSIX rename extension; an unsupported server
// returns an error and keeps both the original file and the staged upload.
func (c *SFTPClient) CommitFile(stagedPath, targetPath string, overwrite bool) error {
	if c.sftpClient == nil {
		return fmt.Errorf("sftp client not connected")
	}
	rename := c.sftpClient.Rename
	if overwrite {
		rename = c.sftpClient.PosixRename
	}
	return commitRemoteStagedFile(stagedPath, targetPath, overwrite, c.mcpLstat, rename)
}

func (c *FTPClient) ValidateRootPath(root, target string, allowMissingLeaf bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("ftp client not connected")
	}
	_, _, err := inspectRemoteRootPath(root, target, allowMissingLeaf, c.mcpLstatLocked)
	return err
}

// mcpLstatLocked retains FTP Entry.Type, which the general-purpose FileEntry
// representation does not expose. A successful parent listing is required to
// classify a missing leaf: permission/protocol errors are never treated as ENOENT.
func (c *FTPClient) mcpLstatLocked(remotePath string) (os.FileMode, error) {
	parent := path.Dir(remotePath)
	if remotePath == "/" {
		parent = "/"
	}
	entries, err := c.conn.List(parent)
	if err != nil {
		return 0, err
	}
	var found *ftp.Entry
	for _, entry := range entries {
		if entry.Name == "." || entry.Name == ".." {
			continue
		}
		if err := ValidateEntryName(entry.Name); err != nil {
			return 0, err
		}
		if entry.Name == path.Base(remotePath) {
			if found != nil {
				return 0, fmt.Errorf("ambiguous FTP directory entry: %s", remotePath)
			}
			found = entry
		}
	}
	if remotePath == "/" {
		return os.ModeDir, nil
	}
	if found == nil {
		// The FTP library silently skips unparseable LIST lines. Cross-check a
		// raw name listing before classifying a leaf as absent, including hidden
		// staging names. Unsupported/denied NLST fails closed.
		names, err := c.conn.NameList("-a " + parent)
		if err != nil {
			return 0, err
		}
		for _, name := range names {
			name = strings.TrimSuffix(name, "/")
			if name == "." || name == ".." || name == parent {
				continue
			}
			base := path.Base(name)
			if err := ValidateEntryName(base); err != nil {
				return 0, err
			}
			if name != base && name != path.Join(parent, base) {
				return 0, fmt.Errorf("unsupported FTP name listing entry")
			}
			if base == path.Base(remotePath) {
				return 0, fmt.Errorf("FTP entry metadata could not be verified: %s", remotePath)
			}
		}
		return 0, fmt.Errorf("stat %s: %w", remotePath, os.ErrNotExist)
	}
	switch found.Type {
	case ftp.EntryTypeFile:
		return 0, nil
	case ftp.EntryTypeFolder:
		return os.ModeDir, nil
	case ftp.EntryTypeLink:
		return os.ModeSymlink, nil
	default:
		return 0, fmt.Errorf("unsupported FTP entry type: %s", remotePath)
	}
}

// FTP has no portable atomic no-clobber rename. The preflight rejects an
// existing target when overwrite is false, but cannot exclude another writer
// racing RNFR/RNTO. A rejected rename never triggers DELETE of the old file.
func (c *FTPClient) CommitFile(stagedPath, targetPath string, overwrite bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("ftp client not connected")
	}
	return commitRemoteStagedFile(stagedPath, targetPath, overwrite, c.mcpLstatLocked, c.conn.Rename)
}

type remotePathStat func(string) (os.FileMode, error)

func cleanRemoteAbsolutePath(value string) (string, error) {
	if !path.IsAbs(value) || strings.ContainsAny(value, "\\\x00\r\n") {
		return "", fmt.Errorf("remote path must be an absolute POSIX path")
	}
	// Cleaning a/../b before checking a could erase a symlink traversal while
	// the server later evaluates the original path. Reject such input instead.
	for _, component := range strings.Split(value, "/") {
		if component == "." || component == ".." {
			return "", fmt.Errorf("remote path must not contain dot components")
		}
	}
	return path.Clean(value), nil
}

func inspectRemoteRootPath(root, target string, allowMissingLeaf bool, lstat remotePathStat) (os.FileMode, bool, error) {
	root, err := cleanRemoteAbsolutePath(root)
	if err != nil {
		return 0, false, err
	}
	target, err = cleanRemoteAbsolutePath(target)
	if err != nil {
		return 0, false, err
	}
	if root != "/" && target != root && !strings.HasPrefix(target, root+"/") {
		return 0, false, fmt.Errorf("remote path is outside its configured root")
	}
	components := []string{"/"}
	current := ""
	if target != "/" {
		for _, component := range strings.Split(strings.TrimPrefix(target, "/"), "/") {
			current += "/" + component
			components = append(components, current)
		}
	}
	for i, current := range components {
		leaf := i == len(components)-1
		mode, err := lstat(current)
		if err != nil {
			if leaf && target != root && allowMissingLeaf && errors.Is(err, os.ErrNotExist) {
				return 0, false, nil
			}
			return 0, false, fmt.Errorf("inspect remote path %s: %w", current, err)
		}
		if mode&os.ModeSymlink != 0 {
			return 0, false, fmt.Errorf("remote symlink paths are not allowed: %s", current)
		}
		if (!leaf || current == root) && !mode.IsDir() {
			return 0, false, fmt.Errorf("remote path ancestor is not a directory: %s", current)
		}
		if leaf {
			return mode, true, nil
		}
	}
	panic("absolute remote path has no components")
}

func commitRemoteStagedFile(stagedPath, targetPath string, overwrite bool, lstat remotePathStat, rename func(string, string) error) error {
	staged, err := cleanRemoteAbsolutePath(stagedPath)
	if err != nil {
		return err
	}
	target, err := cleanRemoteAbsolutePath(targetPath)
	if err != nil {
		return err
	}
	if staged == target || path.Dir(staged) != path.Dir(target) {
		return fmt.Errorf("staged file must be a distinct sibling of its target")
	}
	root := path.Dir(target)
	mode, _, err := inspectRemoteRootPath(root, staged, false, lstat)
	if err != nil {
		return err
	}
	if !mode.IsRegular() {
		return fmt.Errorf("staged upload is not a regular file")
	}
	mode, exists, err := inspectRemoteRootPath(root, target, true, lstat)
	if err != nil {
		return err
	}
	if exists {
		if !overwrite {
			return fmt.Errorf("remote target already exists: %w", os.ErrExist)
		}
		if !mode.IsRegular() {
			return fmt.Errorf("remote target is not a regular file")
		}
	}
	if err := rename(staged, target); err != nil {
		return fmt.Errorf("commit staged remote upload: %w", err)
	}
	return nil
}
