package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"IntegTERM/internal/model"
)

func TestUnreadableSitesPreserveSavedWorkspaceAndCanRetry(t *testing.T) {
	s := newAppTestStore(t.TempDir())
	site := regressionSite("saved")
	if err := s.SaveSites([]model.Site{site}); err != nil {
		t.Fatal(err)
	}
	tab := model.Tab{ID: "saved-tab", SiteID: site.ID, Mode: "file", Password: "tab-secret", LocalPath: "/tmp", RemotePath: "/"}
	if err := s.SaveTabs([]model.Tab{tab}); err != nil {
		t.Fatal(err)
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.RestoreTabsOnStart = true
	cfg.LastActiveTab = tab.ID
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	sitesPath := filepath.Join(s.BaseDir(), "sites.json")
	validSites, err := os.ReadFile(sitesPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sitesPath, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	before := make(map[string][]byte)
	for _, name := range []string{"sites.json", "tabs.json", "config.json"} {
		before[name], err = os.ReadFile(filepath.Join(s.BaseDir(), name))
		if err != nil {
			t.Fatal(err)
		}
	}
	a := regressionApp(t, s)
	a.allowRESTAttach = true
	a.storageInitErr = a.loadInitialStateLocked()
	if a.storageInitErr == nil {
		t.Fatal("invalid sites file reported a successful startup")
	}
	if payload := a.Bootstrap(); payload.StorageError == "" {
		t.Fatal("missing recoverable storage error")
	}
	if _, err := a.SaveConfig(cfg); err == nil {
		t.Fatal("preferences were saved with unread initial data")
	}
	if _, err := a.CreateLocalTerminalTab(t.TempDir()); err == nil {
		t.Fatal("new tab bypassed failed restore guard")
	}
	if _, err := a.GetSites(); err == nil {
		t.Fatal("invalid sites file returned a successful site list")
	}
	for _, endpoint := range []string{"/api/sites", "/api/tabs"} {
		response := httptest.NewRecorder()
		a.restMux().ServeHTTP(response, authorizedRequest(http.MethodGet, endpoint, nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s status=%d", endpoint, response.Code)
		}
	}
	a.Shutdown(context.Background())
	for name, data := range before {
		after, err := os.ReadFile(filepath.Join(s.BaseDir(), name))
		if err != nil || !bytes.Equal(data, after) {
			t.Fatalf("failed startup changed %s: %v", name, err)
		}
	}
	if err := os.WriteFile(sitesPath, validSites, 0600); err != nil {
		t.Fatal(err)
	}
	payload := a.Bootstrap()
	if payload.StorageError != "" || len(payload.Sites) != 1 || len(payload.Tabs) != 1 {
		t.Fatalf("retry did not recover saved workspace: error=%s sites=%d tabs=%d", payload.StorageError, len(payload.Sites), len(payload.Tabs))
	}
	if payload.Tabs[0].Password != tab.Password || payload.Sites[0].Password != site.Password || payload.Config.LastActiveTab != tab.ID {
		t.Fatal("recovered workspace lost saved credentials or selection")
	}
}

func TestUnreadableConfigDoesNotEraseSavedTabsOnShutdown(t *testing.T) {
	s := newAppTestStore(t.TempDir())
	if err := s.Ensure(); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(s.BaseDir(), "config.json")
	tabsPath := filepath.Join(s.BaseDir(), "tabs.json")
	if err := os.WriteFile(configPath, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	original := []byte(`[{"id":"preserved","mode":"file"}]`)
	if err := os.WriteFile(tabsPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	a := &App{store: s, allowRESTAttach: true}
	a.storageInitErr = a.loadInitialStateLocked()
	if a.storageInitErr == nil {
		t.Fatal("invalid config reported successful startup")
	}
	a.Shutdown(context.Background())
	after, err := os.ReadFile(tabsPath)
	if err != nil || !bytes.Equal(original, after) {
		t.Fatalf("saved tabs overwritten: %v", err)
	}
}

func TestUnreadableTabsProtectWorkspaceAndRESTReadsCanRecover(t *testing.T) {
	s := newAppTestStore(t.TempDir())
	tab := model.Tab{ID: "saved-tab", Mode: "file", Password: "tab-only-secret"}
	if err := s.SaveTabs([]model.Tab{tab}); err != nil {
		t.Fatal(err)
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.RestoreTabsOnStart = true
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	tabsPath := filepath.Join(s.BaseDir(), "tabs.json")
	validTabs, err := os.ReadFile(tabsPath)
	if err != nil {
		t.Fatal(err)
	}
	before := []byte("{broken")
	if err := os.WriteFile(tabsPath, before, 0600); err != nil {
		t.Fatal(err)
	}
	a := regressionApp(t, s)
	a.allowRESTAttach = true
	a.storageInitErr = a.loadInitialStateLocked()
	if a.storageInitErr == nil {
		t.Fatal("invalid tabs file did not fail startup")
	}
	if _, err := a.CreateSiteFolder("unread-workspace"); err == nil {
		t.Fatal("folder mutation bypassed failed startup guard")
	}
	if _, err := a.SaveSite(model.Site{ID: "new", Name: "Anonymous", Protocol: "ftp", Host: "127.0.0.1", Username: "anonymous", Port: 21, LocalPath: "/tmp", RemotePath: "/"}); err == nil {
		t.Fatal("site mutation bypassed failed startup guard")
	}
	if err := a.ResetWindowToDefaultScale(); err == nil {
		t.Fatal("window reset bypassed failed startup guard")
	}
	for _, endpoint := range []string{"/api/sites", "/api/tabs"} {
		response := httptest.NewRecorder()
		a.restMux().ServeHTTP(response, authorizedRequest(http.MethodGet, endpoint, nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s status=%d", endpoint, response.Code)
		}
	}
	a.Shutdown(context.Background())
	after, err := os.ReadFile(filepath.Join(s.BaseDir(), "tabs.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("unread saved tabs were overwritten: %v", err)
	}
	if err := os.WriteFile(tabsPath, validTabs, 0600); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	a.restMux().ServeHTTP(response, authorizedRequest(http.MethodGet, "/api/tabs", nil))
	if response.Code != http.StatusOK || a.storageInitErr != nil || len(a.tabs) != 1 || a.tabs[0].Password != tab.Password {
		t.Fatalf("read did not recover saved tabs: status=%d error=%v count=%d", response.Code, a.storageInitErr, len(a.tabs))
	}
}

func TestFileWorkspaceStartsAndSaves(t *testing.T) {
	s := newAppTestStore(t.TempDir())
	site := regressionSite("file-site")
	if err := s.SaveSites([]model.Site{site}); err != nil {
		t.Fatal(err)
	}
	tab := model.Tab{ID: "file-tab", SiteID: site.ID, Mode: "file", Password: "tab-secret"}
	if err := s.SaveTabs([]model.Tab{tab}); err != nil {
		t.Fatal(err)
	}
	a := regressionApp(t, s)
	a.storageInitErr = a.loadInitialStateLocked()
	payload := a.Bootstrap()
	if a.storageInitErr != nil || payload.StorageError != "" || len(payload.Sites) != 1 || len(payload.Tabs) != 1 {
		t.Fatal("檔案工作區啟動失敗")
	}
	if payload.Sites[0].Password != site.Password || payload.Tabs[0].Password != tab.Password {
		t.Fatal("檔案工作區遺失憑證")
	}
	site.Name = "重新命名"
	site.Password = "changed-test"
	if _, err := a.SaveSite(site); err != nil {
		t.Fatal(err)
	}
	restarted := regressionApp(t, newAppTestStore(s.BaseDir()))
	if err := restarted.loadInitialStateLocked(); err != nil {
		t.Fatal(err)
	}
	if len(restarted.sites) != 1 || restarted.sites[0].Password != site.Password || restarted.sites[0].Name != site.Name {
		t.Fatal("檔案變更未跨程序保存")
	}
}
