package app

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/VaderChen/Integrate-Terminal/internal/fileaccess"
	"github.com/VaderChen/Integrate-Terminal/internal/keystore"
	"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/session"
	"github.com/VaderChen/Integrate-Terminal/internal/sshutil"
	"github.com/VaderChen/Integrate-Terminal/internal/store"
)

type App struct {
	ctx             context.Context
	store           *store.Store
	sessionManager  *session.Manager
	sites           []model.Site
	tabs            []model.Tab
	config          model.Config
	activityMu      sync.Mutex
	stateMu         sync.RWMutex
	lastActivity    map[string]time.Time
	restServerMu    sync.Mutex
	operationMu     sync.RWMutex
	restServer      *http.Server
	restServerURL   string
	restAttached    bool
	mcpVFS          *mcpVFS
	allowRESTAttach bool
	quitApproved    bool
	operations      map[string]RESTOperation
}

// ApproveQuit allows the native window close event to complete once.
func (a *App) ApproveQuit() {
	a.stateMu.Lock()
	a.quitApproved = true
	a.stateMu.Unlock()
}

func (a *App) consumeQuitApproval() bool {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	approved := a.quitApproved
	a.quitApproved = false
	return approved
}

// ConsumeQuitApprovalForUI is used by the Wails window lifecycle callback.
func (a *App) ConsumeQuitApprovalForUI() bool {
	return a.consumeQuitApproval()
}

func New() *App {
	dataDir := resolveAppDataDir()
	fileaccess.Init(dataDir)
	keystore.Init(dataDir)
	sshutil.Init(dataDir)
	return &App{
		store:          store.New(dataDir),
		sessionManager: session.NewManager(),
		mcpVFS:         newMCPVFS(),
		lastActivity:   make(map[string]time.Time),
		operations:     make(map[string]RESTOperation),
	}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.sessionManager.SetEventContext(ctx)
	a.initialize(true)
}

func (a *App) ServiceStartup() {
	a.ctx = nil
	a.initialize(false)
}

// MCPStartup loads the application state for the standalone local MCP
// stdio process without binding the optional HTTP server itself. When HTTP
// MCP is enabled, the process may attach to the existing background service.
func (a *App) MCPStartup() {
	a.ctx = nil
	a.initialize(true)
}

func (a *App) initialize(allowRESTAttach bool) {
	a.allowRESTAttach = allowRESTAttach
	migrateLegacyDataDir(a.store.BaseDir())
	_ = a.store.Ensure()

	sites, err := a.store.LoadSites()
	if err == nil {
		a.sites = normalizeLoadedSites(sites)
		if !sitesEqualByStoredFields(sites, a.sites) {
			_ = a.store.SaveSites(a.sites)
		}
	}

	cfg, err := a.store.LoadConfig()
	if err == nil {
		a.config = cfg
	}
	a.config.SiteFolders = sanitizeSiteFolders(a.config.SiteFolders, a.sites)
	a.config.RESTServerPort = sanitizeRESTServerPort(a.config.RESTServerPort)
	a.config.RESTServerAllowlist = sanitizeRESTServerAllowlist(a.config.RESTServerAllowlist)
	a.config.TransferRetryCount = sanitizeTransferRetryCount(a.config.TransferRetryCount)
	a.config.TransferConflictStrategy = sanitizeTransferConflictStrategy(a.config.TransferConflictStrategy)
	a.sessionManager.ConfigureTransferPolicy(a.config.TransferRetryCount, a.config.TransferConflictStrategy)
	if a.allowRESTAttach && shouldRunBackgroundService(a.config) {
		_ = a.ensureBackgroundService()
	}
	_ = a.applyRESTServerConfig()

	a.tabs = []model.Tab{}
	if !a.config.RestoreTabsOnStart {
		a.config.LastActiveTab = ""
		return
	}

	tabs, err := a.store.LoadTabs()
	if err != nil {
		a.config.LastActiveTab = ""
		return
	}
	a.tabs = restoreableTabs(tabs)
	if !containsTabID(a.tabs, a.config.LastActiveTab) {
		a.config.LastActiveTab = ""
	}
}

func (a *App) DomReady(ctx context.Context) {
	a.ctx = ctx
	a.sessionManager.SetEventContext(ctx)
	a.applyInitialWindowPlacement()
}

func (a *App) Shutdown(ctx context.Context) {
	fileaccess.Close()
	_ = a.applyRESTServerShutdown()
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.captureWindowState()
	_ = a.persistTabsLocked(a.tabs)
	_ = a.store.SaveConfig(a.config)
}

func (a *App) ServiceShutdown() {
	fileaccess.Close()
	_ = a.applyRESTServerShutdown()
}

func (a *App) Bootstrap() model.BootstrapPayload {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	localPath := defaultLocalPath()
	visibleTabs := visibleTabs(a.tabs)

	if len(visibleTabs) > 0 {
		localPath = visibleTabs[0].LocalPath
	}

	return cloneBootstrapPayload(model.BootstrapPayload{
		Sites:            enrichSites(a.sites),
		Tabs:             visibleTabs,
		Config:           a.config,
		DefaultLocalPath: localPath,
		LocalFiles:       []model.FileEntry{},
		RemoteFiles:      []model.FileEntry{},
		Transfers:        a.sessionManager.SampleTransfers(),
		Logs:             a.sessionManager.SampleLogs(),
	})
}

func (a *App) GetSites() []model.Site {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return enrichSites(a.sites)
}

func (a *App) GetConfig() model.Config {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return cloneConfig(a.config)
}
