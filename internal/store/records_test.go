package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"IntegTERM/internal/model"
)

func TestSitesAndTabsPersistCompleteDataInFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "before")
	s := New(dir)
	sites := []model.Site{{ID: "site", Host: "test.invalid", Password: "site-password-test", PPKPassphrase: "站台金鑰密語-test"}}
	tabs := []model.Tab{{ID: "tab", SiteID: "site", Password: "tab-password-test", PPKPassphrase: "分頁金鑰密語-test"}}
	if err := s.SaveSites(sites); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTabs(tabs); err != nil {
		t.Fatal(err)
	}
	// 直接解析磁碟檔案，確認備份本身包含完整資料。
	diskSites, err := readJSON[[]model.Site](filepath.Join(dir, "sites.json"))
	if err != nil || !reflect.DeepEqual(diskSites, sites) {
		t.Fatalf("站台檔案內容不完整：%v", err)
	}
	diskTabs, err := readJSON[[]model.Tab](filepath.Join(dir, "tabs.json"))
	if err != nil || !reflect.DeepEqual(diskTabs, tabs) {
		t.Fatalf("分頁檔案內容不完整：%v", err)
	}
	moved := filepath.Join(filepath.Dir(dir), "after")
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	restarted := New(moved)
	actualSites, err := restarted.LoadSites()
	if err != nil || !reflect.DeepEqual(actualSites, sites) {
		t.Fatalf("搬移後站台還原失敗：%v", err)
	}
	actualTabs, err := restarted.LoadTabs()
	if err != nil || !reflect.DeepEqual(actualTabs, tabs) {
		t.Fatalf("搬移後分頁還原失敗：%v", err)
	}
	sites[0].Name = "重新命名"
	sites[0].Password = "updated-password-test"
	tabs[0].PPKPassphrase = "updated-passphrase-test"
	if err := restarted.SaveSites(sites); err != nil {
		t.Fatal(err)
	}
	if err := restarted.SaveTabs(tabs); err != nil {
		t.Fatal(err)
	}
	actualSites, err = New(moved).LoadSites()
	if err != nil || !reflect.DeepEqual(actualSites, sites) {
		t.Fatalf("站台更新未保存：%v", err)
	}
	actualTabs, err = New(moved).LoadTabs()
	if err != nil || !reflect.DeepEqual(actualTabs, tabs) {
		t.Fatalf("分頁更新未保存：%v", err)
	}
}

func TestUnknownFieldsDoNotBlockFileLoadingAndEditing(t *testing.T) {
	for _, name := range []string{"sites.json", "tabs.json"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, name)
			original := []byte(`[{"id":"old","host":"test.invalid","credentialRef":"ignored-old-reference","futureField":true}]`)
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			s := New(dir)
			var save func() error
			if name == "sites.json" {
				records, err := s.LoadSites()
				if err != nil || len(records) != 1 || records[0].Host != "test.invalid" || records[0].Password != "" {
					t.Fatalf("站台檔案讀取失敗：%v", err)
				}
				records[0].Password = "reentered-test"
				save = func() error { return s.SaveSites(records) }
			} else {
				records, err := s.LoadTabs()
				if err != nil || len(records) != 1 || records[0].Host != "test.invalid" || records[0].Password != "" {
					t.Fatalf("分頁檔案讀取失敗：%v", err)
				}
				records[0].Password = "reentered-test"
				save = func() error { return s.SaveTabs(records) }
			}
			afterRead, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(afterRead, original) {
				t.Fatal("讀取不應改寫原檔")
			}
			if err := save(); err != nil {
				t.Fatal(err)
			}
			var persisted []map[string]json.RawMessage
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &persisted); err != nil {
				t.Fatal(err)
			}
			if len(persisted) != 1 || string(persisted[0]["password"]) != `"reentered-test"` {
				t.Fatal("重新輸入的密碼未存入檔案")
			}
			if _, ok := persisted[0]["credentialRef"]; ok {
				t.Fatal("新檔案仍保留舊引用")
			}
		})
	}
}

func TestUnreadableRecordsCannotBeOverwritten(t *testing.T) {
	for _, name := range []string{"sites.json", "tabs.json"} {
		for _, data := range []string{"{broken", `[{"id":"bad","port":"invalid"}]`} {
			t.Run(name+"/"+data, func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, name)
				if err := os.WriteFile(path, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
				s := New(dir)
				var err error
				if name == "sites.json" {
					err = s.SaveSites(nil)
				} else {
					err = s.SaveTabs(nil)
				}
				if err == nil {
					t.Fatal("無法讀取的檔案被空資料覆蓋")
				}
				after, err := os.ReadFile(path)
				if err != nil || string(after) != data {
					t.Fatal("原檔內容遭到修改")
				}
			})
		}
	}
}

func TestFailedSavePreservesPreviousFile(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	original := []model.Site{{ID: "site", Password: "old-secret-test"}}
	if err := s.SaveSites(original); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "sites.json"))
	if err != nil {
		t.Fatal(err)
	}
	s.writeRecords = func(string, any) error { return errors.New("模擬提交失敗") }
	if err := s.SaveSites([]model.Site{{ID: "site", Password: "new-secret-test"}}); err == nil {
		t.Fatal("儲存意外成功")
	}
	after, err := os.ReadFile(filepath.Join(dir, "sites.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("儲存失敗改動了原檔")
	}
	loaded, err := New(dir).LoadSites()
	if err != nil || !reflect.DeepEqual(loaded, original) {
		t.Fatalf("原始資料無法重新讀取：%v", err)
	}
}
