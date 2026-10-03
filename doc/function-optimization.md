# 函式級效率與記憶體最佳化

日期：2026-10-03。對應發行版本：`1.26.1003`，Build `2336`。此輪延續既有高頻路徑最佳化，保留操作流程、UI 結構、API 欄位、驗證、錯誤處理及資料排序。日誌保存上限依使用者要求調整。

## 盤點範圍

以 Go 與 TypeScript 語法樹盤點自有程式碼的函式、迴圈與呼叫，再核對資料所有權、鎖定範圍及資源生命週期。盤點涵蓋後端 App／REST／MCP、session、FTP／SFTP、儲存交易、更新器、授權、SSH 金鑰、托盤與前端。完成後的 Go 範圍為 74 個檔案、610 個函式／方法，包含各平台實作。TypeScript 共盤點 727 個函式節點，其中 216 個具名函式／方法，其餘為回呼；不以「修改每個函式」作為最佳化目標。

產生的 Wails 綁定、外部套件及第三方 systray 程式碼不做機械式改寫。StoreKit Swift 邊界、建置／封裝腳本及既有交易機制納入檢視與 Smoke 驗證；保留必要的磁碟同步、資料驗證、網路 I/O 及跨行程鎖定。

## 具體變更

| 區域與函式 | 處理方式 | 保留的行為 |
| --- | --- | --- |
| `collapseNestedDeleteTargets` | 以已正規化的祖先前綴查表取代候選路徑兩兩比對，重用工作切片與 map | 原有長度排序、相對路徑、根目錄、空白與包含判定 |
| `addChildTransfer`、`updateTransferLocked`、`transferProgress`、`removeTransferLocked` | ID 索引搭配雙向連結，新增／查找／移除平均 O(1)；根傳輸不儲存空的父關係；與日誌共用遞增時間戳，避免時鐘回退或重複時間戳產生重複 ID | ID 格式、最新在前、父子取消／暫停、失敗項目保留、完成後 1.2 秒移除 |
| `SampleTransfers`、清除與暫停操作 | 需要輸出時建立獨立快照；刪除節點解除參照，空佇列釋放索引 | 既有快照不因後續操作改變；清除取消列不提前解除工作取消狀態 |
| `mcpVFS.list` | 計算子項數後一次配置快照，不建立含檔案節點的中間 map；解鎖後排序 | 一致快照、目錄優先、名稱排序與既有錯誤 |
| `writeVirtualChunk` | 完成階段借用受 `finalizing` 保護的暫存內容，省去整份檔案複製 | SHA-256 校驗、容量限制、排斥並行重啟、失敗重試與 RAM 內容獨立性 |
| `normalizeMCPVFSPath`、`parseMCPVFSLocation`、`mcpMountRoot` | 逐段迭代與 `strings.Cut`，避免切片及重新串接 | 所有路徑／URI／控制字元驗證 |
| `cleanRemoteAbsolutePath`、`inspectRemoteRootPath` | 逐段借用原路徑前綴，省去中間陣列及累積字串 | 每層 lstat、symlink 拒絕、缺少葉節點與權限錯誤處理 |
| `sortSitesByNameLocked`、`sortSiteFoldersLocked` | 排序鍵各計算一次，交換時同步移動 | 忽略大小寫、空名稱回退 Host、相同鍵維持原順序 |
| `isTerminalFontFamily` | 固定名稱使用 switch，避免每次建立 map | 原有字型關鍵字與精確名稱集合 |
| 分頁計數、重排、`closeMCPRemoteTab` | 只需要數量時直接計數；重用已建立的可見分頁；清空刪除後的切片尾端 | 分頁順序、隱藏分頁、授權數量限制 |
| `telnetTerminalSession.negotiate` | 一般輸出直接交給同步消費者；協商輸出一次配置並批次複製文字區段 | Telnet 協商回覆、跳脫 IAC、分段控制字元與 64 KiB 未完成協商上限 |
| `normalizeTelnetInput`、`maybeAutoLogin` | 依 CR/LF 實際需求配置；自動登入已完成時不再解析輸出尾端 | CRLF、Unicode、登入時機及密碼補送規則 |
| 三種終端串流 | 每條串流只建立一次輸出事件名稱 | session ID、序號及事件內容 |
| `isPathInside`、`actionableEntries`、`decodeFileDrag` | 同批路徑共用基底正規化與父層深度，移除逐筆正規表示式中間陣列 | 拖曳來源綁定、相對路徑邊界、不可操作項目排除 |
| `attachTerminalOutput` | 已初始化且連續的輸出直接寫入，只有快照或序號缺口才進入 Map | 回放、重複序號、亂序、空區塊與回呼重入 |
| `writeLocalEcho` | 先轉換 CR／DEL，再一次寫入整批字串 | 與原本逐字寫入完全相同的位元組順序 |
| `selectAsset`、`normalizedVersion` | 單次掃描選出最高分安裝檔；版本驗證不再拆成陣列後重組 | 安裝檔驗證、分數／名稱優先規則、版本格式及錯誤 |
| `ProductVersion`、`Current`、`UpdateVersion` | 內嵌版本只解析一次，重用不變字串 | 格式、預設版本、build 與無效 JSON 的回退 |
| `boundedlog.Append`、崩潰／REST 記錄 | 共用有容量上限的跨行程寫入器，每次寫入後關閉檔案 | 原日誌路徑及失敗時的 stderr 回退 |

