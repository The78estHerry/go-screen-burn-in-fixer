package app

import (
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/The78estHerry/go-screen-burn-in-fixer/internal/i18n"
	"github.com/The78estHerry/go-screen-burn-in-fixer/internal/ui"
)

// infoBar 控制播放與檢查時左上角資訊列的顯示（Burn-In 與檢查模式共用）。
//
//   - 設定 InfoHidden：平常不顯示。
//   - 設定 N 秒：flash()（切換圖樣 / 階、移動滑鼠）後顯示 N 秒。
//   - 設定 InfoAlways：一直顯示。
//   - 按住 S：按著時顯示；H：切換顯示 / 隱藏。兩者在任何設定下都有效。
type infoBar struct {
	setting   int
	hOn       bool // 用 H 打開
	alwaysOff bool // 「一直顯示」時用 H 關掉
	until     time.Time
}

func newInfoBar(setting int) *infoBar {
	b := &infoBar{setting: setting}
	b.flash()
	return b
}

// flash 依設定短暫顯示資訊列。
func (b *infoBar) flash() {
	if b.setting > 0 {
		b.until = time.Now().Add(time.Duration(b.setting) * time.Second)
	}
}

// update 處理 H 鍵。
func (b *infoBar) update() {
	if !inpututil.IsKeyJustPressed(ebiten.KeyH) {
		return
	}
	if b.visible() {
		b.hOn, b.until = false, time.Time{}
		b.alwaysOff = b.setting == InfoAlways
	} else {
		b.hOn, b.alwaysOff = true, false
	}
}

func (b *infoBar) visible() bool {
	return ebiten.IsKeyPressed(ebiten.KeyS) || b.hOn ||
		(b.setting == InfoAlways && !b.alwaysOff) || time.Now().Before(b.until)
}

// draw 在畫面左上角繪製資訊列：第一行亮色，其餘為暗色。比螢幕寬的行自動換行（英文比中文長）。
func (b *infoBar) draw(screen *ebiten.Image, lines ...string) {
	if !b.visible() || len(lines) == 0 {
		return
	}
	s := math.Max(float64(screen.Bounds().Dy())/1080, 0.6) // 以 1080p 為基準縮放
	size, small := 22*s, 17.6*s
	maxW := float64(screen.Bounds().Dx()) - 64*s
	wrapped := lines[:1:1]
	for _, l := range lines[1:] {
		wrapped = append(wrapped, wrapText(l, small, maxW)...)
	}
	lines = wrapped
	w := 0.0
	for i, l := range lines {
		sz := small
		if i == 0 {
			sz = size
		}
		lw, _ := ui.Measure(l, sz)
		w = math.Max(w, lw)
	}
	h := size*1.4 + float64(len(lines)-1)*small*1.5 + 20*s
	ui.Fill(screen, ui.Rect{X: 16 * s, Y: 16 * s, W: w + 32*s, H: h}, ui.ColorHUD)
	y := 26 * s
	f := breathNow() // 資訊列可能一直顯示，文字也會呼吸
	for i, l := range lines {
		if i == 0 {
			ui.Text(screen, l, 32*s, y, size, dimmed(ui.ColorText, f), ui.Left)
			y += size * 1.45
			continue
		}
		ui.Text(screen, l, 32*s, y, small, dimmed(ui.ColorDim, f), ui.Left)
		y += small * 1.5
	}
}

// infoBarLabel 回傳選單顯示的資訊列設定名稱。
func infoBarLabel(v int) string {
	switch v {
	case InfoHidden:
		return i18n.T("不顯示", "Off")
	case InfoAlways:
		return i18n.T("一直顯示", "Always")
	}
	return formatSeconds(v)
}
