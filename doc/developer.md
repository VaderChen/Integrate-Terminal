# Integrate Terminal 開發指南

## 專案定位

Integrate Terminal 是以 Wails v2 建立的桌面應用，整合 SSH、SFTP、FTP、Telnet、本機終端、背景服務與本機 MCP Server。公開版本不包含 StoreKit、內購限制或 App Sandbox。

## 技術組成

- 後端：Go 1.23
- 桌面框架：Wails v2
- 前端：React 18、TypeScript、Vite
- 終端元件：xterm.js
- 傳輸協定：SFTP、FTP
- 終端協定：SSH、Telnet、本機 Shell

## 主要目錄

- `main.go`：GUI 與 `serve` 背景服務入口。
- `internal/app/`：Wails bind、應用協調、REST 與 MCP Server。
- `internal/session/`：連線、終端、傳輸佇列與執行狀態。
- `internal/transport/`：SFTP 與 FTP 實作。
- `internal/store/`：站台、分頁與設定持久化。
- `frontend/`：React 桌面 GUI。
- `third_party/systray/`：專案使用的 systray fork，沿用其原始授權。

## 開發

```bash
./run.sh
```

腳本會安裝缺少的 Wails CLI、整理 Go module，並在本機暫存目錄啟動 Wails 開發模式。

## 建置

```bash
./build.sh
```

輸出為 `build/bin/IntegTERM.app`。建置腳本預設建立非沙盒、ad-hoc 簽章的本機版本，不讀取 `cert/`，也不嵌入 Provisioning Profile；建置完成後可直接在本機開啟 App。

公開 Repository 不包含 Developer ID、Apple notarization 或 DMG 發布流程。一般開發與原始碼使用只需執行 `build.sh`；如需對外散布，應由散布者自行準備簽章與發布流程。

## 資料遷移

應用資料使用 `os.UserConfigDir()` 下的 `IntegTERM` 目錄。macOS 公開版首次啟動且目標目錄不存在時，會依序嘗試從下列來源搬移既有資料：

1. 舊 App Sandbox 容器 `~/Library/Containers/com.vader.integterm/Data/Library/Application Support/IntegTERM`
2. 舊開發目錄 `data/`

遷移範圍包含站台、分頁、設定、known hosts、PPK registry、PPK 副本、檔案存取 registry 與 REST API token；不搬移 PID 與 crash log。

## 公開前檢查

```bash
zsh -n build.sh run.sh install.sh backup.sh
gofmt -w ./internal ./main.go
go test -exec /usr/bin/true ./...
cd frontend && npm run build
```

不得提交 `cert/`、`data/`、`.env*`、`frontend/node_modules/`、建置產物、安裝包、簽章資產或測試用帳密。

## 授權策略

公開原始碼採 GPLv3。需要閉源整合或其他散布條款者，必須另行取得商業授權。為維持雙軌授權，在正式 Contributor License Agreement 完成前不合併外部程式碼 Pull Request。
