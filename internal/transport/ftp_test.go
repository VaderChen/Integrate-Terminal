package transport

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"IntegTERM/internal/model"
)

type ftpFixture struct {
	listing     string
	names       string
	size        int64
	bytesSent   int64
	finalReply  string
	renameReply string
	mu          sync.Mutex
	commands    []string
}

func connectFTPFixture(t *testing.T, fixture *ftpFixture) *FTPClient {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan net.Conn, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		control, err := listener.Accept()
		if err != nil {
			return
		}
		accepted <- control
		defer control.Close()
		control.SetDeadline(time.Now().Add(8 * time.Second))
		fmt.Fprint(control, "220 test ftp\r\n")
		scanner := bufio.NewScanner(control)
		var passive net.Listener
		for scanner.Scan() {
			command := scanner.Text()
			fixture.mu.Lock()
			fixture.commands = append(fixture.commands, command)
			fixture.mu.Unlock()
			switch {
			case strings.HasPrefix(command, "USER "):
				fmt.Fprint(control, "230 logged in\r\n")
			case command == "FEAT":
				fmt.Fprint(control, "500 unsupported\r\n")
			case command == "TYPE I":
				fmt.Fprint(control, "200 binary\r\n")
			case command == "EPSV":
				if passive != nil {
					passive.Close()
				}
				passive, err = net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					return
				}
				defer passive.Close()
				fmt.Fprintf(control, "229 Entering Extended Passive Mode (|||%d|)\r\n", passive.Addr().(*net.TCPAddr).Port)
			case strings.HasPrefix(command, "SIZE "):
				fmt.Fprintf(control, "213 %d\r\n", fixture.size)
			case strings.HasPrefix(command, "RETR "), strings.HasPrefix(command, "LIST "), strings.HasPrefix(command, "NLST "):
				if passive == nil {
					return
				}
				data, err := passive.Accept()
				if err != nil {
					return
				}
				data.SetDeadline(time.Now().Add(5 * time.Second))
				fmt.Fprint(control, "150 Opening data\r\n")
				if strings.HasPrefix(command, "LIST ") {
					_, err = io.WriteString(data, fixture.listing)
				} else if strings.HasPrefix(command, "NLST ") {
					_, err = io.WriteString(data, fixture.names)
				} else {
					_, err = io.Copy(data, io.LimitReader(ftpZeroReader{}, fixture.bytesSent))
				}
				data.Close()
				if err != nil {
					return
				}
				reply := "226 Transfer complete"
				if fixture.finalReply != "" && strings.HasPrefix(command, "RETR ") {
					reply = fixture.finalReply
				}
				fmt.Fprintf(control, "%s\r\n", reply)
			case command == "PWD":
				fmt.Fprint(control, "257 \"/\"\r\n")
			case strings.HasPrefix(command, "RNFR ") && fixture.renameReply != "":
				fmt.Fprint(control, "350 Ready for target\r\n")
			case strings.HasPrefix(command, "RNTO ") && fixture.renameReply != "":
				fmt.Fprintf(control, "%s\r\n", fixture.renameReply)
			case command == "QUIT":
				fmt.Fprint(control, "221 Bye\r\n")
				return
			default:
				fmt.Fprint(control, "500 unsupported\r\n")
			}
		}
	}()
	client := &FTPClient{}
	t.Cleanup(func() {
		client.Close()
		listener.Close()
		select {
		case control := <-accepted:
			control.Close()
		default:
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("FTP fixture did not shut down")
		}
	})
	if err := client.Connect(model.Site{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, Username: "test"}); err != nil {
		t.Fatal(err)
	}
	return client
}

type ftpZeroReader struct{}

func (ftpZeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestFTPDownloadCompletesBeforeNextControlCommand(t *testing.T) {
	const size = 32 << 20
	fixture := &ftpFixture{size: size, bytesSent: size}
	client := connectFTPFixture(t, fixture)
	done := make(chan error, 1)
	go func() { done <- client.Download("/file", io.Discard, nil) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(4 * time.Second):
		client.Close()
		<-done
		t.Fatal("download blocked waiting for a control command before reading RETR")
	}
	if dir, err := client.CurrentDir(); err != nil || dir != "/" {
		t.Fatalf("control channel left out of sync: dir=%q err=%v", dir, err)
	}
	fixture.mu.Lock()
	commands := strings.Join(fixture.commands, "\n")
	fixture.mu.Unlock()
	if strings.Index(commands, "SIZE /file") > strings.Index(commands, "RETR /file") {
		t.Fatalf("SIZE followed RETR: %s", commands)
	}
}

func TestFTPDownloadReportsFinalServerFailure(t *testing.T) {
	client := connectFTPFixture(t, &ftpFixture{size: 5, bytesSent: 5, finalReply: "451 Transfer aborted"})
	if err := client.Download("/file", io.Discard, nil); err == nil {
		t.Fatal("final server failure was ignored")
	}
	if _, err := client.CurrentDir(); err != nil {
		t.Fatalf("final reply not consumed: %v", err)
	}
}

func TestFTPDownloadRejectsTruncatedFile(t *testing.T) {
	client := connectFTPFixture(t, &ftpFixture{size: 10, bytesSent: 5})
	if err := client.Download("/file", io.Discard, nil); err == nil {
		t.Fatal("size mismatch accepted")
	}
}

func TestFTPListRejectsTraversalFromActualParser(t *testing.T) {
	client := connectFTPFixture(t, &ftpFixture{listing: "-rw-r--r-- 1 owner group 23 Jan 01 2026 ../outside.txt\r\n"})
	if entries, err := client.List("/remote"); err == nil {
		t.Fatalf("unsafe FTP entries accepted: %+v", entries)
	}
}

func TestFTPStatLooksUpFileInParentListing(t *testing.T) {
	fixture := &ftpFixture{listing: "-rw-r--r-- 1 owner group 23 Jan 01 2026 a.txt\r\n"}
	client := connectFTPFixture(t, fixture)
	entry, err := client.Stat("/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if entry.IsDir || entry.Path != "/a.txt" || entry.Size != 23 {
		t.Fatalf("wrong stat: %+v", entry)
	}
	fixture.mu.Lock()
	commands := strings.Join(fixture.commands, "\n")
	fixture.mu.Unlock()
	if strings.Contains(commands, "LIST -a /a.txt") || !strings.Contains(commands, "LIST -a /") {
		t.Fatalf("stat listed target instead of parent: %s", commands)
	}
}

func TestFTPListCannotInterleaveWithDownload(t *testing.T) {
	client := connectFTPFixture(t, &ftpFixture{size: 5, bytesSent: 5})
	paused := make(chan struct{})
	resume := make(chan struct{})
	downloadDone := make(chan error, 1)
	go func() {
		downloadDone <- client.Download("/file", io.Discard, func(transferred, total, speed int64) bool {
			if transferred == 0 {
				close(paused)
				<-resume
			}
			return true
		})
	}()
	<-paused
	listDone := make(chan error, 1)
	go func() { _, err := client.List("/"); listDone <- err }()
	select {
	case err := <-listDone:
		close(resume)
		<-downloadDone
		t.Fatalf("LIST interleaved with an active data response: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(resume)
	if err := <-downloadDone; err != nil {
		t.Fatal(err)
	}
	if err := <-listDone; err != nil {
		t.Fatalf("queued LIST failed: %v", err)
	}
}
