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
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"IntegTERM/internal/crashlog"
	"IntegTERM/internal/model"
	"IntegTERM/internal/store"
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
	token := ""
	err := store.New(baseDir).WithTransaction(func(_ *store.Transaction) error {
		tokenPath := filepath.Join(baseDir, restTokenFilename)
		if data, err := os.ReadFile(tokenPath); err == nil {
			if token = strings.TrimSpace(string(data)); token != "" {
				return nil
			}
		}
		buffer := make([]byte, 32)
		if _, err := rand.Read(buffer); err != nil {
			return err
		}
		token = hex.EncodeToString(buffer)
		return os.WriteFile(tokenPath, []byte(token+"\n"), 0o600)
	})
	if err != nil {
		return ""
	} // Failure must not create divergent/predictable tokens.
	return token
}

func (a *App) applyRESTServerConfig(config model.Config, allowAttach bool) error {
	port := sanitizeRESTServerPort(config.RESTServerPort)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	a.restServerMu.Lock()
	if config.RESTServerEnabled && a.restServer != nil && a.restServerURL == baseURL {
		a.restServerMu.Unlock()
		return nil
	}
	previous := a.detachRESTServerLocked()
	a.restServerMu.Unlock()
	if err := shutdownRESTServer(previous); err != nil {
		return err
	}
	if !config.RESTServerEnabled {
		return nil
	}
	a.restServerMu.Lock()
	defer a.restServerMu.Unlock()
	return a.startRESTServerLocked(port, allowAttach)
}

func (a *App) applyRESTServerShutdown() error {
	a.runtimeConfigMu.Lock()
	defer a.runtimeConfigMu.Unlock()
	a.restServerMu.Lock()
	previous := a.detachRESTServerLocked()
	a.restServerMu.Unlock()
	return shutdownRESTServer(previous)
}

func (a *App) startRESTServerLocked(port int, allowAttach bool) error {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if allowAttach {
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
			if a.restServer == server {
				a.restServer = nil
				a.restServerURL = ""
				a.restAttached = false
			}
			a.restServerMu.Unlock()
		}
	}()

	return nil
}

func (a *App) detachRESTServerLocked() *http.Server {
	server := a.restServer
	a.restServer = nil
	a.restServerURL = ""
	a.restAttached = false
	return server
}

func shutdownRESTServer(server *http.Server) error {
	if server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		// Ensure timed-out requests cannot leave an untracked old service alive.
		_ = server.Close()
		return err
	}
	return nil
}

func (a *App) getRESTServerStatusLocked() model.RESTServerStatus {
	a.restServerMu.Lock()
	defer a.restServerMu.Unlock()
	return model.RESTServerStatus{
		Enabled:  a.config.RESTServerEnabled,
		MCPURL:   fmt.Sprintf("http://127.0.0.1:%d/mcp", sanitizeRESTServerPort(a.config.RESTServerPort)),
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
	mux.Handle("/mcp", a.newMCPHTTPHandler())
	mux.Handle("/", a.restRoutesMux())
	return a.withRESTSecurity(mux)
}

// Internal MCP dispatch uses the same REST handlers after its transport is authorized.
func (a *App) restRoutesMux() http.Handler {
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
	return mux
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
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-IntegTERM-Token, MCP-Protocol-Version, MCP-Session-Id, MCP-Method, MCP-Params")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path != "/api/status" && !a.isAuthorizedRESTRequest(r) {
			writeError(w, http.StatusUnauthorized, "invalid or missing API token")
			return
		}

		w.Header().Set("Cache-Control", "no-store")
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
	value, err := url.Parse(origin)
	if err != nil || value.Scheme != "http" || value.User != nil || value.Path != "" || value.RawQuery != "" || value.Fragment != "" {
		return false
	}
	switch value.Hostname() {
	case "localhost", "127.0.0.1", "::1":
	default:
		return false
	}
	if port := value.Port(); port != "" {
		number, err := strconv.Atoi(port)
		return err == nil && number > 0 && number <= 65535
	}
	return true
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
