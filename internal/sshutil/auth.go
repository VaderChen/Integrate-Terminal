package sshutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/kayrus/putty"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func SignerFromPPK(filePath string, passphrase string) (ssh.Signer, error) {
	puttyKey, err := putty.NewFromFile(filePath)
	if err != nil {
		return nil, err
	}

	var rawKey interface{}
	if puttyKey.Encryption != "none" {
		rawKey, err = puttyKey.ParseRawPrivateKey([]byte(passphrase))
	} else {
		rawKey, err = puttyKey.ParseRawPrivateKey(nil)
	}
	if err != nil {
		return nil, err
	}

	return ssh.NewSignerFromKey(rawKey)
}

type HostTrustRequiredError struct {
	Host              string `json:"host"`
	Port              int    `json:"port"`
	HostPattern       string `json:"hostPattern"`
	KeyType           string `json:"keyType"`
	FingerprintSHA256 string `json:"fingerprintSHA256"`
	AuthorizedKey     string `json:"authorizedKey"`
}

func (e *HostTrustRequiredError) Error() string {
	payload, err := json.Marshal(e)
	if err != nil {
		return "HOST_TRUST_REQUIRED"
	}
	return "HOST_TRUST_REQUIRED:" + string(payload)
}

func KnownHostsCallback() (ssh.HostKeyCallback, error) {
	paths, err := knownHostsFiles()
	if err != nil {
		return nil, err
	}

	var callback ssh.HostKeyCallback
	if len(paths) > 0 {
		callback, err = knownhosts.New(paths...)
		if err != nil {
			return nil, fmt.Errorf("load known_hosts: %w", err)
		}
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if callback != nil {
			if err := callback(hostname, remote, key); err == nil {
				return nil
			} else {
				var keyErr *knownhosts.KeyError
				if errors.As(err, &keyErr) && len(keyErr.Want) == 0 {
					host, port, hostPattern := resolveHostTrustTarget(hostname, remote.String())
					return &HostTrustRequiredError{
						Host:              host,
						Port:              port,
						HostPattern:       hostPattern,
						KeyType:           key.Type(),
						FingerprintSHA256: ssh.FingerprintSHA256(key),
						AuthorizedKey:     strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))),
					}
				}
				return err
			}
		}

		host, port, hostPattern := resolveHostTrustTarget(hostname, remote.String())
		return &HostTrustRequiredError{
			Host:              host,
			Port:              port,
			HostPattern:       hostPattern,
			KeyType:           key.Type(),
			FingerprintSHA256: ssh.FingerprintSHA256(key),
			AuthorizedKey:     strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))),
		}
	}, nil
}

func knownHostsFiles() ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return nil, fmt.Errorf("cannot resolve user home for known_hosts")
	}

	candidates := []string{
		filepath.Join(homeDir, ".ssh", "known_hosts"),
		filepath.Join(homeDir, ".ssh", "known_hosts2"),
	}

	paths := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		info, statErr := os.Stat(candidate)
		if statErr == nil && !info.IsDir() {
			paths = append(paths, candidate)
		}
	}

	return paths, nil
}

func ApproveHost(hostPattern string, authorizedKey string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return fmt.Errorf("cannot resolve user home for known_hosts")
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("create ssh dir: %w", err)
	}

	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	if _, err := os.Stat(knownHostsPath); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(knownHostsPath, []byte{}, 0o600); err != nil {
			return fmt.Errorf("create known_hosts: %w", err)
		}
	}
	_ = os.Chmod(knownHostsPath, 0o600)

	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(authorizedKey))
	if err != nil {
		return fmt.Errorf("parse host public key: %w", err)
	}

	line := knownhosts.Line([]string{hostPattern}, publicKey)
	existing, err := os.ReadFile(knownHostsPath)
	if err != nil {
		return fmt.Errorf("read known_hosts: %w", err)
	}
	if strings.Contains(string(existing), line) {
		return nil
	}

	file, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open known_hosts: %w", err)
	}
	defer file.Close()

	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("append newline to known_hosts: %w", err)
		}
	}
	if _, err := file.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("append host to known_hosts: %w", err)
	}
	return nil
}

func splitHostPort(address string) (string, int) {
	host, portValue, err := net.SplitHostPort(address)
	if err != nil {
		return address, 22
	}
	port, err := net.LookupPort("tcp", portValue)
	if err != nil {
		return host, 22
	}
	return host, port
}

func knownHostPattern(host string, port int) string {
	if port == 22 {
		return host
	}
	return fmt.Sprintf("[%s]:%d", host, port)
}

func resolveHostTrustTarget(hostname string, remoteAddress string) (string, int, string) {
	host, port := splitKnownHostsHostname(hostname)
	if host == "" {
		host, port = splitHostPort(remoteAddress)
	}
	return host, port, knownHostPattern(host, port)
}

func splitKnownHostsHostname(hostname string) (string, int) {
	if strings.HasPrefix(hostname, "[") {
		host, port := splitHostPort(hostname)
		return host, port
	}

	if strings.Count(hostname, ":") == 1 {
		host, port := splitHostPort(hostname)
		return host, port
	}

	return hostname, 22
}