## 效能與記憶體量測

環境：Apple M4 Pro、darwin arm64、Go 1.27.1。各 Go benchmark 執行 3 次，`-benchtime=100ms`，表內為中位數。比較基準是此輪開始時的工作區，已含前一輪終端與操作日誌最佳化。最後一次效能量測在完整 Smoke 與建置結束後單獨執行。

「配置量」為每次受測操作累計配置的 B/op，不是整個應用程式的常駐記憶體或峰值。

| 受測操作 | 本輪修改前 | 本輪修改後 | 配置量前 → 後 |
| --- | ---: | ---: | ---: |
| 合併 1,000 筆獨立刪除路徑 | 28.49 ms | 0.0754 ms | 87,432 → 71,048 B |
| VFS 列出 2,000 個檔案 | 0.567 ms | 0.443 ms | 1,008,241 → 355,993 B |
| 完成 8 MiB RAM 分塊檔案 | 3.20 ms | 2.99 ms | 16,777,705 → 8,389,048 B |
| 排序 1,000 個站台並取得結果 | 0.903 ms | 0.411 ms | 699,184 → 335,536 B |
| 字型名稱篩選 | 282 ns | 61 ns | 952 → 16 B |
| VFS 遠端路徑解析 | 256 ns | 146 ns | 224 → 0 B |
| 約 4 KiB Telnet 一般輸出 | 31.96 µs | 0.0501 µs | 21,184 → 0 B |
| 11 KiB Telnet CRLF 輸入 | 21.03 µs | 11.37 µs | 32,704 → 12,288 B |
| 10,000 筆傳輸中更新並讀取最舊項目進度 | 37.62 µs | 0.0270 µs | 0 → 0 B |
| 新增 1,000 筆傳輸並取得快照 | 0.452 ms | 0.147 ms | 339,463 → 327,359 B |
| 新增 10,000 筆傳輸並取得快照 | 58.49 ms | 1.50 ms | 6,094,896 → 3,038,528 B |
| 逐層檢查遠端路徑（20 層、模擬 lstat） | 1.50 µs | 0.733 µs | 4,309 → 0 B |
| 讀取產品顯示版本 | 200 ns | 直接讀取已快取字串 | 56 → 0 B |
| 讀取更新版本 | 393 ns | 直接讀取已快取字串 | 80 → 0 B |

傳輸索引增加節點配置次數，但減少總配置量：1,000 筆案例的配置次數由 2,028 增至 3,025，累計配置量下降約 3.6%，新增時間縮短約 67.5%；10,000 筆案例的配置次數由 20,112 增至 30,087，累計配置量下降約 50.1%，新增時間縮短約 97.4%。ID 查找與移除不再隨佇列長度線性增長。

