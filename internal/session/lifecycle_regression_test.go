package session

import (
	"github.com/creack/pty"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"IntegTERM/internal/model"
)

func TestCloseTelnetInterruptsBlockedInput(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	m := NewManager()
	s := &telnetTerminalSession{id: "blocked", conn: client}
	m.telnetSessions[s.id] = s
	done := make(chan error, 1)
	go func() { done <- m.WriteTelnetInput(s.id, strings.Repeat("x", 1024)) }()
	// 確認寫入持有狀態鎖，再驗證關閉不會等待它。
	deadline := time.Now().Add(time.Second)
	for s.lock.TryLock() {
		s.lock.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("write did not start")
		}
		time.Sleep(time.Millisecond)
	}
	closed := make(chan struct{})
	go func() { _ = m.CloseTelnetSession(s.id); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		_ = server.Close()
		t.Fatal("close blocked behind input")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("blocked write unexpectedly succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("write not interrupted")
	}
}

func TestLocalExitClosesPTY(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()
	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	m := NewManager()
	s := &localTerminalSession{id: "exited", cmd: cmd, ptyFile: write}
	m.localSessions[s.id] = s
	m.watchLocalExit(nil, s)
	if _, err := write.Write([]byte("unexpected")); err == nil {
		t.Fatal("terminal file descriptor remains open")
	}
	if _, ok := m.localSessions[s.id]; ok {
		t.Fatal("exited session retained")
	}
}

func TestTelnetIPv6Connection(t *testing.T) {
	listener, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 unavailable: %v", err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	m := NewManager()
	id, err := m.StartTelnetSession(nil, model.Site{Host: "::1", Port: listener.Addr().(*net.TCPAddr).Port})
	if err != nil {
		t.Fatal(err)
	}
	defer m.CloseTelnetSession(id)
	select {
	case conn := <-accepted:
		_ = conn.Close()
	case <-time.After(time.Second):
		t.Fatal("no IPv6 connection")
	}
}

func TestLocalExitDrainsFinalOutput(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "i=0; while [ $i -lt 2000 ]; do printf 'last-output\\n'; i=$((i+1)); done")
	file, err := pty.Start(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	m := NewManager()
	s := &localTerminalSession{id: "drain", cmd: cmd, ptyFile: file, outputDone: make(chan struct{})}
	m.localSessions[s.id] = s
	go m.streamLocalOutput(nil, s)
	m.watchLocalExit(nil, s)
	s.lock.Lock()
	defer s.lock.Unlock()
	if got := strings.Count(string(s.outputBuffer), "last-output"); got != 2000 {
		t.Fatalf("lost final output: %d/2000", got)
	}
}
