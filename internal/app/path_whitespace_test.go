package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBatchFileActionsPreserveTrailingWhitespace(t *testing.T) {
	for _, operation := range []string{"delete", "move"} {
		t.Run(operation, func(t *testing.T) {
			directory := t.TempDir()
			plain := filepath.Join(directory, "report")
			spaced := plain + " "
			for _, name := range []string{plain, spaced} {
				if err := os.WriteFile(name, []byte(name), 0600); err != nil {
					t.Fatal(err)
				}
			}
			a := regressionApp(t, nil)
			if operation == "delete" {
				if err := a.DeleteEntries("", "local", []string{spaced}); err != nil {
					t.Fatal(err)
				}
			} else {
				target := filepath.Join(directory, "dest")
				if err := os.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
				if err := a.MoveEntriesToDirectory("", "local", []string{spaced}, target); err != nil {
					t.Fatal(err)
				}
				if data, err := os.ReadFile(filepath.Join(target, "report ")); err != nil || string(data) != spaced {
					t.Fatalf("moved wrong name: %v", err)
				}
			}
			if data, err := os.ReadFile(plain); err != nil || string(data) != plain {
				t.Fatalf("unselected lookalike changed: %v", err)
			}
			if _, err := os.Stat(spaced); !os.IsNotExist(err) {
				t.Fatalf("selected source still exists: %v", err)
			}
		})
	}
}
