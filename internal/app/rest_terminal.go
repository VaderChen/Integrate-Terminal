package app

import (
	"net/http"
	"strings"

"github.com/VaderChen/Integrate-Terminal/internal/model"
)

func (a *App) handleRESTSSHExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		Site           model.Site `json:"site"`
		Command        string     `json:"command"`
		TimeoutSeconds int        `json:"timeoutSeconds"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := a.ExecuteSSHCommand(payload.Site, payload.Command, payload.TimeoutSeconds)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sshExecuteEnvelope{
		OK:       result["ok"].(bool),
		Stdout:   result["stdout"].(string),
		Stderr:   result["stderr"].(string),
		ExitCode: result["exitCode"].(int),
	})
}

func (a *App) handleRESTTerminalOutput(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	sessionID := r.URL.Query().Get("sessionId")
	if strings.TrimSpace(sessionID) == "" {
		writeError(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sessionId": sessionID,
		"output":    a.GetSSHOutputBuffer(sessionID),
	})
}

func (a *App) handleRESTTerminalInput(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		SessionID string `json:"sessionId"`
		Data      string `json:"data"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.WriteSSHInput(payload.SessionID, payload.Data); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, terminalActionEnvelope{OK: true})
}

func (a *App) handleRESTTerminalResize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		SessionID string `json:"sessionId"`
		Cols      uint16 `json:"cols"`
		Rows      uint16 `json:"rows"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.ResizeSSHSession(payload.SessionID, payload.Cols, payload.Rows); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, terminalActionEnvelope{OK: true})
}

func (a *App) handleRESTTerminalClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		SessionID string `json:"sessionId"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.CloseSSHSession(payload.SessionID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, terminalActionEnvelope{OK: true})
}
