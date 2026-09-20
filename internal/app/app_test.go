package app

import (
	"os"
	"path/filepath"
	"testing"

	"IntegTERM/internal/model"
	"IntegTERM/internal/purchase"
	"IntegTERM/internal/session"
)

func TestBootstrapDefersFileSystemListings(t *testing.T) {
	localPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(localPath, "startup-delay-check.txt"), []byte("test"), 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}

	instance := &App{
		sessionManager: session.NewManager(),
		tabs: []model.Tab{
			{ID: "tab-1", Mode: "files", LocalPath: localPath, RemotePath: "/"},
		},
	}

	payload := instance.Bootstrap()
	if len(payload.LocalFiles) != 0 {
		t.Fatalf("expected local files to load after bootstrap, got %d entries", len(payload.LocalFiles))
	}
	if len(payload.RemoteFiles) != 0 {
		t.Fatalf("expected remote files to load after bootstrap, got %d entries", len(payload.RemoteFiles))
	}
}

func TestEnsureTabCreationAllowed_FreePlanWithinLimit(t *testing.T) {
	instance := &App{
		config: model.Config{ProUnlock: false},
		tabs: []model.Tab{
			{ID: "tab-1"},
		},
	}

	if err := instance.ensureTabCreationAllowed(); err != nil {
		t.Fatalf("expected tab creation to be allowed, got error: %v", err)
	}
}

func TestEnsureTabCreationAllowed_FreePlanAtLimit(t *testing.T) {
	instance := &App{
		config: model.Config{ProUnlock: false},
		tabs: []model.Tab{
			{ID: "tab-1"},
			{ID: "tab-2"},
		},
	}

	if err := instance.ensureTabCreationAllowed(); err == nil {
		t.Fatal("expected tab creation to be blocked at the free plan limit")
	}
}

func TestEnsureTabCreationAllowed_ProUnlockUnlimited(t *testing.T) {
	instance := &App{
		verifiedProUnlock: true,
		config:            model.Config{ProUnlock: true},
		tabs: []model.Tab{
			{ID: "tab-1"},
			{ID: "tab-2"},
			{ID: "tab-3"},
		},
	}

	if err := instance.ensureTabCreationAllowed(); err != nil {
		t.Fatalf("expected pro unlock to bypass tab limit, got error: %v", err)
	}
}

func TestMergePurchaseStatusRejectsUnverifiedConfig(t *testing.T) {
	instance := &App{
		config: model.Config{ProUnlock: true},
		tabs: []model.Tab{
			{ID: "tab-1"},
			{ID: "tab-2"},
			{ID: "tab-3"},
		},
	}

	status := instance.mergePurchaseStatus(purchase.State{
		ProductID:   purchase.ProUnlockProductID,
		PlanName:    "Free",
		Source:      "test",
		ProUnlock:   false,
		CanPurchase: true,
		CanRestore:  true,
	}, nil)
	if status.ProUnlock {
		t.Fatal("an editable config must not unlock Pro")
	}
	if status.MaxTabs != freePlanTabLimit {
		t.Fatalf("expected free limit, got %d", status.MaxTabs)
	}
}
