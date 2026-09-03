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
- MCP 虛擬層，提供 RAM 工作區與遠端站台掛載整合能力。
- 背景服務與系統列狀態控制。
- 支援在設定中手動檢查 GitHub Release，並於每日隨機時段自動檢查一次；有更新時可驗證並開啟適用平台的更新檔。
- 英文、日文、韓文、繁體中文與簡體中文介面。

## 公開版本

公開版本不使用 StoreKit、不限制連線數，也不啟用 App Sandbox。程式可存取目前登入帳號原本就有權限的檔案與目錄；macOS 的隱私保護目錄仍可能由系統要求使用者授權。

程式會在第一次啟動時嘗試搬移舊沙盒版的站台、設定、known hosts、PPK 副本與 REST API token。公開原始碼不包含任何 Apple 簽章憑證、Provisioning Profile、私鑰或個人站台資料。

## MCP 整合（VFS 預設啟用）

IntegTERM 內建本機 VFS MCP（stdio）與可選的 Streamable HTTP MCP。虛擬層統一 RAM 工作區與已儲存遠端站台的資源路徑。`integterm-vfs://` 是 MCP 連線內的 Resource URI／工具路徑，不是可直接連線的 URL。

### 本機 VFS MCP（stdio，預設）

本機 Agent 應以 `mcp` 參數啟動 IntegTERM，透過 stdio 連線：

```json
{
  "mcpServers": {
    "integterm-vfs": {
      "command": "/Applications/IntegTERM.app/Contents/MacOS/IntegTERM",
      "args": ["mcp"]
    }
  }
}
```

從原始碼執行時可使用 `go run . mcp`。連線後先呼叫 `tools/list`，再使用 `vfs_list`（空白 path 或 `integterm-vfs://workspace/mcp`）瀏覽工作區；不要把 `integterm-vfs://workspace/mcp` 填入 HTTP URL 或當作 shell 指令。

### Agent 呼叫流程

Agent 連線後請依序：

1. 呼叫 `tools/list` 取得目前工具結構。
2. 呼叫 `vfs_workspace_info` 確認根 URI 與工作區限制。
3. 呼叫 `vfs_list`，可省略 `path`（或傳入 `integterm-vfs://workspace/mcp`）列出根目錄。
4. 對檔案使用 `vfs_stat`、`vfs_read`、`vfs_write` 等工具；只有在客戶端支援 MCP Resources 時，才使用 `resources/read` 讀取 Resource URI。

`integterm-vfs://workspace/mcp` 本身不會啟動連線，也不是要交給 shell 執行的指令。

### MCP Server（HTTP 預設關閉）

1. 啟動 IntegTERM，開啟「設定」→「MCP」。
2. 本機 VFS MCP 預設透過 stdio 提供，不會監聽網路埠。
3. 只有需要讓外部 Agent 透過 HTTP 連線時，才開啟 MCP Server；預設埠為 `18080`、白名單為 `127.0.0.1`，端點為 `http://127.0.0.1:18080/mcp`。

### 虛擬工作區：RAM 與遠端站台

虛擬根 URI 為 `integterm-vfs://workspace/mcp`。根目錄下未使用 `sites` 命名空間的路徑是純 RAM 資料；`sites/{siteID}` 則代表已儲存遠端站台，第一次執行 `vfs_connect` 或檔案操作時會自動建立連線。RAM 資料會在背景服務停止後清除；遠端操作會直接作用於站台設定的遠端根目錄。

遠端站台的路徑格式為 `integterm-vfs://workspace/mcp/sites/{siteID}/{relativeRemotePath}`。先列出 `sites` 取得站台 ID，再使用 `vfs_list`、`vfs_stat`、`vfs_read`、`vfs_write`、`vfs_mkdir`、`vfs_rename` 與 `vfs_delete` 進行一般檔案操作。虛擬 URI 不包含密碼或私密金鑰。

### 透過網路：既有操作與虛擬工作區

