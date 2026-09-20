package transport

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkg/sftp"
)

func TestRemoteRootPathFailsClosed(t *testing.T) {
	paths := map[string]os.FileMode{
		"/": os.ModeDir, "/root": os.ModeDir, "/root/dir": os.ModeDir,
		"/root/file": 0, "/root/dir/file": 0, "/root/link": os.ModeSymlink,
		"/linked-root": os.ModeSymlink,
	}
	for _, test := range []struct {
		name, root, target string
		missing, valid     bool
	}{
		{"existing-file", "/root", "/root/file", false, true},
		{"root-directory", "/root", "/root", false, true},
		{"missing-leaf", "/root", "/root/new", true, true},
		{"required-leaf", "/root", "/root/new", false, false},
		{"missing-parent", "/root", "/root/absent/new", true, false},
		{"missing-root", "/absent", "/absent", true, false},
		{"sibling-prefix", "/root", "/root-other/file", true, false},
		{"relative-root", "root", "/root/file", false, false},
		{"relative-target", "/root", "root/file", false, false},
		{"dot-component", "/root", "/root/link/../file", false, false},
		{"nul", "/root", "/root/file\x00", true, false},
		{"ftp-command-injection", "/root", "/root/new\r\nDELE old", true, false},
		{"symlink-leaf", "/root", "/root/link", true, false},
		{"symlink-parent", "/root", "/root/link/new", true, false},
		{"symlink-root", "/linked-root", "/linked-root/new", true, false},
		{"file-parent", "/root", "/root/file/new", true, false},
		{"permission-not-missing", "/root", "/root/denied", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			lstat := func(value string) (os.FileMode, error) {
				if value == "/root/denied" {
					return 0, os.ErrPermission
				}
				mode, ok := paths[value]
				if !ok {
					return 0, os.ErrNotExist
				}
				return mode, nil
			}
			_, _, err := inspectRemoteRootPath(test.root, test.target, test.missing, lstat)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%t error=%v", test.valid, err)
			}
			if test.name == "permission-not-missing" && !errors.Is(err, os.ErrPermission) {
				t.Fatalf("permission error lost: %v", err)
			}
		})
	}
}

func TestSFTPValidateRootPathRejectsAncestorAndLeafSymlinks(t *testing.T) {
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(directory, "root")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(directory, "linked-root")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(directory, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	client := connectSFTPFixture(t, directory)
	if err := client.ValidateRootPath(root, filepath.Join(root, "file"), false); err != nil {
		t.Fatal(err)
	}
	if err := client.ValidateRootPath(root, filepath.Join(root, "new"), true); err != nil {
		t.Fatalf("missing leaf should be permitted: %v", err)
	}
	for _, test := range [][2]string{
		{root, filepath.Join(root, "escape")},
		{root, filepath.Join(root, "escape", "new")},
		{filepath.Join(directory, "linked-root"), filepath.Join(directory, "linked-root", "file")},
	} {
		if err := client.ValidateRootPath(test[0], test[1], true); err == nil {
			t.Fatalf("symlink path accepted: %v", test)
		}
	}
}

func TestFTPValidateRootPathUsesParsedLinkType(t *testing.T) {
	client := connectFTPFixture(t, &ftpFixture{listing: "lrwxrwxrwx 1 owner group 8 Jan 01 2026 link -> /outside\r\n-rw-r--r-- 1 owner group 4 Jan 01 2026 file\r\n"})
	if err := client.ValidateRootPath("/", "/file", false); err != nil {
		t.Fatal(err)
	}
	if err := client.ValidateRootPath("/", "/new", true); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/link", "/link/file"} {
		if err := client.ValidateRootPath("/", target, true); err == nil {
			t.Fatalf("FTP symlink accepted: %s", target)
		}
	}
}

func TestFTPValidateRootPathDoesNotMistakeUnparsedEntryForMissing(t *testing.T) {
	fixture := &ftpFixture{listing: "unrecognized metadata for hidden-link\r\n", names: "hidden-link\r\n"}
	client := connectFTPFixture(t, fixture)
	if err := client.ValidateRootPath("/", "/hidden-link", true); err == nil {
		t.Fatal("unparsed FTP entry was accepted as a missing leaf")
	}
	fixture.mu.Lock()
	commands := strings.Join(fixture.commands, "\n")
	fixture.mu.Unlock()
	if !strings.Contains(commands, "LIST -a /") || !strings.Contains(commands, "NLST -a /") {
		t.Fatalf("hidden/raw name verification missing: %s", commands)
	}
}

func TestFTPCommitDoesNotDeleteOriginalOnFailure(t *testing.T) {
	for _, overwrite := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-clobber", true: "rename-rejected"}[overwrite], func(t *testing.T) {
			fixture := &ftpFixture{listing: "-rw-r--r-- 1 owner group 4 Jan 01 2026 target\r\n-rw-r--r-- 1 owner group 3 Jan 01 2026 staged\r\n", renameReply: "550 Rename rejected"}
			client := connectFTPFixture(t, fixture)
			// The fixture accepts RNFR but rejects RNTO. Never fall back to DELE.
			if err := client.CommitFile("/staged", "/target", overwrite); err == nil {
				t.Fatal("failed commit returned success")
			}
			fixture.mu.Lock()
			commands := strings.Join(fixture.commands, "\n")
			fixture.mu.Unlock()
			if strings.Contains(commands, "DELE ") {
				t.Fatalf("commit deleted a file before/after failed rename: %s", commands)
			}
			if strings.Contains(commands, "RNFR ") != overwrite {
				t.Fatalf("overwrite=%t wrong rename behavior: %s", overwrite, commands)
			}
			if strings.Contains(commands, "RNTO /target") != overwrite {
				t.Fatalf("overwrite=%t target rename not verified: %s", overwrite, commands)
			}
		})
	}
}

