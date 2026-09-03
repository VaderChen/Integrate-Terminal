package app

import (
	"fmt"
	"strings"

	"github.com/VaderChen/Integrate-Terminal/internal/model"
)

func (a *App) SaveConfig(config model.Config) (model.Config, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	previousConfig := cloneConfig(a.config)
	previousTabs := cloneTabs(a.tabs)
	a.config = cloneConfig(config)
	a.config.SiteFolders = sanitizeSiteFolders(a.config.SiteFolders, a.sites)
	a.config.RESTServerPort = sanitizeRESTServerPort(a.config.RESTServerPort)
	a.config.RESTServerAllowlist = sanitizeRESTServerAllowlist(a.config.RESTServerAllowlist)
	a.config.TransferRetryCount = sanitizeTransferRetryCount(a.config.TransferRetryCount)
	a.config.TransferConflictStrategy = sanitizeTransferConflictStrategy(a.config.TransferConflictStrategy)
	a.sessionManager.ConfigureTransferPolicy(a.config.TransferRetryCount, a.config.TransferConflictStrategy)
	if a.config.RESTServerEnabled {
		a.config.ShowTrayIcon = true
	}
	if !a.config.RememberWindowPosition {
		a.config.WindowWidth = 0
		a.config.WindowHeight = 0
		a.config.WindowX = 0
		a.config.WindowY = 0
	}
	if !a.config.RestoreTabsOnStart {
		a.config.LastActiveTab = ""
		if err := a.store.SaveTabs([]model.Tab{}); err != nil {
			a.config = previousConfig
			return cloneConfig(a.config), err
		}
	}

	if a.allowRESTAttach {
		if err := a.store.SaveConfig(a.config); err != nil {
			a.config = previousConfig
			return cloneConfig(a.config), err
		}
		if shouldRunBackgroundService(a.config) {
			if err := a.ensureBackgroundService(); err != nil {
				a.config = previousConfig
				_ = a.store.SaveConfig(previousConfig)
				return cloneConfig(a.config), err
			}
		}
		a.syncAttachedRESTState()
	} else {
		if err := a.applyRESTServerConfig(); err != nil {
			a.config = previousConfig
			_ = a.applyRESTServerConfig()
			return cloneConfig(a.config), err
		}
		if err := a.store.SaveConfig(a.config); err != nil {
			a.config = previousConfig
			_ = a.applyRESTServerConfig()
			if previousConfig.RestoreTabsOnStart {
				_ = a.store.SaveTabs(restoreableTabs(previousTabs))
			} else {
				_ = a.store.SaveTabs([]model.Tab{})
			}
			return cloneConfig(a.config), err
		}
	}
	if a.config.RestoreTabsOnStart {
		if err := a.persistTabsLocked(a.tabs); err != nil {
			a.config = previousConfig
			return cloneConfig(a.config), err
		}
	}
	return cloneConfig(a.config), nil
}

func (a *App) persistTabsLocked(tabs []model.Tab) error {
	if !a.config.RestoreTabsOnStart {
		return a.store.SaveTabs([]model.Tab{})
	}

	tabs = restoreableTabs(tabs)
	if !containsTabID(tabs, a.config.LastActiveTab) {
		a.config.LastActiveTab = ""
	}
	return a.store.SaveTabs(tabs)
}

func restoreableTabs(tabs []model.Tab) []model.Tab {
	restored := make([]model.Tab, 0, len(tabs))
	for _, tab := range tabs {
		if tab.Mode != "file" || tab.Hidden {
			continue
		}
		tab.Connected = false
		tab.SessionID = ""
		restored = append(restored, tab)
	}
	return restored
}

func visibleTabs(tabs []model.Tab) []model.Tab {
	visible := make([]model.Tab, 0, len(tabs))
	for _, tab := range tabs {
		if tab.Hidden {
			continue
		}
		visible = append(visible, tab)
	}
	return visible
}

func containsTabID(tabs []model.Tab, tabID string) bool {
	if strings.TrimSpace(tabID) == "" {
		return false
	}
	for _, tab := range tabs {
		if tab.ID == tabID {
			return true
		}
	}
	return false
}

func validateSiteByProtocol(site model.Site) error {
	username := strings.TrimSpace(site.Username)
	password := strings.TrimSpace(site.Password)
	ppkPath := strings.TrimSpace(site.PPKPath)

	switch site.Protocol {
	case "ftp":
		if username == "" {
			return fmt.Errorf("username is required for ftp")
		}
	case "sftp":
		if username == "" {
			return fmt.Errorf("username is required for sftp")
		}
		if password == "" && ppkPath == "" {
			return fmt.Errorf("password or ppk key is required for sftp")
		}
	default:
		return fmt.Errorf("unsupported protocol: %s", site.Protocol)
	}

	return nil
}

func sanitizeTransferRetryCount(value int) int {
	if value < 0 {
		return 0
	}
	if value > 10 {
		return 10
	}
	return value
}

func sanitizeTransferConflictStrategy(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "skip":
		return "skip"
	case "fail":
		return "fail"
	default:
		return "overwrite"
	}
}
