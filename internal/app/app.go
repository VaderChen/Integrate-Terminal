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
	mcpMu             sync.Mutex
	mcpVFS            *mcpVFS
	ctx               context.Context
	store             *store.Store
	purchaseService   purchaseProvider
	verifiedProUnlock bool
	purchaseMu        sync.Mutex
	sessionManager    *session.Manager
	sites             []model.Site
	tabs              []model.Tab
	config            model.Config
	storageInitErr    error
	activityMu        sync.Mutex
	stateMu           sync.RWMutex
	lastActivity      map[string]time.Time
	restServerMu      sync.Mutex
	runtimeConfigMu   sync.Mutex
	operationMu       sync.RWMutex
	restServer        *http.Server
	restServerURL     string
	restServerToken   string
	restAttached      bool
	allowRESTAttach   bool
	operations        map[string]RESTOperation
}

func New() *App {
	dataDir := resolveAppDataDir()
	migrateLegacyDataDir(dataDir)
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
	a.stateMu.Lock()
	a.ctx = ctx
	a.stateMu.Unlock()
	a.sessionManager.SetEventContext(ctx)
	a.initialize(true)
}

func (a *App) ServiceStartup() {
	a.stateMu.Lock()
	a.ctx = nil
	a.stateMu.Unlock()
	a.initialize(false)
}

func (a *App) initialize(allowRESTAttach bool) {
	a.stateMu.Lock()
	defer func() { a.stateMu.Unlock(); _, _ = a.ReloadRuntimeConfig() }()
	a.allowRESTAttach = allowRESTAttach
	if a.purchaseService != nil {
		go a.GetPurchaseStatus()
	}
	migrateLegacyDataDir(a.store.BaseDir())
	_ = a.store.Ensure()

	a.storageInitErr = a.loadInitialStateLocked()
}

func (a *App) DomReady(ctx context.Context) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.ctx = ctx
	a.sessionManager.SetEventContext(ctx)
	a.applyInitialWindowPlacement()
}

func (a *App) Shutdown(ctx context.Context) {
	_ = a.applyRESTServerShutdown()
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	// A locked vault or unreadable file must never turn saved data into an empty workspace.
	if a.storageInitErr != nil {
		return
	}
	a.captureWindowState()
	_ = a.persistTabs()
	_, _ = a.store.UpdateConfig(func(cfg *model.Config) error {
		cfg.LastActiveTab = a.config.LastActiveTab
		if cfg.RememberWindowPosition {
			cfg.WindowWidth = a.config.WindowWidth
			cfg.WindowHeight = a.config.WindowHeight
			cfg.WindowX = a.config.WindowX
			cfg.WindowY = a.config.WindowY
		}
		return nil
	})
}

func (a *App) ServiceShutdown() {
	_ = a.applyRESTServerShutdown()
}

func (a *App) bootstrapLocked() model.BootstrapPayload {
	var storageErr error
	if a.storageInitErr != nil {
		storageErr = a.retryInitialStorageLocked()
	} else if a.store != nil {
		storageErr = a.reloadSitesFromStoreLocked()
	}
	localPath := defaultLocalPath()
	visibleTabs := visibleTabs(a.tabs)

	if len(visibleTabs) > 0 {
		localPath = visibleTabs[0].LocalPath
	}

	return model.BootstrapPayload{
		StorageError:     storageErrorText(storageErr),
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

func (a *App) getSitesLocked() ([]model.Site, error) {
	if err := a.retryInitialStorageLocked(); err != nil {
		return nil, err
	}
	if a.store != nil {
		if err := a.reloadSitesFromStoreLocked(); err != nil {
			return nil, err
		}
	}
	return enrichSites(a.sites), nil
}

func (a *App) getConfigLocked() model.Config {
	return a.config
}

type purchaseProvider interface {
	CurrentState(context.Context) (purchase.State, error)
	Refresh(context.Context) (purchase.State, error)
	PurchaseProUnlock(context.Context) (purchase.State, error)
	RestorePurchases(context.Context) (purchase.State, error)
}

func (a *App) finishPurchaseQuery(state purchase.State, err error) model.PurchaseStatus {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	if err == nil {
		_ = a.applyPurchaseState(state)
	}
	return a.mergePurchaseStatus(state, err)
}

func (a *App) GetPurchaseStatus() model.PurchaseStatus {
	a.purchaseMu.Lock()
	defer a.purchaseMu.Unlock()
	if a.purchaseService == nil {
		return a.finishPurchaseQuery(purchase.State{}, nil)
	}
	status, err := a.purchaseService.CurrentState(a.purchaseContext())
	return a.finishPurchaseQuery(status, err)
}

func (a *App) RefreshPurchaseStatus() (model.PurchaseStatus, error) {
	a.purchaseMu.Lock()
	defer a.purchaseMu.Unlock()
	status, err := a.purchaseService.Refresh(a.purchaseContext())
	return a.finishPurchaseQuery(status, err), err
}
func (a *App) PurchaseProUnlock() (model.PurchaseStatus, error) {
	a.purchaseMu.Lock()
	defer a.purchaseMu.Unlock()
	status, err := a.purchaseService.PurchaseProUnlock(a.purchaseContext())
	return a.finishPurchaseQuery(status, err), err
}
func (a *App) RestorePurchases() (model.PurchaseStatus, error) {
	a.purchaseMu.Lock()
	defer a.purchaseMu.Unlock()
	status, err := a.purchaseService.RestorePurchases(a.purchaseContext())
	return a.finishPurchaseQuery(status, err), err
}
