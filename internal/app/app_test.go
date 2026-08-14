package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/session"
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
