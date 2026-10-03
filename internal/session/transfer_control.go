package session

import (
	"fmt"

	"IntegTERM/internal/model"
)

func (m *Manager) ClearCompletedTransfers() []model.TransferItem {
	m.mu.Lock()
	for entry := m.newestTransfer; entry != nil; {
		next := entry.older
		if entry.item.Status == "done" || entry.item.Status == "cancelled" {
			m.unlinkTransferLocked(entry)
		}
		entry = next
	}
	m.notifyStateLocked()
	m.mu.Unlock()
	return m.SampleTransfers()
}

func (m *Manager) ClearAllTransfers() []model.TransferItem {
	m.mu.Lock()
	for entry := m.newestTransfer; entry != nil; entry = entry.older {
		item := &entry.item
		if item.Status == "running" || item.Status == "paused" {
			m.cancelledTransfers[item.ID] = true
		}
	}
	m.transferItems = nil
	m.newestTransfer = nil
	m.pausedTransfers = make(map[string]bool)
	m.pauseAllTransfers = false
	m.notifyStateLocked()
	m.mu.Unlock()
	return m.SampleTransfers()
}

func (m *Manager) CancelTransfer(itemID string) []model.TransferItem {
	m.mu.Lock()
	m.cancelledTransfers[itemID] = true
	delete(m.pausedTransfers, itemID)
	if entry := m.transferItems[itemID]; entry != nil {
		item := &entry.item
		if item.Status == "running" || item.Status == "paused" {
			item.Status = "cancelled"
			item.SpeedBps = 0
			m.addLogLocked(fmt.Sprintf("已取消傳輸: %s", item.Name), "failed")
		}
	}
	m.notifyStateLocked()
	m.mu.Unlock()
	return m.SampleTransfers()
}

func (m *Manager) TogglePauseTransfer(itemID string) []model.TransferItem {
	m.mu.Lock()
	if entry := m.transferItems[itemID]; entry != nil {
		item := &entry.item
		if item.Status == "running" {
			m.pausedTransfers[itemID] = true
			item.Status = "paused"
			item.SpeedBps = 0
			m.addLogLocked(fmt.Sprintf("已暫停傳輸: %s", item.Name), "running")
		} else if item.Status == "paused" {
			delete(m.pausedTransfers, itemID)
			item.Status = "running"
			m.addLogLocked(fmt.Sprintf("已繼續傳輸: %s", item.Name), "running")
		}
	}
	m.notifyStateLocked()
	m.mu.Unlock()
	return m.SampleTransfers()
}

func (m *Manager) TogglePauseAllTransfers() []model.TransferItem {
	m.mu.Lock()
	shouldPause := !m.pauseAllTransfers
	m.pauseAllTransfers = shouldPause
	if shouldPause {
		for entry := m.newestTransfer; entry != nil; entry = entry.older {
			item := &entry.item
			if item.Status == "running" {
				m.pausedTransfers[item.ID] = true
				item.Status = "paused"
				item.SpeedBps = 0
			}
		}
		m.addLogLocked("已暫停全部傳輸", "running")
	} else {
		for entry := m.newestTransfer; entry != nil; entry = entry.older {
			item := &entry.item
			if item.Status == "paused" {
				delete(m.pausedTransfers, item.ID)
				item.Status = "running"
			}
		}
		m.addLogLocked("已繼續全部傳輸", "running")
	}
	m.notifyStateLocked()
	m.mu.Unlock()
	return m.SampleTransfers()
}

func (m *Manager) ClearLogs() []model.LogItem {
	m.mu.Lock()
	m.logs = []model.LogItem{}
	m.logStart = 0
	m.notifyStateLocked()
	m.mu.Unlock()
	return m.SampleLogs()
}
