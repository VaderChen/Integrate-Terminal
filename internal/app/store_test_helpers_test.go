package app

import "IntegTERM/internal/store"

// 測試與正式程式使用相同的檔案儲存路徑，資料目錄由各測試隔離。
func newAppTestStore(dir string) *store.Store {
	return store.New(dir)
}
