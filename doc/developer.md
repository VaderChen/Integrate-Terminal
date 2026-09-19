# IntegTERM Developer Guide

## 專案定位

IntegTERM 是以 Wails 為基礎的桌面應用，提供本地 GUI 形式的 SSH、SFTP、FTP、Telnet 與 Local Terminal 操作。同時，專案也內建一組本機 `Restful API Server`，目標是提供 AI / 自動化整合使用。

目前開發準則：

- Wails App GUI：提供本地使用者操作。
- Restful API：提供 AI / automation 使用。
- 兩者可以共用底層功能，但不應混成同一層語意。

## 技術組成

- 後端：Go
- 桌面框架：Wails v2
- 前端：React 18 + TypeScript + Vite
- macOS 內購：StoreKit 2（Swift bridge）
- 終端元件：xterm.js
- 傳輸協定：SFTP、FTP
- 終端協定：SSH、Telnet、本地 shell

## 主要目錄

- `main.go`
  - 應用入口；支援預設 GUI 模式與 `serve` 背景服務模式。
- `internal/app/`
  - 應用協調層、Wails bind 與 Restful API Server。
- `internal/session/`
  - 連線、終端、傳輸佇列與 log 管理；已依 client、transfer operation、transfer state、tab factory 等責任拆檔。
- `internal/transport/`
  - SFTP / FTP client 抽象與實作。
- `internal/store/`
  - `sites.json`、`tabs.json`、`config.json` 的本地持久化。
- `frontend/`
  - 桌面 GUI 前端；語系、終端工具與站台操作已拆成獨立模組與 hooks。
- `internal/purchase/`
  - 授權狀態、`pro_unlock` 規則與 macOS StoreKit 2 bridge。
- `internal/trayservice/`
  - 背景服務模式、tray 狀態與功能選單協調。
- `internal/trayicon/`
  - tray icon 資產。
- `third_party/systray/`
  - 專案內 fork 的 systray，已針對 macOS 自訂 popover panel 與 tray title 排版。
- `doc/`
  - 專案文件與更新紀錄。

## 本機開發

### 啟動前端與 Wails 開發模式

```bash
./run.sh
```

目前 `run.sh` 會額外處理：

- 先編譯 StoreKit 2 Swift bridge
- 注入 `DYLD_LIBRARY_PATH`
- 固定 `MACOSX_DEPLOYMENT_TARGET=12.0`
- 固定 `CGO_CFLAGS / CGO_LDFLAGS = -mmacosx-version-min=12.0`
- 在本機 `/tmp/integterm-dev.*` 建立開發鏡像，再執行 `wails dev`
- 以 checksum 同步原專案內容，只有檔案內容真正變更時才觸發 Vite / Wails rebuild
- 結束開發模式時清理 watcher、Vite、暫存 tray service 與開發鏡像

> 專案位於外接磁碟時，請勿直接執行 `wails dev`。外接磁碟可能產生 `._*` AppleDouble 檔，導致 Wails self-sign / codesign 失敗。請統一使用 `./run.sh` 或 `run.command`。

### 建置桌面應用

```bash
./build.sh
```

備註：

```bash
wails build -clean
```

- `build/darwin/Info.plist` 與 `build/darwin/Info.dev.plist` 的 `CFBundleIdentifier` 已固定為 `com.vader.integterm`
- 不應再使用 `com.wails.IntegTERM`
- `build.sh` 會先編譯 `internal/purchase/swift/StoreKit2Bridge.swift`
- 打包後會把 `libintegtermstorekit2.dylib` 放進 App bundle 的 `Contents/Frameworks`
- Wails 產生 bindings 時也會注入對應的 `DYLD_LIBRARY_PATH`，避免臨時 binary 找不到原生 bridge
- Wails production build 與 codesign 會在本機暫存目錄完成，再複製回專案目錄
- 複製前後會移除 `._*`、`.DS_Store` 與舊 `_CodeSignature`

### 安裝與重新啟動

執行 `./install.sh` 或 `install.command`，會先準備新版，再自動關閉 IntegTERM、安裝並重新啟動。以 `INSTALL_TARGET_DIR` 可指定安裝目錄（預設 `/Applications`）。

安裝時會暫存舊版為 `.bak`，新版啟動指令成功後才刪除；替換失敗會嘗試還原並啟動舊版。若新版啟動指令失敗，會顯示保留的備份路徑。

### 建立原始碼備份

```bash
./backup.command
```

或：

```bash
./backup.sh
```

輸出格式：

- `IntegTERM_YYYYMMDD.zip`

目前會排除：

- `build/`
- `frontend/node_modules/`
- `frontend/dist/`
- `.DS_Store`、`._*`
- `*.pkg`、`*.dmg`、`*.zip`

目前會保留：

