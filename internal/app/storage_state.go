package app

import (
	"fmt"

	"IntegTERM/internal/model"
	"IntegTERM/internal/store"
)

// Only publish initial state after every file and credential has been read.
// The caller holds stateMu; retries are safe while storageInitErr blocks writes.
func (a *App) loadInitialStateLocked() error {
	var sites []model.Site
	var tabs []model.Tab
	var cfg model.Config
	err := a.store.WithTransaction(func(tx *store.Transaction) error {
		var err error
		if cfg, err = tx.LoadConfig(); err != nil {
			return fmt.Errorf("load settings: %w", err)
		}
		if sites, err = tx.LoadSites(); err != nil {
			return fmt.Errorf("load saved connections: %w", err)
		}
		if tabs, err = tx.LoadTabs(); err != nil {
			return fmt.Errorf("load saved tabs: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	a.sites = normalizeLoadedSites(sites)
	cfg.ProUnlock = a.verifiedProUnlock
	cfg.SiteFolders = sanitizeSiteFolders(cfg.SiteFolders, a.sites)
	cfg.RESTServerPort = sanitizeRESTServerPort(cfg.RESTServerPort)
	a.config = cfg
	a.tabs = []model.Tab{}
	if cfg.RestoreTabsOnStart {
		a.tabs = restoreableTabs(tabs)
	}
	if !containsTabID(a.tabs, a.config.LastActiveTab) {
		a.config.LastActiveTab = ""
	}
	return nil
}

func storageErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (a *App) retryInitialStorageLocked() error {
	if a.storageInitErr != nil {
		a.storageInitErr = a.loadInitialStateLocked()
	}
	return a.storageInitErr
}
