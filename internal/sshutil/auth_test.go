package sshutil

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestKnownHostsCallbackRequestsApprovalForChangedKey(t *testing.T) {
	previousDataDir := dataDir
	Init(t.TempDir())
	t.Cleanup(func() { Init(previousDataDir) })

	host := "host-key-change.invalid"
	oldKey := testHostPublicKey(t)
	newKey := testHostPublicKey(t)
	if err := ApproveHost(host, strings.TrimSpace(string(ssh.MarshalAuthorizedKey(oldKey)))); err != nil {
		t.Fatalf("approve initial host key: %v", err)
	}

	callback, err := KnownHostsCallback()
	if err != nil {
		t.Fatalf("create known_hosts callback: %v", err)
	}
	err = callback(host+":22", &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 22}, newKey)
	var trustErr *HostTrustRequiredError
	if !errors.As(err, &trustErr) {
		t.Fatalf("expected host trust prompt, got %v", err)
	}
	if !trustErr.ReplacesExisting {
		t.Fatal("changed host key was not marked as replacing an existing key")
	}
	if trustErr.FingerprintSHA256 != ssh.FingerprintSHA256(newKey) {
		t.Fatalf("unexpected replacement fingerprint: %s", trustErr.FingerprintSHA256)
	}

	if err := ApproveHost(trustErr.HostPattern, trustErr.AuthorizedKey); err != nil {
		t.Fatalf("approve replacement host key: %v", err)
	}
	callback, err = KnownHostsCallback()
	if err != nil {
		t.Fatalf("reload known_hosts callback: %v", err)
	}
	if err := callback(host+":22", &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 22}, newKey); err != nil {
		t.Fatalf("replacement host key was not trusted: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dataDir, "known_hosts"))
	if err != nil {
		t.Fatalf("read replaced known_hosts: %v", err)
	}
	if strings.Contains(string(content), strings.TrimSpace(string(ssh.MarshalAuthorizedKey(oldKey)))) {
		t.Fatal("stale host key remained after approval")
	}
	if count := strings.Count(string(content), host+" "+newKey.Type()+" "); count != 1 {
		t.Fatalf("expected one replacement host entry, got %d", count)
	}
}

func TestKnownHostsCallbackMarksFirstConnection(t *testing.T) {
	previousDataDir := dataDir
	Init(t.TempDir())
	t.Cleanup(func() { Init(previousDataDir) })

	host := "first-connection.invalid"
	key := testHostPublicKey(t)
	callback, err := KnownHostsCallback()
	if err != nil {
		t.Fatalf("create known_hosts callback: %v", err)
	}
	err = callback(host+":22", &net.TCPAddr{IP: net.ParseIP("192.0.2.11"), Port: 22}, key)
	var trustErr *HostTrustRequiredError
	if !errors.As(err, &trustErr) {
		t.Fatalf("expected host trust prompt, got %v", err)
	}
	if trustErr.ReplacesExisting {
		t.Fatal("first connection was incorrectly marked as replacing an existing key")
	}
}

func testHostPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	key, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatalf("create SSH host key: %v", err)
	}
	return key
}
