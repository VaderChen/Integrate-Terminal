package session

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/transport"
)

type Manager struct {
	mu                       sync.RWMutex
	clients                  map[string]transport.Client
	sshSessions              map[string]*sshTerminalSession
	telnetSessions           map[string]*telnetTerminalSession
	localSessions            map[string]*localTerminalSession
	transfers                []model.TransferItem
	cancelledTransfers       map[string]bool
	pausedTransfers          map[string]bool
	pauseAllTransfers        bool
	transferRetryCount       int
	transferConflictStrategy string
	logs                     []model.LogItem
	eventCtx                 context.Context
	stateEvents              chan struct{}
}

func NewManager() *Manager {
	manager := &Manager{
		clients:                  make(map[string]transport.Client),
		sshSessions:              make(map[string]*sshTerminalSession),
		telnetSessions:           make(map[string]*telnetTerminalSession),
		localSessions:            make(map[string]*localTerminalSession),
		transfers:                make([]model.TransferItem, 0),
		cancelledTransfers:       make(map[string]bool),
		pausedTransfers:          make(map[string]bool),
		transferRetryCount:       2,
		transferConflictStrategy: "overwrite",
		logs:                     make([]model.LogItem, 0),
		stateEvents:              make(chan struct{}, 1),
	}
	go manager.runStateEventLoop()
	return manager
}

func (m *Manager) ConfigureTransferPolicy(retryCount int, conflictStrategy string) {
	if retryCount < 0 {
		retryCount = 0
	}
	if retryCount > 10 {
		retryCount = 10
	}
	if conflictStrategy != "skip" && conflictStrategy != "fail" {
		conflictStrategy = "overwrite"
	}
	m.mu.Lock()
	m.transferRetryCount = retryCount
	m.transferConflictStrategy = conflictStrategy
	m.mu.Unlock()
}

func (m *Manager) transferPolicy() (int, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.transferRetryCount, m.transferConflictStrategy
}

func (m *Manager) SetEventContext(ctx context.Context) {
	m.mu.Lock()
	m.eventCtx = ctx
	m.mu.Unlock()
}

func (m *Manager) runStateEventLoop() {
	for range m.stateEvents {
		time.Sleep(80 * time.Millisecond)
		for {
			select {
			case <-m.stateEvents:
				continue
			default:
			}
			break
		}

		m.mu.RLock()
		ctx := m.eventCtx
		transfers := append([]model.TransferItem(nil), m.transfers...)
		logs := append([]model.LogItem(nil), m.logs...)
		m.mu.RUnlock()
		if ctx != nil {
			emitSessionEvent(ctx, "transfer:state", map[string]any{
				"transfers": transfers,
				"logs":      logs,
			})
		}
	}
}

func (m *Manager) notifyStateLocked() {
	select {
	case m.stateEvents <- struct{}{}:
	default:
	}
}

func (m *Manager) SampleLocalFiles(basePath string) []model.FileEntry {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return []model.FileEntry{}
	}

	items := make([]model.FileEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, model.FileEntry{
			Name:     entry.Name(),
			Path:     filepath.Join(basePath, entry.Name()),
			Size:     info.Size(),
			Modified: info.ModTime().Format("2006-01-02 15:04"),
			IsDir:    entry.IsDir(),
			Side:     "local",
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return items[i].Name < items[j].Name
	})

	return items
}

func (m *Manager) SampleRemoteFiles(basePath string) []model.FileEntry {
	return []model.FileEntry{}
}

func (m *Manager) SampleTransfers() []model.TransferItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.TransferItem, len(m.transfers))
	copy(out, m.transfers)
	return out
}

func (m *Manager) SampleLogs() []model.LogItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.LogItem, len(m.logs))
	copy(out, m.logs)
	return out
}

func (m *Manager) AppendLog(message string, status string) {
	m.addLog(message, status)
}
