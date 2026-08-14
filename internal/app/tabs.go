package app

import (
	"fmt"
	"strings"

	"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/session"
)

func (a *App) GetTabs() []model.Tab {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return cloneTabs(a.tabs)
}

func (a *App) CreateTab(site model.Site) ([]model.Tab, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	tab := session.MakeTab(site)
	remotePath, err := a.sessionManager.Connect(tab)
	if err != nil {
		return cloneTabs(a.tabs), err
	}
	if remotePath != "" {
		tab.RemotePath = remotePath
	}
	tab.Connected = true
	previousLastActiveTab := a.config.LastActiveTab
	nextTabs := append(cloneTabs(a.tabs), tab)
	a.config.LastActiveTab = tab.ID
	if err := a.persistTabsLocked(nextTabs); err != nil {
		a.config.LastActiveTab = previousLastActiveTab
		_ = a.sessionManager.Disconnect(tab.ID)
		return cloneTabs(a.tabs), err
	}
	a.tabs = nextTabs
	a.markTabActivity(tab.ID)
	a.touchSiteLastUsedLocked(site.ID)
	return cloneTabs(a.tabs), nil
}

func (a *App) CreateSSHTab(site model.Site) ([]model.Tab, error) {
	return a.createTerminalTab(site, "ssh")
}

func (a *App) CreateTelnetTab(site model.Site) ([]model.Tab, error) {
	return a.createTerminalTab(site, "telnet")
}

func (a *App) createTerminalTab(site model.Site, protocol string) ([]model.Tab, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()

	var (
		sessionID string
		err       error
		tab       model.Tab
	)
	switch protocol {
	case "ssh":
		sessionID, err = a.sessionManager.StartSSHSession(a.ctx, site)
		if err == nil {
			tab = session.MakeSSHTab(site, sessionID)
		}
	case "telnet":
		sessionID, err = a.sessionManager.StartTelnetSession(a.ctx, site)
		if err == nil {
			tab = session.MakeTelnetTab(site, sessionID)
		}
	default:
		return cloneTabs(a.tabs), fmt.Errorf("unsupported terminal protocol: %s", protocol)
	}
	if err != nil {
		return cloneTabs(a.tabs), err
	}

	previousLastActiveTab := a.config.LastActiveTab
	nextTabs := append(cloneTabs(a.tabs), tab)
	a.config.LastActiveTab = tab.ID
	if err := a.persistTabsLocked(nextTabs); err != nil {
		a.config.LastActiveTab = previousLastActiveTab
		_ = a.sessionManager.CloseSSHSession(sessionID)
		return cloneTabs(a.tabs), err
	}
	a.tabs = nextTabs
	a.markTabActivity(tab.ID)
	a.touchSiteLastUsedLocked(site.ID)
	return cloneTabs(a.tabs), nil
}

func (a *App) CreateLocalTerminalTab(cwd string) ([]model.Tab, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	sessionID, err := a.sessionManager.StartLocalSession(a.ctx, cwd)
	if err != nil {
		return cloneTabs(a.tabs), err
	}

	tab := session.MakeLocalTerminalTab(sessionID, cwd)
	previousLastActiveTab := a.config.LastActiveTab
	nextTabs := append(cloneTabs(a.tabs), tab)
	a.config.LastActiveTab = tab.ID
	if err := a.persistTabsLocked(nextTabs); err != nil {
		a.config.LastActiveTab = previousLastActiveTab
		_ = a.sessionManager.CloseLocalSession(sessionID)
		return cloneTabs(a.tabs), err
	}
	a.tabs = nextTabs
	a.markTabActivity(tab.ID)
	return cloneTabs(a.tabs), nil
}

func (a *App) CloseTab(tabID string) ([]model.Tab, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	index := -1
	for tabIndex := range a.tabs {
		if a.tabs[tabIndex].ID == tabID {
			index = tabIndex
			break
		}
	}
	if index < 0 {
		return cloneTabs(a.tabs), fmt.Errorf("tab not found: %s", tabID)
	}

	tab := a.tabs[index]
	if tab.Mode == "terminal" && tab.SessionID != "" {
		if err := a.sessionManager.CloseSSHSession(tab.SessionID); err != nil {
			return cloneTabs(a.tabs), err
		}
	} else if err := a.sessionManager.Disconnect(tabID); err != nil {
		return cloneTabs(a.tabs), err
	}

	nextTabs := append(cloneTabs(a.tabs[:index]), a.tabs[index+1:]...)
	previousLastActiveTab := a.config.LastActiveTab
	if len(nextTabs) > 0 {
		a.config.LastActiveTab = nextTabs[0].ID
	} else {
		a.config.LastActiveTab = ""
	}
	if err := a.persistTabsLocked(nextTabs); err != nil {
		a.config.LastActiveTab = previousLastActiveTab
		return cloneTabs(a.tabs), err
	}
	a.tabs = nextTabs
	a.clearTabActivity(tabID)
	return cloneTabs(a.tabs), nil
}

