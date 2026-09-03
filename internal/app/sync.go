package app

import (
	"fmt"
	"strings"

	"github.com/VaderChen/Integrate-Terminal/internal/model"
)

func (a *App) CompareDirectories(tabID string, localPath string, remotePath string) ([]model.FileComparison, error) {
	if strings.TrimSpace(tabID) == "" {
		return nil, fmt.Errorf("tab id is required")
	}
	a.markTabActivity(tabID)
	return a.sessionManager.CompareDirectories(tabID, localPath, remotePath)
}

func (a *App) SyncDirectories(tabID string, localPath string, remotePath string, direction string) error {
	if strings.TrimSpace(tabID) == "" {
		return fmt.Errorf("tab id is required")
	}
	a.markTabActivity(tabID)
	return a.sessionManager.SyncDirectories(tabID, localPath, remotePath, direction)
}
