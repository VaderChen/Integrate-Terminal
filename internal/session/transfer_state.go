package session

import (
	"fmt"
	"strings"
	"time"

	"IntegTERM/internal/model"
)

// 日誌僅保存在記憶體，固定保留最近 1,000 筆。
const maxLogEntries = 1000

// 傳輸與日誌共用遞增時間戳；時鐘回退或解析度不足時仍維持 ID 唯一。
func (m *Manager) nextItemTimestampLocked() int64 {
	next := time.Now().UnixNano()
	if next <= m.lastItemTimestamp {
		next = m.lastItemTimestamp + 1
	}
	m.lastItemTimestamp = next
	return next
}

// ID 索引提供固定成本的進度更新，雙向連結保留最新在前的顯示順序。
// 移除時解除連結，佇列清空時釋放索引，不保留過去的最大容量。
type transferEntry struct {
	item         model.TransferItem
	newer, older *transferEntry
}

func (m *Manager) insertTransferLocked(item model.TransferItem) {
	if m.transferItems == nil {
		m.transferItems = make(map[string]*transferEntry)
	}
	entry := &transferEntry{item: item, older: m.newestTransfer}
	if entry.older != nil {
		entry.older.newer = entry
	}
	m.newestTransfer = entry
	m.transferItems[item.ID] = entry
}

func (m *Manager) unlinkTransferLocked(entry *transferEntry) {
	if entry.newer == nil {
		m.newestTransfer = entry.older
	} else {
		entry.newer.older = entry.older
	}
	if entry.older != nil {
		entry.older.newer = entry.newer
	}
	delete(m.transferItems, entry.item.ID)
	entry.newer, entry.older = nil, nil
	if len(m.transferItems) == 0 {
		m.transferItems = nil
	}
}

func (m *Manager) updateTransfer(itemID string, progress int, speedBps int64, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateTransferLocked(itemID, progress, speedBps, status)
}

func (m *Manager) updateTransferLocked(itemID string, progress int, speedBps int64, status string) {
	// 取消後仍可能收到 worker 已取出的進度，不能恢復執行中狀態。
	if (status == "running" || status == "paused") && m.isTransferCancelledLocked(itemID) {
		return
	}
	if status == "done" || status == "cancelled" || status == "failed" {
		// Only the worker reports terminal state through this method. UI removal
		// may happen earlier, but cancellation must remain effective until now.
		delete(m.cancelledTransfers, itemID)
		delete(m.pausedTransfers, itemID)
		delete(m.transferParents, itemID)
	}
	entry := m.transferItems[itemID]
	if entry == nil {
		return
	}
	item := &entry.item
	if status == "done" || status == "cancelled" || status == "failed" {
		item.Progress, item.SpeedBps, item.Status = progress, speedBps, status
		m.notifyStateLocked()
		if status != "failed" {
			time.AfterFunc(1200*time.Millisecond, func() {
				m.removeTransfer(itemID)
			})
		}
		return
	}
	if m.isTransferPausedLocked(itemID) && status != "paused" {
		status = item.Status
	}
	if item.Progress == progress && item.SpeedBps == speedBps && item.Status == status {
		return
	}
	item.Progress, item.SpeedBps, item.Status = progress, speedBps, status
	m.notifyStateLocked()
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
	if entry := m.transferItems[itemID]; entry != nil {
		m.unlinkTransferLocked(entry)
		m.notifyStateLocked()
	}
}

func (m *Manager) addTransfer(name string, direction string) string {
	return m.addChildTransfer(name, direction, "")
}

func (m *Manager) addChildTransfer(name string, direction string, parentID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	itemID := fmt.Sprintf("transfer-%d", m.nextItemTimestampLocked())
	// 根傳輸沒有父項，不必為空關係占用索引；移除列後仍保留子項的控制關係。
	if parentID != "" {
		m.transferParents[itemID] = parentID
	}
	m.insertTransferLocked(model.TransferItem{
		ID:        itemID,
		Direction: direction,
		Name:      name,
		Progress:  0,
		SpeedBps:  0,
		Status:    "running",
	})
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
	return m.isTransferCancelledLocked(itemID)
}

func (m *Manager) isTransferCancelledLocked(itemID string) bool {
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
	if entry := m.transferItems[itemID]; entry != nil {
		return entry.item.Progress
	}
	return 0
}

func (m *Manager) addLog(message string, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addLogLocked(message, status)
}

func (m *Manager) addLogLocked(message string, status string) {
	item := model.LogItem{
		ID:        fmt.Sprintf("log-%d", m.nextItemTimestampLocked()),
		Message:   message,
		Status:    status,
		CreatedAt: time.Now().Format("15:04:05"),
	}
	// 滿載後覆寫最舊位置；快照依循環索引還原最新在前的順序。
	if len(m.logs) < maxLogEntries {
		m.logs = append(m.logs, item)
	} else {
		m.logs[m.logStart] = item
		m.logStart = (m.logStart + 1) % maxLogEntries
	}
	m.notifyStateLocked()
}

func sanitizeDirectoryName(name string) string {
	return strings.TrimSpace(strings.Trim(name, "/"))
}
