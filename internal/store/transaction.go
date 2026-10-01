package store

import (
	"IntegTERM/internal/model"
	"encoding/json"
	"os"
	"path/filepath"
)

// Transaction serializes read-modify-write operations across GUI and service
// processes. Call its methods rather than Store methods inside the callback.
type Transaction struct {
	store   *Store
	pending map[string]json.RawMessage
	order   []string
}

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
	if err := s.recoverTransaction(); err != nil {
		return err
	}
	tx := &Transaction{store: s, pending: make(map[string]json.RawMessage)}
	if err := work(tx); err != nil {
		return err
	}
	return tx.commit()
}

func (t *Transaction) LoadSites() ([]model.Site, error) {
	if raw, ok := t.pending["sites.json"]; ok {
		var v []model.Site
		err := json.Unmarshal(raw, &v)
		if v == nil {
			v = []model.Site{}
		}
		return v, err
	}
	return t.store.loadSites()
}
func (t *Transaction) SaveSites(v []model.Site) error {
	if _, err := t.LoadSites(); err != nil {
		return err
	}
	return t.stage("sites.json", v)
}
func (t *Transaction) LoadTabs() ([]model.Tab, error) {
	if raw, ok := t.pending["tabs.json"]; ok {
		var v []model.Tab
		err := json.Unmarshal(raw, &v)
		if v == nil {
			v = []model.Tab{}
		}
		return v, err
	}
	return t.store.loadTabs()
}
func (t *Transaction) SaveTabs(v []model.Tab) error {
	if _, err := t.LoadTabs(); err != nil {
		return err
	}
	return t.stage("tabs.json", v)
}
func (t *Transaction) LoadConfig() (model.Config, error) {
	if raw, ok := t.pending["config.json"]; ok {
		var v model.Config
		err := json.Unmarshal(raw, &v)
		return v, err
	}
	return t.store.loadConfig()
}
func (t *Transaction) SaveConfig(v model.Config) error {
	if _, err := t.LoadConfig(); err != nil {
		return err
	}
	v.ProUnlock = false
	return t.stage("config.json", v)
}
func (t *Transaction) stage(name string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, ok := t.pending[name]; !ok {
		t.order = append(t.order, name)
	}
	t.pending[name] = raw
	return nil
}

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