外部 Agent 使用的 MCP 端點為 `http://127.0.0.1:18080/mcp`，提供站台管理、SSH、Telnet、SFTP、FTP、本機終端、指令、互動式終端、檔案傳輸，以及上述 `integterm-vfs` 虛擬工作區工具。`integterm-vfs://` 是資源 URI，不是另一個 HTTP 端點；HTTP Agent 也應透過 `vfs_list` 等工具操作它。

### 網路 MCP 客戶端設定

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

UI 預設採單一實例；再次啟動時會喚醒已開啟的視窗，不會建立第二個 UI。在 macOS 點擊視窗關閉鈕會結束 UI；按下 `Cmd+Q` 時，會詢問要完全關閉程式、轉為背景運作或取消。從 Tray 結束背景服務時，所有 IntegTERM UI 也會一併關閉。開發時若確實需要多開 UI，可使用特殊參數：

```bash
./run.sh --multi-instance
```

此參數只允許 UI 多開，背景主服務仍維持單一實例。

## 建置桌面程式

### macOS Apple Silicon

```bash
./build.sh
```

輸出位於 `dist/IntegTERM.app`。預設採 ad-hoc 簽章且不啟用 App Sandbox，建置完成後可直接在本機開啟使用。也可以雙擊 `build.command` 建置，或雙擊 `run.command` 啟動已建置的 App；需要開發模式時使用 `run.command --dev`。

### macOS 發布封裝

DMG 的簽署與發布僅限受控的本機流程處理；任何簽章識別資料、公證設定與發布憑證資訊均不記載於本文件，也不得提交至儲存庫。開發與本機驗證請使用上述一般建置流程。

### Windows x64

```powershell
powershell -ExecutionPolicy Bypass -File .\build-windows.ps1
```

輸出位於 `dist\IntegTERM.exe`。腳本只建立 x64 執行檔，不建立安裝程式或簽章。

### Linux x64

```bash
./build-linux.sh
```

輸出位於 `dist/IntegTERM`。腳本會偵測 WebKitGTK 與 AppIndicator 版本，只建立 x64 執行檔，不建立 AppImage、DEB 或 RPM。

GitHub 公開版本不包含簽章憑證、私鑰或其他發布機密；散布者須使用自己的平台發布資產完成簽章、公證與發布要求。

## 資料與安全

應用資料位於各平台 `os.UserConfigDir()` 下的 `IntegTERM` 目錄，例如 macOS 的 `~/Library/Application Support/IntegTERM`、Windows 的 `%AppData%\IntegTERM`，以及 Linux 的 `~/.config/IntegTERM`。站台密碼及 PPK passphrase 目前會保存在本機站台資料與站台備份 ZIP 中，請限制檔案權限並妥善保管備份。REST/MCP 服務的存取控制完全依賴 IP/CIDR 白名單，預設只接受 `127.0.0.1`；若要允許其他來源，請先將來源 IP 或 CIDR 加入白名單。

請勿提交 `cert/`、`data/`、`.env*`、安裝包、簽章資產或任何包含真實帳密的檔案。安全問題請參閱 [SECURITY.md](SECURITY.md)。

## 授權

本專案採雙軌授權：

1. 開放原始碼使用遵循 [GNU General Public License v3.0](LICENSE)。
2. 無法遵循 GPLv3、需要閉源整合或其他商業條款者，可另行取得[商業授權](COMMERCIAL-LICENSE.md)。

商業授權僅涵蓋授權方有權另行授權的程式碼與資產，不包含第三方套件、圖示、字型、資料集、AI 模型或其他第三方內容；這些項目仍適用各自的授權條款。第三方清冊請參閱 [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md)；完整授權文字會在建置時產生並隨發行產物提供。

建置流程會將 GPLv3、第三方授權文件及 `build-metadata.json` 放入發行產物。中繼資料記錄來源 Git tag、commit 與工作區狀態，讓執行檔可追溯至對應原始碼。

目前在正式 Contributor License Agreement 完成前，僅接受問題回報與討論；詳細說明請參閱 [CONTRIBUTING.md](CONTRIBUTING.md)。