func (a *App) Connect(tabID string) ([]model.Tab, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	for index := range a.tabs {
		if a.tabs[index].ID != tabID {
			continue
		}
		remotePath, err := a.sessionManager.Connect(a.tabs[index])
		if err != nil {
			return cloneTabs(a.tabs), err
		}
		nextTabs := cloneTabs(a.tabs)
		if remotePath != "" {
			nextTabs[index].RemotePath = remotePath
		}
		nextTabs[index].Connected = true
		if err := a.persistTabsLocked(nextTabs); err != nil {
			_ = a.sessionManager.Disconnect(tabID)
			return cloneTabs(a.tabs), err
		}
		a.tabs = nextTabs
		a.markTabActivity(tabID)
		a.touchSiteLastUsedLocked(a.tabs[index].SiteID)
		return cloneTabs(a.tabs), nil
	}
	return cloneTabs(a.tabs), fmt.Errorf("tab not found: %s", tabID)
}

func (a *App) Disconnect(tabID string) ([]model.Tab, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	index := -1
	for tabIndex := range a.tabs {
		if a.tabs[tabIndex].ID == tabID {
			index = tabIndex
			break
		}
	}
	if index < 0 {
		return cloneTabs(a.tabs), fmt.Errorf("tab not found: %s", tabID)
	}
	if err := a.sessionManager.Disconnect(tabID); err != nil {
		return cloneTabs(a.tabs), err
	}
	nextTabs := cloneTabs(a.tabs)
	nextTabs[index].Connected = false
	if err := a.persistTabsLocked(nextTabs); err != nil {
		return cloneTabs(a.tabs), err
	}
	a.tabs = nextTabs
	a.clearTabActivity(tabID)
	return cloneTabs(a.tabs), nil
}

func (a *App) UpdateTabPaths(tabID string, localPath string, remotePath string) ([]model.Tab, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	for index := range a.tabs {
		if a.tabs[index].ID != tabID {
			continue
		}
		nextTabs := cloneTabs(a.tabs)
		if strings.TrimSpace(localPath) != "" {
			nextTabs[index].LocalPath = localPath
		}
		if strings.TrimSpace(remotePath) != "" {
			nextTabs[index].RemotePath = remotePath
		}
		previousLastActiveTab := a.config.LastActiveTab
		a.config.LastActiveTab = tabID
		if err := a.persistTabsLocked(nextTabs); err != nil {
			a.config.LastActiveTab = previousLastActiveTab
			return cloneTabs(a.tabs), err
		}
		a.tabs = nextTabs
		a.markTabActivity(tabID)
		return cloneTabs(a.tabs), nil
	}
	return cloneTabs(a.tabs), fmt.Errorf("tab not found: %s", tabID)
}

func (a *App) ReorderTabs(tabIDs []string) ([]model.Tab, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	visible := visibleTabs(a.tabs)
	var nextTabs []model.Tab
	if len(tabIDs) == len(visible) && len(visible) != len(a.tabs) {
		tabByID := make(map[string]model.Tab, len(visible))
		for _, tab := range visible {
			tabByID[tab.ID] = tab
		}
		reorderedVisible := make([]model.Tab, 0, len(visible))
		seen := make(map[string]struct{}, len(tabIDs))
		for _, tabID := range tabIDs {
			tab, ok := tabByID[tabID]
			if !ok {
				return cloneTabs(a.tabs), fmt.Errorf("visible tab not found: %s", tabID)
			}
			if _, exists := seen[tabID]; exists {
				return cloneTabs(a.tabs), fmt.Errorf("duplicate visible tab id: %s", tabID)
			}
			seen[tabID] = struct{}{}
			reorderedVisible = append(reorderedVisible, tab)
		}
		nextTabs = make([]model.Tab, 0, len(a.tabs))
		visibleIndex := 0
		for _, tab := range a.tabs {
			if tab.Hidden {
				nextTabs = append(nextTabs, tab)
				continue
			}
			nextTabs = append(nextTabs, reorderedVisible[visibleIndex])
			visibleIndex++
		}
	} else {
		if len(tabIDs) != len(a.tabs) {
			return cloneTabs(a.tabs), fmt.Errorf("tab reorder length mismatch")
		}
		indexByID := make(map[string]model.Tab, len(a.tabs))
		for _, tab := range a.tabs {
			indexByID[tab.ID] = tab
		}
		nextTabs = make([]model.Tab, 0, len(a.tabs))
		seen := make(map[string]struct{}, len(tabIDs))
		for _, tabID := range tabIDs {
			tab, ok := indexByID[tabID]
			if !ok {
				return cloneTabs(a.tabs), fmt.Errorf("tab not found: %s", tabID)
			}
			if _, exists := seen[tabID]; exists {
				return cloneTabs(a.tabs), fmt.Errorf("duplicate tab id: %s", tabID)
			}
			seen[tabID] = struct{}{}
			nextTabs = append(nextTabs, tab)
		}
	}

	previousLastActiveTab := a.config.LastActiveTab
	if !containsTabID(nextTabs, a.config.LastActiveTab) && len(nextTabs) > 0 {
		a.config.LastActiveTab = nextTabs[0].ID
	}
	if err := a.persistTabsLocked(nextTabs); err != nil {
		a.config.LastActiveTab = previousLastActiveTab
		return cloneTabs(a.tabs), err
	}
	a.tabs = nextTabs
	return cloneTabs(a.tabs), nil
}
