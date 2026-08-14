package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/sshutil"

	"golang.org/x/crypto/ssh"
)

func (a *App) StartSSHSession(site model.Site) (string, error) {
	return a.sessionManager.StartSSHSession(a.ctx, site)
}

func (a *App) StartTelnetSession(site model.Site) (string, error) {
	return a.sessionManager.StartTelnetSession(a.ctx, site)
}

func (a *App) ApproveHost(prompt model.HostTrustPrompt) error {
	return sshutil.ApproveHost(prompt.HostPattern, prompt.AuthorizedKey)
}

func (a *App) WriteSSHInput(sessionID string, data string) error {
	a.markSessionActivity(sessionID)
	return a.sessionManager.WriteSSHInput(sessionID, data)
}

func (a *App) CloseIdleHiddenConnections(idleLimit time.Duration) int {
	if idleLimit <= 0 {
		return 0
	}

	now := time.Now()
	idleTabIDs := make([]string, 0)
	for _, tab := range a.tabs {
		if !tab.Hidden || !tab.Connected {
			continue
		}
		lastActivity := a.lastActivityForTab(tab.ID)
		if lastActivity.IsZero() {
			a.markTabActivity(tab.ID)
			continue
		}
		if now.Sub(lastActivity) < idleLimit {
			continue
		}
		idleTabIDs = append(idleTabIDs, tab.ID)
	}

	closedCount := 0
	for _, tabID := range idleTabIDs {
		tab := a.findTab(tabID)
		title := tabID
		if tab != nil && strings.TrimSpace(tab.Title) != "" {
			title = tab.Title
		}
		a.sessionManager.AppendLog(fmt.Sprintf("%s 閒置超過 %d 分鐘，已自動關閉背景連線", title, int(idleLimit/time.Minute)), "done")
		if _, err := a.CloseTab(tabID); err == nil {
			closedCount++
		}
	}

	return closedCount
}

func (a *App) ClearBackgroundConnections() int {
	backgroundTabIDs := make([]string, 0)
	for _, tab := range a.tabs {
		if tab.Hidden && tab.Connected {
			backgroundTabIDs = append(backgroundTabIDs, tab.ID)
		}
	}

	closedCount := 0
	for _, tabID := range backgroundTabIDs {
		tab := a.findTab(tabID)
		title := tabID
		if tab != nil && strings.TrimSpace(tab.Title) != "" {
			title = tab.Title
		}
		a.sessionManager.AppendLog(fmt.Sprintf("%s 已手動清除背景連線", title), "done")
		if _, err := a.CloseTab(tabID); err == nil {
			closedCount++
		}
	}

	return closedCount
}

func (a *App) markTabActivity(tabID string) {
	if strings.TrimSpace(tabID) == "" {
		return
	}
	a.activityMu.Lock()
	a.lastActivity[tabID] = time.Now()
	a.activityMu.Unlock()
}

func (a *App) markSessionActivity(sessionID string) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	for _, tab := range a.tabs {
		if tab.SessionID == sessionID {
			a.markTabActivity(tab.ID)
			return
		}
	}
}

func (a *App) lastActivityForTab(tabID string) time.Time {
	a.activityMu.Lock()
	defer a.activityMu.Unlock()
	return a.lastActivity[tabID]
}

func (a *App) clearTabActivity(tabID string) {
	if strings.TrimSpace(tabID) == "" {
		return
	}
	a.activityMu.Lock()
	delete(a.lastActivity, tabID)
	a.activityMu.Unlock()
}

func (a *App) findTab(tabID string) *model.Tab {
	for index := range a.tabs {
		if a.tabs[index].ID == tabID {
			return &a.tabs[index]
		}
	}
	return nil
}

func (a *App) GetSSHOutputBuffer(sessionID string) string {
	return a.sessionManager.GetSSHOutputBuffer(sessionID)
}

func (a *App) ListSystemFonts() []string {
	fonts, err := listSystemFonts()
	if err != nil || len(fonts) == 0 {
		return fallbackTerminalFonts()
	}
	return fonts
}

func (a *App) ResizeSSHSession(sessionID string, cols uint16, rows uint16) error {
	return a.sessionManager.ResizeSSHSession(sessionID, cols, rows)
}

func (a *App) CloseSSHSession(sessionID string) error {
	return a.sessionManager.CloseSSHSession(sessionID)
}

func (a *App) ExecuteSSHCommand(site model.Site, command string, timeoutSeconds int) (map[string]any, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}
	if strings.TrimSpace(site.Host) == "" {
		return nil, fmt.Errorf("host is required")
	}
	if strings.TrimSpace(site.Username) == "" {
		return nil, fmt.Errorf("username is required")
	}

	authMethods := make([]ssh.AuthMethod, 0, 2)
	if site.Password != "" {
		authMethods = append(authMethods, ssh.Password(site.Password))
	}
	if site.PPKPath != "" {
		signer, err := sshutil.SignerFromPPK(site.PPKPath, site.PPKPassphrase)
		if err != nil {
			return nil, fmt.Errorf("load ppk: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("missing ssh auth method")
	}

	hostKeyCallback, err := sshutil.KnownHostsCallback()
	if err != nil {
		return nil, err
	}

	timeout := 10 * time.Second
	if timeoutSeconds > 0 {
		timeout = time.Duration(timeoutSeconds) * time.Second
	}

	client, err := sshutil.DialWithRouteRetry("tcp", fmt.Sprintf("%s:%d", site.Host, site.Port), &ssh.ClientConfig{
		User:            site.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         timeout,
	})
	if err != nil {
		return nil, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var stdoutBuilder strings.Builder
	var stderrBuilder strings.Builder
	session.Stdout = &stdoutBuilder
	session.Stderr = &stderrBuilder

	runErr := session.Run(command)
	exitCode := 0
	if runErr != nil {
		var exitErr *ssh.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitStatus()
		} else {
			return nil, runErr
		}
	}

	return map[string]any{
		"stdout":   stdoutBuilder.String(),
		"stderr":   stderrBuilder.String(),
		"exitCode": exitCode,
		"ok":       exitCode == 0,
	}, nil
}