func TestFTPCommitCreatesMissingTargetWithRename(t *testing.T) {
	fixture := &ftpFixture{listing: "-rw-r--r-- 1 owner group 3 Jan 01 2026 .staged\r\n", names: ".staged\r\n", renameReply: "250 Renamed"}
	client := connectFTPFixture(t, fixture)
	if err := client.CommitFile("/.staged", "/created", false); err != nil {
		t.Fatal(err)
	}
	fixture.mu.Lock()
	commands := strings.Join(fixture.commands, "\n")
	fixture.mu.Unlock()
	if !strings.Contains(commands, "RNFR /.staged\nRNTO /created") || strings.Contains(commands, "DELE ") {
		t.Fatalf("unexpected commit commands: %s", commands)
	}
}

type unsupportedPosixRename struct{ sftp.FileCmder }

func (unsupportedPosixRename) PosixRename(*sftp.Request) error { return sftp.ErrSSHFxOpUnsupported }

func connectMCPMemorySFTP(t *testing.T, unsupportedRename bool) *SFTPClient {
	t.Helper()
	handlers := sftp.InMemHandler()
	if unsupportedRename {
		handlers.FileCmd = unsupportedPosixRename{handlers.FileCmd}
	}
	return connectMCPRequestSFTP(t, handlers)
}

func connectMCPRequestSFTP(t *testing.T, handlers sftp.Handlers) *SFTPClient {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	server := sftp.NewRequestServer(serverConn, handlers)
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
			t.Error("memory SFTP server did not exit")
		}
	})
	return &SFTPClient{sftpClient: client}
}

func writeMCPTestFile(t *testing.T, client *SFTPClient, name, data string) {
	t.Helper()
	file, err := client.sftpClient.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(file, data); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func readMCPTestFile(t *testing.T, client *SFTPClient, name string) string {
	t.Helper()
	file, err := client.sftpClient.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSFTPCommitUsesPosixRenameAndKeepsOldFileOnUnsupportedServer(t *testing.T) {
	for _, unsupported := range []bool{false, true} {
		t.Run(map[bool]string{false: "replace", true: "unsupported"}[unsupported], func(t *testing.T) {
			client := connectMCPMemorySFTP(t, unsupported)
			writeMCPTestFile(t, client, "/target", "old-content")
			writeMCPTestFile(t, client, "/staged", "new-content")
			err := client.CommitFile("/staged", "/target", true)
			if unsupported {
				if err == nil {
					t.Fatal("unsupported POSIX rename reported success")
				}
				if got := readMCPTestFile(t, client, "/target"); got != "old-content" {
					t.Fatalf("old target was changed: %q", got)
				}
				if got := readMCPTestFile(t, client, "/staged"); got != "new-content" {
					t.Fatalf("staged file was removed: %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := readMCPTestFile(t, client, "/target"); got != "new-content" {
				t.Fatalf("replacement content=%q", got)
			}
			if _, err := client.sftpClient.Lstat("/staged"); !os.IsNotExist(err) {
				t.Fatalf("staged file should have moved: %v", err)
			}
		})
	}
}

func TestSFTPCommitRefusesExistingTargetAndInvalidStage(t *testing.T) {
	client := connectMCPMemorySFTP(t, false)
	writeMCPTestFile(t, client, "/target", "old")
	writeMCPTestFile(t, client, "/staged", "new")
	if err := client.CommitFile("/staged", "/target", false); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing target must be rejected: %v", err)
	}
	for _, staged := range []string{"/missing", "/target", "/other/staged"} {
		if err := client.CommitFile(staged, "/target", true); err == nil {
			t.Fatalf("invalid staging path accepted: %s", staged)
		}
	}
	if got := readMCPTestFile(t, client, "/target"); got != "old" {
		t.Fatalf("old file changed on rejected commit: %q", got)
	}
	if err := client.CommitFile("/staged", "/created", false); err != nil {
		t.Fatal(err)
	}
	if got := readMCPTestFile(t, client, "/created"); got != "new" {
		t.Fatalf("new file content=%q", got)
	}
}

type closeFailureFileWriter struct{ sftp.FileWriter }

func (w closeFailureFileWriter) Filewrite(request *sftp.Request) (io.WriterAt, error) {
	writer, err := w.FileWriter.Filewrite(request)
	if err != nil {
		return nil, err
	}
	return closeFailureWriter{writer}, nil
}

type closeFailureWriter struct{ io.WriterAt }

func (w closeFailureWriter) Close() error {
	if closer, ok := w.WriterAt.(io.Closer); ok {
		_ = closer.Close()
	}
	return errors.New("test server rejected final upload close")
}

func TestSFTPUploadReportsServerCloseFailureBeforeCommit(t *testing.T) {
	handlers := sftp.InMemHandler()
	handlers.FilePut = closeFailureFileWriter{handlers.FilePut}
	client := connectMCPRequestSFTP(t, handlers)
	source := filepath.Join(t.TempDir(), "source")
	if err := os.WriteFile(source, []byte("new-content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := client.Upload(source, "/staged", nil); err == nil {
		t.Fatal("server close failure was reported as a completed upload")
	}
}
