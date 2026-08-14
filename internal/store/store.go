package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

"github.com/VaderChen/Integrate-Terminal/internal/model"
)

type Store struct {
	baseDir string
}

func New(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

func (s *Store) BaseDir() string {
	return s.baseDir
}

func (s *Store) Ensure() error {
	if err := os.MkdirAll(s.baseDir, 0o700); err != nil {
		return err
	}
	return os.Chmod(s.baseDir, 0o700)
}

func (s *Store) LoadSites() ([]model.Site, error) {
	records, err := readJSON[[]model.Site](filepath.Join(s.baseDir, "sites.json"))
	if errors.Is(err, os.ErrNotExist) {
		return sampleSites(), nil
	}
	return records, err
}

func (s *Store) SaveSites(records []model.Site) error {
	return writeJSON(filepath.Join(s.baseDir, "sites.json"), records)
}

func (s *Store) LoadTabs() ([]model.Tab, error) {
	records, err := readJSON[[]model.Tab](filepath.Join(s.baseDir, "tabs.json"))
	if errors.Is(err, os.ErrNotExist) {
		return sampleTabs(), nil
	}
	return records, err
}

func (s *Store) SaveTabs(records []model.Tab) error {
	return writeJSON(filepath.Join(s.baseDir, "tabs.json"), records)
}

func (s *Store) LoadConfig() (model.Config, error) {
	record, err := readJSON[model.Config](filepath.Join(s.baseDir, "config.json"))
	if errors.Is(err, os.ErrNotExist) {
		return model.Config{
			WindowWidth:                  1440,
			WindowHeight:                 920,
			WindowX:                      0,
			WindowY:                      0,
			RestoreTabsOnStart:           true,
			CloseTerminalTabOnDisconnect: true,
			ShowHiddenFiles:              false,
			ShowTrayIcon:                 false,
			RememberWindowPosition:       false,
			TelnetLocalEcho:              true,
			RESTServerEnabled:            false,
			RESTServerPort:               18080,
			RESTServerAllowlist:          []string{"127.0.0.1"},
			FontScale:                    "medium",
			Language:                     "",
			Theme:                        "neutral",
			SiteFolders:                  []string{},
		}, nil
	}
	return record, err
}

func (s *Store) SaveConfig(record model.Config) error {
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
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
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func sampleSites() []model.Site {
	return []model.Site{}
}

func sampleTabs() []model.Tab {
	return []model.Tab{}
}
