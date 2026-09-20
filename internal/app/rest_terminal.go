package app

import (
	"net/http"
	"strings"
	"time"

	"IntegTERM/internal/model"
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
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(sshCommandTimeout(payload.TimeoutSeconds) + 5*time.Second))
	result, err := a.executeSSHCommand(r.Context(), payload.Site, payload.Command, payload.TimeoutSeconds)
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
	snapshot := a.GetTerminalOutputSnapshot(sessionID)
	writeJSON(w, http.StatusOK, map[string]any{
		"sessionId": sessionID,
		"output":    snapshot.Output,
		"sequence":  snapshot.Sequence,
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
