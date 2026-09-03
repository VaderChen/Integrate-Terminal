# 開源內容盤點

本文件記錄首次公開及後續維護時納入與排除的內容範圍。

## 預計公開

- Go、React、TypeScript 與 Wails 原始碼。
- SFTP、FTP、SSH、Telnet、本機終端、背景服務與 MCP 功能。
- 通用開發及 macOS Apple Silicon、Windows x64、Linux x64 執行檔建置腳本。
- App icon、第三方 systray fork 與其原始授權。
- GPLv3、商業授權說明、安全性政策與開發文件。

## 已從公開候選排除

- `cert/` 全部憑證、私鑰、P12 與 Provisioning Profile。
- `data/` 個人站台、密碼、分頁與本機設定。
- `internal/purchase/` StoreKit 與 App Store 內購程式。
- 平台簽章、公證、DMG 與發布腳本等受控發布資產。
- 本機備份與安裝維護腳本。
- 內購宣傳圖、測試優惠碼、內部發行檢查表與舊審查文件。
- `build/bin/`、`dist/`、`frontend/dist/`、`frontend/node_modules/` 與所有安裝包。
- `.codex-tmp/`、`.bak` 與本機 ZIP 備份。

## 發布維護檢查

- GPLv3＋商業授權維持雙軌條款。
- Contributor License Agreement 完成前，不合併外部程式碼 Pull Request。
- 每次發布前重新掃描密碼、Token、私鑰與個人路徑。
- Repository 名稱使用 `Integrate-Terminal`，產品名稱顯示為「Integrate Terminal」。
