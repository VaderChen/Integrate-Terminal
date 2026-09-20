package session

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"IntegTERM/internal/model"
	"IntegTERM/internal/transport"
)

type transferTestClient struct {
	list     func(string) ([]model.FileEntry, error)
	stat     func(string) (model.FileEntry, error)
	upload   func(string, string, func(int64, int64, int64) bool) error
	download func(string, io.Writer, func(int64, int64, int64) bool) error
}

func (*transferTestClient) Connect(model.Site) error                   { return nil }
func (*transferTestClient) CurrentDir() (string, error)                { return "/", nil }
func (c *transferTestClient) List(p string) ([]model.FileEntry, error) { return c.list(p) }
func (c *transferTestClient) Stat(p string) (model.FileEntry, error)   { return c.stat(p) }
func (c *transferTestClient) Upload(a, b string, p func(int64, int64, int64) bool) error {
	return c.upload(a, b, p)
}
func (c *transferTestClient) Download(p string, w io.Writer, progress func(int64, int64, int64) bool) error {
	return c.download(p, w, progress)
}
func (*transferTestClient) Mkdir(string) error          { return nil }
func (*transferTestClient) Remove(string) error         { return nil }
func (*transferTestClient) Rename(string, string) error { return nil }
func (*transferTestClient) Close() error                { return nil }

func TestCancelDirectoryStopsChildren(t *testing.T) {
	for _, direction := range []string{"upload", "download"} {
		t.Run(direction, func(t *testing.T) {
			m := NewManager()
			root := t.TempDir()
			for _, n := range []string{"a", "b"} {
				if err := os.WriteFile(filepath.Join(root, n), []byte(n), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var transferred []string
			transfer := func(name string, progress func(int64, int64, int64) bool) error {
				transferred = append(transferred, filepath.Base(name))
				if len(transferred) == 1 {
					for _, item := range m.SampleTransfers() {
						if item.Name == "root" {
							m.CancelTransfer(item.ID)
						}
					}
				}
				if !progress(1, 1, 1) {
					return transport.ErrTransferCancelled
				}
				return nil
			}
			c := &transferTestClient{
				stat:     func(string) (model.FileEntry, error) { return model.FileEntry{IsDir: true}, nil },
				list:     func(string) ([]model.FileEntry, error) { return []model.FileEntry{{Name: "a"}, {Name: "b"}}, nil },
				upload:   func(local, remote string, p func(int64, int64, int64) bool) error { return transfer(local, p) },
				download: func(remote string, w io.Writer, p func(int64, int64, int64) bool) error { return transfer(remote, p) },
			}
			var err error
			if direction == "upload" {
				err = m.uploadPathWithQueue(c, root, "/remote", "root")
			} else {
				err = m.downloadPathWithQueue(c, "/remote", filepath.Join(t.TempDir(), "root"), "root")
			}
			if !errors.Is(err, transport.ErrTransferCancelled) || !reflect.DeepEqual(transferred, []string{"a"}) {
				t.Fatalf("transferred=%v err=%v", transferred, err)
			}
			for _, item := range m.SampleTransfers() {
				if item.Status != "cancelled" {
					t.Fatalf("cancelled subtree status: %+v", item)
				}
			}
		})
	}
}

func TestPauseDirectoryBlocksActiveChild(t *testing.T) {
	m := NewManager()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("a"), 0600); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	proceed := make(chan struct{})
	done := make(chan error, 1)
	c := &transferTestClient{upload: func(_, _ string, p func(int64, int64, int64) bool) error {
		close(started)
		<-proceed
		if !p(1, 1, 1) {
			return transport.ErrTransferCancelled
		}
		return nil
	}}
	go func() { done <- m.uploadPathWithQueue(c, root, "/remote", "root") }()
	<-started
	var parentID string
	for _, item := range m.SampleTransfers() {
		if item.Name == "root" {
			parentID = item.ID
		}
	}
	m.TogglePauseTransfer(parentID)
	close(proceed)
	select {
	case err := <-done:
		t.Fatalf("paused directory finished: %v", err)
	case <-time.After(180 * time.Millisecond):
	}
	m.TogglePauseTransfer(parentID)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("resume did not release child")
	}
}

