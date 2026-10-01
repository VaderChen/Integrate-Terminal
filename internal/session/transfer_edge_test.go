package session

import (
	"IntegTERM/internal/model"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLateProgressCannotReactivateCancelledTransfer(t *testing.T) {
	m := NewManager()
	id := m.addTransfer("cancelled", "download")
	m.CancelTransfer(id)
	m.updateTransfer(id, 20, 100, "running")
	if got := m.SampleTransfers()[0]; got.Status != "cancelled" || got.SpeedBps != 0 {
		t.Fatalf("late progress reactivated cancellation: %+v", got)
	}
	m.finishPathTransfer(id, nil)
}

type cycleUploadClient struct {
	transferTestClient
	directories int
}

func (c *cycleUploadClient) Mkdir(string) error {
	c.directories++
	if c.directories > 8 {
		return errors.New("test recursion limit")
	}
	return nil
}
func TestUploadStopsDirectorySymlinkCycle(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(root, filepath.Join(root, "again")); err != nil {
		t.Fatal(err)
	}
	c := &cycleUploadClient{transferTestClient: transferTestClient{}}
	// 避免舊版失敗路徑繼續查詢網路。
	c.stat = func(string) (entry model.FileEntry, err error) { return entry, os.ErrNotExist }
	err := NewManager().uploadPathWithQueue(c, root, "/remote", "root")
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle not detected: directories=%d err=%v", c.directories, err)
	}
	if c.directories > 1 {
		t.Fatalf("re-entered ancestor %d times", c.directories)
	}
}

func TestUploadAllowsNonCyclicDirectoryAliases(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "data")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dir, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	var uploads int
	c := &transferTestClient{upload: func(_, _ string, _ func(int64, int64, int64) bool) error { uploads++; return nil }}
	if err := NewManager().uploadPathWithQueue(c, root, "/remote", "root"); err != nil {
		t.Fatal(err)
	}
	if uploads != 2 {
		t.Fatalf("valid aliases skipped: %d", uploads)
	}
}

func TestUploadRejectsSocketBeforeOpeningIt(t *testing.T) {
	// macOS 的 Unix socket 路徑有長度上限，避免沿用較長的系統暫存目錄。
	directory, err := os.MkdirTemp("/tmp", "integterm-socket-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	target := filepath.Join(directory, "socket")
	listener, err := net.Listen("unix", target)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	c := &transferTestClient{upload: func(_, _ string, _ func(int64, int64, int64) bool) error {
		t.Fatal("transport opened special file")
		return nil
	}}
	if err := NewManager().uploadPathWithQueue(c, target, "/remote", "socket"); err == nil {
		t.Fatal("special file accepted")
	}
}

func TestRemoteBasePreservesTrailingWhitespace(t *testing.T) {
	for input, want := range map[string]string{"/data ": "/data ", "folder ": "/home/folder ", "~/folder ": "/home/folder "} {
		if got := resolveRemoteBasePath("/home", input); got != want {
			t.Fatalf("%q resolved to %q, want %q", input, got, want)
		}
	}
}
