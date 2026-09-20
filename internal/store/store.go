package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"IntegTERM/internal/credentials"
	"IntegTERM/internal/model"
)

type Store struct {
	baseDir      string
	credentials  credentials.Backend
	writeRecords func(string, any) error
	lockTimeout  time.Duration
}

func New(baseDir string) *Store {
	return NewWithCredentials(baseDir, credentials.New())
}

// NewWithCredentials allows tests to use an isolated in-memory backend.
// A nil backend fails closed whenever a record needs credentials.
func NewWithCredentials(baseDir string, backend credentials.Backend) *Store {
	return NewWithCredentialsAndLockTimeout(baseDir, backend, 0)
}

// NewWithCredentialsAndLockTimeout bounds only waiting to acquire a transaction's
// file lock. Once acquired, the callback runs to completion without a deadline.
// A non-positive timeout retains the blocking behavior of New/NewWithCredentials.
// The option is immutable so concurrent transactions can safely share the Store.
func NewWithCredentialsAndLockTimeout(baseDir string, backend credentials.Backend, timeout time.Duration) *Store {
	return &Store{baseDir: baseDir, credentials: backend, writeRecords: writeJSON, lockTimeout: timeout}
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
	return loadCredentialRecords[model.Site](s, filepath.Join(s.baseDir, "sites.json"))
}
func (s *Store) saveSites(records []model.Site) error {
	return saveCredentialRecords(s, filepath.Join(s.baseDir, "sites.json"), records)
}
func (s *Store) loadTabs() ([]model.Tab, error) {
	return loadCredentialRecords[model.Tab](s, filepath.Join(s.baseDir, "tabs.json"))
}
func (s *Store) saveTabs(records []model.Tab) error {
	return saveCredentialRecords(s, filepath.Join(s.baseDir, "tabs.json"), records)
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