- `cert/`

### 啟動背景服務模式

```bash
go run . serve
```

背景服務模式會：

- 啟動 tray icon / custom panel
- 啟動本機 Restful API Server
- 提供 `Open Main Window` 與 `Quit Background Service`
- 讓 GUI 在啟動時附著到既有 background service

### GUI 首屏載入原則

前端啟動時會先呼叫 `Bootstrap()` 取得站台、分頁、設定、傳輸與 log 等輕量狀態，完成後立即顯示主介面。為避免首頁目錄或 StoreKit 偶發延遲拖住站台列表：

- `Bootstrap()` 不同步列舉本機或遠端檔案。
- 恢復分頁後，再由 `ListLocal()` / `ListRemote()` 背景載入檔案列表。
- `GetPurchaseStatus()` 在首屏顯示後背景查詢；查詢期間先使用本地 fallback 狀態。
- Bootstrap 或後續查詢失敗時，不應讓整個 GUI 永久停留在 loading 畫面。

### 驗證

前端：

```bash
cd frontend
npm run build
```

後端：

```bash
go test -exec /usr/bin/true ./...
go test -race ./internal/session ./internal/store
go vet ./...
```

說明：

- `go test -exec /usr/bin/true ./...` 用於確認所有 package 與測試均可編譯。
- `internal/session` 與 `internal/store` 可直接執行並進行 race 測試。
- 若要實際執行 `internal/app` 測試，需先 `go test -c`，再用 `install_name_tool -add_rpath "$PWD/internal/purchase/native"` 加入 StoreKit bridge rpath。
- `build.sh` / `run.sh` 已內建應用程式所需的動態庫處理。

## 授權與內購

### 產品型態

- 目前只規劃一個內購商品：`pro_unlock`
- 類型：`Non-Consumable`
- 用途：解除免費版的連線數量限制

### 目前規則

- Free：最多 2 個連線
- Pro：無限制

這個限制由後端統一控制，不依賴前端單獨判斷。

### 主要檔案

- `internal/purchase/service.go`
  - 購買服務抽象。
- `internal/purchase/provider_darwin.go`
  - macOS provider 入口。
- `internal/purchase/storekit_bridge_darwin.go`
  - Go / cgo 與原生 bridge 的接點。
- `internal/purchase/swift/StoreKit2Bridge.swift`
  - StoreKit 2 實作。

### StoreKit 2 設計原則

- 不再使用 StoreKit 1 transaction observer 當長期方案
- 以 `Product.products(for:)` 取得商品
- 以 `product.purchase()` 進行購買
- 以 `Transaction.currentEntitlements` 判斷目前授權
- 以 `AppStore.sync()` 進行還原購買
- Go 層只吃 JSON 結果，不直接處理 Apple 原生交易物件

## Restful API Server

### 用途

Restful API Server 主要給 AI / automation 使用，不是桌面 GUI 的替代品。

### 啟用方式

可於設定頁開關啟用，也可透過儲存設定讓：

- `restServerEnabled`
- `restServerPort`

寫入 `config.json`

目前行為：

- 若 GUI 啟動時偵測到既有 background service，會附著到既有服務，不自行搶占同一個 port。
- 只有 `restServerEnabled=true` 時，GUI 啟動才會自動拉起 `serve` 背景服務。
- background service 與 GUI 共用同一份 `config.json`、`sites.json`、`tabs.json`。

### 預設設定

- Host：`127.0.0.1`
- Port：`18080`
- Authentication：`Authorization: Bearer <token>`
- Token 檔案：App data 目錄內的 `rest-api.token`

除 `GET /api/status` 外，其餘端點都需要 Bearer token。瀏覽器 Origin 只允許 localhost / `127.0.0.1`。

### 設定頁 Token 與文件操作

`設定 > AI Skill` 會提供目前 API Token，供本機使用者建立 REST request：

- Token 預設遮罩，需主動按下眼睛圖示才會顯示。
- Token 可透過小型複製圖示寫入剪貼簿。
- 關閉設定視窗或切離 AI Skill 頁面後，前端會清除 Token 顯示狀態。
- Markdown 複製與匯出使用小型圖示按鈕，並保留 tooltip 與 accessibility label。
- Token 仍只由既有 Wails bind `GetRESTServerToken()` 提供，不新增免驗證 HTTP 端點。

### 文件端點

```text
GET /api/docs.md
GET /api/status
GET /api/operations/{id}
```

### 核心 API 類別

#### SSH

- `POST /api/ssh/execute`
- `POST /api/tabs/ssh`
- `POST /api/terminal/input`
- `GET /api/terminal/output`
- `POST /api/terminal/resize`
- `POST /api/terminal/close`

#### SFTP

