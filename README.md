<div align="center">
  <img src="assets/appicon.png" alt="Integrate Terminal icon" width="128" />
  <h1>Integrate Terminal</h1>
  <p>以 Go、Wails、React 與 TypeScript 建立的跨協定桌面終端與檔案傳輸工具。</p>
</div>

<p align="center">
  <a href="README.md">繁體中文</a> |
  <a href="README.en.md">English</a> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.ko.md">한국어</a>
</p>

## 功能

- SSH、Telnet 與本機 Shell 終端（本機 Shell 支援 macOS 與 Linux）。
- SFTP、FTP 檔案瀏覽及傳輸佇列。
- 站台分組、分頁恢復、ZIP 備份與還原。
- 本機 MCP Streamable HTTP Server，供 AI 與自動化工具整合。
- 背景服務與系統列狀態控制。
- 支援在設定中手動檢查 GitHub Release，並於每日隨機時段自動檢查一次；有更新時可驗證並開啟適用平台的更新檔。
- 英文、日文、韓文、繁體中文與簡體中文介面。

## 公開版本

公開版本不使用 StoreKit、不限制連線數，也不啟用 App Sandbox。程式可存取目前登入帳號原本就有權限的檔案與目錄；macOS 的隱私保護目錄仍可能由系統要求使用者授權。

程式會在第一次啟動時嘗試搬移舊沙盒版的站台、設定、known hosts、PPK 副本與 REST API token。公開原始碼不包含任何 Apple 簽章憑證、Provisioning Profile、私鑰或個人站台資料。

## MCP 整合

IntegTERM 內建標準 MCP Streamable HTTP Server。支援 MCP 的 AI 或自動化客戶端可以管理已儲存站台、開啟 SSH、Telnet、SFTP、FTP 與 macOS／Linux 本機終端分頁、執行 SSH 指令、讀寫互動式終端、處理檔案傳輸，以及查詢傳輸佇列與執行紀錄。

### 啟用 MCP Server

1. 啟動 IntegTERM，開啟「設定」→「MCP」。
2. 確認監聽埠與 IP 白名單。預設埠為 `18080`，白名單為 `127.0.0.1`。
3. 開啟 MCP Server；預設端點為 `http://127.0.0.1:18080/mcp`。

### MCP 客戶端設定

```json
{
  "mcpServers": {
    "integterm": {
      "type": "streamable-http",
      "url": "http://127.0.0.1:18080/mcp"
    }
  }
}
```

MCP 端點不需要 API token 或自訂驗證標頭，存取控制完全依賴來源 IP／CIDR 白名單。除非受信任的遠端客戶端確實需要連線，否則請維持預設的 `127.0.0.1`，不要加入過大的網段。App 內的 MCP 頁面會顯示目前連線位置與依執行狀態產生的完整工具文件；客戶端應先透過 `tools/list` 取得實際工具結構。

## 開發環境

- macOS 12 或更新版本（Apple Silicon）、Windows 10/11 x64，或 Linux x64
- Go 1.23 或更新版本
- Node.js 與 npm
- macOS 建置需要 Xcode Command Line Tools
- Linux 建置需要 GTK3、WebKitGTK、AppIndicator 與 `pkg-config`
- 專案腳本會依 `go.mod` 自動使用對應版本的 Wails v2 CLI

## 啟動開發模式

```bash
git clone https://github.com/VaderChen/Integrate-Terminal.git
cd Integrate-Terminal
./run.sh
```

`run.sh` 會在本機暫存目錄建立開發鏡像，避免外接磁碟產生的 AppleDouble 檔案干擾 Wails 建置。

## 建置桌面程式

### macOS Apple Silicon

```bash
./build.sh
```

輸出位於 `build/bin/IntegTERM.app`。預設採 ad-hoc 簽章且不啟用 App Sandbox，建置完成後可直接在本機開啟使用。

### Windows x64

```powershell
powershell -ExecutionPolicy Bypass -File .\build-windows.ps1
```

輸出位於 `build\bin\IntegTERM.exe`。腳本只建立 x64 執行檔，不建立安裝程式或簽章。

### Linux x64

```bash
./build-linux.sh
```

輸出位於 `build/bin/IntegTERM`。腳本會偵測 WebKitGTK 與 AppIndicator 版本，只建立 x64 執行檔，不建立 AppImage、DEB 或 RPM。

GitHub 公開版本不包含 Developer ID、Apple notarization、DMG 或其他平台的發布封裝流程；散布者須自行處理簽章、安裝程式與發布要求。

## 資料與安全

應用資料位於各平台 `os.UserConfigDir()` 下的 `IntegTERM` 目錄，例如 macOS 的 `~/Library/Application Support/IntegTERM`、Windows 的 `%AppData%\IntegTERM`，以及 Linux 的 `~/.config/IntegTERM`。站台密碼及 PPK passphrase 目前會保存在本機站台資料與站台備份 ZIP 中，請限制檔案權限並妥善保管備份。REST/MCP 服務預設只綁定 `127.0.0.1`，啟用對外來源前請正確設定 IP 白名單。

請勿提交 `cert/`、`data/`、`.env*`、安裝包、簽章資產或任何包含真實帳密的檔案。安全問題請參閱 [SECURITY.md](SECURITY.md)。

## 授權

本專案採雙軌授權：

1. 開放原始碼使用遵循 [GNU General Public License v3.0](LICENSE)。
2. 無法遵循 GPLv3、需要閉源整合或其他商業條款者，可另行取得[商業授權](COMMERCIAL-LICENSE.md)。

商業授權僅涵蓋授權方有權另行授權的程式碼與資產，不包含第三方套件、圖示、字型、資料集、AI 模型或其他第三方內容；這些項目仍適用各自的授權條款。第三方清冊請參閱 [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md)；完整授權文字會在建置時產生並隨發行產物提供。

建置流程會將 GPLv3、第三方授權文件及 `build-metadata.json` 放入發行產物。中繼資料記錄來源 Git tag、commit 與工作區狀態，讓執行檔可追溯至對應原始碼。

目前在正式 Contributor License Agreement 完成前，僅接受問題回報與討論；詳細說明請參閱 [CONTRIBUTING.md](CONTRIBUTING.md)。
