package app

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"IntegTERM/internal/model"
	"IntegTERM/internal/purchase"
)

func (a *App) SaveConfig(config model.Config) (model.Config, error) {
	a.runtimeConfigMu.Lock()
	defer a.runtimeConfigMu.Unlock()
	a.stateMu.Lock()
	allowAttach := a.allowRESTAttach
	var change *restServerChange
	var previous model.Config
	saved, err := a.saveConfigChangesLocked(config, func(before, next model.Config) error {
		previous = before
		var prepareErr error
		change, prepareErr = a.prepareRESTServerConfig(next, allowAttach)
		return prepareErr
	})
	a.stateMu.Unlock()
	if change != nil {
		defer change.close()
	}
	if err != nil {
		return saved, err
	}
	if allowAttach {
		if shouldRunBackgroundService(saved) {
			if err := a.ensureBackgroundService(saved); err != nil {
				// 背景服務與 GUI 分屬不同程序，啟動失敗時只還原本次修改的欄位。
				restored, restoreErr := a.store.UpdateConfig(func(latest *model.Config) error {
					before, after, target := reflect.ValueOf(previous), reflect.ValueOf(saved), reflect.ValueOf(latest).Elem()
					for i := 0; i < target.NumField(); i++ {
						if target.Type().Field(i).Name != "ProUnlock" && reflect.DeepEqual(target.Field(i).Interface(), after.Field(i).Interface()) {
							target.Field(i).Set(before.Field(i))
						}
					}
					return nil
				})
				if restoreErr != nil {
					return saved, fmt.Errorf("%w; 還原設定失敗: %v", err, restoreErr)
				}
				a.stateMu.Lock()
				restored.ProUnlock = a.verifiedProUnlock
				a.config = restored
				a.stateMu.Unlock()
				return cloneConfig(restored), err
			}
		}
		a.syncAttachedRESTState(saved, allowAttach)
	} else {
		a.commitRESTServerConfig(change)
	}
	// 確認服務設定已生效後，再更新分頁還原資料。
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	if !a.config.RestoreTabsOnStart {
		err = a.store.SaveTabs([]model.Tab{})
	} else if a.allowRESTAttach {
		err = a.persistTabs()
	}
	return cloneConfig(a.config), err
}

func (a *App) saveConfigChangesLocked(config model.Config, prepare func(model.Config, model.Config) error) (model.Config, error) {
	previous := cloneConfig(a.config)
	if a.storageInitErr != nil {
		return previous, a.storageInitErr
	}
	// General preferences cannot grant/revoke a StoreKit entitlement.
	config.ProUnlock = previous.ProUnlock
	saved, err := a.store.UpdateConfig(func(latest *model.Config) error {
		beforeSave := cloneConfig(*latest)
		before, requested, target := reflect.ValueOf(previous), reflect.ValueOf(config), reflect.ValueOf(latest).Elem()
		for i := 0; i < target.NumField(); i++ {
			if target.Type().Field(i).Name == "ProUnlock" || target.Type().Field(i).Name == "SiteFolders" {
				continue
			}
			if !reflect.DeepEqual(before.Field(i).Interface(), requested.Field(i).Interface()) {
				target.Field(i).Set(requested.Field(i))
			}
		}
		latest.SiteFolders = sanitizeSiteFolders(latest.SiteFolders, nil)
		latest.RESTServerPort = sanitizeRESTServerPort(latest.RESTServerPort)
		latest.ProUnlock = false
		if latest.RESTServerEnabled {
			latest.ShowTrayIcon = true
		}
		if !latest.RememberWindowPosition {
			latest.WindowWidth = 0
			latest.WindowHeight = 0
			latest.WindowX = 0
			latest.WindowY = 0
		}
		if !latest.RestoreTabsOnStart {
			latest.LastActiveTab = ""
		}
		return prepare(beforeSave, *latest)
	})
	if err != nil {
		return previous, err
	}
	a.config = saved
	a.config.ProUnlock = a.verifiedProUnlock
	return cloneConfig(a.config), nil
}

func (a *App) persistTabs() error {
	if a.storageInitErr != nil {
		return a.storageInitErr
	}
	if !a.allowRESTAttach {
		return nil
	}
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
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return cloneConfig(a.config), fmt.Errorf("Pro entitlement can only be changed by verified purchase results")
}

func (a *App) currentTabLimit() int {
	if a.verifiedProUnlock {
		return 0
	}
	return freePlanTabLimit
}

func (a *App) ensureTabCreationAllowed() error {
	if a.storageInitErr != nil {
		return a.storageInitErr
	}
	limit := a.currentTabLimit()
	if limit == 0 {
		return nil
	}
	if len(visibleTabs(a.tabs))+a.pendingTabCreations < limit {
		return nil
	}
	return fmt.Errorf("目前未解鎖 Pro，最多只能開啟 %d 個 TAB", limit)
}

func (a *App) appContext() context.Context {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.ctx
}

func (a *App) purchaseContext() context.Context {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) applyPurchaseState(state purchase.State) error {
	a.verifiedProUnlock = state.ProUnlock
	a.config.ProUnlock = state.ProUnlock
	return nil
}

func (a *App) mergePurchaseStatus(state purchase.State, sourceErr error) model.PurchaseStatus {
	proUnlock := a.verifiedProUnlock
	planName := "Free"
	if proUnlock {
		planName = "Pro"
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
