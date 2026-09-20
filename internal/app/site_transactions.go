package app

import (
	"IntegTERM/internal/model"
	"IntegTERM/internal/store"
)

// The OS file lock spans the complete read-modify-write cycle, including
// requests from a different GUI/background-service process.
func (a *App) mutateSiteLibraryLocked(work func() error) error {
	if a.storageInitErr != nil {
		return a.storageInitErr
	}
	previousSites, previousConfig := a.sites, cloneConfig(a.config)
	err := a.store.WithTransaction(func(tx *store.Transaction) error {
		sites, err := tx.LoadSites()
		if err != nil {
			return err
		}
		cfg, err := tx.LoadConfig()
		if err != nil {
			return err
		}
		a.sites = normalizeLoadedSites(sites)
		a.config.SiteFolders = sanitizeSiteFolders(cfg.SiteFolders, a.sites)
		if err := work(); err != nil {
			return err
		}
		cfg.SiteFolders = append([]string(nil), a.config.SiteFolders...)
		if err := tx.SaveSites(a.sites); err != nil {
			return err
		}
		return tx.SaveConfig(cfg)
	})
	if err != nil {
		a.sites, a.config = previousSites, previousConfig
	}
	return err
}

func (a *App) SaveSite(site model.Site) (result []model.Site, err error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	err = a.mutateSiteLibraryLocked(func() error {
		result, err = a.saveSiteLocked(site)
		return err
	})
	return
}

func (a *App) DeleteSite(id string) (result []model.Site, err error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	err = a.mutateSiteLibraryLocked(func() error {
		result, err = a.deleteSiteLocked(id)
		return err
	})
	return
}

func (a *App) SortSitesByName() (result []model.Site, err error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	err = a.mutateSiteLibraryLocked(func() error {
		result, err = a.sortSitesByNameLocked()
		return err
	})
	return
}

func (a *App) CreateSiteFolder(name string) (result model.Config, err error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	err = a.mutateSiteLibraryLocked(func() error {
		result, err = a.createSiteFolderLocked(name)
		return err
	})
	result = cloneConfig(result)
	return
}

func (a *App) SortSiteFolders() (result model.Config, err error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	err = a.mutateSiteLibraryLocked(func() error {
		result, err = a.sortSiteFoldersLocked()
		return err
	})
	result = cloneConfig(result)
	return
}

func (a *App) RenameSiteFolder(name string, nextName string) (result model.SiteLibraryMutationResult, err error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	err = a.mutateSiteLibraryLocked(func() error {
		result, err = a.renameSiteFolderLocked(name, nextName)
		return err
	})
	result.Config = cloneConfig(result.Config)
	return
}

func (a *App) ReorderSiteFolders(folderNames []string) (result model.Config, err error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	err = a.mutateSiteLibraryLocked(func() error {
		result, err = a.reorderSiteFoldersLocked(folderNames)
		return err
	})
	result = cloneConfig(result)
	return
}

func (a *App) DeleteSiteFolder(name string) (result model.SiteLibraryMutationResult, err error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	err = a.mutateSiteLibraryLocked(func() error {
		result, err = a.deleteSiteFolderLocked(name)
		return err
	})
	result.Config = cloneConfig(result.Config)
	return
}

func (a *App) ReorderSites(siteIDs []string) (result []model.Site, err error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	err = a.mutateSiteLibraryLocked(func() error {
		result, err = a.reorderSitesLocked(siteIDs)
		return err
	})
	return
}
