package store

import (
	"os"
	"path/filepath"
	"testing"

	"IntegTERM/internal/model"
)

func TestSaveConfigPreservesUnreadableExistingJSON(t *testing.T) {
	directory := t.TempDir()
	store := New(directory)
	file := filepath.Join(directory, "config.json")
	original := []byte(`{"theme":"dark",`)
	if err := os.WriteFile(file, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveConfig(model.Config{Theme: "light"}); err == nil {
		t.Fatal("corrupt settings overwritten")
	}
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("original config changed")
	}
}
