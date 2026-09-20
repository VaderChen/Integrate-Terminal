package transport

import (
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"

	"IntegTERM/internal/model"
)

type FTPClient struct {
	mu          sync.Mutex
	networkMu   sync.Mutex
	connections map[*ftpConnection]struct{}
	closed      bool
	conn        *ftp.ServerConn
}

// Deadlines apply to each network operation, so a stalled peer cannot keep a
// transfer blocked forever. Pausing between reads does not consume the timeout.
type ftpConnection struct {
	net.Conn
	owner *FTPClient
}

func (c *ftpConnection) Read(p []byte) (int, error) {
	if err := c.Conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return 0, err
	}
	return c.Conn.Read(p)
}

func (c *ftpConnection) Write(p []byte) (int, error) {
	if err := c.Conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return 0, err
	}
	return c.Conn.Write(p)
}

func (c *ftpConnection) Close() error {
	err := c.Conn.Close()
	c.owner.networkMu.Lock()
	delete(c.owner.connections, c)
	c.owner.networkMu.Unlock()
	return err
}

func (c *FTPClient) Connect(site model.Site) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.networkMu.Lock()
	c.closed = false
	c.networkMu.Unlock()
	address := net.JoinHostPort(site.Host, strconv.Itoa(site.Port))
	conn, err := ftp.Dial(address, ftp.DialWithForceListHidden(true), ftp.DialWithDialFunc(func(network, address string) (net.Conn, error) {
		conn, err := net.DialTimeout(network, address, 10*time.Second)
		if err != nil {
			return nil, err
		}
		wrapped := &ftpConnection{Conn: conn, owner: c}
		c.networkMu.Lock()
		if c.closed {
			c.networkMu.Unlock()
			_ = conn.Close()
			return nil, net.ErrClosed
		}
		if c.connections == nil {
			c.connections = make(map[*ftpConnection]struct{})
		}
		c.connections[wrapped] = struct{}{}
		c.networkMu.Unlock()
		return wrapped, nil
	}))
	if err != nil {
		return err
	}

	if err := conn.Login(site.Username, site.Password); err != nil {
		_ = conn.Quit()
		return err
	}

	c.conn = conn
	return nil
}

func (c *FTPClient) List(remotePath string) ([]model.FileEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil, fmt.Errorf("ftp client not connected")
	}
	if remotePath == "" {
		remotePath = "."
	}

	entries, err := c.conn.List(remotePath)
	if err != nil {
		return nil, err
	}

	items := make([]model.FileEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Name == "." || entry.Name == ".." {
			continue
		}
		if err := ValidateEntryName(entry.Name); err != nil {
			return nil, err
		}
		items = append(items, model.FileEntry{
			Name:     entry.Name,
			Path:     path.Join(remotePath, entry.Name),
			Size:     int64(entry.Size),
			Modified: entry.Time.Format("2006-01-02 15:04"),
			IsDir:    entry.Type == ftp.EntryTypeFolder,
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

func (c *FTPClient) Stat(remotePath string) (model.FileEntry, error) {
	remotePath = path.Clean(remotePath)
	// Root and cwd are directory references, not filenames in their parent.
	if remotePath == "/" || remotePath == "." {
		if _, err := c.List(remotePath); err != nil {
			return model.FileEntry{}, err
		}
		return model.FileEntry{Name: path.Base(remotePath), Path: remotePath, IsDir: true, Side: "remote"}, nil
	}
	entries, err := c.List(path.Dir(remotePath))
	if err != nil {
		return model.FileEntry{}, err
	}
	for _, entry := range entries {
		if entry.Name == path.Base(remotePath) {
			entry.Path = remotePath
			return entry, nil
		}
	}
	return model.FileEntry{}, fmt.Errorf("stat %s: %w", remotePath, os.ErrNotExist)
}

func (c *FTPClient) CurrentDir() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return "", fmt.Errorf("ftp client not connected")
	}
	return c.conn.CurrentDir()
}

func (c *FTPClient) Upload(localPath, remotePath string, progress func(transferred int64, total int64, speedBps int64) bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("ftp client not connected")
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

	return c.conn.Stor(remotePath, newProgressReader(src, info.Size(), progress))
}

func (c *FTPClient) Download(remotePath string, destination io.Writer, progress func(transferred int64, total int64, speedBps int64) bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("ftp client not connected")
	}

	size, err := c.conn.FileSize(remotePath)
	sizeKnown := err == nil
	if err != nil {
		size = 0
	}

	src, err := c.conn.Retr(remotePath)
	if err != nil {
		return err
	}
	transferred, copyErr := io.Copy(newProgressWriter(destination, size, progress), src)
	// Close consumes the final FTP reply. Do not issue another command until it
	// completes, and never commit a file when the server rejects the transfer.
	closeErr := src.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if sizeKnown && transferred != size {
		return fmt.Errorf("FTP download size mismatch: got %d, expected %d", transferred, size)
	}
	return nil
}

func (c *FTPClient) Mkdir(remotePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("ftp client not connected")
	}
	return c.conn.MakeDir(remotePath)
}

func (c *FTPClient) Remove(remotePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("ftp client not connected")
	}
	return c.conn.Delete(remotePath)
}

func (c *FTPClient) Rename(oldPath, newPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("ftp client not connected")
	}
	return c.conn.Rename(oldPath, newPath)
}

func (c *FTPClient) RemoveDir(remotePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("ftp client not connected")
	}
	return c.conn.RemoveDir(remotePath)
}

func (c *FTPClient) Close() error {
	// Closing must interrupt a blocked operation instead of waiting for mu or
	// sending QUIT into another command's response stream.
	c.networkMu.Lock()
	c.closed = true
	connections := make([]*ftpConnection, 0, len(c.connections))
	for conn := range c.connections {
		connections = append(connections, conn)
	}
	c.networkMu.Unlock()
	var firstError error
	for _, conn := range connections {
		if err := conn.Close(); err != nil && firstError == nil {
			firstError = err
		}
	}
	return firstError
}
