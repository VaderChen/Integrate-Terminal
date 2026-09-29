# IntegTERM 1.26.0929（Build 1335）正式版

發布標籤：`v1.26.0929.1335`。平台：macOS 13 以上、Apple Silicon（arm64）。

## 本次更新

- 參照 YourDesk，改用固定 Developer ID Application 簽章，啟用 Hardened Runtime 與可信時間戳記。
- 內嵌動態庫、執行檔及 App 由內而外簽署；App 和 DMG 分別完成 Apple 公證並附加票根。
- App 與 DMG 的 Gatekeeper 均通過，來源為 Notarized Developer ID。
- 新增可重用的正式發行流程 `scripts/release-macos.py`，簽章或公證失敗即停止，不降級為 ad-hoc；驗證全部通過後才發布產物。
- 功能延續 Build 1305：雙欄站台編輯、標籤檔案儲存、PPK 驗證提示與本機診斷工具。

## 驗證與附件

- macOS arm64 正式建置、Developer ID 簽章、公證票根及 Gatekeeper 驗證通過。
- DMG 校驗、唯讀掛載與映像內 App 簽章／票根／Gatekeeper Smoke 通過。
- 正式發行流程的失敗保護測試通過；應用程式功能未變更，沿用 Build 1305 已通過的完整回歸結果。
- 附件包含已簽署與公證的 DMG、`SHA256SUMS.txt` 及 `notarization.json`。SHA-256 在完成公證票根附加後計算。

## 相容性

本版取代 Build 1305 成為 Latest。PPK 使用者個案仍待直接檔案診斷確認；本次簽章更新不代表該連線問題已修復。

本分支與 9/12 正式版的功能集合不同，尚未包含收藏站台、資料夾授權與強制更新介面。資料模型不保留未知 JSON 欄位，使用前請備份站台、分頁與設定檔。App 直接讀取指定 PPK 路徑，不讀取舊版保存的金鑰副本。

PPK 診斷可從原始碼執行 `scripts/check-ppk.command`；本次附件僅提供完成 Developer ID 簽署與 Apple 公證的 App／DMG 發行包。
