package session

import (
	"fmt"
	"time"

"github.com/VaderChen/Integrate-Terminal/internal/model"
)

func (m *Manager) updateTransfer(itemID string, progress int, speedBps int64, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateTransferLocked(itemID, progress, speedBps, status)
}

func (m *Manager) updateTransferLocked(itemID string, progress int, speedBps int64, status string) {
	for i := range m.transfers {
		if m.transfers[i].ID == itemID {
			if status == "done" || status == "cancelled" {
				m.transfers[i].Progress = progress
				m.transfers[i].SpeedBps = speedBps
				m.transfers[i].Status = status
				m.notifyStateLocked()
				time.AfterFunc(1200*time.Millisecond, func() {
					m.removeTransfer(itemID)
				})
				return
			}
			m.transfers[i].Progress = progress
			m.transfers[i].SpeedBps = speedBps
			if !m.isTransferPausedLocked(itemID) || status == "paused" {
				m.transfers[i].Status = status
			}
			m.notifyStateLocked()
			return
		}
	}
	if status == "done" || status == "cancelled" || status == "failed" {
		delete(m.cancelledTransfers, itemID)
		delete(m.pausedTransfers, itemID)
	}
}

func (m *Manager) removeTransfer(itemID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeTransferLocked(itemID)
}

func (m *Manager) removeTransferLocked(itemID string) {
	for i := range m.transfers {
		if m.transfers[i].ID == itemID {
			m.transfers = append(m.transfers[:i], m.transfers[i+1:]...)
			delete(m.cancelledTransfers, itemID)
			delete(m.pausedTransfers, itemID)
			m.notifyStateLocked()
			return
		}
	}
}

func (m *Manager) addTransfer(name string, direction string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	itemID := fmt.Sprintf("transfer-%d", time.Now().UnixNano())
	m.transfers = append([]model.TransferItem{{
		ID:        itemID,
		Direction: direction,
		Name:      name,
		Progress:  0,
		SpeedBps:  0,
		Status:    "running",
	}}, m.transfers...)
	if m.pauseAllTransfers {
		m.pausedTransfers[itemID] = true
		m.updateTransferLocked(itemID, 0, 0, "paused")
	}
	m.notifyStateLocked()
	return itemID
}

func (m *Manager) isTransferCancelled(itemID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cancelledTransfers[itemID]
}

func (m *Manager) isTransferPaused(itemID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isTransferPausedLocked(itemID)
}

func (m *Manager) isTransferPausedLocked(itemID string) bool {
	return m.pausedTransfers[itemID]
}

func (m *Manager) awaitTransferActive(itemID string, progress int) bool {
	for {
		if m.isTransferCancelled(itemID) {
			return false
		}
		if !m.isTransferPaused(itemID) {
			return true
		}
		m.updateTransfer(itemID, progress, 0, "paused")
		time.Sleep(120 * time.Millisecond)
	}
}

func (m *Manager) transferProgress(itemID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := range m.transfers {
		if m.transfers[i].ID == itemID {
			return m.transfers[i].Progress
		}
	}
	return 0
}

func (m *Manager) addLog(message string, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addLogLocked(message, status)
}

func (m *Manager) addLogLocked(message string, status string) {
	m.logs = append([]model.LogItem{{
		ID:        fmt.Sprintf("log-%d", time.Now().UnixNano()),
		Message:   message,
		Status:    status,
		CreatedAt: time.Now().Format("15:04:05"),
	}}, m.logs...)
	m.notifyStateLocked()
}
