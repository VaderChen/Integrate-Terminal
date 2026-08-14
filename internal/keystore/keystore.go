// Package keystore 相容舊沙盒版本保存的 PPK 私鑰與 security-scoped bookmark。
//
// 沙箱中透過檔案對話框取得的存取權只在該次執行期間有效，重啟後直接用存下的路徑
// 開檔會得到 operation not permitted。這裡採雙軌保險：
//
//  1. security-scoped bookmark —— 主要手段，能讀到使用者原始位置的檔案，
//     使用者之後更換金鑰內容時 App 會自動看到新版。
//  2. 容器內副本 —— 後備手段。bookmark 解析失敗時（檔案被搬走、外接磁碟未掛載、
//     或 bookmark 因系統變更失效）仍能連線。
//
// 兩者都在使用者選檔當下（App 仍持有存取權時）建立。
package keystore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

"github.com/VaderChen/Integrate-Terminal/internal/macsecurity"
)

const (
	registryFileName = "ppk-keys.json"
	copiesDirName    = "keys"
)

type entry struct {
	Path     string `json:"path"`
	Bookmark string `json:"bookmark,omitempty"`
	CopyName string `json:"copyName,omitempty"`
}

type registry struct {
	Entries map[string]entry `json:"entries"`
	// Directories 是資料夾範圍的 bookmark（資料夾路徑 -> bookmark）。
	// 使用者選一次資料夾，其下所有金鑰檔就都能讀取，
	// 免去既有站台逐一重新選檔的麻煩。
	Directories map[string]string `json:"directories,omitempty"`
}

var (
	mu      sync.Mutex
	baseDir string
)

// Init 設定註冊表所在目錄，必須在使用其他函式前呼叫（App 與背景服務啟動時各呼叫一次）。
func Init(dir string) {
	mu.Lock()
	defer mu.Unlock()
	baseDir = dir
}

func registryPath() string { return filepath.Join(baseDir, registryFileName) }
func copiesDir() string    { return filepath.Join(baseDir, copiesDirName) }

func copyNameFor(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:16]) + ".ppk"
}

func newRegistry() registry {
	return registry{
		Entries:     map[string]entry{},
		Directories: map[string]string{},
	}
}

func loadRegistry() registry {
	loaded := newRegistry()
	if baseDir == "" {
		return loaded
	}
	data, err := os.ReadFile(registryPath())
	if err != nil {
		return loaded
	}
	if err := json.Unmarshal(data, &loaded); err != nil {
		return newRegistry()
	}
	if loaded.Entries == nil {
		loaded.Entries = map[string]entry{}
	}
	if loaded.Directories == nil {
		loaded.Directories = map[string]string{}
	}
	return loaded
}

