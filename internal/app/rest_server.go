package app

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"IntegTERM/internal/crashlog"
	"IntegTERM/internal/model"
)

const defaultRESTServerPort = 18080

const restTokenFilename = "rest-api.token"

type siteEnvelope struct {
	Sites []model.Site `json:"sites"`
}

type tabEnvelope struct {
	Tabs      []model.Tab `json:"tabs"`
	SessionID string      `json:"sessionId,omitempty"`
}

type fileEnvelope struct {
	Entries []model.FileEntry `json:"entries"`
}

type transferEnvelope struct {
	Transfers []model.TransferItem `json:"transfers"`
}

type logEnvelope struct {
	Logs []model.LogItem `json:"logs"`
}

type configEnvelope struct {
	Config model.Config `json:"config"`
}

type terminalActionEnvelope struct {
	OK bool `json:"ok"`
}

type sshExecuteEnvelope struct {
	OK       bool   `json:"ok"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

func sanitizeRESTServerPort(port int) int {
	if port <= 0 || port > 65535 {
		return defaultRESTServerPort
	}
	return port
}

func loadOrCreateRESTToken(baseDir string) string {
	tokenPath := filepath.Join(baseDir, restTokenFilename)
	if data, err := os.ReadFile(tokenPath); err == nil {
		if token := strings.TrimSpace(string(data)); token != "" {
			return token
		}
	}

	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	token := hex.EncodeToString(buffer)
	if err := os.MkdirAll(baseDir, 0o755); err == nil {
		_ = os.WriteFile(tokenPath, []byte(token+"\n"), 0o600)
	}
	return token
}

func (a *App) applyRESTServerConfig() error {
	a.restServerMu.Lock()
	defer a.restServerMu.Unlock()

	a.config.RESTServerPort = sanitizeRESTServerPort(a.config.RESTServerPort)
	if !a.config.RESTServerEnabled {
		return a.stopRESTServerLocked()
	}
	return a.startRESTServerLocked()
}

func (a *App) applyRESTServerShutdown() error {
	a.restServerMu.Lock()
	defer a.restServerMu.Unlock()
	return a.stopRESTServerLocked()
}

func (a *App) startRESTServerLocked() error {
	port := sanitizeRESTServerPort(a.config.RESTServerPort)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if a.restServer != nil {
		if a.restServerURL == baseURL {
			return nil
		}
		if err := a.stopRESTServerLocked(); err != nil {
			return err
		}
	}
	if a.allowRESTAttach {
		if attached := detectExistingRESTServer(baseURL); attached {
			a.restServer = nil
			a.restServerURL = baseURL
			a.restAttached = true
			return nil
		}
		a.restServer = nil
		a.restServerURL = ""
		a.restAttached = false
		return nil
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}

	server := &http.Server{
		Handler:      a.restMux(),
		ErrorLog:     log.New(mustOpenCrashLogWriter(), "rest-server: ", log.LstdFlags|log.Lshortfile),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	a.restServer = server
	a.restServerURL = baseURL
	a.restAttached = false

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.restServerMu.Lock()
			a.restServer = nil
			a.restServerURL = ""
			a.restAttached = false
			a.restServerMu.Unlock()
		}
	}()

	return nil
}

func (a *App) stopRESTServerLocked() error {
	if a.restAttached {
		a.restAttached = false
		a.restServerURL = ""
		a.restServer = nil
		return nil
	}
	if a.restServer == nil {
		a.restServerURL = ""
		a.restAttached = false
		return nil
	}
	server := a.restServer
	a.restServer = nil
	a.restServerURL = ""
	a.restAttached = false
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

func (a *App) GetRESTServerStatus() model.RESTServerStatus {
	a.restServerMu.Lock()
	defer a.restServerMu.Unlock()
	return model.RESTServerStatus{
		Enabled:  a.config.RESTServerEnabled,
		Running:  a.restServer != nil || a.restAttached,
		BaseURL:  a.restServerURL,
		Port:     sanitizeRESTServerPort(a.config.RESTServerPort),
		Attached: a.restAttached,
	}
}

func (a *App) GetRESTServerToken() string {
	return a.restServerToken
}

func detectExistingRESTServer(baseURL string) bool {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get(baseURL + "/api/status")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return false
	}
	var status model.RESTServerStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return false
	}
	return status.Running
}

func (a *App) restMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/docs.md", a.handleRESTDocsMarkdown)
	mux.HandleFunc("/api/status", a.handleRESTStatus)
	mux.HandleFunc("/api/sites", a.handleRESTSites)
	mux.HandleFunc("/api/sites/", a.handleRESTSiteByID)
	mux.HandleFunc("/api/sites/reorder", a.handleRESTSitesReorder)
	mux.HandleFunc("/api/tabs", a.handleRESTTabs)
	mux.HandleFunc("/api/tabs/file", a.handleRESTCreateFileTab)
	mux.HandleFunc("/api/tabs/ssh", a.handleRESTCreateSSHTab)
	mux.HandleFunc("/api/tabs/telnet", a.handleRESTCreateTelnetTab)
	mux.HandleFunc("/api/tabs/local", a.handleRESTCreateLocalTab)
	mux.HandleFunc("/api/tabs/", a.handleRESTTabByID)
	mux.HandleFunc("/api/files/local", a.handleRESTListLocal)
	mux.HandleFunc("/api/files/remote", a.handleRESTListRemote)
	mux.HandleFunc("/api/files/upload", a.handleRESTUploadPaths)
	mux.HandleFunc("/api/files/download", a.handleRESTDownloadPaths)
	mux.HandleFunc("/api/operations", a.handleRESTOperations)
	mux.HandleFunc("/api/operations/", a.handleRESTOperations)
	mux.HandleFunc("/api/sftp/stat", a.handleRESTSFTPStat)
	mux.HandleFunc("/api/sftp/mkdir", a.handleRESTSFTPMkdir)
	mux.HandleFunc("/api/sftp/rename", a.handleRESTSFTPRename)
	mux.HandleFunc("/api/sftp/delete", a.handleRESTSFTPDelete)
	mux.HandleFunc("/api/ssh/execute", a.handleRESTSSHExecute)
	mux.HandleFunc("/api/terminal/output", a.handleRESTTerminalOutput)
	mux.HandleFunc("/api/terminal/input", a.handleRESTTerminalInput)
	mux.HandleFunc("/api/terminal/resize", a.handleRESTTerminalResize)
	mux.HandleFunc("/api/terminal/close", a.handleRESTTerminalClose)
	mux.HandleFunc("/api/transfers", a.handleRESTTransfers)
	mux.HandleFunc("/api/logs", a.handleRESTLogs)
	mux.HandleFunc("/api/config", a.handleRESTConfig)
	return a.withRESTSecurity(mux)
}

func (a *App) withRESTSecurity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				crashlog.Write("rest "+r.Method+" "+r.URL.Path, recovered)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" && !isAllowedRESTOrigin(origin) {
			writeError(w, http.StatusForbidden, "origin not allowed")
			return
		}
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-IntegTERM-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path != "/api/status" && !a.isAuthorizedRESTRequest(r) {
			writeError(w, http.StatusUnauthorized, "invalid or missing API token")
			return
		}

		if isRESTStatePath(r.URL.Path) {
			needsWriteLock := r.Method != http.MethodGet || strings.HasPrefix(r.URL.Path, "/api/sites")
			if !needsWriteLock {
				a.stateMu.RLock()
				defer a.stateMu.RUnlock()
			} else {
				a.stateMu.Lock()
				defer a.stateMu.Unlock()
				if strings.HasPrefix(r.URL.Path, "/api/sites") {
					if err := a.reloadSitesFromStoreLocked(); err != nil {
						writeError(w, http.StatusInternalServerError, "reload sites: "+err.Error())
						return
					}
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) isAuthorizedRESTRequest(r *http.Request) bool {
	token := strings.TrimSpace(r.Header.Get("X-IntegTERM-Token"))
	if authorization := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(authorization, "Bearer ") {
		token = strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	}
	if token == "" || a.restServerToken == "" || len(token) != len(a.restServerToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(a.restServerToken)) == 1
}

func isAllowedRESTOrigin(origin string) bool {
	return strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "http://localhost:") ||
		origin == "http://127.0.0.1" ||
		origin == "http://localhost"
}

func isRESTStatePath(requestPath string) bool {
	return requestPath == "/api/status" ||
		requestPath == "/api/docs.md" ||
		strings.HasPrefix(requestPath, "/api/sites") ||
		strings.HasPrefix(requestPath, "/api/tabs") ||
		requestPath == "/api/config"
}

func mustOpenCrashLogWriter() *os.File {
	path := crashlog.Path()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return os.Stderr
	}
	return file
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func (a *App) handleRESTDocsMarkdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	doc, err := a.GetRestAPIDocsMarkdown()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	_, _ = w.Write([]byte(doc))
}

func (a *App) handleRESTStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, a.GetRESTServerStatus())
}
