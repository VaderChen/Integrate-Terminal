package app

import (
	"errors"
	"time"

	"IntegTERM/internal/session"
	"IntegTERM/internal/store"
)

// NewMCP 延後載入站台 JSON 檔案；檔案鎖等待有上限。
func NewMCP() *App {
	return newMCPWithStore(store.NewWithLockTimeout(resolveAppDataDir(), 2*time.Second))
}

func newMCPWithStore(data *store.Store) *App {
	return &App{
		store:          data,
		sessionManager: session.NewManager(),
		lastActivity:   make(map[string]time.Time),
		operations:     make(map[string]RESTOperation),
	}
}

// MCPStartup leaves persisted state unloaded until site discovery is requested.
// initialize/tools-list and RAM operations do not depend on saved files.
func (a *App) MCPStartup() {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.allowRESTAttach = false
	a.storageInitErr = errors.New("MCP saved state has not been loaded")
}
