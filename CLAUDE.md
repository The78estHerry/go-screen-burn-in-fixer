# go-screen-burn-in-fixer：實作規格（給 Claude Code）

一個**即時全螢幕播放**螢幕修復畫面（burn-in fix）的工具：快速閃爍的雜訊、條紋、純色輪播、移動色條，用來運動面板上每個子像素，減輕暫時性殘影或卡住的像素。另有節奏緩慢的螢幕保護模式。

目標：**單一 exe，放到任何 Windows 電腦點兩下就能執行**，不需要安裝 Go、ffmpeg 或任何 runtime。不產生影片檔，畫面由 GPU 即時算出，所以沒有轉檔時間、沒有壓縮失真，解析度自動跟螢幕一致。

> 所有文件、註解、commit 訊息一律使用台灣繁體中文與台灣用語。

---

## 1. 背景（為什麼這樣做）

- 原本的做法是下載 YouTube 上的 burn-in fix 影片再轉檔。問題：影片是固定解析度、壓縮後雜訊畫面嚴重失真，換一台電腦或換一種螢幕就要重新轉檔（1080p60 雜訊畫面在 CPU 上轉一次要 1–2 小時）。
- 改成即時產生畫面：每個像素由 GPU shader 用 hash 函式算出，任何解析度、任何更新率都一樣快、一樣清楚。
- 選 Go + [Ebitengine](https://ebitengine.org/)：Windows 上不需要 cgo，`go build` 就能產生單一 exe；Ebitengine 跨平台，之後要支援 macOS / Linux 時，只有 EDID 讀取與電源控制要另外寫。
- **誠實說明**：真正的 OLED 烙印是永久性的面板老化，這類畫面無法修復；對暫時性殘影（image retention）或卡住的像素可能有幫助。README 與程式內都要這樣寫，不可宣稱能「修復烙印」。

---

## 2. 目錄結構

```
main.go                    參數解析、啟動 Ebitengine
console_windows.go         -H windowsgui 建置時接上主控台輸出（-list 等）、錯誤訊息視窗
app/app.go                 App（ebiten.Game）與場景切換：視窗 / 全螢幕、防睡眠
app/scenes.go              警告、Burn-In 播放、倒數、螢幕保護、訊息等場景
app/check.go               檢查進行中（依序顯示各階、手動 / 自動、標記、十字游標）
app/infobar.go             資訊列顯示邏輯（不顯示 / N 秒 / 一直顯示、按住 S、H），Burn-In 與檢查共用
app/menu.go                設定選單（三種模式；版面以 1000×720 邏輯座標描述）
app/menu_widgets.go        選單小元件（selector、radio、checkbox、button）
app/config.go              設定檔：exe 旁邊的 go-screen-burn-in-fixer.json
app/snapshot.go            開發用：-snapshot 把各畫面存成 PNG（不操作桌面）
internal/gfx/              Kage shader 共用函式（common.kage）、延遲編譯、繪製
internal/ui/               字型（Noto Sans TC 子集）、顏色、繪圖輔助
patterns/registry.go       Burn-In 圖樣介面 + Register()
patterns/builtin.go        內建圖樣註冊與預設參數；shader 在 patterns/*.kage
screensaver/registry.go    螢幕保護畫面介面 + Register()
screensaver/builtin.go     內建螢幕保護畫面；shader 在 screensaver/*.kage
checks/registry.go         檢查項目介面（Check、可選的 Statuser）+ Register()
checks/builtin.go          內建檢查項目；shader 在 checks/*.kage
program/program.go         節目表（圖樣:秒數序列、總長度、循環）
actions/actions.go         後續動作：quit / shutdown / sleep / screensaver
actions/power_windows.go   防睡眠（SetThreadExecutionState）、關機、睡眠
actions/power_other.go     其他平台：回傳不支援
display/display.go         使用中的顯示器列表
display/display_windows.go QueryDisplayConfig + 登錄檔 EDID（只讀使用中的顯示器）
display/display_other.go   其他平台：回傳不支援
display/edid.go            EDID 解析（與平台無關，有單元測試）
cmd/preview/main.go        開發用：產生 previews/*.mp4（需要 ffmpeg，不會打包進 exe）
tools/fontsubset/subset.py 開發用：產生字型子集（需要 fonttools）
previews/                  預覽影片 + README.md（納入版本控制）
```

相依套件：`github.com/hajimehoshi/ebiten/v2`、`golang.org/x/sys`、`golang.org/x/image`（字型）。不要再加其他相依套件，除非先和使用者確認。

### 字型

- 內建字型 `internal/ui/NotoSansTC-Subset.otf`：Noto Sans TC 子集（SIL OFL 1.1，授權檔 `internal/ui/NotoSansTC-LICENSE.txt` 必須一起保留）。不可改用微軟正黑體等系統字型（授權不允許再發布）。
- 收錄 Big5 常用字 + 專案 `.go` 檔中出現的所有非 ASCII 字元。`go test ./internal/ui` 會檢查畫面用到的字串（`cmd/` 除外）是否都有字形；缺字時執行 `python -P tools/fontsubset/subset.py <NotoSansTC-Regular.otf> internal/ui/NotoSansTC-Subset.otf`。

---

## 3. Burn-In 圖樣

### 3.1 介面

```go
type State struct {
    Frame int     // 從 0 開始的影格編號（決定隨機雜訊）
    Time  float64 // 秒（決定移動與輪播）
}

type Pattern interface {
    Name() string        // 參數與檔名使用的名稱，小寫加連字號
    Description() string // 選單與 README 使用的一句說明
    Draw(dst *ebiten.Image, st State)
}
```

- 每個圖樣在 `init()` 呼叫 `patterns.Register(p)`；`patterns.All()` 依註冊順序回傳。
- 畫面必須**只由 (像素座標, Frame, Time, 參數) 決定**，不依賴實際 FPS 或亂數狀態。這樣預覽影片與實際播放完全一致，也能重現。

### 3.2 圖樣清單

| 名稱 | 畫面 |
|---|---|
| `gray-noise` | 每格重新產生的隨機灰階雜訊（1 像素，以中灰為中心，對比 0.8） |
| `rgb-noise` | 每個 R / G / B 子像素各自隨機全開或全關（比灰階雜訊更細） |
| `vstripes` / `vstripes-fine` | 隨機顏色的直條紋，每格重新排列（寬 4 / 1 像素） |
| `hstripes` / `hstripes-fine` | 隨機顏色的橫條紋，每格重新排列（寬 4 / 1 像素） |
| `cross` / `cross-fine` | 直條紋 XOR 橫條紋，形成十字格紋（寬 4 / 1 像素） |
| `solid-cycle` | 全螢幕紅 → 綠 → 藍 → 白 → 黑輪播 |
| `moving-bar` | 黑底白色色條，先水平掃過、再垂直掃過，交替進行 |

- 條紋顏色從 8 種「子像素全開 / 全關」組合中隨機選（黑、紅、綠、藍、青、洋紅、黃、白），讓每個 R / G / B 子像素都在全亮與全暗之間切換。
- hash 函式用 Dave Hoskins 的 “hash without sine”（只用 `fract` 與 `dot`，GPU 間結果穩定）；影格編號先取模（避免浮點精度問題）。

### 3.3 新增圖樣的步驟

1. 新增 `patterns/<name>.kage`（`//kage:unit pixels`，`Fragment` 只使用 `dstPos` 與 uniform）。
2. 新增 `patterns/<name>.go`：`//go:embed`、在 `init()` 用 `gfx` 輔助函式建立並 `Register`。
3. `go run ./cmd/preview -only <name>` 產生預覽，更新 `previews/README.md`。

---

## 4. 螢幕保護模式

- 原則：**暗、持續移動、沒有長時間靜止的亮點**（避免造成新的烙印），不可快速閃爍。亮度上限 30%。
- 動畫速度倍率 `screensaver.Speed = 5`（使用者確認過：太慢的畫面停留太久，同樣可能留下殘影）。
- 介面與擴充方式同第 3 節（`screensaver.Register`），`State` 另含 `Now time.Time` 供時鐘使用。

| 名稱 | 畫面 |
|---|---|
| `drift-clock` | 黑底低亮度時鐘，在整個螢幕範圍內漂移（距邊緣保留 2%），顏色緩慢變換 |
| `aurora` | 深色漸層色帶流動（domain warp 的 fbm） |

---

## 5. 定時與後續動作

- 定時：播放時間（預設等於節目總長度）。時間到執行後續動作。
- 後續動作：

| 動作 | 做法 |
|---|---|
| `quit` | 結束程式 |
| `shutdown` | 全螢幕倒數 60 秒（任意鍵取消）→ `AdjustTokenPrivileges` 取得 `SeShutdownPrivilege` → `ExitWindowsEx(EWX_SHUTDOWN\|EWX_POWEROFF)` |
| `sleep` | 倒數 60 秒 → 解除防睡眠 → `powrprof.dll` `SetSuspendState(false, false, false)` |
| `screensaver` | 切換成螢幕保護畫面，按鍵或移動滑鼠結束（之後回到選單或關閉程式，可設定） |

- 倒數秒數可設定；取消後回到設定選單。
- `-dry-run`：倒數結束時只顯示「將會關機 / 睡眠」，不真的執行。**測試時一律使用 `-dry-run`，真的關機 / 睡眠必須先取得使用者同意。**

---

## 6. 設定選單與參數

- 點兩下 exe：警告畫面（「內含快速閃爍畫面，光敏性癲癇患者請勿觀看」，按任意鍵繼續）→ 設定選單（一般視窗）：模式（Burn-In 修復 / 只播螢幕保護）、螢幕、圖樣（可多選）與每段秒數、播放時間、時間到後的動作、螢幕保護畫面、結束螢幕保護後（回到選單 / 關閉程式）。滑鼠指到或焦點所在的圖樣會在右側預覽框即時播放。設定在開始播放與結束程式時存到 exe 旁邊的 `go-screen-burn-in-fixer.json`。
- 選單按鍵：↑↓（Tab）選擇、←→ 調整、空白鍵勾選 / 按下按鈕、Enter 開始播放、Esc 結束。
- 帶參數時跳過警告與選單直接播放（給排程使用；參數不會寫回設定檔）：

| 參數 | 說明 |
|---|---|
| `-program "gray-noise:600,vstripes:300"` | 節目表（預設：全部圖樣輪流，總長 50 分鐘） |
| `-timer 30m` | 定時 |
| `-after quit\|shutdown\|sleep\|screensaver` | 後續動作 |
| `-screensaver drift-clock` | 螢幕保護畫面 |
| `-monitor N` | 使用第 N 個螢幕 |
| `-list` / `-list-monitors` | 列出圖樣 / 螢幕（目前解析度與 EDID 原生解析度） |
| `-windowed` | 視窗模式（開發測試用） |
| `-dry-run` | 不真的關機 / 睡眠 |
| `-countdown 60s` | 關機 / 睡眠前的倒數時間 |
| `-snapshot <資料夾>` | 開發用：把各畫面存成 PNG 後結束 |

- 播放按鍵：Esc 回到設定選單（帶參數執行時直接結束）、空白鍵暫停、← / → 切換圖樣、H 顯示 / 隱藏資訊列（圖樣、剩餘時間、FPS；移動滑鼠也會顯示 3 秒）。播放時隱藏滑鼠游標。
- `ebiten.SetTPS(ebiten.SyncWithFPS)`：每次畫面更新換一格雜訊，配合螢幕更新率（120Hz 螢幕就是每秒 120 格）。
- 解析度：全螢幕，以裝置像素繪製（`Layout` 回傳 `外框大小 × DeviceScaleFactor()`），避免 Windows 縮放造成模糊。

---

## 7. 預覽影片

- `go run ./cmd/preview`：用與正式程式**相同的 shader**，把每個圖樣畫到 640x480 離屏影像，固定每格 1/60 秒，`ReadPixels` 後透過 stdin 送給 ffmpeg 編碼。
- 輸出：`previews/<name>.mp4`、`previews/screensaver-<name>.mp4`（640x480、60fps、3 秒，實際速度）。預覽是 1:1 的局部畫面，不是縮小版。
- 雜訊類（`gray-noise`、`rgb-noise`、`cross`、`cross-fine`）幾乎無法壓縮，預覽縮短成 0.5 秒（畫質不變），清單在 `cmd/preview` 的 `shortSeconds`。全部預覽合計約 65 MB；新增圖樣時若單支超過約 5 MB，也加進這份清單。
- 1 像素彩色圖樣（`rgb-noise`、`*-fine`）用 yuv444p 編碼（yuv420p 會讓色度糊掉），清單在 `cmd/preview` 的 `fullChroma`。Windows 內建播放器不支援 H.264 yuv444p，所以同時輸出 `<name>-2x.mp4`（1280x960、yuv420p、nearest-neighbor 放大，2×2 區塊對齊色度取樣，顏色不會糊）。兩種都要保留（使用者指定）。
- 參數：`-out previews -size 640x480 -fps 60 -seconds 3 -only <name>`。
- 圖樣或參數有改動時，重新產生對應預覽並更新 `previews/README.md`。

---

## 8. 建置

```
go build -trimpath -ldflags "-s -w -H windowsgui" -o dist/go-screen-burn-in-fixer.exe .
```

`CGO_ENABLED=0` 也可以建置。之後可以用 go-winres 加圖示與版本資訊（選用）。

---

## 9. 驗證

1. `go vet ./...`、`go test ./...`（EDID 解析、節目表、字型缺字檢查、實際查詢顯示器）。
2. `-list`、`-list-monitors` 輸出正確。以 windowsgui 建置的 exe 在 PowerShell 中不會等它結束，測試時用 `Start-Process -Wait -RedirectStandardOutput <檔案>`。
3. 版面：`go run . -snapshot <資料夾>` 產生 PNG 檢查。**不要用 SendKeys 之類的方式操作桌面**（按鍵可能送到使用者其他視窗）。
4. 流程：`-windowed -program moving-bar:3 -timer 4s -after quit`（應自行結束）、`-after shutdown -dry-run -countdown 3s`（記錄應顯示「略過關機」）。自動測試優先用 `moving-bar`，避免在使用者面前突然出現快速閃爍的畫面。
5. 全螢幕、多螢幕、實際按鍵操作、真的關機 / 睡眠：請使用者手動測試，或先取得同意。
6. exe 複製到空資料夾單獨執行，不需要其他檔案。

---

## 11. 檢查模式（已實作）

- 選單「模式」三選一：Burn-In 修復／檢查畫面／只播螢幕保護。檢查畫面的選單版面與操作和 Burn-In 一致，「圖樣」換成「檢查項目」。
- 檢查項目（`checks/`，介面 `checks.Check`，可擴充）：`gray-field`、`gray-steps`（階數 2–256 可自訂，預設 32；◀▶ 切換 8/16/32/64/128/256，選到該列時可直接輸入數字）、`color-field`、`retention-test`、`black`、`white`、`near-black`、`gradient`。有多「階」的項目，秒數指每一階。
- 每個項目的秒數選項：手動、5、10、15、20、30、45、60、120 秒（手動在最左邊）。上方「全部設為」(•) 自動 10 秒 ( ) 手動：選了就套用到全部項目；項目各自不同時兩者都不選取（代表自訂）。預設全部自動 10 秒。
- 檢查進行中：Enter／空白鍵 下一階（手動必須按；自動可提早跳過）、← 上一階、Esc 取消回到檢查選單、H 顯示／隱藏資訊列。全部完成後回到檢查選單。注意：Burn-In 播放時空白鍵是暫停，檢查時是下一個（使用者指定）。
- **檢查畫面上不放任何文字**（會擋住檢查區域，也可能自己留下殘影）。狀態文字（例如殘影測試倒數）由可選介面 `checks.Statuser` 提供，只顯示在資訊列。
- 標記：滑鼠移動時顯示十字游標與像素座標，點一下標記可疑位置（小圓圈），切換項目時保留；存檔到 exe 旁邊，下次開啟保留；選單可清除，檢查中按 C 清除。
- 命令列：`-check "gray-field:5,gray-steps:manual,white"`（依參數順序；秒數或 manual，省略用選單設定）、`-gray-levels N`。帶 `-check` 時跳過選單，完成或 Esc 後結束程式。
- shader 一律用 `pixelPos(dstPos)` 取得像素座標（見 `internal/gfx/common.kage`）；直接用 `dstPos` 在小影像（例如選單預覽框）上會因 atlas 位移而錯位。
- **資訊列顯示設定（Burn-In 與檢查模式共用）**：不顯示／3／5／10 秒（切換階或項目、移動滑鼠時顯示 N 秒，預設 5 秒）／一直顯示。
  - **按住 S**：資訊列顯示，放開即隱藏（使用者指定，主要給「不顯示」使用；其他設定下同樣有效）。
  - H：切換顯示／隱藏（任何設定下都有效）。

## 10. 開發進度

- [x] 專案建立、文件
- [x] shader 與預覽影片（使用者已確認全部畫面）
- [x] 主程式（選單、定時、後續動作、螢幕偵測、EDID、字型）
- [x] 上傳 Gitea
- [x] 檢查模式（預覽已確認、選單、手動 / 自動、灰階分階、標記、資訊列設定）
- [ ] 使用者手動測試：全螢幕、選單操作、螢幕保護、檢查模式、真的關機 / 睡眠