另以隔離的 10,000 筆根傳輸案例，在保留 Manager 並強制 GC 後比較 `runtime.ReadMemStats.HeapAlloc` 差額，中位數約從 1.87 MB 降至 1.72 MB；清空後的差額從約 1.55 MB 降至量測底噪範圍（本次中位數約 1.6 KB）。這是特定工作負載的保留量，不能代表整個 App 的 RSS 或所有父子傳輸組合。

前端 5,000 筆檔案路徑篩選，以相同 Node 程序、相同資料、暖機後 5 組量測取中位數，從約 4.25 ms 降至 1.96 ms。本機回顯 10,000 個一般字元從 10,000 次 `term.write` 合併為 1 次；輸出內容比對一致。上述數據不表示實際網路吞吐量或原生 GUI 操作速度有相同比例的提升。

目前版本可重跑的基準命令（macOS 需先依測試腳本設定 StoreKit 動態庫 rpath）：

```bash
env -u GOROOT GOTOOLCHAIN=go1.27.1 go test \
  ./internal/app ./internal/session ./internal/version ./internal/transport \
  -run '^$' \
  -bench 'Benchmark(CollapseDeleteTargets|VFSList|VFSFinalize|SiteSort|TerminalFontFilter|VFSLocation|Telnet|TransferProgressLookup|AppendTransfers|Current|UpdateVersion|RemotePathInspection)' \
  -benchmem -benchtime=100ms -count=3
```

## 日誌保留政策

- 操作日誌：最近 1,000 筆，記憶體循環緩衝，仍以最新在前回傳，每筆內容保持完整。
- `service-crash.log`：最多 5 MiB，崩潰與 REST 伺服器錯誤共用。超限時淘汰舊內容，保存新紀錄；單筆超過上限時只保留尾端 5 MiB。既有超大檔案在下次追加時套用上限。
- `install-update.log`：每次 macOS 安裝開始時覆寫，只保留最近一次安裝執行內容，避免跨多次更新無限追加。

容量控制使用檔案鎖串行化 GUI／背景服務寫入；不在記憶體中讀回整份磁碟日誌，也不保留長駐檔案控制代碼。

## 驗證

- 完整 `scripts/test.sh` 通過：全專案 Go race 測試、`go vet`、Wails／封裝／更新／sandbox 腳本測試、StoreKit 模擬邊界測試及 TypeScript 型別檢查。
- 前端 54 項行為測試全部通過，`npm run build` 正式建置成功。
- 新增驗證涵蓋路徑與原演算法一致、穩定排序、VFS 同時完成／重啟／重試、傳輸隨機移除與並行工作、時鐘回退下的 ID 唯一性、取消／暫停／清除、Telnet 分段與輸入控制字元、終端序號重入，以及多執行緒／多行程共用日誌容量。ID 保護加入後再次通過 session／app race 與 vet。
- 額外以修改前 `.bak` 比對前端 324 組路徑組合與 100 組 Unicode／控制字元回顯，結果一致。
- 新增日誌元件通過 Windows amd64 與 Linux amd64 交叉編譯；跨平台實機行為未在此次環境執行。
- 原生 App 的人工 GUI／外部真實伺服器連線測試未執行；FTP、SFTP、REST、MCP 與安裝流程使用專案既有本機測試伺服器及模擬環境驗證。
- 發行前以新版版本資料重新通過完整 Smoke 與正式 App 建置；已簽章、公證的 DMG 掛載後，包內 MCP 啟動、工具探索、RAM 檔案讀寫刪除及分塊 SHA-256 驗證均通過，並以系統 sandbox 禁止存取使用者資料及網路。

修改前建立 `.bak`；完成語法、Smoke、建置與差異檢查後移除。最佳化內容與日誌保存政策整合於[版本更新紀錄](release_1.26.1003.2336.md)；正式發行使用既有 Developer ID 與 Apple 公證流程。
