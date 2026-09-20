package app

import (
	"fmt"
	"net/http"
)

func (a *App) handleRESTTransfers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, transferEnvelope{Transfers: a.GetTransfers()})
}

func (a *App) handleRESTLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, logEnvelope{Logs: a.GetLogs()})
}

func (a *App) handleRESTConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, configEnvelope{Config: a.GetConfig()})
	case http.MethodPut:
		var payload configEnvelope
		if err := decodeJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		a.stateMu.Lock()
		config, err := a.saveConfigChangesLocked(payload.Config)
		a.stateMu.Unlock()
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, configEnvelope{Config: config})
		// Shutdown must run outside the handler it is waiting to drain.
		go func() {
			if _, err := a.ReloadRuntimeConfig(); err != nil {
				a.sessionManager.AppendLog("更新 REST 設定失敗: "+err.Error(), "failed")
			}
		}()
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *App) getRESTServerPortLocked() int {
	return sanitizeRESTServerPort(a.config.RESTServerPort)
}

func (a *App) getRESTServerBaseURLLocked() string {
	status := a.getRESTServerStatusLocked()
	if status.BaseURL != "" {
		return status.BaseURL
	}
	return fmt.Sprintf("http://127.0.0.1:%d", sanitizeRESTServerPort(a.config.RESTServerPort))
}
