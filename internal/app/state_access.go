package app

import (
	"IntegTERM/internal/model"
	"time"
)

func cloneConfig(config model.Config) model.Config {
	config.SiteFolders = append([]string(nil), config.SiteFolders...)
	return config
}

// Public state methods own the lock. Internal ...Locked methods require it.
// Return snapshots so HTTP encoding and Wails callers never retain shared slices.
func (a *App) Bootstrap() model.BootstrapPayload {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	value := a.bootstrapLocked()
	value.Config = cloneConfig(value.Config)
	return value
}

func (a *App) GetSites() ([]model.Site, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.getSitesLocked()
}

func (a *App) GetConfig() model.Config {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return cloneConfig(a.getConfigLocked())
}

func (a *App) GetTabs() ([]model.Tab, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	if err := a.retryInitialStorageLocked(); err != nil {
		return nil, err
	}
	return append([]model.Tab{}, a.getTabsLocked()...), nil
}

func (a *App) CreateTab(site model.Site) ([]model.Tab, error) {
	return a.createFileTab(site, false)
}

func (a *App) CreateSSHTab(site model.Site) ([]model.Tab, error) {
	return a.createSSHTab(site, false)
}

func (a *App) CreateTelnetTab(site model.Site) ([]model.Tab, error) {
	return a.createTelnetTab(site, false)
}

func (a *App) CreateLocalTerminalTab(cwd string) ([]model.Tab, error) {
	return a.createLocalTerminalTab(cwd, false)
}

func (a *App) CloseTab(tabID string) ([]model.Tab, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	value, err := a.closeTabLocked(tabID)
	return append([]model.Tab{}, value...), err
}

func (a *App) Connect(tabID string) ([]model.Tab, error) {
	return a.connectTab(tabID)
}

func (a *App) Disconnect(tabID string) ([]model.Tab, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	value, err := a.disconnectLocked(tabID)
	return append([]model.Tab{}, value...), err
}

func (a *App) UpdateTabPaths(tabID string, localPath string, remotePath string) ([]model.Tab, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	value, err := a.updateTabPathsLocked(tabID, localPath, remotePath)
	return append([]model.Tab{}, value...), err
}

func (a *App) ReorderTabs(tabIDs []string) ([]model.Tab, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	value, err := a.reorderTabsLocked(tabIDs)
	return append([]model.Tab{}, value...), err
}

func (a *App) CloseIdleHiddenConnections(idleLimit time.Duration) int {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.closeIdleHiddenConnectionsLocked(idleLimit)
}

func (a *App) ClearBackgroundConnections() int {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.clearBackgroundConnectionsLocked()
}

func (a *App) GetRESTServerStatus() model.RESTServerStatus {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.getRESTServerStatusLocked()
}

func (a *App) GetRESTServerPort() int {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.getRESTServerPortLocked()
}

func (a *App) GetRESTServerBaseURL() string {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.getRESTServerBaseURLLocked()
}

func (a *App) GetRestAPIDocsMarkdown() (string, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.getRestAPIDocsMarkdownLocked()
}

func (a *App) ResetWindowToDefaultScale() error {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.resetWindowToDefaultScaleLocked()
}
