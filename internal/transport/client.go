package transport

import "github.com/VaderChen/Integrate-Terminal/internal/model"

type Client interface {
	Connect(site model.Site) error
	CurrentDir() (string, error)
	List(path string) ([]model.FileEntry, error)
	Upload(localPath, remotePath string, progress func(transferred int64, total int64, speedBps int64) bool) error
	Download(remotePath, localPath string, progress func(transferred int64, total int64, speedBps int64) bool) error
	Mkdir(path string) error
	Remove(path string) error
	Rename(oldPath, newPath string) error
	Close() error
}
