package store

import (
	"os"
	"path/filepath"
	"testing"

"github.com/VaderChen/Integrate-Terminal/internal/model"
)

func TestWriteJSONAtomically(t *testing.T) {
	baseDir := t.TempDir()
	instance := New(baseDir)
	sites := []model.Site{{ID: "site-1", Name: "example", Host: "example.com"}}

	if err := instance.SaveSites(sites); err != nil {
		t.Fatalf("save sites: %v", err)
	}
	loaded, err := instance.LoadSites()
	if err != nil {
		t.Fatalf("load sites: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != sites[0].ID {
		t.Fatalf("unexpected sites: %#v", loaded)
	}

	tempFiles, err := filepath.Glob(filepath.Join(baseDir, ".integterm-*.tmp"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(tempFiles) != 0 {
		t.Fatalf("temporary files were not cleaned up: %v", tempFiles)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "sites.json")); err != nil {
		t.Fatalf("sites file missing: %v", err)
	}
}
