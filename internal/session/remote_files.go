package session

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/transport"
)

func (m *Manager) ReadRemoteFile(tabID string, remotePath string, offset int64, limit int64, maxBytes int64) ([]byte, model.FileEntry, error) {
	if offset < 0 {
		return nil, model.FileEntry{}, fmt.Errorf("offset must not be negative")
	}
	if limit <= 0 {
		return nil, model.FileEntry{}, fmt.Errorf("limit must be greater than zero")
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
	if maxBytes > 0 && entry.Size > maxBytes {
		return nil, model.FileEntry{}, fmt.Errorf("remote file exceeds %d bytes; use the network transfer tools for larger files", maxBytes)
	}
	if entry.Size > 0 && offset >= entry.Size {
		return []byte{}, entry, nil
	}

	temporaryFile, err := os.CreateTemp("", "integterm-mcp-read-")
	if err != nil {
		return nil, model.FileEntry{}, err
	}
	temporaryPath := temporaryFile.Name()
	if err := temporaryFile.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return nil, model.FileEntry{}, err
	}
	defer os.Remove(temporaryPath)

	if err := client.Download(remotePath, temporaryPath, func(int64, int64, int64) bool { return true }); err != nil {
		return nil, model.FileEntry{}, err
	}
	file, err := os.Open(temporaryPath)
	if err != nil {
		return nil, model.FileEntry{}, err
	}
	defer file.Close()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, model.FileEntry{}, err
	}
	data, err := io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		return nil, model.FileEntry{}, err
	}
	return data, entry, nil
}

func (m *Manager) WriteRemoteFile(tabID string, remotePath string, data []byte, overwrite bool, maxBytes int64) (model.FileEntry, error) {
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return model.FileEntry{}, fmt.Errorf("remote file exceeds %d bytes", maxBytes)
	}
	client, err := m.remoteClient(tabID)
	if err != nil {
		return model.FileEntry{}, err
	}
	if existing, statErr := client.Stat(remotePath); statErr == nil {
		if existing.IsDir {
			return model.FileEntry{}, fmt.Errorf("remote path is a directory: %s", remotePath)
		}
		if !overwrite {
			return model.FileEntry{}, fmt.Errorf("remote file already exists: %s", remotePath)
		}
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
	if err := client.Upload(temporaryPath, remotePath, func(int64, int64, int64) bool { return true }); err != nil {
		return model.FileEntry{}, err
	}
	entry, err := client.Stat(remotePath)
	if err != nil {
		return model.FileEntry{Name: path.Base(remotePath), Path: remotePath, Size: int64(len(data)), Side: "remote"}, nil
	}
	m.addLog(fmt.Sprintf("已寫入遠端檔案: %s", remotePath), "done")
	return entry, nil
}

func (m *Manager) remoteClient(tabID string) (transport.Client, error) {
	m.mu.RLock()
	client, ok := m.clients[tabID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tab not connected: %s", tabID)
	}
	return client, nil
}
