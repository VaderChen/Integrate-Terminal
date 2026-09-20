package app

import (
	"errors"
	"time"

	"IntegTERM/internal/credentials"
	"IntegTERM/internal/session"
	"IntegTERM/internal/store"
)

// NewMCP constructs a headless process without touching user data or Keychain.
// Its first remote-site operation loads state with noninteractive credentials
// and a bounded lock wait, so an unattended client never waits for an OS dialog.
func NewMCP() *App {
	return newMCPWithStore(store.NewWithCredentialsAndLockTimeout(
		resolveAppDataDir(), credentials.NewNonInteractive(), 2*time.Second,
	))
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
// initialize/tools-list and RAM operations do not depend on saved credentials.
func (a *App) MCPStartup() {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.allowRESTAttach = false
	a.storageInitErr = errors.New("MCP saved state has not been loaded")
}
