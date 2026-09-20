package app

import (
	"IntegTERM/internal/model"
	"IntegTERM/internal/purchase"
	"IntegTERM/internal/session"
	"IntegTERM/internal/store"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

func regressionApp(t *testing.T, shared *store.Store) *App {
	t.Helper()
	if shared == nil {
		shared = newAppTestStore(t.TempDir())
	}
	cfg, err := shared.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	return &App{store: shared, config: cfg, sessionManager: session.NewManager(), restServerToken: "test", lastActivity: make(map[string]time.Time)}
}
func regressionSite(id string) model.Site {
	return model.Site{ID: id, Name: id, Protocol: "sftp", Host: "example.invalid", Port: 22, Username: "test", Password: "fake-only", LocalPath: "/tmp", RemotePath: "/"}
}
func authorizedRequest(method, url string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, url, body)
	r.Header.Set("Authorization", "Bearer test")
	return r
}

func TestSiteWritesMergeAcrossGUIAndREST(t *testing.T) {
	shared := newAppTestStore(t.TempDir())
	ui := regressionApp(t, shared)
	service := regressionApp(t, shared)
	original := regressionSite("original")
	if _, err := ui.SaveSite(original); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(regressionSite("api-added"))
	reply := httptest.NewRecorder()
	service.restMux().ServeHTTP(reply, authorizedRequest(http.MethodPost, "/api/sites", bytes.NewReader(payload)))
	if reply.Code != http.StatusOK {
		t.Fatal(reply.Body.String())
	}
	original.Name = "edited in GUI"
	if _, err := ui.SaveSite(original); err != nil {
		t.Fatal(err)
	}
	saved, err := shared.LoadSites()
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 2 {
		t.Fatalf("API site lost: %#v", saved)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			instance := ui
			if i%2 == 0 {
				instance = service
			}
			if _, err := instance.SaveSite(regressionSite(fmt.Sprintf("concurrent-%d", i))); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	saved, err = shared.LoadSites()
	if err != nil || len(saved) != 14 {
		t.Fatalf("lost concurrent sites: len=%d err=%v", len(saved), err)
	}
}

func TestSettingsDoNotReviveDeletedSiteFolders(t *testing.T) {
	shared := newAppTestStore(t.TempDir())
	ui := regressionApp(t, shared)
	service := regressionApp(t, shared)
	site := regressionSite("original")
	site.Folder = "old-folder"
	if _, err := ui.SaveSite(site); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RenameSiteFolder("old-folder", "new-folder"); err != nil {
		t.Fatal(err)
	}
	cfg := ui.GetConfig()
	cfg.Theme = "changed"
	if _, err := ui.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	saved, err := shared.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.SiteFolders) != 1 || saved.SiteFolders[0] != "new-folder" {
		t.Fatalf("stale folder revived: %v", saved.SiteFolders)
	}
}

func TestLegacyMigrationWithExistingTokenDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	legacy := newAppTestStore("data")
	if err := legacy.SaveSites([]model.Site{regressionSite("legacy")}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "profile")
	if token := loadOrCreateRESTToken(target); token == "" {
		t.Fatal("missing token")
	}
	migrateLegacyDataDir(target)
	records, err := newAppTestStore(target).LoadSites()
	if err != nil || len(records) != 1 || records[0].ID != "legacy" {
		t.Fatalf("legacy data not migrated: %v %v", records, err)
	}
	if err := newAppTestStore(target).SaveSites([]model.Site{regressionSite("newer")}); err != nil {
		t.Fatal(err)
	}
	migrateLegacyDataDir(target)
	records, _ = newAppTestStore(target).LoadSites()
	if records[0].ID != "newer" {
		t.Fatal("migration overwrote current data")
	}
}

func TestConcurrentTokenCreationReturnsOnePrivateToken(t *testing.T) {
	dir := t.TempDir()
	tokens := make(chan string, 12)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); tokens <- loadOrCreateRESTToken(dir) }()
	}
	wg.Wait()
	close(tokens)
	first := ""
	for token := range tokens {
		if len(token) != 64 {
			t.Fatalf("invalid token length: %d", len(token))
		}
		if first == "" {
			first = token
		}
		if token != first {
			t.Fatal("processes received different tokens")
		}
	}
}

