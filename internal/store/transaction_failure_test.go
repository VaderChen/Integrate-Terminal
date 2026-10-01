package store

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"IntegTERM/internal/model"
)

func TestTransactionCallbackFailureDoesNotCommit(t *testing.T) {
	s := New(t.TempDir())
	if err := s.SaveSites([]model.Site{{ID: "before"}}); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("later operation failed")
	err := s.WithTransaction(func(tx *Transaction) error {
		if err := tx.SaveSites([]model.Site{{ID: "after"}}); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	sites, err := s.LoadSites()
	if err != nil || len(sites) != 1 || sites[0].ID != "before" {
		t.Fatalf("failed transaction committed: %+v %v", sites, err)
	}
}

func TestTransactionSecondWriteFailureRestoresAllFiles(t *testing.T) {
	s := New(t.TempDir())
	if err := s.SaveSites([]model.Site{{ID: "before"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveConfig(model.Config{Theme: "dark"}); err != nil {
		t.Fatal(err)
	}
	originalSites, _ := os.ReadFile(filepath.Join(s.baseDir, "sites.json"))
	originalConfig, _ := os.ReadFile(filepath.Join(s.baseDir, "config.json"))
	writes := 0
	s.writeRecords = func(path string, value any) error {
		writes++
		if writes == 2 {
			return errors.New("simulated second write failure")
		}
		return writeJSON(path, value)
	}
	err := s.WithTransaction(func(tx *Transaction) error {
		if err := tx.SaveSites([]model.Site{{ID: "after"}}); err != nil {
			return err
		}
		return tx.SaveConfig(model.Config{Theme: "light"})
	})
	if err == nil {
		t.Fatal("injected write failure ignored")
	}
	for name, want := range map[string][]byte{"sites.json": originalSites, "config.json": originalConfig} {
		got, err := os.ReadFile(filepath.Join(s.baseDir, name))
		if err != nil || string(got) != string(want) {
			t.Fatalf("%s changed after failed commit: %v", name, err)
		}
	}
}

func TestInterruptedTransactionWorker(t *testing.T) {
	directory := os.Getenv("INTEGTERM_TRANSACTION_CRASH_FIXTURE")
	if directory == "" {
		return
	}
	s := New(directory)
	s.writeRecords = func(path string, value any) error {
		if err := writeJSON(path, value); err != nil {
			return err
		}
		os.Exit(23) // 模擬第一個檔案提交後程序中斷，不執行 defer。
		return nil
	}
	_ = s.WithTransaction(func(tx *Transaction) error {
		if err := tx.SaveSites([]model.Site{{ID: "after"}}); err != nil {
			return err
		}
		return tx.SaveConfig(model.Config{Theme: "light"})
	})
	t.Fatal("crash fixture did not exit")
}

func TestTransactionRecoveryAfterProcessExit(t *testing.T) {
	directory := t.TempDir()
	s := New(directory)
	if err := s.SaveSites([]model.Site{{ID: "before"}}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(directory, "sites.json"))
	cmd := exec.Command(os.Args[0], "-test.run=^TestInterruptedTransactionWorker$")
	cmd.Env = append(os.Environ(), "INTEGTERM_TRANSACTION_CRASH_FIXTURE="+directory)
	if err := cmd.Run(); err == nil {
		t.Fatal("worker did not fail")
	}
	info, err := os.Stat(filepath.Join(directory, transactionJournalName))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("missing private recovery record: %v", err)
	}
	sites, err := New(directory).LoadSites()
	if err != nil || len(sites) != 1 || sites[0].ID != "before" {
		t.Fatalf("recovery failed: %+v %v", sites, err)
	}
	got, _ := os.ReadFile(filepath.Join(directory, "sites.json"))
	if string(got) != string(before) {
		t.Fatal("recovery did not preserve exact original bytes")
	}
	for _, name := range []string{"config.json", transactionJournalName} {
		if _, err := os.Stat(filepath.Join(directory, name)); !os.IsNotExist(err) {
			t.Fatalf("unexpected file after rollback: %s %v", name, err)
		}
	}
}

func TestTransactionReadsOwnStagedSnapshot(t *testing.T) {
	s := New(t.TempDir())
	err := s.WithTransaction(func(tx *Transaction) error {
		sites := []model.Site{{ID: "first"}}
		if err := tx.SaveSites(sites); err != nil {
			return err
		}
		sites[0].ID = "mutated-without-save"
		staged, err := tx.LoadSites()
		if err != nil {
			return err
		}
		if staged[0].ID != "first" {
			t.Fatal("staged snapshot aliases caller memory")
		}
		staged[0].ID = "second"
		return tx.SaveSites(staged)
	})
	if err != nil {
		t.Fatal(err)
	}
	sites, err := s.LoadSites()
	if err != nil || len(sites) != 1 || sites[0].ID != "second" {
		t.Fatalf("wrong committed snapshot: %+v %v", sites, err)
	}
}

func TestInvalidRecoveryRecordCannotModifyFiles(t *testing.T) {
	s := New(t.TempDir())
	if err := s.SaveSites([]model.Site{{ID: "before"}}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(s.baseDir, "sites.json"))
	journal := transactionJournal{Version: 1, Files: []transactionBackup{
		{Name: "sites.json", Exists: true, Data: []byte(`[]`)},
		{Name: "../outside", Exists: true, Data: []byte(`[]`)},
	}}
	if err := writeJSON(filepath.Join(s.baseDir, transactionJournalName), journal); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadSites(); err == nil {
		t.Fatal("invalid recovery accepted")
	}
	got, _ := os.ReadFile(filepath.Join(s.baseDir, "sites.json"))
	if string(got) != string(before) {
		t.Fatal("partially applied invalid recovery record")
	}
}
