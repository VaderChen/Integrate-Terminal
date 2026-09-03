package transport

import (
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"time"

	"github.com/jlaffaye/ftp"

	"github.com/VaderChen/Integrate-Terminal/internal/model"
)

type FTPClient struct {
	conn *ftp.ServerConn
}

func (c *FTPClient) Connect(site model.Site) error {
	address := fmt.Sprintf("%s:%d", site.Host, site.Port)
	conn, err := ftp.Dial(address, ftp.DialWithTimeout(10*time.Second))
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
	if c.conn == nil {
		return model.FileEntry{}, fmt.Errorf("ftp client not connected")
	}
	size, err := c.conn.FileSize(remotePath)
	if err == nil {
		modified := ""
		if c.conn.IsGetTimeSupported() {
			if value, timeErr := c.conn.GetTime(remotePath); timeErr == nil {
				modified = value.Local().Format("2006-01-02 15:04")
			}
		}
		return model.FileEntry{Name: path.Base(remotePath), Path: remotePath, Size: size, Modified: modified, Side: "remote"}, nil
	}
	entries, listErr := c.conn.List(remotePath)
	if listErr != nil {
		return model.FileEntry{}, err
	}
	if len(entries) == 0 {
		return model.FileEntry{Name: path.Base(remotePath), Path: remotePath, IsDir: true, Side: "remote"}, nil
	}
	return model.FileEntry{Name: path.Base(remotePath), Path: remotePath, IsDir: true, Side: "remote"}, nil
}

func (c *FTPClient) CurrentDir() (string, error) {
	if c.conn == nil {
		return "", fmt.Errorf("ftp client not connected")
	}
	return c.conn.CurrentDir()
}

func (c *FTPClient) Upload(localPath, remotePath string, progress func(transferred int64, total int64, speedBps int64) bool) error {
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

func (c *FTPClient) Download(remotePath, localPath string, progress func(transferred int64, total int64, speedBps int64) bool) error {
	if c.conn == nil {
		return fmt.Errorf("ftp client not connected")
	}

	src, err := c.conn.Retr(remotePath)
	if err != nil {
		return err
	}
	defer src.Close()

	size, err := c.conn.FileSize(remotePath)
	if err != nil {
		size = 0
	}

	dst, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(newProgressWriter(dst, size, progress), src)
	return err
}

func (c *FTPClient) Mkdir(remotePath string) error {
	if c.conn == nil {
		return fmt.Errorf("ftp client not connected")
	}
	return c.conn.MakeDir(remotePath)
}

func (c *FTPClient) Remove(remotePath string) error {
	if c.conn == nil {
		return fmt.Errorf("ftp client not connected")
	}
	return c.conn.Delete(remotePath)
}

func (c *FTPClient) Rename(oldPath, newPath string) error {
	if c.conn == nil {
		return fmt.Errorf("ftp client not connected")
	}
	return c.conn.Rename(oldPath, newPath)
}

func (c *FTPClient) RemoveDir(remotePath string) error {
	if c.conn == nil {
		return fmt.Errorf("ftp client not connected")
	}
	return c.conn.RemoveDir(remotePath)
}

func (c *FTPClient) Close() error {
	if c.conn != nil {
		return c.conn.Quit()
	}
	return nil
}
