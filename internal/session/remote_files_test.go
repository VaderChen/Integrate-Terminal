package session

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	"IntegTERM/internal/model"
)

type fakeRemoteFileClient struct {
	mu                            sync.Mutex
	files                         map[string][]byte
	dirs                          map[string]bool
	statErr, uploadErr, commitErr error
	metadataSize                  *int64
	uploads                       int
	shortUpload                   bool
	removed                       []string
}

func (c *fakeRemoteFileClient) Connect(model.Site) error    { return nil }
func (c *fakeRemoteFileClient) CurrentDir() (string, error) { return "/workspace", nil }
func (c *fakeRemoteFileClient) List(dir string) ([]model.FileEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var entries []model.FileEntry
	for p, data := range c.files {
		if path.Dir(p) == dir {
			entries = append(entries, model.FileEntry{Name: path.Base(p), Path: p, Size: int64(len(data))})
		}
	}
	for p := range c.dirs {
		if path.Dir(p) == dir && p != dir {
			entries = append(entries, model.FileEntry{Name: path.Base(p), Path: p, IsDir: true})
		}
	}
	return entries, nil
}
func (c *fakeRemoteFileClient) Stat(p string) (model.FileEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.statErr != nil {
		return model.FileEntry{}, c.statErr
	}
	if c.dirs[p] {
		return model.FileEntry{Name: path.Base(p), Path: p, IsDir: true}, nil
	}
	data, ok := c.files[p]
	if !ok {
		return model.FileEntry{}, os.ErrNotExist
	}
	size := int64(len(data))
	if c.metadataSize != nil {
		size = *c.metadataSize
	}
	return model.FileEntry{Name: path.Base(p), Path: p, Size: size, Side: "remote"}, nil
}
func (c *fakeRemoteFileClient) Upload(local, remote string, progress func(int64, int64, int64) bool) error {
	data, err := os.ReadFile(local)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.uploads++
	if c.uploadErr != nil || c.shortUpload {
		data = data[:len(data)/2]
	}
	c.files[remote] = append([]byte(nil), data...)
	return c.uploadErr
}
func (c *fakeRemoteFileClient) Download(p string, w io.Writer, progress func(int64, int64, int64) bool) error {
	c.mu.Lock()
	data, ok := c.files[p]
	data = append([]byte(nil), data...)
	c.mu.Unlock()
	if !ok {
		return os.ErrNotExist
	}
	// Deliberately provide many writes so range collection and capacity checks
	// cannot depend on io.Copy using a single buffer.
	for len(data) > 0 {
		n := min(3, len(data))
		if _, err := w.Write(data[:n]); err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}
func (c *fakeRemoteFileClient) Mkdir(p string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dirs[p] = true
	return nil
}
func (c *fakeRemoteFileClient) Remove(p string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.files, p)
	delete(c.dirs, p)
	c.removed = append(c.removed, p)
	return nil
}
func (c *fakeRemoteFileClient) Rename(a, b string) error { return c.CommitFile(a, b, false) }
func (c *fakeRemoteFileClient) Close() error             { return nil }
func (c *fakeRemoteFileClient) CommitFile(a, b string, overwrite bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.commitErr != nil {
		return c.commitErr
	}
	if _, exists := c.files[b]; exists && !overwrite {
		return os.ErrExist
	}
	data, exists := c.files[a]
	if !exists {
		return os.ErrNotExist
	}
	c.files[b] = data
	delete(c.files, a)
	return nil
}
func (c *fakeRemoteFileClient) ValidateRootPath(root, target string, missing bool) error {
	if target != root && !strings.HasPrefix(target, root+"/") {
		return fmt.Errorf("outside root")
	}
	_, err := c.Stat(target)
	if missing && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func newRemoteFileTestManager() (*Manager, *fakeRemoteFileClient) {
	m := NewManager()
	c := &fakeRemoteFileClient{files: map[string][]byte{"/workspace/readme.txt": []byte("hello world")}, dirs: map[string]bool{"/workspace": true}}
	m.clients["tab-1"] = c
	return m, c
}
func TestRemoteFileReadAndWrite(t *testing.T) {
	m, c := newRemoteFileTestManager()
	data, entry, err := m.ReadRemoteFile("tab-1", "/workspace/readme.txt", 1, 3, 1024)
	if err != nil || string(data) != "ell" || entry.Size != 11 {
		t.Fatalf("read = %q, %+v, %v", data, entry, err)
	}
	if _, err := m.WriteRemoteFile("tab-1", "/workspace/readme.txt", []byte("updated"), false, 1024); err == nil {
		t.Fatal("overwrite=false accepted")
	}
	updated, err := m.WriteRemoteFile("tab-1", "/workspace/readme.txt", []byte("updated"), true, 1024)
	if err != nil || updated.Size != 7 || string(c.files["/workspace/readme.txt"]) != "updated" {
		t.Fatalf("write = %+v, %v", updated, err)
	}
	if len(c.files) != 1 {
		t.Fatalf("staging files leaked: %v", c.files)
	}
}
func TestRemoteFileReadBoundsActualBytesAndHandlesLargeOffset(t *testing.T) {
	m, c := newRemoteFileTestManager()
	understated := int64(0)
	c.metadataSize = &understated
	if _, _, err := m.ReadRemoteFile("tab-1", "/workspace/readme.txt", 0, 3, 5); err == nil {
		t.Fatal("lying metadata bypassed actual download limit")
	}
	data, entry, err := m.ReadRemoteFile("tab-1", "/workspace/readme.txt", math.MaxInt64, 3, 1024)
	if err != nil || len(data) != 0 || entry.Size != 11 {
		t.Fatalf("max offset = %q, %+v, %v", data, entry, err)
	}
	data, entry, err = m.ReadRemoteFile("tab-1", "/workspace/readme.txt", 7, 3, 1024)
	if err != nil || string(data) != "orl" || entry.Size != 11 {
		t.Fatalf("window = %q, %+v, %v", data, entry, err)
	}
	if _, _, err := m.ReadRemoteFile("tab-1", "/workspace/readme.txt", 0, math.MaxInt64, 1024); err == nil {
		t.Fatal("unbounded read accepted")
	}
}
func TestRemoteFileFailedUploadOrCommitPreservesExistingFile(t *testing.T) {
	for _, failure := range []string{"upload", "commit", "short upload", "stat"} {
		t.Run(failure, func(t *testing.T) {
			m, c := newRemoteFileTestManager()
			original := bytes.Clone(c.files["/workspace/readme.txt"])
			sentinel := errors.New("injected " + failure)
			switch failure {
			case "upload":
				c.uploadErr = sentinel
			case "commit":
				c.commitErr = sentinel
			case "short upload":
				c.shortUpload = true
			case "stat":
				c.statErr = sentinel
			}
			_, err := m.WriteRemoteFile("tab-1", "/workspace/readme.txt", []byte("replacement"), true, 1024)
			if err == nil {
				t.Fatal("failure was hidden")
			}
			if !bytes.Equal(c.files["/workspace/readme.txt"], original) || len(c.files) != 1 {
				t.Fatalf("original changed or staging leaked: %v", c.files)
			}
			if failure == "stat" && c.uploads != 0 {
				t.Fatal("stat permission error was treated as nonexistent")
			}
		})
	}
}
func TestRemoteFileDeleteRecursiveFlagAndRootProtection(t *testing.T) {
	m, c := newRemoteFileTestManager()
	for _, p := range []string{"", "/", ".", "/workspace/..", "/workspace/../secret"} {
		if err := m.DeleteRemotePathWithRecursive("tab-1", p, true); err == nil {
			t.Fatalf("allowed root/traversal %q", p)
		}
	}
	if err := m.DeleteRemotePathWithRecursive("tab-1", "/workspace", false); err == nil {
		t.Fatal("deleted nonempty directory without recursive")
	}
	if len(c.removed) != 0 {
		t.Fatalf("failed deletion mutated remote: %v", c.removed)
	}
	if err := m.DeleteRemotePathWithRecursive("tab-1", "/workspace", true); err != nil {
		t.Fatal(err)
	}
	if len(c.files) != 0 || len(c.dirs) != 0 {
		t.Fatal("recursive deletion left children")
	}
}
func TestRemoteRenameRejectsExistingTargetAndStatErrors(t *testing.T) {
	m, c := newRemoteFileTestManager()
	c.files["/workspace/other"] = []byte("preserve")
	if err := m.RenameRemotePathNoReplace("tab-1", "/workspace/readme.txt", "/workspace/other"); err == nil {
		t.Fatal("overwrote destination")
	}
	c.statErr = os.ErrPermission
	if err := m.RenameRemotePathNoReplace("tab-1", "/workspace/readme.txt", "/workspace/new"); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("permission error = %v", err)
	}
	if string(c.files["/workspace/other"]) != "preserve" || len(c.files) != 2 {
		t.Fatal("rename mutated files on failed check")
	}
}
