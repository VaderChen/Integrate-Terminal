package app

import (
	"context"
	"fmt"
	"strings"

	"IntegTERM/internal/model"
	"IntegTERM/internal/session"
)

func (a *App) getTabsLocked() []model.Tab {
	return a.tabs
}

// createConnectedTab 先保留名額，再於鎖外等待連線；提交時一次設定分頁可見性。
func (a *App) createConnectedTab(hidden bool, open func(context.Context) (model.Tab, error)) ([]model.Tab, error) {
	a.stateMu.Lock()
	if err := a.ensureTabCreationAllowed(); err != nil {
		tabs := append([]model.Tab{}, a.tabs...)
		a.stateMu.Unlock()
		return tabs, err
	}
	a.pendingTabCreations++
	ctx := a.ctx
	a.stateMu.Unlock()

	tab, err := open(ctx)
	a.stateMu.Lock()
	a.pendingTabCreations--
	if err == nil {
		err = a.ensureTabCreationAllowed()
	}
	accepted := err == nil
	if accepted {
		tab.Hidden = hidden
		a.tabs = append(a.tabs, tab)
		a.config.LastActiveTab = tab.ID
		a.markTabActivity(tab.ID)
		err = a.persistTabs()
	}
	tabs := append([]model.Tab{}, a.tabs...)
	a.stateMu.Unlock()
	if !accepted && tab.ID != "" {
		if tab.Mode == "terminal" {
			_ = a.sessionManager.CloseSSHSession(tab.SessionID)
		} else {
			_ = a.sessionManager.Disconnect(tab.ID)
		}
	}
	return tabs, err
}

func (a *App) createFileTab(site model.Site, hidden bool) ([]model.Tab, error) {
	return a.createConnectedTab(hidden, func(context.Context) (model.Tab, error) {
		tab := session.MakeTab(site)
		remotePath, err := a.sessionManager.Connect(tab)
		if remotePath != "" {
			tab.RemotePath = remotePath
		}
		tab.Connected = err == nil
		return tab, err
	})
}

func (a *App) createSSHTab(site model.Site, hidden bool) ([]model.Tab, error) {
	return a.createConnectedTab(hidden, func(ctx context.Context) (model.Tab, error) {
		sessionID, err := a.sessionManager.StartSSHSession(ctx, site)
		if err != nil {
			return model.Tab{}, err
		}
		return session.MakeSSHTab(site, sessionID), nil
	})
}

func (a *App) createTelnetTab(site model.Site, hidden bool) ([]model.Tab, error) {
	return a.createConnectedTab(hidden, func(ctx context.Context) (model.Tab, error) {
		sessionID, err := a.sessionManager.StartTelnetSession(ctx, site)
		if err != nil {
			return model.Tab{}, err
		}
		return session.MakeTelnetTab(site, sessionID), nil
	})
}

func (a *App) createLocalTerminalTab(cwd string, hidden bool) ([]model.Tab, error) {
	return a.createConnectedTab(hidden, func(ctx context.Context) (model.Tab, error) {
		sessionID, err := a.sessionManager.StartLocalSession(ctx, cwd)
		if err != nil {
			return model.Tab{}, err
		}
		return session.MakeLocalTerminalTab(sessionID, cwd), nil
	})
}

func (a *App) closeTabLocked(tabID string) ([]model.Tab, error) {
	delete(a.tabConnections, tabID)
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

func (a *App) connectTab(tabID string) ([]model.Tab, error) {
	a.stateMu.Lock()
	var target model.Tab
	for _, tab := range a.tabs {
		if tab.ID == tabID {
			target = tab
			break
		}
	}
	if target.ID == "" {
		tabs := append([]model.Tab{}, a.tabs...)
		err := a.persistTabs()
		a.stateMu.Unlock()
		return tabs, err
	}
	if a.tabConnections == nil {
		a.tabConnections = make(map[string]*model.Tab)
	}
	attempt := &target
	a.tabConnections[tabID] = attempt
	a.stateMu.Unlock()

	connection, err := a.sessionManager.PrepareConnection(target)
	a.stateMu.Lock()
	cleanup := func() {}
	if a.tabConnections[tabID] != attempt {
		if err == nil {
			err = fmt.Errorf("連線請求已取消: %s", tabID)
		}
	} else {
		delete(a.tabConnections, tabID)
		if err == nil {
			for i := range a.tabs {
				if a.tabs[i].ID == tabID {
					cleanup = a.sessionManager.CommitConnection(tabID, connection)
					if connection.RemotePath != "" {
						a.tabs[i].RemotePath = connection.RemotePath
					}
					a.tabs[i].Connected = true
					a.markTabActivity(tabID)
					connection = nil
					err = a.persistTabs()
					break
				}
			}
		}
	}
	if connection != nil && err == nil {
		err = fmt.Errorf("連線請求已取消: %s", tabID)
	}
	tabs := append([]model.Tab{}, a.tabs...)
	a.stateMu.Unlock()
	cleanup()
	if connection != nil {
		_ = connection.Close()
	}
	return tabs, err
}

func (a *App) disconnectLocked(tabID string) ([]model.Tab, error) {
	delete(a.tabConnections, tabID)
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

func (a *App) updateTabPathsLocked(tabID string, localPath string, remotePath string) ([]model.Tab, error) {
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

func (a *App) reorderTabsLocked(tabIDs []string) ([]model.Tab, error) {
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
