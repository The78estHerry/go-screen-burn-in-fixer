# 畫面預覽

每支影片都是 640x480、60fps，由 `go run ./cmd/preview` 產生，使用的 shader 與正式程式完全相同。長度 3 秒；但 `gray-noise`、`rgb-noise`、`cross`、`cross-fine` 這 4 支只有 **0.5 秒**：完全隨機的雜訊幾乎無法壓縮，3 秒的檔案會大到 60 MB 以上，所以縮短長度，畫質不變。建議用播放器的「重複播放」觀看。

- 影片的 1 個像素就是螢幕上的 1 個像素（預覽是實際畫面的 640x480 局部，不是縮小版）。**請用 100% 原始大小觀看**；全螢幕播放時播放器會放大影片，條紋和雜訊會看起來比實際粗。
- 1 像素的彩色圖樣（`rgb-noise`、`*-fine`）各有兩個版本：
  - `<名稱>.mp4`：640x480、yuv444p，1:1 原始像素。Windows 內建播放器無法播放 yuv444p，請用 [VLC](https://www.videolan.org/) 或 mpv。
  - `<名稱>-2x.mp4`：1280x960、yuv420p，同一個畫面放大 2 倍（每個像素變成 2×2），顏色一樣清楚，任何播放器都能播放。

> ⚠️ Burn-In 圖樣內含快速閃爍畫面，光敏性癲癇患者請勿觀看。

## Burn-In 圖樣

| 影片 | 名稱 | 說明 |
|---|---|---|
| [gray-noise.mp4](gray-noise.mp4) | `gray-noise` | 每格重新產生的灰階雜訊（1 像素），以中灰為中心 |
| [rgb-noise.mp4](rgb-noise.mp4)（[2x](rgb-noise-2x.mp4)） | `rgb-noise` | 每個紅、綠、藍子像素各自隨機全開或全關，比灰階雜訊更細 |
| [vstripes.mp4](vstripes.mp4) | `vstripes` | 隨機顏色的直條紋（寬 4 像素），每格重新排列；顏色只用子像素全開 / 全關的 8 種組合 |
| [vstripes-fine.mp4](vstripes-fine.mp4)（[2x](vstripes-fine-2x.mp4)） | `vstripes-fine` | 同上，寬 1 像素 |
| [hstripes.mp4](hstripes.mp4) | `hstripes` | 隨機顏色的橫條紋（寬 4 像素） |
| [hstripes-fine.mp4](hstripes-fine.mp4)（[2x](hstripes-fine-2x.mp4)） | `hstripes-fine` | 同上，寬 1 像素 |
| [cross.mp4](cross.mp4) | `cross` | 直條紋 XOR 橫條紋，形成十字格紋（寬 4 像素） |
| [cross-fine.mp4](cross-fine.mp4)（[2x](cross-fine-2x.mp4)） | `cross-fine` | 同上，寬 1 像素 |
| [solid-cycle.mp4](solid-cycle.mp4) | `solid-cycle` | 全螢幕紅 → 綠 → 藍 → 白 → 黑，每種顏色 0.5 秒 |
| [moving-bar.mp4](moving-bar.mp4) | `moving-bar` | 黑底白色色條，每 1.5 秒掃過一趟，水平、垂直輪流 |

## 檢查畫面

檢查畫面大多是靜止的，所以預覽用 PNG：每一階排成一格（640x480），格子下方的文字只是預覽圖的標示，**實際檢查畫面上沒有任何文字**；需要的資訊（目前項目、倒數秒數、要看什麼）按 H 或移動滑鼠時才會出現在可隱藏的資訊列。

| 預覽 | 名稱 | 檢查什麼 |
|---|---|---|
| [checks/gray-field.png](checks/gray-field.png) | `gray-field` | 灰階全畫面，預設 13–191 手動 7 階（自動時連續漸進）：烙印的影子（18–35% 最明顯） |
| [checks/gray-steps.png](checks/gray-steps.png) | `gray-steps` | 灰階分階，階數可自訂（2–256；預覽為 16、32、64、256 階）：螢幕能分辨的灰階數、暗部細節 |
| [checks/color-field.png](checks/color-field.png) | `color-field` | 紅、綠、藍各 25、50、100%：子像素老化不平均、判斷哪個子像素壞了 |
| [checks/retention-test.png](checks/retention-test.png)（[影片](checks/retention-test.mp4)） | `retention-test` | 黑白格紋 10 秒後切換成灰色：暫時性殘影多久消失 |
| [checks/black.png](checks/black.png) | `black` | 全黑：亮點、卡住的像素 |
| [checks/white.png](checks/white.png) | `white` | 全白：暗點、死像素、灰塵 |
| [checks/near-black.png](checks/near-black.png) | `near-black` | 近黑 1、2、3、5%：暗部不均勻（雲斑、mura）、角落偏亮 |
| [checks/gradient.png](checks/gradient.png) | `gradient` | 灰階與紅綠藍漸層（水平／垂直）：不規則的斷層或色偏 |

百分比是 8 位元訊號值的比例（例如 25% = 64/255），不是亮度。

## 螢幕保護畫面

亮度上限 30%，不閃爍，持續移動，不會長時間停在同一個位置。

| 影片 | 名稱 | 說明 |
|---|---|---|
| [screensaver-drift-clock.mp4](screensaver-drift-clock.mp4) | `drift-clock` | 低亮度時鐘，在整個螢幕範圍內漂移、慢慢變色 |
| [screensaver-aurora.mp4](screensaver-aurora.mp4) | `aurora` | 流動的深色漸層色帶 |
