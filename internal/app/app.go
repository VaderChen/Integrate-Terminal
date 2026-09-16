package app

import (
	"context"
	"net/http"
	"sync"
	"time"

	"IntegTERM/internal/model"
	"IntegTERM/internal/purchase"
	"IntegTERM/internal/session"
	"IntegTERM/internal/store"
)

const freePlanTabLimit = 2

type App struct {
	ctx             context.Context
	store           *store.Store
	purchaseService *purchase.Service
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
	restServerToken string
	restAttached    bool
	allowRESTAttach bool
	operations      map[string]RESTOperation
}

func New() *App {
	dataDir := resolveAppDataDir()
	return &App{
		store:           store.New(dataDir),
		purchaseService: purchase.NewService(),
		sessionManager:  session.NewManager(),
		lastActivity:    make(map[string]time.Time),
		restServerToken: loadOrCreateRESTToken(dataDir),
		operations:      make(map[string]RESTOperation),
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
	_ = a.applyRESTServerShutdown()
	a.captureWindowState()
	_ = a.persistTabs()
	_ = a.store.SaveConfig(a.config)
}

func (a *App) ServiceShutdown() {
	_ = a.applyRESTServerShutdown()
}

func (a *App) Bootstrap() model.BootstrapPayload {
	localPath := defaultLocalPath()
	visibleTabs := visibleTabs(a.tabs)

	if len(visibleTabs) > 0 {
		localPath = visibleTabs[0].LocalPath
	}

	return model.BootstrapPayload{
		Sites:            enrichSites(a.sites),
		Tabs:             visibleTabs,
		Config:           a.config,
		DefaultLocalPath: localPath,
		LocalFiles:       []model.FileEntry{},
		RemoteFiles:      []model.FileEntry{},
		Transfers:        a.sessionManager.SampleTransfers(),
		Logs:             a.sessionManager.SampleLogs(),
	}
}

func (a *App) GetSites() []model.Site {
	return enrichSites(a.sites)
}

func (a *App) GetConfig() model.Config {
	return a.config
}

func (a *App) GetPurchaseStatus() model.PurchaseStatus {
	status, err := a.purchaseService.CurrentState(a.purchaseContext())
	return a.mergePurchaseStatus(status, err)
}

func (a *App) RefreshPurchaseStatus() (model.PurchaseStatus, error) {
	status, err := a.purchaseService.Refresh(a.purchaseContext())
	merged := a.mergePurchaseStatus(status, err)
	if err == nil {
		if saveErr := a.applyPurchaseState(status); saveErr != nil {
			return merged, saveErr
		}
	}
	return merged, err
}

func (a *App) PurchaseProUnlock() (model.PurchaseStatus, error) {
	status, err := a.purchaseService.PurchaseProUnlock(a.purchaseContext())
	merged := a.mergePurchaseStatus(status, err)
	if err == nil {
		if saveErr := a.applyPurchaseState(status); saveErr != nil {
			return merged, saveErr
		}
	}
	return merged, err
}

func (a *App) RestorePurchases() (model.PurchaseStatus, error) {
	status, err := a.purchaseService.RestorePurchases(a.purchaseContext())
	merged := a.mergePurchaseStatus(status, err)
	if err == nil {
		if saveErr := a.applyPurchaseState(status); saveErr != nil {
			return merged, saveErr
		}
	}
	return merged, err
}
