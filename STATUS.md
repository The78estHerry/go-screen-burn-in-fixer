# 工作狀態

> 最後更新：2026-10-29。下次開工時先讀這份檔案，提醒使用者做到哪裡、哪些還需要確認。

## 目前進度

最新版本 **v0.2.1**（commit `01a8cf9`），Release 附 Windows / macOS（Apple Silicon、Intel）/ Linux x64 四個檔案：
https://gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/releases

| 版本 | 內容 |
|---|---|
| v0.1.0 | Burn-In 圖樣、定時與後續動作、螢幕保護、檢查模式（烙印 / 殘影 / 壞點）、設定選單 |
| v0.2.0 | 多螢幕同時播放、洗牌播放、播放時間直接輸入、EDID 擴充區塊、exe 圖示與版本資訊、選單反白修正 |
| v0.2.1 | macOS 與 Linux 版本、播放結束後選單回到原本的螢幕 |

功能都已實作，自動測試（`go vet`、`go test`）與能在單一螢幕上做的流程測試都通過。灰階全畫面改成可設定最暗／最亮／階數、右鍵拖曳調整，使用者已實機確認；之後依使用者要求把自動模式從連續漸進改回分階前進（與舊版相同）。**尚未發布新版**。

## 需要使用者實機確認

自動測試只能用視窗模式、單一螢幕、不閃爍的畫面，下面未勾選的項目還沒實際確認過：

### Windows
- [x] 全螢幕播放：Burn-In、檢查畫面、螢幕保護，解析度正確、沒有模糊。（2026-10-29 使用者確認）
- [x] **多螢幕**：同時播放、Esc 全部停止、同步換階、分開標記、關機／睡眠倒數、沒有殘留程序、全螢幕在正確的螢幕上。（2026-10-29 使用者確認）
- [x] 播放結束後設定選單回到原本的螢幕與位置。（2026-10-29 使用者確認）
- [x] **EDID**：Odyssey G91F 的「EDID 原生」顯示 5120×1440。（2026-10-29 使用者確認）
- [x] 選單：播放時間直接輸入、灰階分階直接輸入、「全部設為」反白不再蓋住文字。（2026-10-29 使用者確認）
- [x] **灰階全畫面改版**（2026-10-29，未發布）：最暗／最亮／階數設定與直接輸入、手動 n 階、右鍵拖曳、多螢幕同步。（2026-10-29 使用者確認）
- [ ] 灰階全畫面自動模式改回分階前進（每階 N 秒），多螢幕時各台一起換階。
- [ ] 洗牌播放：資訊列「本輪 已播放 / 總時間」、圖樣交錯是否符合預期。
- [ ] 真的關機 / 睡眠（先用 `-dry-run` 測，再實際試）。

### macOS（完全沒有實測，只確認能編譯、檔案格式正確）
- [ ] Apple Silicon 與 Intel 各自能開啟；「強制打開」或 `xattr -cr` 的流程與 README 說的一致。
- [ ] 全螢幕播放、選單操作、Retina 解析度是否清楚。
- [ ] 播放中螢幕不會關閉（`caffeinate`）；關機（第一次會詢問控制「系統事件」）、睡眠（`pmset sleepnow`）。
- [ ] 設定檔存在 `~/Library/Application Support/go-screen-burn-in-fixer/`。

### Ubuntu（完全沒有實測）
- [ ] 解壓縮後能執行；X11 與 Wayland（XWayland）工作階段都試。
- [ ] 播放中螢幕不會關閉：GNOME 是否接受 `systemd-inhibit --what=idle` 與 `xdg-screensaver reset`（**最不確定的一項**）。
- [ ] `systemctl poweroff` / `suspend` 不需要密碼即可執行。

## 已知限制與可能的後續工作

- **macOS / Linux 沒有螢幕名稱與 EDID**：`display` 只有 Windows 實作。Linux 可讀 `/sys/class/drm/*/edid`，macOS 需要 IOKit。
- **同解析度的多台螢幕**：Ebitengine 的螢幕和作業系統的顯示器是用解析度對應的，兩台相同解析度時名稱與標記的對應可能互換。
- **10 位元 / HDR 灰階**：目前輸出 8 位元 SDR，灰階最多 256 階。要支援需自己建立 10 位元 / HDR10 swap chain，使用者決定目前不做。
- **程式碼簽章**：Windows（SmartScreen）與 macOS（Gatekeeper）都沒有簽章，使用者目前不打算購買。
- **儲存庫是私人的**：Release 的檔案要登入 Gitea 才能下載；要給別人直接下載需改成公開。

## 發新版的步驟

1. 更新 `winres/winres.json` 裡所有版本號 → `go-winres make --arch amd64,arm64`。
2. `go vet ./...`、`go test ./...`。
3. `python tools/release/build.py` → `dist/release/` 四個檔案。
4. commit、push（claude-bot），用 Gitea API 建立 Release 並上傳 `dist/release/*`（token 在 Git Credential Manager；用 `git credential fill` 取得），下載回來比對 SHA-256。