func saveRegistry(value registry) error {
	if baseDir == "" {
		return errors.New("keystore 尚未初始化")
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(registryPath(), data, 0o600)
}

// Remember 在使用者剛選取 path 時呼叫，建立 bookmark 與容器副本。
//
// 兩者只要有一個成功就算成功；都失敗才回傳錯誤。這樣即使在非沙箱的開發模式下
// （bookmark 可能無法建立）也不會擋住流程。
func Remember(path string) error {
	if path == "" {
		return errors.New("PPK 路徑為空")
	}

	mu.Lock()
	defer mu.Unlock()

	if baseDir == "" {
		return errors.New("keystore 尚未初始化")
	}

	current := loadRegistry()
	saved := entry{Path: path}
	var problems []error

	// 先複製內容：此刻一定還有存取權，之後未必。
	content, err := os.ReadFile(path)
	if err != nil {
		problems = append(problems, fmt.Errorf("讀取金鑰檔失敗: %w", err))
	} else {
		if err := os.MkdirAll(copiesDir(), 0o700); err != nil {
			problems = append(problems, fmt.Errorf("建立金鑰目錄失敗: %w", err))
		} else {
			name := copyNameFor(path)
			if err := os.WriteFile(filepath.Join(copiesDir(), name), content, 0o600); err != nil {
				problems = append(problems, fmt.Errorf("寫入金鑰副本失敗: %w", err))
			} else {
				saved.CopyName = name
			}
		}
	}

	if bookmark, err := macsecurity.CreateBookmark(path); err != nil {
		problems = append(problems, fmt.Errorf("建立 bookmark 失敗: %w", err))
	} else {
		saved.Bookmark = bookmark
	}

	if saved.Bookmark == "" && saved.CopyName == "" {
		return errors.Join(problems...)
	}

	current.Entries[path] = saved
	return saveRegistry(current)
}

// RememberDirectory 為資料夾建立範圍 bookmark，其下所有金鑰檔即可直接讀取。
//
// 既有站台的 PPK 路徑沒有個別 bookmark，讓使用者選一次金鑰所在資料夾，
// 比逐一重選每個檔案實際得多。
func RememberDirectory(dir string) error {
	if dir == "" {
		return errors.New("資料夾路徑為空")
	}

	mu.Lock()
	defer mu.Unlock()

	if baseDir == "" {
		return errors.New("keystore 尚未初始化")
	}

	bookmark, err := macsecurity.CreateBookmark(dir)
	if err != nil {
		return fmt.Errorf("建立資料夾 bookmark 失敗: %w", err)
	}

	current := loadRegistry()
	current.Directories[filepath.Clean(dir)] = bookmark
	return saveRegistry(current)
}

// readViaDirectories 嘗試用已登記的資料夾 bookmark 讀取 path。
func readViaDirectories(directories map[string]string, path string) ([]byte, bool) {
	cleaned := filepath.Clean(path)
	for dir, bookmark := range directories {
		relative, err := filepath.Rel(dir, cleaned)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		access, err := macsecurity.ResolveBookmark(bookmark)
		if err != nil {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(access.Path, relative))
		access.Release()
		if readErr == nil {
			return content, true
		}
	}
	return nil, false
}

// Read 取得 path 對應的 PPK 內容。
//
// 依序嘗試檔案 bookmark、容器副本、資料夾 bookmark、直接開檔，任一成功即回傳。
func Read(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("PPK 路徑為空")
	}

	mu.Lock()
	current := loadRegistry()
	saved, hasEntry := current.Entries[path]
	directories := current.Directories
	dir := baseDir
	mu.Unlock()

	var problems []error

	if hasEntry && saved.Bookmark != "" {
		content, refreshed, err := readViaBookmark(saved.Bookmark)
		if err == nil {
			if refreshed != "" {
				mu.Lock()
				latest := loadRegistry()
				if existing, ok := latest.Entries[path]; ok {
					existing.Bookmark = refreshed
					latest.Entries[path] = existing
					_ = saveRegistry(latest)
				}
				mu.Unlock()
			}
			return content, nil
		}
		problems = append(problems, err)
	}

	if hasEntry && saved.CopyName != "" && dir != "" {
		content, err := os.ReadFile(filepath.Join(dir, copiesDirName, saved.CopyName))
		if err == nil {
			return content, nil
		}
		problems = append(problems, fmt.Errorf("讀取金鑰副本失敗: %w", err))
	}

	// 既有站台沒有個別 bookmark，但使用者可能已授權整個金鑰資料夾。
	if content, ok := readViaDirectories(directories, path); ok {
		return content, nil
	}

	// 非沙箱環境（開發模式、背景服務尚未沙箱化）直接開檔即可。
	content, err := os.ReadFile(path)
	if err == nil {
		return content, nil
	}
	problems = append(problems, err)

	if hasEntry {
		return nil, fmt.Errorf("無法讀取金鑰檔 %s（bookmark 與副本皆失效）: %w", path, errors.Join(problems...))
	}
	return nil, fmt.Errorf(
		"無法讀取金鑰檔 %s。請在站台設定中重新選取此金鑰，或授權其所在資料夾一次即可涵蓋所有金鑰: %w",
		path, errors.Join(problems...))
}

// readViaBookmark 解析 bookmark 並讀取內容。
// 若 bookmark 已過期，回傳的第二個值是重建後的新 bookmark。
func readViaBookmark(bookmark string) ([]byte, string, error) {
	access, err := macsecurity.ResolveBookmark(bookmark)
	if err != nil {
		return nil, "", err
	}
	defer access.Release()

	content, err := os.ReadFile(access.Path)
	if err != nil {
		return nil, "", fmt.Errorf("透過 bookmark 讀取失敗: %w", err)
	}

	refreshed := ""
	if access.Stale {
		// 仍持有存取權時重建，下次啟動才不會失敗。
		if rebuilt, err := macsecurity.CreateBookmark(access.Path); err == nil {
			refreshed = rebuilt
		}
	}
	return content, refreshed, nil
}

// Forget 移除 path 的 bookmark 與容器副本。
func Forget(path string) {
	if path == "" {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if baseDir == "" {
		return
	}

	current := loadRegistry()
	saved, ok := current.Entries[path]
	if !ok {
		return
	}
	if saved.CopyName != "" {
		_ = os.Remove(filepath.Join(copiesDir(), saved.CopyName))
	}
	delete(current.Entries, path)
	_ = saveRegistry(current)
}

// NeedsReselect 回報 path 目前是否讀不到，需要使用者重新授權。
//
// 實際嘗試讀取而非只看註冊表，因為 bookmark 可能已失效、
// 或使用者已用資料夾授權涵蓋了這個檔案。
func NeedsReselect(path string) bool {
	if path == "" {
		return false
	}
	_, err := Read(path)
	return err != nil
}