type fixedPurchaseProvider struct {
	state purchase.State
	err   error
}

func (p *fixedPurchaseProvider) CurrentState(context.Context) (purchase.State, error) {
	return p.state, p.err
}
func (p *fixedPurchaseProvider) Refresh(context.Context) (purchase.State, error) {
	return p.state, p.err
}
func (p *fixedPurchaseProvider) PurchaseProUnlock(context.Context) (purchase.State, error) {
	return p.state, p.err
}
func (p *fixedPurchaseProvider) RestorePurchases(context.Context) (purchase.State, error) {
	return p.state, p.err
}

func TestVerifiedEntitlementControlsBothStatusAndEnforcement(t *testing.T) {
	a := regressionApp(t, nil)
	a.tabs = []model.Tab{{ID: "1"}, {ID: "2"}}
	provider := &fixedPurchaseProvider{state: purchase.State{ProUnlock: true, PlanName: "Pro"}}
	a.purchaseService = provider
	status := a.GetPurchaseStatus()
	if !status.ProUnlock || status.MaxTabs != 0 || a.ensureTabCreationAllowed() != nil {
		t.Fatalf("valid entitlement not enforced: %#v", status)
	}
	cfg := a.GetConfig()
	cfg.ProUnlock = false
	if _, err := a.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if !a.GetPurchaseStatus().ProUnlock {
		t.Fatal("preferences revoked verified entitlement")
	}
	provider.state = purchase.State{ProUnlock: false, PlanName: "Free"}
	status, err := a.RefreshPurchaseStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.ProUnlock || status.MaxTabs != 2 || a.ensureTabCreationAllowed() == nil {
		t.Fatalf("revoked entitlement still active: %#v", status)
	}
	cfg = a.GetConfig()
	cfg.ProUnlock = true
	if _, err := a.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if a.ensureTabCreationAllowed() == nil {
		t.Fatal("preferences granted unverified Pro")
	}
	if _, err := a.SetProUnlock(true); err == nil {
		t.Fatal("public setter granted Pro")
	}
	saved, _ := a.store.LoadConfig()
	if saved.ProUnlock {
		t.Fatal("editable config persisted entitlement")
	}
}

func TestRESTAndTrayConcurrentStateAccess(t *testing.T) {
	a := regressionApp(t, nil)
	for i := 0; i < 50; i++ {
		a.tabs = append(a.tabs, model.Tab{ID: fmt.Sprintf("tab-%d", i), Mode: "file", Hidden: true, Connected: true})
	}
	handler := a.restMux()
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				switch worker {
				case 0:
					handler.ServeHTTP(httptest.NewRecorder(), authorizedRequest(http.MethodGet, "/api/files/remote?tabId=missing", nil))
				case 1:
					handler.ServeHTTP(httptest.NewRecorder(), authorizedRequest(http.MethodDelete, fmt.Sprintf("/api/tabs/tab-%d", i%50), nil))
				case 2:
					a.CloseIdleHiddenConnections(time.Nanosecond)
					a.ConnectionCounts()
				case 3:
					a.GetTabs()
					a.GetRESTServerStatus()
					a.GetConfig()
				}
			}
		}(worker)
	}
	wg.Wait()
}

func TestRESTConfigDisablesServerWithoutWaitingOnOwnHandler(t *testing.T) {
	a := regressionApp(t, nil)
	server := httptest.NewServer(a.restMux())
	defer server.Close()
	defer a.applyRESTServerShutdown()
	_, portText, _ := net.SplitHostPort(server.Listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	a.config.RESTServerEnabled = true
	a.config.RESTServerPort = port
	a.restServer = server.Config
	a.restServerURL = server.URL
	if err := a.store.SaveConfig(a.config); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		client := &http.Client{Timeout: time.Second}
		for {
			select {
			case <-stop:
				return
			default:
			}
			for _, endpoint := range []string{"/api/status", "/api/tabs"} {
				req, _ := http.NewRequest(http.MethodGet, server.URL+endpoint, nil)
				req.Header.Set("Authorization", "Bearer test")
				if resp, err := client.Do(req); err == nil {
					_, _ = io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
			}
		}
	}()
	defer func() { close(stop); <-done }()
	next := cloneConfig(a.config)
	next.RESTServerEnabled = false
	payload, _ := json.Marshal(configEnvelope{Config: next})
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/api/config", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer test")
	started := time.Now()
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	deadline := time.Now().Add(time.Second)
	for a.GetRESTServerStatus().Running && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	a.runtimeConfigMu.Lock()
	a.runtimeConfigMu.Unlock()
	if a.GetRESTServerStatus().Running || time.Since(started) > 2*time.Second {
		t.Fatal("server shutdown deadlocked")
	}
	saved, _ := a.store.LoadConfig()
	if saved.RESTServerEnabled {
		t.Fatal("disable unexpectedly rolled back")
	}
}

