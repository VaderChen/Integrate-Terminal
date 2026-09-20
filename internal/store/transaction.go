package store

import (
	"IntegTERM/internal/model"
	"os"
	"path/filepath"
)

// Transaction serializes read-modify-write operations across GUI and service
// processes. Call its methods rather than Store methods inside the callback.
type Transaction struct{ store *Store }

func (s *Store) WithTransaction(work func(*Transaction) error) error {
	if err := s.Ensure(); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(s.baseDir, ".store.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := lockFileWithTimeout(file, s.lockTimeout); err != nil {
		return err
	}
	defer unlockFile(file)
	return work(&Transaction{store: s})
}

func (t *Transaction) LoadSites() ([]model.Site, error)  { return t.store.loadSites() }
func (t *Transaction) SaveSites(v []model.Site) error    { return t.store.saveSites(v) }
func (t *Transaction) LoadTabs() ([]model.Tab, error)    { return t.store.loadTabs() }
func (t *Transaction) SaveTabs(v []model.Tab) error      { return t.store.saveTabs(v) }
func (t *Transaction) LoadConfig() (model.Config, error) { return t.store.loadConfig() }
func (t *Transaction) SaveConfig(v model.Config) error   { return t.store.saveConfig(v) }

func (s *Store) LoadSites() (v []model.Site, err error) {
	err = s.WithTransaction(func(t *Transaction) error { v, err = t.LoadSites(); return err })
	return
}
func (s *Store) SaveSites(v []model.Site) error {
	return s.WithTransaction(func(t *Transaction) error { return t.SaveSites(v) })
}
func (s *Store) LoadTabs() (v []model.Tab, err error) {
	err = s.WithTransaction(func(t *Transaction) error { v, err = t.LoadTabs(); return err })
	return
}
func (s *Store) SaveTabs(v []model.Tab) error {
	return s.WithTransaction(func(t *Transaction) error { return t.SaveTabs(v) })
}
func (s *Store) LoadConfig() (v model.Config, err error) {
	err = s.WithTransaction(func(t *Transaction) error { v, err = t.LoadConfig(); return err })
	return
}
func (s *Store) SaveConfig(v model.Config) error {
	return s.WithTransaction(func(t *Transaction) error { return t.SaveConfig(v) })
}

func (s *Store) UpdateConfig(work func(*model.Config) error) (result model.Config, err error) {
	err = s.WithTransaction(func(t *Transaction) error {
		result, err = t.LoadConfig()
		if err != nil {
			return err
		}
		if err = work(&result); err != nil {
			return err
		}
		return t.SaveConfig(result)
	})
	return
}
