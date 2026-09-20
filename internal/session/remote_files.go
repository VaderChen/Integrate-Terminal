package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"IntegTERM/internal/model"
	"IntegTERM/internal/transport"
)

const remoteFileMaxBytes int64 = 32 << 20
const remoteFileMaxReadBytes int64 = 1 << 20

// ReadRemoteFile enforces the limit on bytes actually received, not just remote
// metadata. Only the requested window is retained in memory.
func (m *Manager) ReadRemoteFile(tabID, remotePath string, offset, limit, maxBytes int64) ([]byte, model.FileEntry, error) {
	if offset < 0 || limit <= 0 || limit > remoteFileMaxReadBytes {
		return nil, model.FileEntry{}, fmt.Errorf("invalid read offset or limit")
	}
	if maxBytes <= 0 || maxBytes > remoteFileMaxBytes {
		return nil, model.FileEntry{}, fmt.Errorf("invalid remote file size limit")
	}
	client, err := m.remoteClient(tabID)
	if err != nil {
		return nil, model.FileEntry{}, err
	}
	entry, err := client.Stat(remotePath)
	if err != nil {
		return nil, model.FileEntry{}, err
	}
	if entry.IsDir {
		return nil, model.FileEntry{}, fmt.Errorf("remote path is a directory: %s", remotePath)
	}
	if entry.Size > maxBytes {
		return nil, model.FileEntry{}, fmt.Errorf("remote file exceeds %d bytes; use the network transfer tools for larger files", maxBytes)
	}
	window := &remoteReadWindow{offset: offset, limit: limit, maxBytes: maxBytes}
	if err := client.Download(remotePath, window, func(int64, int64, int64) bool { return true }); err != nil {
		return nil, model.FileEntry{}, err
	}
	entry.Size = window.received
	return window.data, entry, nil
}

type remoteReadWindow struct {
	offset, limit, maxBytes, received int64
	data                              []byte
}

func (w *remoteReadWindow) Write(p []byte) (int, error) {
	if int64(len(p)) > w.maxBytes-w.received {
		return 0, fmt.Errorf("remote file exceeds %d bytes", w.maxBytes)
	}
	start := int64(0)
	if w.offset > w.received {
		start = w.offset - w.received
	}
	w.received += int64(len(p))
	if start < int64(len(p)) && int64(len(w.data)) < w.limit {
		count := min(int64(len(p))-start, w.limit-int64(len(w.data)))
		w.data = append(w.data, p[start:start+count]...)
	}
	return len(p), nil
}

