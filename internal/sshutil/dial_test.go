package sshutil

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestDialTimeoutIncludesHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := listener.Accept()
		if err == nil {
			defer c.Close()
			_, _ = io.Copy(io.Discard, c)
		}
	}()
	started := time.Now()
	client, err := Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{User: "test", HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 50 * time.Millisecond})
	if client != nil {
		_ = client.Close()
	}
	if err == nil {
		t.Fatal("stalled SSH handshake succeeded")
	}
	if time.Since(started) > time.Second {
		t.Fatal("handshake ignored timeout")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("failed handshake leaked connection")
	}
}

func TestDialClearsHandshakeDeadline(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(key)
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
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
		go func() {
			for ch := range channels {
				_ = ch.Reject(ssh.UnknownChannelType, "unused")
			}
		}()
		for req := range requests {
			_ = req.Reply(true, nil)
		}
	}()
	client, err := Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{User: "test", HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()), Timeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	time.Sleep(250 * time.Millisecond)
	ok, _, err := client.SendRequest("smoke", true, nil)
	if err != nil || !ok {
		t.Fatalf("established session retained handshake deadline: %v", err)
	}
	_ = client.Close()
	<-done
}
