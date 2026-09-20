package app

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"IntegTERM/internal/model"
	"IntegTERM/internal/transport"
)

// startMCPFTPFixture exercises the real FTP login and control connection. It
// owns every accepted socket; cleanup also works when the tested App leaves a
// file tab connected. Server goroutines never call testing.T after cleanup.
func startMCPFTPFixture(t *testing.T) (host string, port int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	connections := make(map[net.Conn]struct{})
	var handlers sync.WaitGroup
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			connections[conn] = struct{}{}
			mu.Unlock()
			handlers.Add(1)
			go func() {
				defer handlers.Done()
				defer func() {
					_ = conn.Close()
					mu.Lock()
					delete(connections, conn)
					mu.Unlock()
				}()
				serveMCPFTPControl(conn)
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		// Stop accepting before collecting sockets or waiting for handlers, so
		// neither a late accepted socket nor WaitGroup.Add can race cleanup.
		<-acceptDone
		mu.Lock()
		for conn := range connections {
			_ = conn.Close()
		}
		mu.Unlock()
		handlers.Wait()
	})
	return "127.0.0.1", listener.Addr().(*net.TCPAddr).Port
}

func serveMCPFTPControl(conn net.Conn) {
	_ = conn.SetDeadline(time.Now().Add(time.Minute))
	if _, err := io.WriteString(conn, "220 IntegTERM loopback FTP fixture\r\n"); err != nil {
		return
	}
	cwd := "/"
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		command, argument, _ := strings.Cut(scanner.Text(), " ")
		reply := "502 Unsupported fixture command\r\n"
		switch strings.ToUpper(command) {
		case "USER":
			reply = "331 Password required\r\n"
		case "PASS":
			reply = "230 Logged in\r\n"
		case "FEAT":
			reply = "211-Features\r\n UTF8\r\n211 End\r\n"
		case "TYPE", "OPTS", "NOOP":
			reply = "200 OK\r\n"
		case "CWD":
			if path.IsAbs(argument) {
				cwd = path.Clean(argument)
			} else {
				cwd = path.Join(cwd, argument)
			}
			reply = "250 Directory changed\r\n"
		case "PWD":
			reply = fmt.Sprintf("257 \"%s\" is the current directory\r\n", strings.ReplaceAll(cwd, "\"", "\"\""))
		case "QUIT":
			_, _ = io.WriteString(conn, "221 Goodbye\r\n")
			return
		}
		if _, err := io.WriteString(conn, reply); err != nil {
			return
		}
	}
}

func TestMCPFTPFixtureUsesRealLoginAndClosesIdleConnections(t *testing.T) {
	var idle net.Conn
	t.Run("server", func(t *testing.T) {
		host, port := startMCPFTPFixture(t)
		for range 3 {
			client := &transport.FTPClient{}
			if err := client.Connect(model.Site{Host: host, Port: port, Username: "fixture", Password: "fixture"}); err != nil {
				t.Fatal(err)
			}
			directory, err := client.CurrentDir()
			if err != nil || directory != "/" {
				_ = client.Close()
				t.Fatalf("PWD = %q, %v", directory, err)
			}
			if err := client.Close(); err != nil {
				t.Fatal(err)
			}
		}
		var err error
		idle, err = net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprint(port)), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		_ = idle.SetDeadline(time.Now().Add(5 * time.Second))
		if _, err := bufio.NewReader(idle).ReadString('\n'); err != nil {
			_ = idle.Close()
			t.Fatal(err)
		}
		// Leave this socket idle to exercise fixture-owned cleanup.
	})
	if idle == nil {
		return
	}
	defer idle.Close()
	if _, err := io.Copy(io.Discard, idle); err != nil {
		t.Fatalf("fixture did not close its idle connection: %v", err)
	}
}
