package app

import (
	"net/http"
	"strings"

"github.com/VaderChen/Integrate-Terminal/internal/model"
)

func (a *App) handleRESTSites(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, siteEnvelope{Sites: a.GetSites()})
	case http.MethodPost:
		var site model.Site
		if err := decodeJSON(r, &site); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		sites, err := a.SaveSite(site)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, siteEnvelope{Sites: sites})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *App) handleRESTSiteByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/sites/")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "site id is required")
		return
	}
	sites, err := a.DeleteSite(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, siteEnvelope{Sites: sites})
}

func (a *App) handleRESTSitesReorder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		SiteIDs []string `json:"siteIDs"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sites, err := a.ReorderSites(payload.SiteIDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, siteEnvelope{Sites: sites})
}
