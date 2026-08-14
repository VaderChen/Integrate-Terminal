package session

import (
	"fmt"

"github.com/VaderChen/Integrate-Terminal/internal/model"
)

func (m *Manager) ClearCompletedTransfers() []model.TransferItem {
	m.mu.Lock()
	filtered := make([]model.TransferItem, 0, len(m.transfers))
	for _, item := range m.transfers {
		if item.Status != "done" && item.Status != "cancelled" {
			filtered = append(filtered, item)
		}
	}
	m.transfers = filtered
	m.notifyStateLocked()
	m.mu.Unlock()
	return m.SampleTransfers()
}

func (m *Manager) ClearAllTransfers() []model.TransferItem {
	m.mu.Lock()
	for _, item := range m.transfers {
		if item.Status == "running" || item.Status == "paused" {
			m.cancelledTransfers[item.ID] = true
		}
	}
	m.transfers = []model.TransferItem{}
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
	for i := range m.transfers {
		if m.transfers[i].ID == itemID && (m.transfers[i].Status == "running" || m.transfers[i].Status == "paused") {
			m.transfers[i].Status = "cancelled"
			m.transfers[i].SpeedBps = 0
			m.addLogLocked(fmt.Sprintf("已取消傳輸: %s", m.transfers[i].Name), "failed")
			break
		}
	}
	m.notifyStateLocked()
	m.mu.Unlock()
	return m.SampleTransfers()
}

func (m *Manager) TogglePauseTransfer(itemID string) []model.TransferItem {
	m.mu.Lock()
	for i := range m.transfers {
		if m.transfers[i].ID != itemID {
			continue
		}
		if m.transfers[i].Status == "running" {
			m.pausedTransfers[itemID] = true
			m.transfers[i].Status = "paused"
			m.transfers[i].SpeedBps = 0
			m.addLogLocked(fmt.Sprintf("已暫停傳輸: %s", m.transfers[i].Name), "running")
		} else if m.transfers[i].Status == "paused" {
			delete(m.pausedTransfers, itemID)
			m.transfers[i].Status = "running"
			m.addLogLocked(fmt.Sprintf("已繼續傳輸: %s", m.transfers[i].Name), "running")
		}
		break
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
		for i := range m.transfers {
			if m.transfers[i].Status == "running" {
				m.pausedTransfers[m.transfers[i].ID] = true
				m.transfers[i].Status = "paused"
				m.transfers[i].SpeedBps = 0
			}
		}
		m.addLogLocked("已暫停全部傳輸", "running")
	} else {
		for i := range m.transfers {
			if m.transfers[i].Status == "paused" {
				delete(m.pausedTransfers, m.transfers[i].ID)
				m.transfers[i].Status = "running"
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
	m.notifyStateLocked()
	m.mu.Unlock()
	return m.SampleLogs()
}