func TestDownloadUsesStatInsteadOfListToClassifyFile(t *testing.T) {
	m := NewManager()
	var downloaded []string
	c := &transferTestClient{
		stat: func(string) (model.FileEntry, error) { return model.FileEntry{Name: "a.txt"}, nil },
		list: func(string) ([]model.FileEntry, error) { return []model.FileEntry{{Name: "a.txt"}}, nil },
		download: func(remote string, w io.Writer, _ func(int64, int64, int64) bool) error {
			downloaded = append(downloaded, remote)
			_, err := io.WriteString(w, "file contents")
			return err
		},
	}
	local := filepath.Join(t.TempDir(), "a.txt")
	if err := m.downloadPathWithQueue(c, "/a.txt", local, "a.txt"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(downloaded, []string{"/a.txt"}) {
		t.Fatalf("download paths=%v", downloaded)
	}
	if data, err := os.ReadFile(local); err != nil || string(data) != "file contents" {
		t.Fatalf("file=%q err=%v", data, err)
	}
}

func TestDownloadPreservesExistingFileUntilSuccessfulCommit(t *testing.T) {
	for _, outcome := range []string{"cancel", "error", "success"} {
		t.Run(outcome, func(t *testing.T) {
			m := NewManager()
			dir := t.TempDir()
			local := filepath.Join(dir, "existing.txt")
			if err := os.WriteFile(local, []byte("original content"), 0640); err != nil {
				t.Fatal(err)
			}
			c := &transferTestClient{stat: func(string) (model.FileEntry, error) { return model.FileEntry{Name: "existing.txt"}, nil }, download: func(_ string, w io.Writer, p func(int64, int64, int64) bool) error {
				if b, err := os.ReadFile(local); err != nil || string(b) != "original content" {
					t.Errorf("old file changed before commit: %q %v", b, err)
				}
				if _, err := io.WriteString(w, "new content"); err != nil {
					return err
				}
				if outcome == "error" {
					return errors.New("remote transfer failed")
				}
				if outcome == "cancel" {
					for _, item := range m.SampleTransfers() {
						m.CancelTransfer(item.ID)
					}
					if !p(11, 11, 0) {
						return transport.ErrTransferCancelled
					}
				}
				return nil
			}}
			err := m.downloadPathWithQueue(c, "/existing.txt", local, "existing.txt")
			want := "original content"
			if outcome == "success" {
				want = "new content"
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("expected failed transfer")
			}
			if data, err := os.ReadFile(local); err != nil || string(data) != want {
				t.Fatalf("destination=%q err=%v want=%q", data, err, want)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary files left behind: %v %v", entries, err)
			}
		})
	}
}

func TestDownloadRejectsUnsafeRemoteEntryNames(t *testing.T) {
	for _, name := range []string{"../outside.txt", "..", ".", "a/b", "a\\b", "/absolute", "bad\x00name"} {
		t.Run(name, func(t *testing.T) {
			m := NewManager()
			downloaded := false
			c := &transferTestClient{stat: func(string) (model.FileEntry, error) { return model.FileEntry{IsDir: true}, nil }, list: func(string) ([]model.FileEntry, error) { return []model.FileEntry{{Name: name}}, nil }, download: func(string, io.Writer, func(int64, int64, int64) bool) error { downloaded = true; return nil }}
			if err := m.downloadPathWithQueue(c, "/remote", filepath.Join(t.TempDir(), "download"), "remote"); err == nil {
				t.Fatal("unsafe filename accepted")
			}
			if downloaded {
				t.Fatal("unsafe name reached file transfer")
			}
		})
	}
}

func TestDownloadCannotFollowSymlinkOutsideRoot(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "directory"}[directory], func(t *testing.T) {
			base := t.TempDir()
			outside := t.TempDir()
			target := filepath.Join(outside, "existing")
			if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			linkTarget := target
			if directory {
				linkTarget = outside
			}
			if err := os.Symlink(linkTarget, filepath.Join(base, "link")); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(base)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			relative := "link"
			if directory {
				relative = filepath.Join("link", "existing")
			}
			err = downloadFile(root, relative, func(w io.Writer) error { _, err := io.WriteString(w, "overwrite"); return err })
			if err == nil {
				t.Fatal("download followed symlink outside root")
			}
			if data, err := os.ReadFile(target); err != nil || string(data) != "keep" {
				t.Fatalf("outside file changed: %q %v", data, err)
			}
		})
	}
}
