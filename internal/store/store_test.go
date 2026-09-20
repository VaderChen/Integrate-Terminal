package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"IntegTERM/internal/model"
)

func TestWriteJSONAtomically(t *testing.T) {
	baseDir := t.TempDir()
	instance := newTestStore(baseDir)
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

func TestPrivatePermissionsAlsoRepairExistingFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions")
	}
	dir := filepath.Join(t.TempDir(), "profile")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sites.json", "tabs.json", "config.json", "rest-api.token"} {
		data := []byte("{}")
		if name == "sites.json" || name == "tabs.json" {
			data = []byte("[]")
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	s := newTestStore(dir)
	if err := s.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSites([]model.Site{{Password: "fake-only"}}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "sites.json", "tabs.json", "config.json", "rest-api.token", ".store.lock"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		want := os.FileMode(0600)
		if name == "" {
			want = 0700
		}
		if info.Mode().Perm() != want {
			t.Errorf("%q mode=%o want %o", name, info.Mode().Perm(), want)
		}
	}
}

func TestStoreTransactionWorker(t *testing.T) {
	dir := os.Getenv("INTEGTERM_TEST_TRANSACTION_DIR")
	if dir == "" {
		return
	}
	s := newTestStore(dir)
	for i := 0; i < 12; i++ {
		if _, err := s.UpdateConfig(func(cfg *model.Config) error { cfg.WindowX++; return nil }); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStoreTransactionsSerializeDifferentProcesses(t *testing.T) {
	dir := t.TempDir()
	s := newTestStore(dir)
	if err := s.SaveConfig(model.Config{}); err != nil {
		t.Fatal(err)
	}
	commands := make([]*exec.Cmd, 4)
	for i := range commands {
		commands[i] = exec.Command(os.Args[0], "-test.run=^TestStoreTransactionWorker$")
		commands[i].Env = append(os.Environ(), "INTEGTERM_TEST_TRANSACTION_DIR="+dir)
		if err := commands[i].Start(); err != nil {
			t.Fatal(err)
		}
	}
	for _, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WindowX != 48 {
		t.Fatalf("lost concurrent updates: got %d want 48", cfg.WindowX)
	}
}
