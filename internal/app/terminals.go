package app

import (
	"fmt"
	"strings"
	"time"

	"IntegTERM/internal/model"
	"IntegTERM/internal/session"
	"IntegTERM/internal/sshutil"
)

func (a *App) StartSSHSession(site model.Site) (string, error) {
	return a.sessionManager.StartSSHSession(a.appContext(), site)
}

func (a *App) StartTelnetSession(site model.Site) (string, error) {
	return a.sessionManager.StartTelnetSession(a.appContext(), site)
}

func (a *App) ApproveHost(prompt model.HostTrustPrompt) error {
	return sshutil.ApproveHost(prompt.HostPattern, prompt.AuthorizedKey)
}

func (a *App) WriteSSHInput(sessionID string, data string) error {
	a.markSessionActivity(sessionID)
	return a.sessionManager.WriteSSHInput(sessionID, data)
}

func (a *App) closeIdleHiddenConnectionsLocked(idleLimit time.Duration) int {
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
		if _, err := a.closeTabLocked(tabID); err == nil {
			closedCount++
		}
	}

	return closedCount
}

func (a *App) clearBackgroundConnectionsLocked() int {
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
		if _, err := a.closeTabLocked(tabID); err == nil {
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
	if a.lastActivity == nil {
		a.lastActivity = make(map[string]time.Time)
	}
	a.lastActivity[tabID] = time.Now()
	a.activityMu.Unlock()
}

func (a *App) markSessionActivity(sessionID string) {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
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

func (a *App) GetTerminalOutputSnapshot(sessionID string) session.TerminalOutputSnapshot {
	return a.sessionManager.GetTerminalOutputSnapshot(sessionID)
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
