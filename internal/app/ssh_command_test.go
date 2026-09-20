package app

import (
	"IntegTERM/internal/model"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"golang.org/x/crypto/ssh"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

func startCommandTestServer(t *testing.T, run func(ssh.Channel, <-chan struct{})) (model.Site, ssh.HostKeyCallback) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { return nil, nil }}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		raw, err := listener.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		conn, channels, requests, err := ssh.NewServerConn(raw, config)
		if err != nil {
			return
		}
		defer conn.Close()
		go ssh.DiscardRequests(requests)
		closed := make(chan struct{})
		go func() { _ = conn.Wait(); close(closed) }()
		for next := range channels {
			if next.ChannelType() != "session" {
				next.Reject(ssh.UnknownChannelType, "session only")
				continue
			}
			channel, requests, err := next.Accept()
			if err != nil {
				return
			}
			for request := range requests {
				if request.Type == "exec" {
					_ = request.Reply(true, nil)
					run(channel, closed)
					_ = channel.Close()
					return
				}
				_ = request.Reply(false, nil)
			}
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); wg.Wait() })
	host, portString, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portString)
	return model.Site{Host: host, Port: port, Username: "test", Password: "fake-only"}, ssh.FixedHostKey(signer.PublicKey())
}

func TestSSHCommandDeadlineCoversRemoteExecution(t *testing.T) {
	site, callback := startCommandTestServer(t, func(_ ssh.Channel, closed <-chan struct{}) { <-closed })
	started := time.Now()
	_, err := runSSHCommand(context.Background(), site, "never exits", 100*time.Millisecond, callback)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("SSH command ignored total deadline")
	}
}

func TestSSHCommandRequestCancellationClosesRemoteSession(t *testing.T) {
	entered := make(chan struct{})
	site, callback := startCommandTestServer(t, func(_ ssh.Channel, closed <-chan struct{}) { close(entered); <-closed })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := runSSHCommand(ctx, site, "wait", time.Minute, callback); result <- err }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("server did not receive command")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SSH session ignored cancellation")
	}
}

func TestSSHCommandOutputLimitStopsRemoteSession(t *testing.T) {
	site, callback := startCommandTestServer(t, func(channel ssh.Channel, closed <-chan struct{}) {
		_, _ = channel.Write(bytes.Repeat([]byte("x"), maxSSHCommandOutput+65536))
		<-closed
	})
	_, err := runSSHCommand(context.Background(), site, "verbose", time.Second, callback)
	if !errors.Is(err, errSSHOutputLimit) {
		t.Fatalf("expected bounded output error, got %v", err)
	}
}

func TestSSHCommandPreservesOutputAndExitStatus(t *testing.T) {
	site, callback := startCommandTestServer(t, func(channel ssh.Channel, _ <-chan struct{}) {
		_, _ = channel.Write([]byte("stdout"))
		_, _ = channel.Stderr().Write([]byte("stderr"))
		_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{7}))
	})
	result, err := runSSHCommand(context.Background(), site, "command", time.Second, callback)
	if err != nil {
		t.Fatal(err)
	}
	if result["stdout"] != "stdout" || result["stderr"] != "stderr" || result["exitCode"] != 7 || result["ok"] != false {
		t.Fatalf("unexpected command result: %#v", result)
	}
}
