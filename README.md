<div align="center">
  <img src="assets/appicon.png" alt="Integrate Terminal icon" width="128" />
  <h1>Integrate Terminal</h1>
  <p>以 Go、Wails、React 與 TypeScript 建立的跨協定桌面終端與檔案傳輸工具。</p>
</div>

## 功能

- SSH、Telnet 與本機 Shell 終端。
- SFTP、FTP 檔案瀏覽及傳輸佇列。
- 站台分組、分頁恢復、ZIP 備份與還原。
- 本機 MCP Streamable HTTP Server，供 AI 與自動化工具整合。
- 背景服務與系統列狀態控制。
- 英文、日文、韓文、繁體中文與簡體中文介面。

## 公開版本

公開版本不使用 StoreKit、不限制連線數，也不啟用 App Sandbox。程式可存取目前登入帳號原本就有權限的檔案與目錄；macOS 的隱私保護目錄仍可能由系統要求使用者授權。

Bundle ID 維持 `com.vader.integterm`，並在第一次啟動時嘗試搬移舊沙盒版的站台、設定、known hosts、PPK 副本與 REST API token。公開原始碼不包含任何 Apple 簽章憑證、Provisioning Profile、私鑰或個人站台資料。

## 開發環境

- macOS 12 或更新版本
- Go 1.23 或更新版本
- Node.js 與 npm
- Xcode Command Line Tools
- Wails v2 CLI；若未安裝，專案腳本會透過 `go install` 安裝

## 啟動開發模式

```bash
git clone https://github.com/VaderChen/Integrate-Terminal.git
cd Integrate-Terminal
./run.sh
```

`run.sh` 會在本機暫存目錄建立開發鏡像，避免外接磁碟產生的 AppleDouble 檔案干擾 Wails 建置。

## 建置 macOS App

```bash
./build.sh
```

輸出位於 `build/bin/IntegTERM.app`。預設採 ad-hoc 簽章且不啟用 App Sandbox，建置完成後可直接在本機開啟使用。

GitHub 公開版本只提供一般 `.app` 建置流程，不包含 Developer ID、Apple notarization 或 DMG 發布腳本。若要將自行建置的 App 散布給其他使用者，需由散布者另外處理 Apple 簽章與發布要求。

## 資料與安全

應用資料預設位於 macOS 的 `~/Library/Application Support/IntegTERM`。站台密碼及 PPK passphrase 目前會保存在本機站台資料與站台備份 ZIP 中，請限制檔案權限並妥善保管備份。REST/MCP 服務預設只綁定 `127.0.0.1`，啟用對外來源前請正確設定 IP 白名單。

請勿提交 `cert/`、`data/`、`.env*`、安裝包、簽章資產或任何包含真實帳密的檔案。安全問題請參閱 [SECURITY.md](SECURITY.md)。

## 授權

本專案採雙軌授權：

1. 開放原始碼使用遵循 [GNU General Public License v3.0](LICENSE)。
2. 無法遵循 GPLv3、需要閉源整合或其他商業條款者，可另行取得[商業授權](COMMERCIAL-LICENSE.md)。

目前在正式 Contributor License Agreement 完成前，僅接受問題回報與討論；詳細說明請參閱 [CONTRIBUTING.md](CONTRIBUTING.md)。
