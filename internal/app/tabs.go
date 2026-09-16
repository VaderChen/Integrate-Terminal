package app

import (
	"fmt"
	"strings"

	"IntegTERM/internal/model"
	"IntegTERM/internal/session"
)

func (a *App) GetTabs() []model.Tab {
	return a.tabs
}

func (a *App) CreateTab(site model.Site) ([]model.Tab, error) {
	if err := a.ensureTabCreationAllowed(); err != nil {
		return a.tabs, err
	}
	tab := session.MakeTab(site)
	remotePath, err := a.sessionManager.Connect(tab)
	if err != nil {
		return a.tabs, err
	}
	if remotePath != "" {
		tab.RemotePath = remotePath
	}
	tab.Connected = true
	a.tabs = append(a.tabs, tab)
	a.config.LastActiveTab = tab.ID
	a.markTabActivity(tab.ID)
	return a.tabs, a.persistTabs()
}

func (a *App) CreateSSHTab(site model.Site) ([]model.Tab, error) {
	if err := a.ensureTabCreationAllowed(); err != nil {
		return a.tabs, err
	}
	sessionID, err := a.sessionManager.StartSSHSession(a.ctx, site)
	if err != nil {
		return a.tabs, err
	}

	tab := session.MakeSSHTab(site, sessionID)
	a.tabs = append(a.tabs, tab)
	a.config.LastActiveTab = tab.ID
	a.markTabActivity(tab.ID)
	return a.tabs, a.persistTabs()
}

func (a *App) CreateTelnetTab(site model.Site) ([]model.Tab, error) {
	if err := a.ensureTabCreationAllowed(); err != nil {
		return a.tabs, err
	}
	sessionID, err := a.sessionManager.StartTelnetSession(a.ctx, site)
	if err != nil {
		return a.tabs, err
	}

	tab := session.MakeTelnetTab(site, sessionID)
	a.tabs = append(a.tabs, tab)
	a.config.LastActiveTab = tab.ID
	a.markTabActivity(tab.ID)
	return a.tabs, a.persistTabs()
}

func (a *App) CreateLocalTerminalTab(cwd string) ([]model.Tab, error) {
	if err := a.ensureTabCreationAllowed(); err != nil {
		return a.tabs, err
	}
	sessionID, err := a.sessionManager.StartLocalSession(a.ctx, cwd)
	if err != nil {
		return a.tabs, err
	}

	tab := session.MakeLocalTerminalTab(sessionID, cwd)
	a.tabs = append(a.tabs, tab)
	a.config.LastActiveTab = tab.ID
	a.markTabActivity(tab.ID)
	return a.tabs, a.persistTabs()
}

func (a *App) CloseTab(tabID string) ([]model.Tab, error) {
	for _, tab := range a.tabs {
		if tab.ID == tabID {
			if tab.Mode == "terminal" && tab.SessionID != "" {
				_ = a.sessionManager.CloseSSHSession(tab.SessionID)
			} else {
				_ = a.sessionManager.Disconnect(tabID)
			}
			break
		}
	}
	filtered := make([]model.Tab, 0, len(a.tabs))
	for _, tab := range a.tabs {
		if tab.ID != tabID {
			filtered = append(filtered, tab)
		}
	}
	a.tabs = filtered
	a.clearTabActivity(tabID)
	if len(a.tabs) > 0 {
		a.config.LastActiveTab = a.tabs[0].ID
	} else {
		a.config.LastActiveTab = ""
	}
	return a.tabs, a.persistTabs()
}

func (a *App) Connect(tabID string) ([]model.Tab, error) {
	for i := range a.tabs {
		if a.tabs[i].ID == tabID {
			remotePath, err := a.sessionManager.Connect(a.tabs[i])
			if err != nil {
				return a.tabs, err
			}
			if remotePath != "" {
				a.tabs[i].RemotePath = remotePath
			}
			a.tabs[i].Connected = true
			a.markTabActivity(a.tabs[i].ID)
			return a.tabs, a.persistTabs()
		}
	}
	return a.tabs, a.persistTabs()
}

func (a *App) Disconnect(tabID string) ([]model.Tab, error) {
	if err := a.sessionManager.Disconnect(tabID); err != nil {
		return a.tabs, err
	}
	for i := range a.tabs {
		if a.tabs[i].ID == tabID {
			a.tabs[i].Connected = false
			a.clearTabActivity(tabID)
			break
		}
	}
	return a.tabs, a.persistTabs()
}

func (a *App) UpdateTabPaths(tabID string, localPath string, remotePath string) ([]model.Tab, error) {
	for i := range a.tabs {
		if a.tabs[i].ID != tabID {
			continue
		}
		if strings.TrimSpace(localPath) != "" {
			a.tabs[i].LocalPath = localPath
		}
		if strings.TrimSpace(remotePath) != "" {
			a.tabs[i].RemotePath = remotePath
		}
		a.config.LastActiveTab = a.tabs[i].ID
		a.markTabActivity(tabID)
		return a.tabs, a.persistTabs()
	}
	return a.tabs, fmt.Errorf("tab not found: %s", tabID)
}

func (a *App) ReorderTabs(tabIDs []string) ([]model.Tab, error) {
	visible := visibleTabs(a.tabs)
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
				return a.tabs, fmt.Errorf("visible tab not found: %s", tabID)
			}
			if _, exists := seen[tabID]; exists {
				return a.tabs, fmt.Errorf("duplicate visible tab id: %s", tabID)
			}
			seen[tabID] = struct{}{}
			reorderedVisible = append(reorderedVisible, tab)
		}

		nextTabs := make([]model.Tab, 0, len(a.tabs))
		visibleIndex := 0
		for _, tab := range a.tabs {
			if tab.Hidden {
				nextTabs = append(nextTabs, tab)
				continue
			}
			nextTabs = append(nextTabs, reorderedVisible[visibleIndex])
			visibleIndex++
		}
		a.tabs = nextTabs
		if !containsTabID(visibleTabs(a.tabs), a.config.LastActiveTab) && len(visibleTabs(a.tabs)) > 0 {
			a.config.LastActiveTab = visibleTabs(a.tabs)[0].ID
		}
		return a.tabs, a.persistTabs()
	}

	if len(tabIDs) != len(a.tabs) {
		return a.tabs, fmt.Errorf("tab reorder length mismatch")
	}

	indexByID := make(map[string]model.Tab, len(a.tabs))
	for _, tab := range a.tabs {
		indexByID[tab.ID] = tab
	}

	reordered := make([]model.Tab, 0, len(a.tabs))
	seen := make(map[string]struct{}, len(tabIDs))
	for _, tabID := range tabIDs {
		tab, ok := indexByID[tabID]
		if !ok {
			return a.tabs, fmt.Errorf("tab not found: %s", tabID)
		}
		if _, exists := seen[tabID]; exists {
			return a.tabs, fmt.Errorf("duplicate tab id: %s", tabID)
		}
		seen[tabID] = struct{}{}
		reordered = append(reordered, tab)
	}

	a.tabs = reordered
	if !containsTabID(a.tabs, a.config.LastActiveTab) && len(a.tabs) > 0 {
		a.config.LastActiveTab = a.tabs[0].ID
	}
	return a.tabs, a.persistTabs()
}
