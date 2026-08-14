package session

import (
	"testing"

"github.com/VaderChen/Integrate-Terminal/internal/model"
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
