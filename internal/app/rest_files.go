package app

import (
	"net/http"
	"path"

"github.com/VaderChen/Integrate-Terminal/internal/model"
)

func (a *App) handleRESTListLocal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	tabID := r.URL.Query().Get("tabId")
	targetPath := r.URL.Query().Get("path")
	entries := a.ListLocal(tabID, targetPath)
	writeJSON(w, http.StatusOK, fileEnvelope{Entries: entries})
}

func (a *App) handleRESTListRemote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	tabID := r.URL.Query().Get("tabId")
	targetPath := r.URL.Query().Get("path")
	entries, err := a.listRemoteWithError(tabID, targetPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, fileEnvelope{Entries: entries})
}

func (a *App) handleRESTUploadPaths(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		TabID      string      `json:"tabId"`
		LocalPaths []string    `json:"localPaths"`
		RemoteBase string      `json:"remoteBase"`
		Site       *model.Site `json:"site,omitempty"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	operation := a.createRESTOperation("upload")
	go a.runRESTOperation(operation, func() error {
		if payload.Site != nil {
			return a.UploadDroppedPathsToSite(*payload.Site, payload.LocalPaths, payload.RemoteBase)
		}
		return a.UploadDroppedPaths(payload.TabID, payload.LocalPaths, payload.RemoteBase)
	})
	w.Header().Set("Location", "/api/operations/"+operation.ID)
	w.Header().Set("Retry-After", "1")
	writeJSON(w, http.StatusAccepted, operationEnvelope{Operation: operation})
}

func (a *App) handleRESTDownloadPaths(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		TabID       string   `json:"tabId"`
		RemotePaths []string `json:"remotePaths"`
		LocalBase   string   `json:"localBase"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	operation := a.createRESTOperation("download")
	go a.runRESTOperation(operation, func() error {
		return a.DownloadDroppedPaths(payload.TabID, payload.RemotePaths, payload.LocalBase)
	})
	w.Header().Set("Location", "/api/operations/"+operation.ID)
	w.Header().Set("Retry-After", "1")
	writeJSON(w, http.StatusAccepted, operationEnvelope{Operation: operation})
}

func (a *App) handleRESTSFTPStat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	tabID := r.URL.Query().Get("tabId")
	targetPath := r.URL.Query().Get("path")
	entry, err := a.StatRemoteEntry(tabID, targetPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (a *App) handleRESTSFTPMkdir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		TabID string `json:"tabId"`
		Path  string `json:"path"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.CreateDirectory(payload.TabID, "remote", path.Dir(payload.Path), path.Base(payload.Path)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, terminalActionEnvelope{OK: true})
}

func (a *App) handleRESTSFTPRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		TabID   string `json:"tabId"`
		OldPath string `json:"oldPath"`
		NewPath string `json:"newPath"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.sessionManager.RenameRemotePath(payload.TabID, payload.OldPath, payload.NewPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, terminalActionEnvelope{OK: true})
}

func (a *App) handleRESTSFTPDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		TabID string `json:"tabId"`
		Path  string `json:"path"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.sessionManager.DeleteRemotePath(payload.TabID, payload.Path); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, terminalActionEnvelope{OK: true})
}
