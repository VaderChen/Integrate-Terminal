package app

import "IntegTERM/internal/model"

func (a *App) ListLocal(tabID string, path string) []model.FileEntry {
	if path == "" {
		path = defaultLocalPath()
	}
	a.markTabActivity(tabID)
	return filterHiddenEntries(a.sessionManager.SampleLocalFiles(path), a.config.ShowHiddenFiles)
}

func (a *App) ListRemote(tabID string, path string) []model.FileEntry {
	a.markTabActivity(tabID)
	for _, tab := range a.tabs {
		if tab.ID == tabID && tab.Mode == "terminal" {
			return []model.FileEntry{}
		}
	}
	if path == "" {
		path = "/"
	}
	entries, err := a.sessionManager.ListRemote(tabID, path)
	if err != nil {
		return []model.FileEntry{}
	}
	return filterHiddenEntries(entries, a.config.ShowHiddenFiles)
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
