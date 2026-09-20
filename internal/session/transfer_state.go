package session

import (
	"fmt"
	"strings"
	"time"

	"IntegTERM/internal/model"
)

func (m *Manager) updateTransfer(itemID string, progress int, speedBps int64, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateTransferLocked(itemID, progress, speedBps, status)
}

func (m *Manager) updateTransferLocked(itemID string, progress int, speedBps int64, status string) {
	if status == "done" || status == "cancelled" || status == "failed" {
		// Only the worker reports terminal state through this method. UI removal
		// may happen earlier, but cancellation must remain effective until now.
		delete(m.cancelledTransfers, itemID)
		delete(m.pausedTransfers, itemID)
		delete(m.transferParents, itemID)
	}
	for i := range m.transfers {
		if m.transfers[i].ID == itemID {
			if status == "done" || status == "cancelled" || status == "failed" {
				m.transfers[i].Progress = progress
				m.transfers[i].SpeedBps = speedBps
				m.transfers[i].Status = status
				delete(m.pausedTransfers, itemID)
				m.notifyStateLocked()
				if status != "failed" {
					time.AfterFunc(1200*time.Millisecond, func() {
						m.removeTransfer(itemID)
					})
				}
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
}

func (m *Manager) removeTransfer(itemID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeTransferLocked(itemID)
}

func (m *Manager) removeTransferLocked(itemID string) {
	delete(m.cancelledTransfers, itemID)
	delete(m.pausedTransfers, itemID)
	delete(m.transferParents, itemID)
	for i := range m.transfers {
		if m.transfers[i].ID == itemID {
			m.transfers = append(m.transfers[:i], m.transfers[i+1:]...)
			m.notifyStateLocked()
			return
		}
	}
}

func (m *Manager) addTransfer(name string, direction string) string {
	return m.addChildTransfer(name, direction, "")
}

func (m *Manager) addChildTransfer(name string, direction string, parentID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	itemID := fmt.Sprintf("transfer-%d", time.Now().UnixNano())
	m.transferParents[itemID] = parentID
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
	for itemID != "" {
		if m.cancelledTransfers[itemID] {
			return true
		}
		itemID = m.transferParents[itemID]
	}
	return false
}

func (m *Manager) isTransferPaused(itemID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isTransferPausedLocked(itemID)
}

func (m *Manager) isTransferPausedLocked(itemID string) bool {
	for itemID != "" {
		if m.pausedTransfers[itemID] {
			return true
		}
		itemID = m.transferParents[itemID]
	}
	return false
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

func sanitizeDirectoryName(name string) string {
	return strings.TrimSpace(strings.Trim(name, "/"))
}
