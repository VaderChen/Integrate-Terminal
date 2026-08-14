package transport

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/sshutil"
)

type SFTPClient struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	homeDir    string
}

func (c *SFTPClient) Connect(site model.Site) error {
	authMethods := make([]ssh.AuthMethod, 0, 2)

	if site.Password != "" {
		authMethods = append(authMethods, ssh.Password(site.Password))
	}
	if site.PPKPath != "" {
		signer, err := sshutil.SignerFromPPK(site.PPKPath, site.PPKPassphrase)
		if err != nil {
			return fmt.Errorf("load ppk: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if len(authMethods) == 0 {
		return fmt.Errorf("missing sftp auth method")
	}

	hostKeyCallback, err := sshutil.KnownHostsCallback()
	if err != nil {
		return err
	}

	sshConfig := &ssh.ClientConfig{
		User:            site.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	address := fmt.Sprintf("%s:%d", site.Host, site.Port)
	sshClient, err := sshutil.DialWithRouteRetry("tcp", address, sshConfig)
	if err != nil {
		var trustErr *sshutil.HostTrustRequiredError
		if errors.As(err, &trustErr) {
			return trustErr
		}
		return err
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return err
	}

	c.sshClient = sshClient
	c.sftpClient = sftpClient
	c.homeDir = c.resolveHomeDir()
	return nil
}

func (c *SFTPClient) List(remotePath string) ([]model.FileEntry, error) {
	if c.sftpClient == nil {
		return nil, fmt.Errorf("sftp client not connected")
	}
	if remotePath == "" {
		remotePath = "."
	}

	entries, err := c.sftpClient.ReadDir(remotePath)
	if err != nil {
		return nil, err
	}

	items := make([]model.FileEntry, 0, len(entries))
	for _, entry := range entries {
		items = append(items, model.FileEntry{
			Name:     entry.Name(),
			Path:     path.Join(remotePath, entry.Name()),
			Size:     entry.Size(),
			Modified: entry.ModTime().Format("2006-01-02 15:04"),
			IsDir:    entry.IsDir(),
			Side:     "remote",
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return items[i].Name < items[j].Name
	})

	return items, nil
}

func (c *SFTPClient) CurrentDir() (string, error) {
	if c.sftpClient == nil {
		return "", fmt.Errorf("sftp client not connected")
	}
	return c.sftpClient.Getwd()
}

func (c *SFTPClient) HomeDir() string {
	return c.homeDir
}

func (c *SFTPClient) Upload(localPath, remotePath string, progress func(transferred int64, total int64, speedBps int64) bool) error {
	if c.sftpClient == nil {
		return fmt.Errorf("sftp client not connected")
	}

	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return err
	}

	dst, err := c.sftpClient.Create(remotePath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, newProgressReader(src, info.Size(), progress))
	return err
}

func (c *SFTPClient) Download(remotePath, localPath string, progress func(transferred int64, total int64, speedBps int64) bool) error {
	if c.sftpClient == nil {
		return fmt.Errorf("sftp client not connected")
	}

	src, err := c.sftpClient.Open(remotePath)
	if err != nil {
		return err
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return err
	}

	dst, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(newProgressWriter(dst, info.Size(), progress), src)
	return err
}

func (c *SFTPClient) Mkdir(remotePath string) error {
	if c.sftpClient == nil {
		return fmt.Errorf("sftp client not connected")
	}
	return c.sftpClient.Mkdir(remotePath)
}

func (c *SFTPClient) Remove(remotePath string) error {
	if c.sftpClient == nil {
		return fmt.Errorf("sftp client not connected")
	}
	return c.sftpClient.Remove(remotePath)
}

func (c *SFTPClient) Rename(oldPath, newPath string) error {
	if c.sftpClient == nil {
		return fmt.Errorf("sftp client not connected")
	}
	return c.sftpClient.Rename(oldPath, newPath)
}

func (c *SFTPClient) RemoveDir(remotePath string) error {
	if c.sftpClient == nil {
		return fmt.Errorf("sftp client not connected")
	}
	return c.sftpClient.RemoveDirectory(remotePath)
}

func (c *SFTPClient) Close() error {
	if c.sftpClient != nil {
		_ = c.sftpClient.Close()
	}
	if c.sshClient != nil {
		return c.sshClient.Close()
	}
	return nil
}

func (c *SFTPClient) resolveHomeDir() string {
	if c.sshClient == nil {
		return ""
	}

	session, err := c.sshClient.NewSession()
	if err != nil {
		return ""
	}
	defer session.Close()

	output, err := session.Output("pwd")
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}