// The complete payload is uploaded to an unpredictable sibling before the
// transport atomically commits it. A failed upload never truncates the target.
func (m *Manager) WriteRemoteFile(tabID, remotePath string, data []byte, overwrite bool, maxBytes int64) (model.FileEntry, error) {
	if maxBytes <= 0 || maxBytes > remoteFileMaxBytes || int64(len(data)) > maxBytes {
		return model.FileEntry{}, fmt.Errorf("remote file exceeds its size limit")
	}
	if err := validateRemoteMutationPath(remotePath); err != nil {
		return model.FileEntry{}, err
	}
	client, err := m.remoteClient(tabID)
	if err != nil {
		return model.FileEntry{}, err
	}
	committer, ok := client.(interface {
		CommitFile(string, string, bool) error
	})
	if !ok {
		return model.FileEntry{}, fmt.Errorf("remote transport does not support safe file commits")
	}
	if existing, statErr := client.Stat(remotePath); statErr == nil {
		if existing.IsDir {
			return model.FileEntry{}, fmt.Errorf("remote path is a directory: %s", remotePath)
		}
		if !overwrite {
			return model.FileEntry{}, fmt.Errorf("remote file already exists: %s", remotePath)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return model.FileEntry{}, statErr
	}

	temporaryFile, err := os.CreateTemp("", "integterm-mcp-write-")
	if err != nil {
		return model.FileEntry{}, err
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporaryFile.Write(data); err != nil {
		_ = temporaryFile.Close()
		return model.FileEntry{}, err
	}
	if err := temporaryFile.Close(); err != nil {
		return model.FileEntry{}, err
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return model.FileEntry{}, err
	}
	stagedPath := path.Join(path.Dir(remotePath), ".integterm-mcp-"+hex.EncodeToString(nonce)+".tmp")
	if _, err := client.Stat(stagedPath); err == nil {
		return model.FileEntry{}, fmt.Errorf("remote temporary file already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return model.FileEntry{}, err
	}
	defer client.Remove(stagedPath)
	if err := client.Upload(temporaryPath, stagedPath, func(int64, int64, int64) bool { return true }); err != nil {
		return model.FileEntry{}, err
	}
	staged, err := client.Stat(stagedPath)
	if err != nil {
		return model.FileEntry{}, err
	}
	if staged.IsDir || staged.Size != int64(len(data)) {
		return model.FileEntry{}, fmt.Errorf("remote upload size mismatch")
	}
	if err := committer.CommitFile(stagedPath, remotePath, overwrite); err != nil {
		return model.FileEntry{}, err
	}
	entry, err := client.Stat(remotePath)
	if err != nil {
		return model.FileEntry{}, err
	}
	m.addLog(fmt.Sprintf("已寫入遠端檔案: %s", remotePath), "done")
	return entry, nil
}

func (m *Manager) StatRemote(tabID, remotePath string) (model.FileEntry, error) {
	client, err := m.remoteClient(tabID)
	if err != nil {
		return model.FileEntry{}, err
	}
	return client.Stat(remotePath)
}

func (m *Manager) IsConnected(tabID string) bool {
	_, err := m.remoteClient(tabID)
	return err == nil
}

// ValidateRemoteRootPath fails closed for transports which cannot inspect links.
// Remote filesystems can still be modified by other clients between operations;
// server-side isolation is required against adversarial concurrent renames.
func (m *Manager) ValidateRemoteRootPath(tabID, root, target string, allowMissingLeaf bool) error {
	client, err := m.remoteClient(tabID)
	if err != nil {
		return err
	}
	validator, ok := client.(interface {
		ValidateRootPath(string, string, bool) error
	})
	if !ok {
		return fmt.Errorf("remote transport cannot validate the site root")
	}
	return validator.ValidateRootPath(root, target, allowMissingLeaf)
}

func (m *Manager) DeleteRemotePathWithRecursive(tabID, remotePath string, recursive bool) error {
	if err := validateRemoteMutationPath(remotePath); err != nil {
		return err
	}
	client, err := m.remoteClient(tabID)
	if err != nil {
		return err
	}
	entry, err := client.Stat(remotePath)
	if err != nil {
		return err
	}
	if entry.IsDir && !recursive {
		children, err := client.List(remotePath)
		if err != nil {
			return err
		}
		if len(children) != 0 {
			return fmt.Errorf("directory is not empty: %s", remotePath)
		}
		if remover, ok := client.(interface{ RemoveDir(string) error }); ok {
			return remover.RemoveDir(remotePath)
		}
		return client.Remove(remotePath)
	}
	return m.deleteRemotePathRecursive(client, remotePath)
}

func (m *Manager) RenameRemotePathNoReplace(tabID, oldPath, newPath string) error {
	if err := validateRemoteMutationPath(oldPath); err != nil {
		return err
	}
	if err := validateRemoteMutationPath(newPath); err != nil {
		return err
	}
	if strings.HasPrefix(path.Clean(newPath), path.Clean(oldPath)+"/") {
		return fmt.Errorf("cannot move a remote directory into itself")
	}
	client, err := m.remoteClient(tabID)
	if err != nil {
		return err
	}
	if _, err := client.Stat(newPath); err == nil {
		return fmt.Errorf("remote destination already exists: %s", newPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return client.Rename(oldPath, newPath)
}

func validateRemoteMutationPath(value string) error {
	if strings.TrimSpace(value) == "" || path.Clean(value) == "/" || path.Clean(value) == "." {
		return fmt.Errorf("cannot modify the remote root")
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return fmt.Errorf("remote path cannot contain ..")
		}
	}
	for _, r := range value {
		if r < 32 || r == 127 || r == '\\' {
			return fmt.Errorf("invalid remote path")
		}
	}
	return nil
}

func (m *Manager) remoteClient(tabID string) (transport.Client, error) {
	m.mu.RLock()
	client, ok := m.clients[tabID]
	m.mu.RUnlock()
	if !ok || client == nil {
		return nil, fmt.Errorf("tab not connected: %s", tabID)
	}
	return client, nil
}
