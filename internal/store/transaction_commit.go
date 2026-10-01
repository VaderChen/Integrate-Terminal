package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const transactionJournalName = ".store-transaction.json"

type transactionBackup struct {
	Name   string `json:"name"`
	Exists bool   `json:"exists"`
	Data   []byte `json:"data"`
}
type transactionJournal struct {
	Version int                 `json:"version"`
	Files   []transactionBackup `json:"files"`
}

// 呼叫端持有跨程序檔案鎖。多檔交易先保存還原紀錄，提交失敗或程序
// 中斷時，下一個讀寫者會先回復完整舊狀態，才讀取資料。
func (t *Transaction) commit() error {
	if len(t.order) == 0 {
		return nil
	}
	if len(t.order) == 1 {
		name := t.order[0]
		return t.store.writeRecords(filepath.Join(t.store.baseDir, name), t.pending[name])
	}
	journal := transactionJournal{Version: 1}
	for _, name := range t.order {
		data, err := os.ReadFile(filepath.Join(t.store.baseDir, name))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		journal.Files = append(journal.Files, transactionBackup{Name: name, Exists: err == nil, Data: data})
	}
	journalPath := filepath.Join(t.store.baseDir, transactionJournalName)
	if err := writeJSON(journalPath, journal); err != nil {
		return err
	}
	for _, name := range t.order {
		if err := t.store.writeRecords(filepath.Join(t.store.baseDir, name), t.pending[name]); err != nil {
			return errors.Join(err, t.store.recoverTransaction())
		}
	}
	if err := os.Remove(journalPath); err != nil {
		return errors.Join(err, t.store.recoverTransaction())
	}
	return nil
}

func (s *Store) recoverTransaction() error {
	journalPath := filepath.Join(s.baseDir, transactionJournalName)
	raw, err := os.ReadFile(journalPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var journal transactionJournal
	if err := json.Unmarshal(raw, &journal); err != nil {
		return fmt.Errorf("read transaction recovery record: %w", err)
	}
	if journal.Version != 1 || len(journal.Files) == 0 || len(journal.Files) > 3 {
		return fmt.Errorf("invalid transaction recovery record")
	}
	seen := map[string]bool{}
	// 完整驗證紀錄後才還原，不允許路徑或重複項目影響其他檔案。
	for _, file := range journal.Files {
		if (file.Name != "sites.json" && file.Name != "tabs.json" && file.Name != "config.json") || seen[file.Name] || (file.Exists && !json.Valid(file.Data)) {
			return fmt.Errorf("invalid transaction recovery entry")
		}
		seen[file.Name] = true
	}
	for _, file := range journal.Files {
		target := filepath.Join(s.baseDir, file.Name)
		if file.Exists {
			err = writeFileAtomically(target, file.Data)
		} else {
			err = os.Remove(target)
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
		}
		if err != nil {
			return fmt.Errorf("restore transaction file %s: %w", file.Name, err)
		}
	}
	return os.Remove(journalPath)
}
