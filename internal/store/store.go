package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"IntegTERM/internal/model"
)

type Store struct {
	baseDir      string
	writeRecords func(string, any) error
	lockTimeout  time.Duration
}

func New(baseDir string) *Store {
	return NewWithLockTimeout(baseDir, 0)
}

// NewWithLockTimeout 只限制取得檔案鎖的等待時間，取得後交易會執行至結束。
// 非正值與 New 一樣持續等待；設定建立後不再改變，可供並行交易使用。
func NewWithLockTimeout(baseDir string, timeout time.Duration) *Store {
	return &Store{baseDir: baseDir, writeRecords: writeJSON, lockTimeout: timeout}
}

func (s *Store) BaseDir() string {
	return s.baseDir
}

func (s *Store) Ensure() error {
	if err := os.MkdirAll(s.baseDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(s.baseDir, 0o700); err != nil {
		return err
	}
	for _, name := range []string{"sites.json", "tabs.json", "config.json", "rest-api.token", ".store.lock"} {
		if err := os.Chmod(filepath.Join(s.baseDir, name), 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *Store) loadSites() ([]model.Site, error) {
	return loadJSONRecords[model.Site](filepath.Join(s.baseDir, "sites.json"))
}
func (s *Store) saveSites(records []model.Site) error {
	return saveJSONRecords(s, filepath.Join(s.baseDir, "sites.json"), records)
}
func (s *Store) loadTabs() ([]model.Tab, error) {
	return loadJSONRecords[model.Tab](filepath.Join(s.baseDir, "tabs.json"))
}
func (s *Store) saveTabs(records []model.Tab) error {
	return saveJSONRecords(s, filepath.Join(s.baseDir, "tabs.json"), records)
}

func loadJSONRecords[T any](path string) ([]T, error) {
	records, err := readJSON[[]T](path)
	if errors.Is(err, os.ErrNotExist) {
		return []T{}, nil
	}
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = []T{}
	}
	return records, nil
}

func saveJSONRecords[T any](s *Store, path string, records []T) error {
	// 既有檔案無法完整讀取時，保留原檔，不以空資料或新資料覆蓋。
	if _, err := loadJSONRecords[T](path); err != nil {
		return err
	}
	return s.writeRecords(path, records)
}

func (s *Store) loadConfig() (model.Config, error) {
	record, err := readJSON[model.Config](filepath.Join(s.baseDir, "config.json"))
	if errors.Is(err, os.ErrNotExist) {
		return model.Config{
			WindowWidth:                  1440,
			WindowHeight:                 920,
			WindowX:                      0,
			WindowY:                      0,
			ProUnlock:                    false,
			RestoreTabsOnStart:           true,
			CloseTerminalTabOnDisconnect: true,
			ShowHiddenFiles:              false,
			ShowTrayIcon:                 false,
			RememberWindowPosition:       false,
			TelnetLocalEcho:              true,
			RESTServerEnabled:            false,
			RESTServerPort:               18080,
			FontScale:                    "medium",
			Language:                     "",
			Theme:                        "neutral",
			SiteFolders:                  []string{},
		}, nil
	}
	return record, err
}

func (s *Store) saveConfig(record model.Config) error {
	record.ProUnlock = false // An editable cache is never an entitlement authority.
	return writeJSON(filepath.Join(s.baseDir, "config.json"), record)
}

func readJSON[T any](path string) (T, error) {
	var out T

	data, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(data, &out)
	return out, err
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".integterm-*.tmp")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)

	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func sampleSites() []model.Site {
	return []model.Site{}
}

func sampleTabs() []model.Tab {
	return []model.Tab{}
}