func TestRESTListRemoteReportsMissingConnection(t *testing.T) {
	a := regressionApp(t, nil)
	reply := httptest.NewRecorder()
	a.restMux().ServeHTTP(reply, authorizedRequest(http.MethodGet, "/api/files/remote?tabId=missing&path=/", nil))
	if reply.Code == 200 {
		t.Fatalf("missing connection disguised as empty folder: %s", reply.Body.String())
	}
	if _, err := a.StatRemoteEntry("missing", "/file"); err == nil {
		t.Fatal("stat swallowed missing connection")
	}
}

func TestDeleteGuardsProtectCurrentAncestorsAndRoot(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.Mkdir(current, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(current, "keep")
	if err := os.WriteFile(sentinel, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	a := regressionApp(t, nil)
	a.tabs = []model.Tab{{ID: "tab", LocalPath: current, RemotePath: "/home/test/current"}}
	for _, target := range []string{current, root, filepath.VolumeName(root) + string(os.PathSeparator), ".", ".."} {
		if err := a.DeleteEntry("tab", "local", target); err == nil {
			t.Errorf("allowed protected local deletion: %s", target)
		}
	}
	for _, target := range []string{"/", ".", "..", "/home", "/home/test", "/home/test/current"} {
		if err := a.validateDeleteTarget("tab", "remote", target); err == nil {
			t.Errorf("allowed protected remote deletion: %s", target)
		}
	}
	if err := a.DeleteEntries("tab", "local", []string{sentinel, current}); err == nil {
		t.Fatal("allowed mixed unsafe batch")
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatal("unsafe batch partially deleted contents")
	}
	if err := a.DeleteEntry("tab", "local", sentinel); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("ordinary child was not deleted")
	}
}

type delayedPurchaseProvider struct {
	entered         chan struct{}
	release         chan struct{}
	purchaseEntered chan struct{}
}

func (p *delayedPurchaseProvider) CurrentState(context.Context) (purchase.State, error) {
	close(p.entered)
	<-p.release
	return purchase.State{ProUnlock: false}, nil
}
func (p *delayedPurchaseProvider) Refresh(ctx context.Context) (purchase.State, error) {
	return p.CurrentState(ctx)
}
func (p *delayedPurchaseProvider) PurchaseProUnlock(context.Context) (purchase.State, error) {
	close(p.purchaseEntered)
	return purchase.State{ProUnlock: true}, nil
}
func (p *delayedPurchaseProvider) RestorePurchases(ctx context.Context) (purchase.State, error) {
	return p.PurchaseProUnlock(ctx)
}

func TestOlderEntitlementQueryCannotOverrideNewerPurchase(t *testing.T) {
	a := regressionApp(t, nil)
	provider := &delayedPurchaseProvider{entered: make(chan struct{}), release: make(chan struct{}), purchaseEntered: make(chan struct{})}
	a.purchaseService = provider
	queryDone := make(chan struct{})
	go func() { a.GetPurchaseStatus(); close(queryDone) }()
	<-provider.entered
	purchaseDone := make(chan model.PurchaseStatus, 1)
	go func() {
		status, err := a.PurchaseProUnlock()
		if err != nil {
			t.Error(err)
		}
		purchaseDone <- status
	}()
	select {
	case <-provider.purchaseEntered:
		t.Fatal("purchase raced an older entitlement query")
	case <-time.After(10 * time.Millisecond):
	}
	close(provider.release)
	<-queryDone
	status := <-purchaseDone
	if !status.ProUnlock || status.MaxTabs != 0 || !a.GetConfig().ProUnlock {
		t.Fatalf("older query overwrote purchase: %#v", status)
	}
}