- `POST /api/tabs/file`
- `GET /api/files/remote`
- `POST /api/files/upload`
- `POST /api/files/download`
- `GET /api/operations/{id}`
- `GET /api/sftp/stat`
- `POST /api/sftp/mkdir`
- `POST /api/sftp/rename`
- `POST /api/sftp/delete`

#### Local Terminal

- `POST /api/tabs/local`
- `POST /api/terminal/input`
- `GET /api/terminal/output`
- `POST /api/terminal/resize`
- `POST /api/terminal/close`

### 大檔案上下載

`POST /api/files/upload` 與 `POST /api/files/download` 不會同步等待傳輸完成，而是立即回傳 `202 Accepted`：

```json
{
  "operation": {
    "id": "operation-id",
    "kind": "upload",
    "status": "queued"
  }
}
```

呼叫端應依 response 的 `Location` 或 operation ID，每秒查詢：

```text
GET /api/operations/{id}
```

直到狀態變成 `done` 或 `failed`。詳細即時進度可另外查詢 `GET /api/transfers`。

## 發版前建議檢查

### 1. 編譯與資產

- `go build ./...`
- `cd frontend && npm run build`
- 確認 `frontend/dist/` 已更新

### 2. 設定與文件

- 確認 `doc/zh-TW/update_log_*.log`
- 確認 `doc/en/update_log_*.log`
- 確認 `doc/jp/update_log_*.log`
- 確認 `doc/developer.md` 與 API 描述一致

### 3. GUI 基本檢查

- 站台建立 / 編輯 / 刪除
- SFTP 連線與遠端檔案列舉
- SSH Terminal 開啟與輸入
- Local Terminal 快捷鍵
- 語言切換是否立即生效
- 檔案列表拖拉到目標資料夾
- `AI Skill` 頁面與 Markdown 輸出 / 複製
- AI Skill Token 預設遮罩，顯示、隱藏與複製操作正常
- 關閉並重新開啟設定視窗後，Token 會恢復遮罩
- API 文件中的 Bearer token 與 curl 範例可正常使用
- 大檔案上下載會立即回 `202`，operation 可查到最終狀態
- `開啟主視窗`
- `結束背景服務`
- 從 Tray 開啟主視窗後新增站台，關閉 Tray / App 再啟動，站台仍存在
- 多個 SSH / SFTP TAB 快速切換時，不會顯示 OSC / ANSI 操作碼
- 冷啟動時站台列表不需等待本機目錄列舉或 StoreKit 查詢
- 授權頁顯示 `Free / 最大連線數 2 個` 或 Pro 狀態
- Free 狀態下第 3 個連線會被阻擋
- 購買成功後可超過 2 個連線
- 還原購買後可重新解除限制

### 3.1 Tray / 背景服務檢查

- `go run . serve` 後可在 macOS menu bar 看到 tray
- tray title 顯示 `ACT` 與背景連線數
- 點擊 tray 可開啟 custom popover panel
- `狀態` / `功能` 卡片顯示正常
- `後端服務` 卡片可正確顯示目前 base URL
- 切換語言後 tray panel 文案同步更新

### 4. API 基本檢查

- `GET /api/status`
- `GET /api/docs.md`
- `POST /api/ssh/execute`
- `GET /api/files/remote`
- `POST /api/sftp/mkdir`
- `POST /api/files/upload`
- `GET /api/operations/{id}`

### 5. 內購基本檢查

- `pro_unlock` 商品在 App Store Connect 已建立
- Sandbox 帳號可正常登入
- 點擊 `購買 Pro` 會正確跳出 Apple 購買流程
- 取消購買時不應誤解鎖
- `還原購買` 能恢復已購買授權
- App 重啟後仍能從 StoreKit 2 entitlement 正確恢復狀態

## 後續重構方向

目前 Restful API 仍有部分端點沿用 GUI 導向的 `tabId` / `sessionId` 模型。現況另外還有三個實務上的架構點：

- background service 與 GUI 目前仍是兩個 `App` 實例，靠共享 store 與 REST attach 協作；背景服務關閉時不再回寫舊站台，REST 站台操作前會重載最新 store。
- tray custom panel 是建立在 fork 過的 `third_party/systray` 上，macOS 原生排版已被客製化。
- StoreKit 2 目前已可處理購買 / 還原 / entitlement 查詢，後續仍可視需要補強 receipt / server-side 驗證。

後續若要更明確區分 GUI 與 AI / background service 邊界，建議逐步重構成：

- GUI 用 `tabs`
- AI API 用 `connection/session`
- terminal output 改為支援 offset / cursor
- SSH / SFTP 提供更高階、一次一結果的 API
- service state 與 GUI state 進一步收斂，減少雙實例記憶體狀態差異

這份文件定位為發版前快速交接與開發者維護參考。
