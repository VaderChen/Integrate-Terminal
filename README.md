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
- 英文、日文、韓文、繁體中文與簡體中文介面。

## 公開版本

公開版本不使用 StoreKit、不限制連線數，也不啟用 App Sandbox。程式可存取目前登入帳號原本就有權限的檔案與目錄；macOS 的隱私保護目錄仍可能由系統要求使用者授權。

程式會在第一次啟動時嘗試搬移舊沙盒版的站台、設定、known hosts、PPK 副本與 REST API token。公開原始碼不包含任何 Apple 簽章憑證、Provisioning Profile、私鑰或個人站台資料。

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

目前在正式 Contributor License Agreement 完成前，僅接受問題回報與討論；詳細說明請參閱 [CONTRIBUTING.md](CONTRIBUTING.md)。
