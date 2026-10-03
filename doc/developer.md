# IntegTERM 開發者指南

## 專案定位

IntegTERM 是以 Wails 為基礎的桌面應用，提供本地 GUI 形式的 SSH、SFTP、FTP、Telnet 與 Local Terminal 操作。AI 與自動化工具可透過 MCP 或本機 REST API 使用連線與檔案功能。

目前開發準則：

- Wails App GUI：提供本地使用者操作。
- MCP 與 REST API：提供 AI / automation 使用。
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
  - 應用入口；支援預設 GUI、`serve` 背景服務與 `mcp` stdio 模式。
- `internal/app/`
  - 應用協調層、Wails bind、MCP 與 REST API Server。
- `internal/session/`
  - 連線、終端、傳輸佇列與 log 管理；已依 client、transfer operation、transfer state、tab factory 等責任拆檔。
- `internal/transport/`
  - SFTP / FTP client 抽象與實作。
- `internal/store/`
  - `sites.json`、`tabs.json`、`config.json` 的本地檔案讀寫。
- `internal/boundedlog/`
  - 共用磁碟日誌容量限制與跨行程寫入鎖，避免 GUI 與背景服務累積無上限紀錄。
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

需要可自動下載 toolchain 的 Go、Node.js 22.12 以上，以及 Xcode Command Line Tools。主模組與專案內 systray 模組統一使用 Go 1.27.1；建置、開發與測試腳本依主模組的 `go.mod` 選用工具鏈，不受全域 GOTOOLCHAIN 設定影響。依 [Go 1.27 官方系統需求](https://go.dev/doc/go1.27#darwin)，應用程式最低支援 macOS 13；Go/CGO、Swift bridge 與 App plist 已同步此部署目標。前端使用 Vite 7，保留 Safari 15 輸出目標，可涵蓋 macOS 13 以上的 WebView。

### 啟動前端與 Wails 開發模式

`run.sh` 與 `build.sh` 透過 `scripts/ensure-wails.sh` 選用專案專用的 Wails CLI。腳本依 `go.mod` 選定的 Wails、Go 工具鏈與 `golang.org/x/tools` 版本建立 CLI，並核對執行檔的編譯資訊；任何版本改變時會重新建立工具。CLI 的依賴位於獨立暫存模組，不會改寫應用程式的模組設定。

工具預設快取於 `~/Library/Caches/IntegTERM/tools`（設定 `XDG_CACHE_HOME` 時使用該快取根目錄），也可透過 `INTEGTERM_TOOL_CACHE` 指定其他位置。首次啟動可能需要下載與編譯工具依賴；後續啟動重用已驗證的快取。專案不依賴 PATH 中其他專案安裝的 Wails CLI。

```bash
./run.sh
```

目前 `run.sh` 會額外處理：

- 先編譯 StoreKit 2 Swift bridge
- 注入 `DYLD_LIBRARY_PATH`
- 固定 `MACOSX_DEPLOYMENT_TARGET=13.0`
- 固定 `CGO_CFLAGS / CGO_LDFLAGS = -mmacosx-version-min=13.0`
- 在本機 `/tmp/integterm-dev.*` 建立開發鏡像，再執行 `wails dev`
- 以 checksum 同步原專案內容，只有檔案內容真正變更時才觸發 Vite / Wails rebuild
- 結束開發模式時清理 watcher、Vite、暫存 tray service 與開發鏡像

> 專案位於外接磁碟時，請勿直接執行 `wails dev`。外接磁碟可能產生 `._*` AppleDouble 檔，導致 Wails self-sign / codesign 失敗。請統一使用 `./run.sh` 或 `run.command`。

### 建置桌面應用

```bash
./build.sh
```

請使用上述專案入口，以確保 CLI、工具鏈及原生 bridge 一致。

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

- 本機 `cert/` 與 `data/`（存在時）

備份檔供本機還原使用，請自行妥善保管；公開原始碼套件與本機備份應分別製作。

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

### 已儲存連線的憑證

`sites.json`／`tabs.json` 直接保存站台、分頁、密碼與 PPK 密語，與原本的檔案儲存格式一致。GUI、背景服務與 MCP 都只讀寫資料檔案，沒有系統憑證儲存、查詢或自動遷移功能。

資料目錄使用 `0700`，JSON 使用 `0600` 權限。儲存透過交易鎖、暫存檔與原子取代，避免多個程序同時更新時遺失資料。多檔提交會先建立同樣為 `0600` 的 `.store-transaction.json` 回復紀錄，提交成功後移除；若程序中斷，下一次取得交易鎖時先還原完整舊狀態。JSON 內含密碼與密語；備份前請先正常關閉應用程式與背景服務，再複製完整資料目錄。

一般讀取不會改寫原檔；只有偵測到未完成交易時，會先依回復紀錄還原。未知欄位（包括舊版 `credentialRef`）直接忽略，不解析引用，也不查詢其他來源；檔案中沒有保存的密碼與 PPK 密語需在站台編輯畫面重新輸入並儲存。

檔案格式錯誤或無法讀取時保留原始資料，不回傳部分資料，也不允許空資料覆寫。啟動仍須成功讀取設定、站台和分頁才可寫回；GUI 可重新讀取，REST sites/tabs 在資料暫時不可用時回傳 503。儲存回歸測試使用暫存資料目錄。

### 站台編輯與標籤

編輯站台視窗使用雙欄排列；窄視窗切換單欄並允許捲動。標籤位於協定左側，可使用半形逗號、全形逗號或頓號分隔；儲存時去除多餘空白與重複項目。標籤以 `tags` 字串陣列存入 `sites.json`，舊資料沒有此欄位仍可正常載入。標籤變更也會觸發未儲存狀態提示。

### PPK 載入與本機診斷

PPK 是否需要密語由檔案的 `Encryption` 欄位決定。`none` 可留空，並忽略站台中殘留的舊密語；加密金鑰才會使用 PPK 密語。密語不裁切前後空白，也不以 SSH 登入密碼代替。只有公鑰、缺少私鑰的檔案會顯示對應提示。

HMAC 驗證失敗無法單獨證明密語錯誤，也可能來自檔案內容變動或損毀。程式保留完整性驗證，不略過檢查；介面不直接顯示原始雜湊值。

若其他工具可以使用同一份 PPK，可在本機執行：

```bash
./scripts/check-ppk.command
```

從原始碼執行時需要 Go，會依專案 `go.mod` 編譯暫存診斷程式；目前正式版附件提供 DMG 與 SHA-256 校驗檔，診斷腳本保留於原始碼。工具直接讀取指定路徑的 PPK；不查詢 Keychain、不讀保存的金鑰副本、不連線遠端站台。密語在終端隱藏輸入，僅透過標準輸入傳遞，不寫入命令參數、環境變數或檔案。回報時只需提供 PPK 版本、金鑰類型、加密方式及驗證結果，不需提供密語或金鑰內容。

請同時確認實際執行的 App 版本：修改或建置原始碼不會自動更新 `/Applications/IntegTERM.app`。9/12 發行版的 PPK 讀取包含保存副本的後備流程；本分支直接讀取指定檔案，因此不能把不同執行版本的結果視為同一次驗證。既有分頁保存自己的連線設定；更新站台後應從已儲存站台建立新連線進行驗證。

### 驗證

完整回歸檢查：

```bash
./scripts/test.sh
```

此指令會實際執行所有 Go 測試（包含 race detector）、`go vet`、封裝失敗回復測試、沙盒 preflight 純資料測試、Swift ThreadSanitizer 測試、前端行為測試與 TypeScript 檢查。它會把 StoreKit bridge 複製至暫存目錄，透過 linker rpath 載入，結束後移除；封裝測試使用模擬指令，不操作真實 Keychain 或簽章憑證，Swift 測試也不呼叫真實購買。

前端正式建置與依賴漏洞掃描：

```bash
cd frontend
npm run build
npm audit
```

Go 漏洞掃描：

```bash
GOTOOLCHAIN="$(awk '$1 == "go" { print "go" $2; exit }' go.mod)" go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

`go test -exec /usr/bin/true` 只會確認測試可以編譯，不能當作測試通過。`build.sh` / `run.sh` 已內建應用程式所需的動態庫處理。

### 高頻路徑與日誌上限

2026-10-03 的最佳化保留既有操作、UI、終端輸出順序、傳輸控制及儲存交易規則；日誌依新需求增加筆數上限。

- 終端完整 UTF-8 區塊及不含 OSC 的輸出可直接交給後續處理；跨區塊的 UTF-8／OSC 殘片仍保留獨立資料。128 KiB 回放緩衝重用既有配置，截斷仍遵循換行與 UTF-8 邊界，已回傳快照不受影響。
- 終端事件訂閱跟隨分頁及 session 的生命週期；更新工作目錄或重新排序分頁時沿用訂閱，相同路徑不建立新的 React 狀態。
- 檔案多選及拖曳使用 Set 判斷選取狀態，右鍵批次操作以 Map 查找檔案，保留原有選取順序。右鍵選單顯示時才查詢字型樣式。
- 傳輸佇列使用 ID 索引及有序連結，新增、更新、移除不再搬移或掃描全部項目；空佇列會釋放索引。相同的非終止進度不重複發送狀態事件。沒有 GUI 事件 context 的背景服務仍可透過 REST 讀取快照。
- 操作日誌保存在記憶體，最多保留最近 **1,000 筆**；`internal/session/transfer_state.go` 的 `maxLogEntries` 定義上限。滿載後以循環緩衝覆寫最舊紀錄，GUI／REST 仍依最新在前輸出。清空日誌會一併重設緩衝位置。這是筆數上限，每筆訊息內容維持完整。
- 磁碟崩潰日誌與 REST 錯誤共用 `service-crash.log`，上限 **5 MiB**。追加後將超限時，先清除舊內容再保存新紀錄；單筆超限保留尾端。GUI 與背景服務透過檔案鎖共同遵守上限，寫入後關閉檔案，避免 REST 重新啟動時累積控制代碼。macOS 安裝日誌只保留最近一次安裝執行內容。

固定資料基準測試：

```bash
env -u GOROOT GOTOOLCHAIN="$(awk '$1 == "go" { print "go" $2; exit }' go.mod)" \
  go test ./internal/session -run '^$' \
  -bench 'Benchmark(TerminalOutput|AppendLogs|AppendTransfers)' \
  -benchmem -benchtime=100ms -count=3
```

第一輪結果：Apple M4 Pro／darwin arm64／Go 1.27.1，三次量測取中位數；修改前以 `4dc9377` 原始碼搭配同一份 benchmark 執行。後續函式最佳化的最新數據見[函式級最佳化紀錄](function-optimization.md)：

| 路徑 | 修改前耗時 | 修改後耗時 | 修改前配置量 | 修改後配置量 |
| --- | ---: | ---: | ---: | ---: |
| 約 4 KiB 一般終端輸出 | 18.67 µs | 1.61 µs | 315,410 B | 2 B |
| 約 4 KiB ANSI 終端輸出 | 21.33 µs | 3.04 µs | 315,409 B | 4 B |
| 約 4 KiB 含工作目錄 OSC 的終端輸出 | 23.54 µs | 5.08 µs | 321,569 B | 10,279 B |
| 新增 1,000 筆日誌 | 3.33 ms | 0.145 ms | 34,683,592 B | 186,260 B |
| 新增 10,000 筆傳輸並取得快照 | 729.14 ms | 57.60 ms | 4,045,574,960 B | 6,094,888 B |

表內配置量為每次基準操作累計配置，並非常駐記憶體；終端一般輸出的少量平均配置來自緩衝首次擴充。依新上限新增 10,000 筆日誌後，實際保留 1,000 筆，本機量測約 1.38 ms、累計配置 546,474 B。這些結果只代表受測路徑，不等同整體 GUI 或網路傳輸速度。

驗證包含完整 `scripts/test.sh`、51 項前端行為測試、正式前端建置，以及日誌上限加入後的 session race／vet 檢查。新增 Smoke 案例涵蓋 UTF-8／OSC 分段、回放截斷、快照獨立性、日誌多次循環覆寫與清空、訂閱清理，以及 2,000 筆檔案的多選／拖曳。檔案面板與站台列表的代表性資料亦已比對修改前後靜態 HTML，結果一致；尚未以新版原生 App 進行人工 GUI 操作驗證。

函式級最佳化完成後，已再次通過完整 `scripts/test.sh`（含 54 項前端測試）及正式前端建置。函式範圍、效能與記憶體取捨、日誌上限及驗證細節記錄於[函式級最佳化紀錄](function-optimization.md)。

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

## MCP 與 REST API

MCP 提供本機 stdio 與 Streamable HTTP 兩種傳輸；REST API、StoreKit、GUI 與封裝流程使用各自的入口。

- 設定頁 `MCP > 本機` 顯示目前執行檔的絕對路徑，MCP client 使用 `command` 與 `args: ["mcp"]`；不開啟 GUI 或 tray，不必啟用 HTTP。
- stdio 先完成協定啟動與工具探索，首次查詢站台時才載入 JSON 檔案，資料鎖等待上限 2 秒。檔案無法讀取或資料忙碌時明確回報，RAM 操作仍可使用。
- 本機 stdio 提供 RAM workspace 與已儲存站台的 VFS。先呼叫 `vfs_workspace_info`，再以 `vfs_list` 探索；根資源 URI 為 `integterm-vfs://workspace/mcp`。
- 設定頁 `MCP > HTTP` 使用既有 REST 啟停與埠設定，Streamable HTTP endpoint 為 `http://127.0.0.1:<port>/mcp`；此模式另提供既有 REST 操作對應的 MCP tools。
- HTTP 與 REST 共用 loopback listener 及嚴格 Origin 檢查。`/mcp` 的實際 TCP 來源為 `127.0.0.1` 時免金鑰；其他來源與受保護的 REST 端點仍需 Bearer 驗證。不以 Host、Forwarded 或 X-Forwarded-For 判斷來源。stdio 與 HTTP 不共用同一個 RAM workspace；同一個 HTTP server 的 clients 則共用它的 workspace。
- 本機 `127.0.0.1` MCP client 設定不含驗證標頭；需要驗證的文件範例只含 `<YOUR_API_TOKEN>`，真 Token 僅在專屬顯示／複製操作提供。
- RAM 上限 32 MiB／4096 節點；chunk 暫存另外限 32 MiB／64 筆，路徑最多 4096 bytes／64 層，遠端掛載最多 64 個。遠端寫入先傳至暫存檔再提交；SFTP 覆寫需要 POSIX rename extension，不支援時保留原檔並回報錯誤。
- `update_config` 會替換完整設定，須先呼叫 `get_config`，修改後帶回全部欄位。
- MCP 依賴 [官方 Go SDK v1.8.0](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.8.0)，HTTP 與 stdio 單一 JSON 訊息均有大小上限。

### Restful API Server

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

`GET /api/status` 及實際來源為 `127.0.0.1` 的 `/mcp` 免金鑰，其餘端點需要 Bearer token。瀏覽器 Origin 只允許 localhost / `127.0.0.1`。

### 設定頁 Token 與文件操作

`設定 > MCP > HTTP` 會提供目前 API Token，供本機使用者建立 REST request：

- Token 預設遮罩，需主動按下眼睛圖示才會顯示。
- Token 可透過小型複製圖示寫入剪貼簿。
- 關閉設定視窗或切離 MCP HTTP 頁面後，前端會清除 Token 顯示狀態。
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
- `MCP` 頁面與 Markdown 輸出 / 複製
- MCP HTTP Token 預設遮罩，顯示、隱藏與複製操作正常
- 關閉並重新開啟設定視窗後，Token 會恢復遮罩
- API 文件與設定範例僅含 `<YOUR_API_TOKEN>`；填入專屬複製按鈕取得的 Token 後可正常使用
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

## 正式版發行

目前版本為 `1.26.1003`、Build `2336`，對應標籤 `v1.26.1003.2336`。本版整合高頻路徑與全專案函式級效率／記憶體最佳化，並加入操作日誌 1,000 筆、磁碟崩潰日誌 5 MiB 的保存上限；維持既有操作、功能與 UI。公開說明見[版本更新紀錄](release_1.26.1003.2336.md)，量測範圍與限制見[函式級最佳化紀錄](function-optimization.md)。

使用 `python3 scripts/release-macos.py --build` 產生正式發行包。發行環境由維護者在本機設定。
公開附件僅包含 DMG 與 `SHA256SUMS.txt`，內部建置及驗證紀錄不隨 Release 公開。

發行腳本使用 `INTEGTERM_CODESIGN_IDENTITY` 指定有效的 Developer ID Application 憑證，並以 `INTEGTERM_NOTARY_PROFILE` 指定既有 Keychain 公證設定；不將密碼或認證檔寫入原始碼。若已用 `./build.sh` 完成建置，可省略 `--build`，避免重新產生另一組 build 編號。

GitHub Release 使用 `Integrate Terminal 版本 (Build 編號)` 作為標題，內文採繁體中文並省略重複標題；正式版本不標記為 Pre-release，附件完整上傳後才設為 Latest。App、Git 標籤與附件檔名需使用同一組版本。發布前須確認 Developer ID 簽章、App／DMG 公證與票根、Gatekeeper、SHA-256，以及包內程式的 MCP Smoke。

`1.26.1003.2336` 發版已重新通過完整 `scripts/test.sh` 與正式建置。DMG 打包對照 YourDesk 流程，使用 ULMO 壓縮及 Applications 捷徑；App 與 DMG 均通過 Developer ID 簽章、Apple 公證、票根與 Gatekeeper。另完成 DMG 唯讀掛載、包內 App 與已驗證產物逐檔雜湊比對、版本／架構／最低系統版本核對，以及包內 MCP 啟動、10 項工具探索、RAM 檔案讀寫刪除與分塊 SHA-256 Smoke。該 Smoke 以系統 sandbox 禁止使用者資料目錄與網路存取。

更新流程由「關於」頁面的檢查更新啟動。下載時透過 `update:progress` 回報實際位元組、總大小與階段；前端以每次請求 ID 過濾過期事件。下載、檔案驗證及安裝準備分別顯示，不以計時器模擬百分比。保留檔案大小與 SHA-256 驗證，下載失敗可重試，macOS 安裝失敗時回復原 App。

設定已移除授權導覽與 Pro 解鎖頁面。

## 2026-10-01 深度檢查與相容修正

本輪檢查範圍包含啟動與背景服務、設定／站台／分頁儲存、SSH／SFTP／FTP／Telnet、終端輸出與生命週期、檔案傳輸、MCP／REST、前端非同步操作及更新安裝。保留既有功能、畫面與操作流程，延續本機 MCP 免金鑰規則。

已修正的可重現問題：

- 快速連續修改設定會由舊快照產生多筆完整寫入，導致前一筆修改遭覆蓋。前端改以欄位差異依序儲存；單筆失敗只撤回該筆，不阻斷後續設定。
- SSH／本機終端輸入與輸出共用鎖，阻塞的輸入可能讓輸出及關閉操作停滯。輸入使用獨立鎖，關閉直接中斷底層 I/O；Telnet 關閉也先中斷 socket 再清理狀態。
- SSH、Telnet 與本機終端自然結束後缺少資源回收。現在明確關閉連線、PTY 與登入計時器；本機終端先排空結束前的輸出，對持有 PTY 的殘留子程序採有界等待。
- SSH／SFTP／Telnet 手動拼接主機與連接埠，導致 IPv6 位址格式錯誤。統一採用標準位址組合方式。
- SSH／SFTP 原有逾時僅涵蓋 TCP 建立，握手可能永久阻塞。共用 SSH 撥接函式將期限涵蓋握手，完成後清除 deadline，保留長連線行為。
- 背景服務、更新輔助程式與外部程式啟動後缺少 Wait，可能留下未回收子程序。共用非同步啟動函式負責回收；系統匣的空裝置串流交由 os/exec 管理，避免描述元洩漏。
- 設定檔寫入缺少與站台、分頁相同的損毀保護。既有 JSON 無法讀取時拒絕覆寫，保留原始資料。

驗證涵蓋連續設定寫入與失敗重試、阻塞 Telnet 的關閉、IPv6 loopback 連線、PTY 釋放與尾端輸出、SSH 握手逾時與長連線、子程序回收、損毀設定檔保留，以及既有完整測試。測試使用暫存檔案與本機模擬伺服器，不讀取使用者私鑰或連線正式站台；不代表所有遠端伺服器與網路環境均已實測。

### 第二輪深度檢查

沿用上述範圍，追加檢查資料提交、傳輸取消、特殊檔案、完整路徑及更新失敗回復；維持既有介面與操作流程。

- 檔案交易原本僅提供跨程序互斥，回呼或後續寫入失敗仍會留下已寫入的部分資料。現在先暫存交易內容，回呼成功才提交；涉及多個 JSON 檔案時，以權限 `0600` 的 `.store-transaction.json` 保存原始內容。提交失敗或程序中斷後，下一次取得鎖的讀寫會先回復舊狀態。交易內可讀到自己的最新暫存值；不改變既有站台、分頁與設定檔格式。
- 上傳目錄原本會沿著指向祖先的符號連結反覆遞迴。現在以檔案身分追蹤目前路徑上的祖先，遇到循環即回報錯誤；一般目錄別名與檔案連結仍可上傳。Socket、FIFO 等特殊檔案在開啟前即拒絕，避免傳輸永久等待。
- 取消傳輸後，較晚抵達的進度更新可能將狀態改回執行中。共用狀態更新函式會檢查自身與父層取消狀態，保留取消結果。
- 批次刪除、移動及部分遠端路徑處理會裁掉完整路徑的空白，可能誤操作另一個名稱相似的檔案。現在只以空白檢查驗證空輸入，實際操作保留原始完整路徑。
- macOS 新版 App 驗證完成後，如果刪除舊備份失敗，原有退出清理可能把新版替換成已部分刪除的備份。現在以驗證完成作為提交點；提交後的備份清理失敗保留新版，複製或驗證失敗仍回復舊版。未成功排程的安裝腳本也會立即清除。

新增回歸測試涵蓋交易回呼失敗、第二次寫入失敗、子程序中斷後回復、無效回復紀錄、循環與非循環目錄連結、特殊檔案、延遲進度、尾端空白檔名，以及安裝複製失敗與備份清理失敗。安裝 Smoke 使用暫存 App 與模擬系統工具，不替換已安裝程式；交易中斷測試不等同於斷電耐久性驗證。

本輪驗證通過 `./scripts/test.sh`（Go race、go vet、工具鏈／打包／沙盒與原生橋接 Smoke、46 項前端測試及 TypeScript 型別檢查），以及 `frontend` 的 `npm run build`。

正式包另外完成 DMG 掛載、App／前端版本與 arm64 架構核對、SHA-256 校驗，以及直接執行包內程式的 MCP 啟動、工具探索與 RAM 檔案讀寫刪除 Smoke；此檢查不載入使用者站台。
