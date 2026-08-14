package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/session"
	"github.com/VaderChen/Integrate-Terminal/internal/store"
)

func TestRESTSecurityUsesAllowlist(t *testing.T) {
	instance := &App{config: model.Config{RESTServerAllowlist: []string{"127.0.0.1"}}}
	handler := instance.withRESTSecurity(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	blockedRequest := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	blockedRequest.RemoteAddr = "192.0.2.10:54321"
	blocked := httptest.NewRecorder()
	handler.ServeHTTP(blocked, blockedRequest)
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", blocked.Code)
	}

	allowedRequest := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	allowedRequest.RemoteAddr = "127.0.0.1:54321"
	allowed := httptest.NewRecorder()
	handler.ServeHTTP(allowed, allowedRequest)
	if allowed.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", allowed.Code)
	}
}

func TestServiceShutdownDoesNotOverwriteSitesSavedByUI(t *testing.T) {
	sharedStore := store.New(t.TempDir())
	staleSites := []model.Site{{ID: "old", Name: "Old", Host: "old.example.com"}}
	latestSites := []model.Site{{ID: "new", Name: "New", Host: "new.example.com"}}
	if err := sharedStore.SaveSites(staleSites); err != nil {
		t.Fatalf("save stale sites: %v", err)
	}

	serviceApp := &App{store: sharedStore, sites: staleSites}
	if err := sharedStore.SaveSites(latestSites); err != nil {
		t.Fatalf("save latest sites: %v", err)
	}
	serviceApp.ServiceShutdown()

	loaded, err := sharedStore.LoadSites()
	if err != nil {
		t.Fatalf("load sites: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != "new" {
		t.Fatalf("tray shutdown overwrote latest sites: %#v", loaded)
	}
}

func TestReloadSitesFromStoreUsesLatestUIState(t *testing.T) {
	sharedStore := store.New(t.TempDir())
	latestSites := []model.Site{{ID: "new", Name: "New", Host: "new.example.com"}}
	if err := sharedStore.SaveSites(latestSites); err != nil {
		t.Fatalf("save latest sites: %v", err)
	}

	serviceApp := &App{
		store: sharedStore,
		sites: []model.Site{{ID: "old", Name: "Old", Host: "old.example.com"}},
	}
	if err := serviceApp.reloadSitesFromStoreLocked(); err != nil {
		t.Fatalf("reload sites: %v", err)
	}
	if len(serviceApp.sites) != 1 || serviceApp.sites[0].ID != "new" {
		t.Fatalf("service did not reload latest sites: %#v", serviceApp.sites)
	}
}

func TestRESTOperationRunsAsynchronously(t *testing.T) {
	instance := &App{operations: make(map[string]RESTOperation)}
	operation := instance.createRESTOperation("upload")
	release := make(chan struct{})
	go instance.runRESTOperation(operation, func() error {
		<-release
		return nil
	})

	waitForOperationStatus(t, instance, operation.ID, "running")
	close(release)
	waitForOperationStatus(t, instance, operation.ID, "done")
}

func TestRESTUploadReturnsAcceptedOperation(t *testing.T) {
	instance := &App{
		operations:     make(map[string]RESTOperation),
		sessionManager: session.NewManager(),
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/files/upload",
		strings.NewReader(`{"tabId":"missing","localPaths":["/tmp/large.bin"],"remoteBase":"/tmp"}`),
	)
	response := httptest.NewRecorder()

	instance.handleRESTUploadPaths(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", response.Code, response.Body.String())
	}
	if location := response.Header().Get("Location"); !strings.HasPrefix(location, "/api/operations/") {
		t.Fatalf("missing operation location: %q", location)
	}
}

func waitForOperationStatus(t *testing.T, instance *App, id string, expected string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		operation, ok := instance.getRESTOperation(id)
		if ok && operation.Status == expected {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	operation, _ := instance.getRESTOperation(id)
	t.Fatalf("expected status %q, got %#v", expected, operation)
}

func TestRESTSecurityRejectsForeignOrigin(t *testing.T) {
	instance := &App{config: model.Config{RESTServerAllowlist: []string{"127.0.0.1"}}}
	handler := instance.withRESTSecurity(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	request.RemoteAddr = "127.0.0.1:54321"
	request.Header.Set("Origin", "https://example.com")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
}
