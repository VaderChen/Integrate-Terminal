package session

import (
	"fmt"
	"os"
	"testing"

	"github.com/VaderChen/Integrate-Terminal/internal/model"
)

type fakeRemoteFileClient struct {
	files map[string][]byte
}

func (c *fakeRemoteFileClient) Connect(model.Site) error { return nil }

func (c *fakeRemoteFileClient) CurrentDir() (string, error) { return "/workspace", nil }

func (c *fakeRemoteFileClient) List(string) ([]model.FileEntry, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *fakeRemoteFileClient) Stat(remotePath string) (model.FileEntry, error) {
	data, ok := c.files[remotePath]
	if !ok {
		return model.FileEntry{}, fmt.Errorf("not found")
	}
	return model.FileEntry{Name: remotePath, Path: remotePath, Size: int64(len(data)), Side: "remote"}, nil
}

func (c *fakeRemoteFileClient) Upload(localPath string, remotePath string, progress func(int64, int64, int64) bool) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}
	c.files[remotePath] = append([]byte(nil), data...)
	return nil
}

func (c *fakeRemoteFileClient) Download(remotePath string, localPath string, progress func(int64, int64, int64) bool) error {
	data, ok := c.files[remotePath]
	if !ok {
		return fmt.Errorf("not found")
	}
	return os.WriteFile(localPath, data, 0o600)
}

func (c *fakeRemoteFileClient) Mkdir(string) error { return nil }

func (c *fakeRemoteFileClient) Remove(string) error { return nil }

func (c *fakeRemoteFileClient) Rename(string, string) error { return nil }

func (c *fakeRemoteFileClient) Close() error { return nil }

func TestRemoteFileReadAndWrite(t *testing.T) {
	manager := NewManager()
	client := &fakeRemoteFileClient{files: map[string][]byte{"/workspace/readme.txt": []byte("hello world")}}
	manager.clients["tab-1"] = client

	data, entry, err := manager.ReadRemoteFile("tab-1", "/workspace/readme.txt", 1, 3, 1024)
	if err != nil {
		t.Fatalf("ReadRemoteFile() returned error: %v", err)
	}
	if string(data) != "ell" || entry.Size != 11 {
		t.Fatalf("ReadRemoteFile() = %q, %#v", data, entry)
	}

	if _, err := manager.WriteRemoteFile("tab-1", "/workspace/readme.txt", []byte("updated"), false, 1024); err == nil {
		t.Fatal("expected overwrite=false to reject an existing remote file")
	}
	updated, err := manager.WriteRemoteFile("tab-1", "/workspace/readme.txt", []byte("updated"), true, 1024)
	if err != nil {
		t.Fatalf("WriteRemoteFile() returned error: %v", err)
	}
	if updated.Size != 7 || string(client.files["/workspace/readme.txt"]) != "updated" {
		t.Fatalf("WriteRemoteFile() did not update the remote file: %#v", updated)
	}
}
