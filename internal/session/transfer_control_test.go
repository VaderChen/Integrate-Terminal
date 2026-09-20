package session

import (
	"testing"

	"IntegTERM/internal/model"
	"IntegTERM/internal/transport"
)

func TestClearAllTransfersCancelsRunningWork(t *testing.T) {
	manager := NewManager()
	manager.mu.Lock()
	manager.transfers = []model.TransferItem{{ID: "transfer-1", Status: "running"}}
	manager.mu.Unlock()

	if remaining := manager.ClearAllTransfers(); len(remaining) != 0 {
		t.Fatalf("expected empty queue, got %#v", remaining)
	}
	if !manager.isTransferCancelled("transfer-1") {
		t.Fatal("running transfer was not marked cancelled")
	}

	manager.updateTransfer("transfer-1", 0, 0, "cancelled")
	if manager.isTransferCancelled("transfer-1") {
		t.Fatal("cancelled transfer state was not cleaned up")
	}
}

func TestClearCompletedRetainsCancellationUntilWorkerFinishes(t *testing.T) {
	m := NewManager()
	parent := m.addTransfer("folder", "download")
	child := m.addChildTransfer("folder/file", "download", parent)
	m.CancelTransfer(parent)
	m.ClearCompletedTransfers()
	if !m.isTransferCancelled(child) {
		t.Fatal("clearing the cancelled row reactivated its child")
	}
	m.finishPathTransfer(child, transport.ErrTransferCancelled)
	m.finishPathTransfer(parent, transport.ErrTransferCancelled)
	m.ClearCompletedTransfers()
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.transferParents) != 0 || len(m.pausedTransfers) != 0 || len(m.cancelledTransfers) != 0 {
		t.Fatalf("completed control state leaked: parents=%v paused=%v cancelled=%v", m.transferParents, m.pausedTransfers, m.cancelledTransfers)
	}
}
