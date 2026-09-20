package app

import (
	"IntegTERM/internal/model"
	"fmt"
)

func (a *App) ListLocal(tabID string, path string) []model.FileEntry {
	if path == "" {
		path = defaultLocalPath()
	}
	a.markTabActivity(tabID)
	a.stateMu.RLock()
	showHidden := a.config.ShowHiddenFiles
	a.stateMu.RUnlock()
	return filterHiddenEntries(a.sessionManager.SampleLocalFiles(path), showHidden)
}

func (a *App) ListRemote(tabID string, path string) ([]model.FileEntry, error) {
	a.markTabActivity(tabID)
	a.stateMu.RLock()
	for _, tab := range a.tabs {
		if tab.ID == tabID && tab.Mode == "terminal" {
			a.stateMu.RUnlock()
			return nil, fmt.Errorf("tab is not a file connection: %s", tabID)
		}
	}
	showHidden := a.config.ShowHiddenFiles
	a.stateMu.RUnlock()
	if path == "" {
		path = "/"
	}
	entries, err := a.sessionManager.ListRemote(tabID, path)
	if err != nil {
		return nil, err
	}
	return filterHiddenEntries(entries, showHidden), nil
}

func (a *App) GetTransfers() []model.TransferItem {
	return a.sessionManager.SampleTransfers()
}

func (a *App) GetLogs() []model.LogItem {
	return a.sessionManager.SampleLogs()
}

func (a *App) ClearCompletedTransfers() []model.TransferItem {
	return a.sessionManager.ClearCompletedTransfers()
}

func (a *App) ClearAllTransfers() []model.TransferItem {
	return a.sessionManager.ClearAllTransfers()
}

func (a *App) CancelTransfer(itemID string) []model.TransferItem {
	return a.sessionManager.CancelTransfer(itemID)
}

func (a *App) TogglePauseTransfer(itemID string) []model.TransferItem {
	return a.sessionManager.TogglePauseTransfer(itemID)
}

func (a *App) TogglePauseAllTransfers() []model.TransferItem {
	return a.sessionManager.TogglePauseAllTransfers()
}

func (a *App) ClearLogs() []model.LogItem {
	return a.sessionManager.ClearLogs()
}
