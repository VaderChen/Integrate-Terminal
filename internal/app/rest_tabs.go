package app

import (
	"net/http"
	"strings"

"github.com/VaderChen/Integrate-Terminal/internal/model"
)

func (a *App) handleRESTTabs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, tabEnvelope{Tabs: a.GetTabs()})
}

func (a *App) handleRESTCreateFileTab(w http.ResponseWriter, r *http.Request) {
	a.handleRESTTabCreateWithSite(w, r, func(site model.Site) ([]model.Tab, error) {
		tabs, err := a.CreateTab(site)
		if err != nil {
			return tabs, err
		}
		return a.hideLatestTab(), nil
	})
}

func (a *App) handleRESTCreateSSHTab(w http.ResponseWriter, r *http.Request) {
	a.handleRESTTabCreateWithSite(w, r, func(site model.Site) ([]model.Tab, error) {
		tabs, err := a.CreateSSHTab(site)
		if err != nil {
			return tabs, err
		}
		return a.hideLatestTab(), nil
	})
}

func (a *App) handleRESTCreateTelnetTab(w http.ResponseWriter, r *http.Request) {
	a.handleRESTTabCreateWithSite(w, r, func(site model.Site) ([]model.Tab, error) {
		tabs, err := a.CreateTelnetTab(site)
		if err != nil {
			return tabs, err
		}
		return a.hideLatestTab(), nil
	})
}

func (a *App) handleRESTTabCreateWithSite(w http.ResponseWriter, r *http.Request, create func(model.Site) ([]model.Tab, error)) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		Site model.Site `json:"site"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tabs, err := create(payload.Site)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tabEnvelope{Tabs: tabs, SessionID: latestSessionID(tabs)})
}

func (a *App) handleRESTCreateLocalTab(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		Cwd string `json:"cwd"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err := a.CreateLocalTerminalTab(payload.Cwd)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tabs := a.hideLatestTab()
	writeJSON(w, http.StatusOK, tabEnvelope{Tabs: tabs, SessionID: latestSessionID(tabs)})
}

func (a *App) hideLatestTab() []model.Tab {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	if len(a.tabs) == 0 {
		return cloneTabs(a.tabs)
	}
	a.tabs[len(a.tabs)-1].Hidden = true
	_ = a.persistTabsLocked(a.tabs)
	return cloneTabs(a.tabs)
}

func latestSessionID(tabs []model.Tab) string {
	if len(tabs) == 0 {
		return ""
	}
	return tabs[len(tabs)-1].SessionID
}

func (a *App) handleRESTTabByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/tabs/")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "tab id is required")
		return
	}
	tabs, err := a.CloseTab(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tabEnvelope{Tabs: tabs})
}
