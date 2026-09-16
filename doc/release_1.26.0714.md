# IntegTERM 1.26.0714 發布檢查清單

## 版本資訊

- Product Version：`1.26.0714`
- 預定發布日期：`2026-07-14`
- Bundle ID：`com.vader.integterm`
- Team ID：`YOUR_TEAM_ID`
- Minimum macOS：`12.0`
- App Category：Developer Tools
- In-App Purchase：`pro_unlock`（Non-Consumable）

版本來源必須一致：

- `wails.json`
- `internal/version/version.json`
- `build/bin/IntegTERM.app/Contents/Info.plist`

`build.sh` 會依執行日期更新 product version 與 build number。本文件以 2026-07-14 建置產生的 `1.26.0714` 為準；若改日重新建置，必須同步更新版本來源、三語更新紀錄與本檢查清單。

## 1. 原始碼與測試

```bash
go mod tidy
go test -exec /usr/bin/true ./...
go test -race ./internal/session ./internal/store
go vet ./...
npm --prefix frontend run build
```

`internal/app` 測試需要 StoreKit bridge rpath：

```bash
TEST_BIN="$(mktemp -u /tmp/integterm-app-test.XXXXXX)"
go test -c -o "$TEST_BIN" ./internal/app
install_name_tool -add_rpath "$PWD/internal/purchase/native" "$TEST_BIN"
"$TEST_BIN" -test.v
rm -f "$TEST_BIN"
```

確認：

- [ ] Go package 全部可編譯
- [ ] Session / Store race test 通過
- [ ] App 測試實際執行通過
- [ ] TypeScript / Vite build 通過
- [ ] Vite 沒有 chunk size 警告

## 2. GUI Smoke Test

- [ ] 新增、編輯、複製、刪除站台
- [ ] 新增 / 編輯站台使用 dialog 顯示
- [ ] SFTP / FTP 可連線並列出遠端檔案
- [ ] SSH / Telnet / Local Terminal 可建立、輸入、縮放與關閉
- [ ] 多個 SSH / SFTP TAB 快速切換不會顯示 OSC / ANSI 控制碼
- [ ] Local Terminal 按鈕可正常開啟終端
- [ ] 上傳、下載、暫停、繼續、取消與清除傳輸
- [ ] 大檔案傳輸期間 UI 不會凍結
- [ ] 語言、主題、字型與視窗位置設定正常
- [ ] AI Skill 頁面不再顯示 beta
- [ ] 冷啟動連續測試至少 5 次，站台列表不需等待檔案目錄或 StoreKit 即可顯示

## 3. Tray / 背景服務

- [ ] `./run.sh` 啟動後 Vite 不會循環重新啟動
- [ ] Tray Icon 可開啟主視窗
- [ ] 從 Tray 開啟 App 後新增站台
- [ ] 關閉主視窗與背景服務後重新啟動，新增站台仍存在
- [ ] Tray 顯示背景連線數與 REST server 狀態
- [ ] 結束開發模式後不留下 Vite、暫存 tray service 或 `/tmp/integterm-dev.*`

## 4. AI Skill / REST API

- [ ] `GET /api/status` 可在未帶 token 時讀取
- [ ] 其他 API 未帶 token 時回傳 `401`
- [ ] AI Skill Markdown 包含正確 Bearer token 與 curl 範例
- [ ] AI Skill 頁面可顯示／隱藏並複製 API Token，預設維持遮罩
- [ ] 關閉再開啟設定視窗後，API Token 恢復遮罩狀態
- [ ] Token、Markdown 複製與匯出圖示皆有正確 tooltip 且可操作
- [ ] 外部網頁 Origin 無法呼叫本機 API
- [ ] `POST /api/files/upload` 立即回傳 `202 Accepted`
- [ ] `POST /api/files/download` 立即回傳 `202 Accepted`
- [ ] Response 包含 `Location` 與 `Retry-After`
- [ ] `GET /api/operations/{id}` 最終回傳 `done` 或 `failed`
- [ ] `GET /api/transfers` 可查詢傳輸進度

## 5. Free / Pro / StoreKit 2

- [ ] Free 狀態最多可開啟 2 個可見 TAB
- [ ] 第 3 個 TAB 會顯示限制提示
- [ ] `pro_unlock` 商品可從 Sandbox App Store 載入
- [ ] 購買成功後可超過 2 個 TAB
- [ ] 取消購買不會誤解鎖
- [ ] 還原購買可恢復 Pro
- [ ] App 重啟後 entitlement 仍可恢復
- [ ] StoreKit entitlement 查詢較慢時，站台列表與主介面仍可先操作

## 6. App Sandbox 必查項目

提交 App Store 前，必須在實際 sandbox/TestFlight build 驗證：

- [ ] SSH known_hosts 的讀寫流程可用
- [ ] 使用者選取的本機目錄具有持續可用的存取權
- [ ] SFTP / FTP 上下載可存取使用者選取目錄
- [ ] Local Terminal / PTY 在 Sandbox build 的實際行為符合預期
- [ ] 開啟與執行本機檔案的功能符合 Sandbox 規範
- [ ] Tray background service 在 Sandbox / TestFlight 可正常啟動

這些功能涉及 `~/.ssh`、本機 shell、檔案權限與背景程序，不可只用開發版結果判定 App Store build 可用。

## 7. 正式打包

一般 production app：

```bash
./build.sh
codesign --verify --deep --strict --verbose=2 build/bin/IntegTERM.app
```

App Store pkg：

```bash
./package-appstore.sh
pkgutil --check-signature build/bin/IntegTERM.pkg
```

確認：

- [ ] build 使用 production mode，未包含 `-debug` 或 `-devtools`
- [ ] App bundle 內沒有 `._*`、`.DS_Store`
- [ ] `libintegtermstorekit2.dylib` 位於 `Contents/Frameworks`
- [ ] provisioning profile 與 `com.vader.integterm` 相符
- [ ] entitlements 包含 App Sandbox、network client、user-selected read/write
- [ ] app codesign 驗證通過
- [ ] pkg 簽章驗證通過
- [ ] `build/bin/IntegTERM.app/Contents/Info.plist` 顯示 `1.26.0714`
- [ ] 若發布 DMG，先重建 `dist/dmg-root`；不可使用仍標示 `1.26.0711` 的舊產物

## 8. App Store Connect

- [ ] 上傳新的 pkg
- [ ] 選擇正確 build
- [ ] 填入繁中、英文、日文更新說明
- [ ] 確認 Export Compliance：`ITSAppUsesNonExemptEncryption=false`
- [ ] 確認 `pro_unlock` 已送審或可與版本一起審核
- [ ] 附上測試帳號與需要的審核說明
- [ ] 說明 App 使用 SSH / SFTP / FTP / Telnet / Local Terminal 的用途

## 發布文件

- `doc/zh-TW/update_log_1.26.0714.log`
- `doc/en/update_log_1.26.0714.log`
- `doc/jp/update_log_1.26.0714.log`
- `doc/developer.md`
- `doc/review.md`
