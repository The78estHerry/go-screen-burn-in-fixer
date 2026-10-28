# go-screen-burn-in-fixer

在全螢幕即時播放「螢幕修復畫面」的小工具：快速閃爍的雜訊、彩色條紋、純色輪播與移動色條，讓面板上每個紅、綠、藍子像素在全亮與全暗之間反覆切換。播放結束後可以自動關閉程式、關機、睡眠，或切換成節奏緩慢的螢幕保護畫面。

- **單一 exe**：不需要安裝任何東西，點兩下就能執行。
- **即時產生畫面**：由顯示卡即時運算，任何解析度、更新率都清楚，不需要下載或轉檔影片。
- **自動配合螢幕**：預設使用目前螢幕的解析度，也能讀取 EDID 顯示原生解析度。

## ⚠️ 警告

- **本程式內含快速閃爍畫面。光敏性癲癇患者或對閃光敏感的人請勿觀看。** 播放時請避免一直盯著螢幕。
- **真正的 OLED 烙印是永久性的面板老化，無法用任何畫面修復。** 本工具可能有助於減輕**暫時性殘影**（image retention）或喚醒**卡住的像素**，但不保證有效。
- 長時間以最高亮度播放會增加面板負擔，請自行斟酌播放時間。

## 畫面

每種畫面的 3 秒預覽影片放在 [`previews/`](previews/README.md)。

### Burn-In 圖樣

| 名稱 | 畫面 |
|---|---|
| `gray-noise` | 每格重新產生的灰階雜訊（1 像素） |
| `rgb-noise` | 紅、綠、藍子像素各自隨機全開或全關的雜訊 |
| `vstripes` / `vstripes-fine` | 隨機顏色的直條紋（寬 4 像素 / 1 像素） |
| `hstripes` / `hstripes-fine` | 隨機顏色的橫條紋（寬 4 像素 / 1 像素） |
| `cross` / `cross-fine` | 直條紋與橫條紋交錯成的十字格紋（寬 4 像素 / 1 像素） |
| `solid-cycle` | 全螢幕紅 → 綠 → 藍 → 白 → 黑輪播 |
| `moving-bar` | 黑底白色色條，水平、垂直輪流掃過螢幕 |

### 螢幕保護畫面

亮度低、不會閃爍，畫面持續移動，不會長時間停在同一個位置。

| 名稱 | 畫面 |
|---|---|
| `drift-clock` | 在整個螢幕漂移、慢慢變色的時鐘 |
| `aurora` | 流動的深色漸層色帶 |

## 使用方式

### 點兩下執行

1. 執行 `go-screen-burn-in-fixer.exe`。
2. 閱讀警告畫面，按任意鍵繼續。
3. 在設定選單選擇：模式（Burn-In 修復 / 只播螢幕保護）、要使用的螢幕、要播放的圖樣與每段秒數、播放時間、時間到之後的動作、螢幕保護畫面。滑鼠指到的圖樣會在右側即時預覽。
4. 按「開始播放」或 Enter，在選定的螢幕全螢幕播放。設定會存在 exe 旁邊的 `go-screen-burn-in-fixer.json`，下次開啟時沿用。

設定選單可以全程用鍵盤操作：↑↓ 選擇、←→ 調整、空白鍵勾選、Enter 開始播放、Esc 結束。

### 播放時的按鍵

| 按鍵 | 功能 |
|---|---|
| Esc | 回到設定選單（以命令列參數執行時直接結束） |
| 空白鍵 | 暫停 / 繼續 |
| ← / → | 上一個 / 下一個圖樣 |
| H | 顯示 / 隱藏資訊列（目前圖樣、剩餘時間、FPS） |

### 定時與後續動作

| 動作 | 說明 |
|---|---|
| `quit` | 關閉程式 |
| `shutdown` | 倒數 60 秒後關機（倒數中按任意鍵取消） |
| `sleep` | 倒數 60 秒後進入睡眠（倒數中按任意鍵取消） |
| `screensaver` | 停止閃爍，切換成螢幕保護畫面，按鍵或移動滑鼠後結束 |

### 命令列參數

帶播放相關參數（`-program`、`-timer`、`-after`、`-screensaver`、`-monitor`）執行時，會跳過警告畫面與設定選單直接開始播放，方便排程使用；這些參數只用於這次執行，不會改到設定檔：

```
go-screen-burn-in-fixer.exe -program "gray-noise:600,vstripes:300,solid-cycle:120" -timer 30m -after screensaver
```

| 參數 | 說明 |
|---|---|
| `-program` | 節目表，格式為 `圖樣:秒數`，以逗號分隔（預設：全部圖樣輪流，共 50 分鐘） |
| `-timer` | 定時長度，例如 `30m`、`1h` |
| `-after` | 時間到之後的動作：`quit`、`shutdown`、`sleep`、`screensaver` |
| `-screensaver` | 螢幕保護畫面名稱 |
| `-monitor` | 使用第幾個螢幕 |
| `-list` | 列出所有圖樣與螢幕保護畫面 |
| `-list-monitors` | 列出螢幕（目前解析度與 EDID 原生解析度） |
| `-windowed` | 以視窗模式執行 |
| `-dry-run` | 測試用：時間到時不真的關機或睡眠 |
| `-countdown` | 關機 / 睡眠前的倒數時間（預設 `60s`） |

## 建置

需要 Go 1.27 以上：

```
go build -trimpath -ldflags "-s -w -H windowsgui" -o dist/go-screen-burn-in-fixer.exe .
```

產生預覽影片（需要 ffmpeg）：

```
go run ./cmd/preview
```

## 新增圖樣

每個圖樣是 `patterns/` 下的一組 `.go` 與 `.kage`（Ebitengine 的 shader）檔案，在 `init()` 裡註冊，不需要修改其他程式碼。詳細步驟見 [CLAUDE.md](CLAUDE.md) 第 3.3 節。螢幕保護畫面的做法相同，放在 `screensaver/`。

## 授權

本專案採用 [MIT License](LICENSE)。

內嵌字型 Noto Sans TC（子集）採用 [SIL Open Font License 1.1](internal/ui/NotoSansTC-LICENSE.txt)。
