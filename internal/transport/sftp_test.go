package transport

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/sftp"
)

func connectSFTPFixture(t *testing.T, directory string) *SFTPClient {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	server, err := sftp.NewServer(serverConn, sftp.WithServerWorkingDirectory(directory))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); _ = server.Serve() }()
	client, err := sftp.NewClientPipe(clientConn, clientConn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		client.Close()
		server.Close()
		clientConn.Close()
		serverConn.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("SFTP fixture did not exit")
		}
	})
	return &SFTPClient{sftpClient: client}
}

func TestSFTPStatDoesNotFollowDirectorySymlink(t *testing.T) {
	directory := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "keep.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(directory, "link")); err != nil {
		t.Fatal(err)
	}
	client := connectSFTPFixture(t, directory)
	entry, err := client.Stat("link")
	if err != nil {
		t.Fatal(err)
	}
	if entry.IsDir {
		t.Fatal("directory symlink was classified as a directory for recursive deletion")
	}
	if err := client.Remove("link"); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(outside, "keep.txt")); err != nil || string(data) != "keep" {
		t.Fatalf("target was changed: %q %v", data, err)
	}
}

func TestSFTPDownloadStillFollowsRegularFileSymlink(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "file"), []byte("contents"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("file", filepath.Join(directory, "link")); err != nil {
		t.Fatal(err)
	}
	client := connectSFTPFixture(t, directory)
	var output bytes.Buffer
	if err := client.Download("link", &output, nil); err != nil {
		t.Fatal(err)
	}
	if output.String() != "contents" {
		t.Fatalf("download=%q", output.String())
	}
}
