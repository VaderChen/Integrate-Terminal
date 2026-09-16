package app

import (
	"context"
	"fmt"
	"strings"

	"IntegTERM/internal/model"
	"IntegTERM/internal/purchase"
)

func (a *App) SaveConfig(config model.Config) (model.Config, error) {
	previousConfig := a.config
	a.config = config
	a.config.SiteFolders = sanitizeSiteFolders(a.config.SiteFolders, a.sites)
	a.config.RESTServerPort = sanitizeRESTServerPort(a.config.RESTServerPort)
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
			return a.config, err
		}
	}

	if a.allowRESTAttach {
		if err := a.store.SaveConfig(a.config); err != nil {
			a.config = previousConfig
			return a.config, err
		}
		if shouldRunBackgroundService(a.config) {
			if err := a.ensureBackgroundService(); err != nil {
				a.config = previousConfig
				_ = a.store.SaveConfig(previousConfig)
				return a.config, err
			}
		}
		a.syncAttachedRESTState()
	} else {
		if err := a.applyRESTServerConfig(); err != nil {
			a.config = previousConfig
			_ = a.applyRESTServerConfig()
			return a.config, err
		}
		if err := a.store.SaveConfig(a.config); err != nil {
			return a.config, err
		}
	}
	if a.config.RestoreTabsOnStart {
		return a.config, a.persistTabs()
	}
	return a.config, nil
}

func (a *App) persistTabs() error {
	if !a.config.RestoreTabsOnStart {
		return a.store.SaveTabs([]model.Tab{})
	}

	tabs := restoreableTabs(a.tabs)
	if !containsTabID(tabs, a.config.LastActiveTab) {
		a.config.LastActiveTab = ""
	}
	return a.store.SaveTabs(tabs)
}

func (a *App) SetProUnlock(enabled bool) (model.Config, error) {
	a.config.ProUnlock = enabled
	return a.config, a.store.SaveConfig(a.config)
}

func (a *App) currentTabLimit() int {
	if a.config.ProUnlock {
		return 0
	}
	return freePlanTabLimit
}

func (a *App) ensureTabCreationAllowed() error {
	limit := a.currentTabLimit()
	if limit == 0 {
		return nil
	}
	if len(visibleTabs(a.tabs)) < limit {
		return nil
	}
	return fmt.Errorf("目前未解鎖 Pro，最多只能開啟 %d 個 TAB", limit)
}

func (a *App) purchaseContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) applyPurchaseState(state purchase.State) error {
	a.config.ProUnlock = state.ProUnlock
	return a.store.SaveConfig(a.config)
}

func (a *App) mergePurchaseStatus(state purchase.State, sourceErr error) model.PurchaseStatus {
	proUnlock := a.config.ProUnlock || state.ProUnlock
	planName := strings.TrimSpace(state.PlanName)
	if planName == "" {
		if proUnlock {
			planName = "Pro"
		} else {
			planName = "Free"
		}
	}

	statusMessage := strings.TrimSpace(state.StatusMessage)
	if sourceErr != nil && statusMessage == "" {
		statusMessage = sourceErr.Error()
	}
	if statusMessage == "" {
		if proUnlock {
			statusMessage = "目前已解鎖 Pro，TAB 數量無限制"
		} else {
			statusMessage = fmt.Sprintf("目前為 Free，最多可開啟 %d 個 TAB", freePlanTabLimit)
		}
	}

	currentTabs := len(visibleTabs(a.tabs))
	maxTabs := a.currentTabLimit()
	canPurchase := state.CanPurchase
	canRestore := state.CanRestore
	if proUnlock {
		canPurchase = false
		canRestore = true
	}

	source := strings.TrimSpace(state.Source)
	if source == "" {
		source = "config"
	}

	productID := strings.TrimSpace(state.ProductID)
	if productID == "" {
		productID = purchase.ProUnlockProductID
	}

	return model.PurchaseStatus{
		ProductID:     productID,
		PlanName:      planName,
		Source:        source,
		StatusMessage: statusMessage,
		ProUnlock:     proUnlock,
		CanPurchase:   canPurchase,
		CanRestore:    canRestore,
		MaxTabs:       maxTabs,
		CurrentTabs:   currentTabs,
	}
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
