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

	"github.com/VaderChen/Integrate-Terminal/internal/keystore"
)

// SignerFromPPK 讀取 PPK 私鑰。
//
// 內容一律經由 keystore 取得，而非直接開檔：App Sandbox 下，先前存起來的絕對路徑
// 無法直接讀取，必須透過 security-scoped bookmark 或容器內副本。
// 非沙箱環境（開發模式）keystore 會退回直接開檔，行為與過去相同。
func SignerFromPPK(filePath string, passphrase string) (ssh.Signer, error) {
	content, err := keystore.Read(filePath)
	if err != nil {
		return nil, err
	}

	puttyKey, err := putty.New(content)
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
	ReplacesExisting  bool   `json:"replacesExisting,omitempty"`
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
				if errors.As(err, &keyErr) {
					host, port, hostPattern := resolveHostTrustTarget(hostname, remote.String())
					return &HostTrustRequiredError{
						Host:              host,
						Port:              port,
						HostPattern:       hostPattern,
						KeyType:           key.Type(),
						FingerprintSHA256: ssh.FingerprintSHA256(key),
						AuthorizedKey:     strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))),
						ReplacesExisting:  len(keyErr.Want) > 0,
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

// dataDir 是 App 的資料目錄。App Sandbox 下 ~/.ssh 完全無法讀寫，
// 因此主機指紋改存在這裡；非沙箱環境仍會一併讀取 ~/.ssh 既有紀錄。
var dataDir string

// Init 設定資料目錄，App 與背景服務啟動時各呼叫一次。
func Init(dir string) { dataDir = dir }

// managedKnownHostsPath 回傳 App 自有的 known_hosts 路徑（沙箱下唯一可寫的位置）。
func managedKnownHostsPath() string {
	if strings.TrimSpace(dataDir) == "" {
		return ""
	}
	return filepath.Join(dataDir, "known_hosts")
}

func knownHostsFiles() ([]string, error) {
	candidates := make([]string, 0, 3)

	if managed := managedKnownHostsPath(); managed != "" {
		candidates = append(candidates, managed)
	}

	// 沙箱下這兩個會讀不到，靜默略過即可；開發模式仍可沿用既有信任紀錄。
	if homeDir, err := os.UserHomeDir(); err == nil && strings.TrimSpace(homeDir) != "" {
		candidates = append(candidates,
			filepath.Join(homeDir, ".ssh", "known_hosts"),
			filepath.Join(homeDir, ".ssh", "known_hosts2"),
		)
	}

	paths := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		info, statErr := os.Stat(candidate)
		if statErr != nil || info.IsDir() {
			continue
		}
		// 讀不到就別交給 knownhosts.New，否則整個 callback 會直接失敗。
		file, openErr := os.Open(candidate)
		if openErr != nil {
			continue
		}
		_ = file.Close()
		paths = append(paths, candidate)
	}

	return paths, nil
}

func ApproveHost(hostPattern string, authorizedKey string) error {
	knownHostsPath := managedKnownHostsPath()
	if knownHostsPath == "" {
		return fmt.Errorf("資料目錄尚未初始化，無法寫入主機指紋")
	}

	if err := os.MkdirAll(filepath.Dir(knownHostsPath), 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

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
	updated, changed := replaceKnownHostEntry(string(existing), hostPattern, publicKey, line)
	if !changed {
		return nil
	}
	if err := os.WriteFile(knownHostsPath, []byte(updated), 0o600); err != nil {
		return fmt.Errorf("write known_hosts: %w", err)
	}
	return nil
}

func replaceKnownHostEntry(existing, hostPattern string, publicKey ssh.PublicKey, replacement string) (string, bool) {
	keyType := publicKey.Type()
	lines := strings.Split(existing, "\n")
	if strings.HasSuffix(existing, "\n") {
		lines = lines[:len(lines)-1]
	}
	updated := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			updated = append(updated, line)
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != keyType {
			updated = append(updated, line)
			continue
		}

		patterns := strings.Split(fields[0], ",")
		remaining := make([]string, 0, len(patterns))
		matched := false
		for _, pattern := range patterns {
			if pattern == hostPattern {
				matched = true
				continue
			}
			remaining = append(remaining, pattern)
		}
		if !matched {
			updated = append(updated, line)
			continue
		}

		if len(remaining) > 0 {
			updated = append(updated, strings.Join(remaining, ",")+" "+strings.Join(fields[1:], " "))
		}
	}

	updated = appendKnownHostLine(updated, replacement)
	result := strings.Join(updated, "\n")
	result += "\n"
	return result, result != existing
}

func appendKnownHostLine(lines []string, replacement string) []string {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return append(lines, replacement)
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
