package keystore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeKey(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("建立目錄失敗: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("寫入金鑰檔失敗: %v", err)
	}
}

func TestRememberThenReadUsesRegistry(t *testing.T) {
	Init(t.TempDir())

	keyPath := filepath.Join(t.TempDir(), "server.ppk")
	writeKey(t, keyPath, "PuTTY-User-Key-File-3: ssh-rsa")

	if err := Remember(keyPath); err != nil {
		t.Fatalf("Remember 失敗: %v", err)
	}

	content, err := Read(keyPath)
	if err != nil {
		t.Fatalf("Read 失敗: %v", err)
	}
	if string(content) != "PuTTY-User-Key-File-3: ssh-rsa" {
		t.Errorf("內容不符: %q", content)
	}
}

// 原始檔案消失後仍應能從容器副本讀出 —— 這是雙軌設計的重點。
func TestReadFallsBackToContainerCopy(t *testing.T) {
	Init(t.TempDir())

	keyDir := t.TempDir()
	keyPath := filepath.Join(keyDir, "server.ppk")
	writeKey(t, keyPath, "original-key-content")

	if err := Remember(keyPath); err != nil {
		t.Fatalf("Remember 失敗: %v", err)
	}
	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("移除原始檔失敗: %v", err)
	}

	content, err := Read(keyPath)
	if err != nil {
		t.Fatalf("原始檔消失後 Read 應改用容器副本，卻失敗: %v", err)
	}
	if string(content) != "original-key-content" {
		t.Errorf("副本內容不符: %q", content)
	}
}

func TestReadUnregisteredPathGivesActionableError(t *testing.T) {
	Init(t.TempDir())

	_, err := Read(filepath.Join(t.TempDir(), "never-registered.ppk"))
	if err == nil {
		t.Fatal("讀取未登記且不存在的檔案應該失敗")
	}
	if !strings.Contains(err.Error(), "重新選取") {
		t.Errorf("錯誤訊息應告訴使用者怎麼處理，實得: %v", err)
	}
}

func TestDirectoryAuthorizationCoversContainedKeys(t *testing.T) {
	Init(t.TempDir())

	keyDir := t.TempDir()
	keyPath := filepath.Join(keyDir, "nested", "server.ppk")
	writeKey(t, keyPath, "dir-scoped-content")

	if err := RememberDirectory(keyDir); err != nil {
		t.Fatalf("RememberDirectory 失敗: %v", err)
	}

	registry := loadRegistry()
	if len(registry.Directories) != 1 {
		t.Fatalf("預期 1 筆資料夾授權，實得 %d", len(registry.Directories))
	}

	content, ok := readViaDirectories(registry.Directories, keyPath)
	if !ok {
		t.Fatal("資料夾授權應涵蓋其下的金鑰檔")
	}
	if string(content) != "dir-scoped-content" {
		t.Errorf("內容不符: %q", content)
	}
}

// 資料夾授權不得外溢到該資料夾之外的路徑。
func TestDirectoryAuthorizationDoesNotEscape(t *testing.T) {
	Init(t.TempDir())

	authorized := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.ppk")
	writeKey(t, outside, "should-not-be-reachable")

	if err := RememberDirectory(authorized); err != nil {
		t.Fatalf("RememberDirectory 失敗: %v", err)
	}

	registry := loadRegistry()
	if _, ok := readViaDirectories(registry.Directories, outside); ok {
		t.Error("資料夾授權不應涵蓋資料夾外的檔案")
	}
}

func TestForgetRemovesCopy(t *testing.T) {
	Init(t.TempDir())

	keyPath := filepath.Join(t.TempDir(), "server.ppk")
	writeKey(t, keyPath, "content")

	if err := Remember(keyPath); err != nil {
		t.Fatalf("Remember 失敗: %v", err)
	}
	Forget(keyPath)

	if entries := loadRegistry().Entries; len(entries) != 0 {
		t.Errorf("Forget 後註冊表應為空，實得 %d 筆", len(entries))
	}
	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("移除原始檔失敗: %v", err)
	}
	if _, err := Read(keyPath); err == nil {
		t.Error("Forget 且原始檔消失後不應還能讀到")
	}
}
