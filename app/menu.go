package app

import (
	"fmt"
	"image/color"
	"log"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/actions"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/internal/ui"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/patterns"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/screensaver"
)

// 設定選單。版面以 WindowWidth × WindowHeight 的邏輯座標描述，繪製時再依視窗大小縮放。
// 每一格都重新建立 items：同一份資料同時用於繪製、滑鼠點擊與鍵盤操作。

const (
	colLabel   = 40.0  // 左側標籤
	colValue   = 160.0 // 設定值
	rowPattern = 212.0 // 第一個圖樣列的 y
	rowHeight  = 32.0  // 圖樣列高
	secondsX   = 430.0 // 每段秒數選擇器
	previewX   = 640.0 // 預覽框
	previewY   = 176.0
	previewW   = 320.0
	previewH   = 240.0
)

// 鍵盤長按時的重複間隔（影格）。
const (
	repeatDelay    = 24
	repeatInterval = 4
)

type clickArea struct {
	r  ui.Rect
	fn func()
}

type item struct {
	row      ui.Rect // 整列範圍：滑鼠指到時反白，點一下取得焦點
	left     func()  // ← 鍵
	right    func()  // → 鍵
	activate func()  // 空白鍵；按鈕也可以用 Enter
	clicks   []clickArea
	disabled bool
	preview  func(dst *ebiten.Image, frame int) // 焦點或滑鼠在這一列時，預覽框顯示的內容
	draw     func(p *painter, focused, hovered bool)
}

type menuScene struct {
	focus      int
	hover      int
	errMsg     string
	quit       bool
	previewImg *ebiten.Image
	saverStart time.Time
}

func newMenuScene(a *App) *menuScene {
	return &menuScene{focus: len(a.cfg.Segments) + 8, hover: -1, saverStart: time.Now()} // 預設焦點在「開始播放」
}

// painter 把邏輯座標轉成畫面座標後繪製。
type painter struct {
	dst       *ebiten.Image
	s, ox, oy float64
}

func (p *painter) rect(r ui.Rect) ui.Rect {
	return ui.Rect{X: p.ox + r.X*p.s, Y: p.oy + r.Y*p.s, W: r.W * p.s, H: r.H * p.s}
}

func (p *painter) text(str string, x, y, size float64, clr color.Color, align ui.Align) {
	ui.Text(p.dst, str, p.ox+x*p.s, p.oy+y*p.s, size*p.s, clr, align)
}

func (p *painter) fill(r ui.Rect, clr color.Color)   { ui.Fill(p.dst, p.rect(r), clr) }
func (p *painter) stroke(r ui.Rect, clr color.Color) { ui.Stroke(p.dst, p.rect(r), 1.5*p.s, clr) }

func (p *painter) circle(cx, cy, r float64, clr color.Color, filled bool) {
	ui.Circle(p.dst, p.ox+cx*p.s, p.oy+cy*p.s, r*p.s, 1.5*p.s, clr, filled)
}

// ── 小元件 ───────────────────────────────────────────────

func textColor(disabled bool) color.Color {
	if disabled {
		return ui.ColorBorder
	}
	return ui.ColorText
}

// selector：◀ 值 ▶
func selector(x, y, w float64, value func() string, left, right func(), disabled bool) ([]clickArea, func(p *painter)) {
	l := ui.Rect{X: x, Y: y - 4, W: 28, H: 28}
	r := ui.Rect{X: x + w - 28, Y: y - 4, W: 28, H: 28}
	draw := func(p *painter) {
		c := textColor(disabled)
		arrow := color.Color(ui.ColorAccent)
		if disabled {
			arrow = ui.ColorBorder
		}
		p.text("◀", l.X+14, y, 16, arrow, ui.Center)
		p.text(value(), x+w/2, y, 17, c, ui.Center)
		p.text("▶", r.X+14, y, 16, arrow, ui.Center)
	}
	return []clickArea{{l, left}, {r, right}}, draw
}

// radio：○ 標籤
func radio(x, y float64, label string, selected func() bool, fn func(), disabled bool) (clickArea, float64, func(p *painter)) {
	w, _ := ui.Measure(label, 17)
	area := ui.Rect{X: x - 4, Y: y - 4, W: w + 36, H: 28}
	draw := func(p *painter) {
		c := textColor(disabled)
		ring := color.Color(ui.ColorDim)
		if disabled {
			ring = ui.ColorBorder
		}
		p.circle(x+8, y+11, 8, ring, false)
		if selected() {
			dot := color.Color(ui.ColorAccent)
			if disabled {
				dot = ui.ColorBorder
			}
			p.circle(x+8, y+11, 4.5, dot, true)
		}
		p.text(label, x+24, y, 17, c, ui.Left)
	}
	return clickArea{area, fn}, w + 52, draw
}

func checkbox(p *painter, x, y float64, checked, disabled bool) {
	box := ui.Rect{X: x, Y: y + 3, W: 18, H: 18}
	if checked && !disabled {
		p.fill(box, ui.ColorAccent)
		p.text("✓", x+9, y, 15, ui.ColorBackground, ui.Center)
		return
	}
	p.stroke(box, ui.ColorDim)
	if checked {
		p.text("✓", x+9, y, 15, ui.ColorDim, ui.Center)
	}
}

func button(r ui.Rect, label string, primary bool, fn func(), disabled bool) item {
	return item{
		row: r, activate: fn, disabled: disabled,
		draw: func(p *painter, focused, hovered bool) {
			bg := color.Color(ui.ColorPanel)
			if primary {
				bg = ui.ColorAccent
			}
			if hovered && !disabled {
				bg = ui.ColorHover
				if primary {
					bg = color.RGBA{0x6a, 0xb6, 0xff, 0xff}
				}
			}
			p.fill(r, bg)
			if focused {
				p.stroke(ui.Rect{X: r.X - 3, Y: r.Y - 3, W: r.W + 6, H: r.H + 6}, ui.ColorText)
			}
			c := color.Color(ui.ColorText)
			if primary {
				c = ui.ColorBackground
			}
			if disabled {
				c = ui.ColorBorder
			}
			p.text(label, r.X+r.W/2, r.Y+r.H/2-12, 17, c, ui.Center)
		},
	}
}

// ── 版面 ─────────────────────────────────────────────────

func (m *menuScene) items(a *App) []item {
	cfg := &a.cfg
	saverOnly := cfg.Mode == ModeScreensaver
	var items []item

	// 模式
	{
		var clicks []clickArea
		var draws []func(*painter)
		x := colValue
		for _, md := range []struct {
			mode  Mode
			label string
		}{{ModeBurnIn, "Burn-In 修復"}, {ModeScreensaver, "只播螢幕保護"}} {
			md := md
			c, w, d := radio(x, 80, md.label, func() bool { return cfg.Mode == md.mode }, func() { cfg.Mode = md.mode }, false)
			clicks, draws = append(clicks, c), append(draws, d)
			x += w + 30
		}
		toggle := func() {
			if cfg.Mode == ModeBurnIn {
				cfg.Mode = ModeScreensaver
			} else {
				cfg.Mode = ModeBurnIn
			}
		}
		items = append(items, item{
			row: ui.Rect{X: 20, Y: 74, W: 960, H: 34}, left: toggle, right: toggle, clicks: clicks,
			draw: func(p *painter, _, _ bool) {
				p.text("模式", colLabel, 80, 17, ui.ColorDim, ui.Left)
				for _, d := range draws {
					d(p)
				}
			},
		})
	}

	// 螢幕
	{
		cycle := func(dir int) func() {
			return func() {
				if n := len(a.monitors); n > 0 {
					cfg.Monitor = (cfg.Monitor + dir + n) % n
				}
			}
		}
		label := func() string {
			if cfg.Monitor < len(a.monitors) {
				return a.monitors[cfg.Monitor].label
			}
			return "（找不到螢幕）"
		}
		clicks, d := selector(colValue, 122, 760, label, cycle(-1), cycle(1), false)
		items = append(items, item{
			row: ui.Rect{X: 20, Y: 116, W: 960, H: 34}, left: cycle(-1), right: cycle(1), clicks: clicks,
			draw: func(p *painter, _, _ bool) {
				p.text("螢幕", colLabel, 122, 17, ui.ColorDim, ui.Left)
				d(p)
			},
		})
	}

	// 圖樣
	for i := range cfg.Segments {
		seg := &cfg.Segments[i]
		pt, _ := patterns.Get(seg.Pattern)
		y := rowPattern + float64(i)*rowHeight
		toggle := func() { seg.Enabled = !seg.Enabled }
		dec := func() { seg.Seconds = step(segmentSecondsChoices, seg.Seconds, -1) }
		inc := func() { seg.Seconds = step(segmentSecondsChoices, seg.Seconds, 1) }
		secClicks, secDraw := selector(secondsX, y+2, 150, func() string { return formatSeconds(seg.Seconds) }, dec, inc, saverOnly)
		items = append(items, item{
			row:  ui.Rect{X: 30, Y: y - 2, W: 580, H: rowHeight},
			left: dec, right: inc, activate: toggle, disabled: saverOnly,
			clicks: append(secClicks, clickArea{ui.Rect{X: 30, Y: y - 2, W: 390, H: rowHeight}, toggle}),
			preview: func(dst *ebiten.Image, frame int) {
				pt.Draw(dst, patterns.State{Frame: frame, Time: float64(frame) / 60})
			},
			draw: func(p *painter, _, _ bool) {
				checkbox(p, colLabel, y+2, seg.Enabled, saverOnly)
				p.text(seg.Pattern, colLabel+30, y+2, 17, textColor(saverOnly), ui.Left)
				desc := color.Color(ui.ColorDim)
				if saverOnly {
					desc = ui.ColorBorder
				}
				p.text(shortDesc(pt.Description()), colLabel+175, y+4, 14, desc, ui.Left)
				secDraw(p)
			},
		})
	}

	// 全選 / 全不選
	setAll := func(v bool) func() {
		return func() {
			for i := range cfg.Segments {
				cfg.Segments[i].Enabled = v
			}
		}
	}
	items = append(items,
		button(ui.Rect{X: previewX, Y: 470, W: 150, H: 34}, "全選", false, setAll(true), saverOnly),
		button(ui.Rect{X: previewX + 170, Y: 470, W: 150, H: 34}, "全不選", false, setAll(false), saverOnly),
	)

	// 播放時間
	{
		y := 556.0
		dec := func() { cfg.TimerMinutes = step(timerMinutesChoices, cfg.TimerMinutes, -1) }
		inc := func() { cfg.TimerMinutes = step(timerMinutesChoices, cfg.TimerMinutes, 1) }
		clicks, d := selector(colValue, y, 200, func() string { return fmt.Sprintf("%d 分鐘", cfg.TimerMinutes) }, dec, inc, saverOnly)
		items = append(items, item{
			row: ui.Rect{X: 20, Y: y - 6, W: 960, H: 34}, left: dec, right: inc, clicks: clicks, disabled: saverOnly,
			draw: func(p *painter, _, _ bool) {
				p.text("播放時間", colLabel, y, 17, ui.ColorDim, ui.Left)
				d(p)
				p.text("節目會循環播放，直到時間到", colValue+220, y+2, 14, ui.ColorDim, ui.Left)
			},
		})
	}

	// 時間到後
	{
		y := 594.0
		var clicks []clickArea
		var draws []func(*painter)
		x := colValue
		for _, act := range actions.All {
			act := act
			c, w, d := radio(x, y, act.Label(), func() bool { return cfg.After == act }, func() { cfg.After = act }, saverOnly)
			clicks, draws = append(clicks, c), append(draws, d)
			x += w + 18
		}
		cycle := func(dir int) func() {
			return func() {
				for i, act := range actions.All {
					if act == cfg.After {
						cfg.After = actions.All[(i+dir+len(actions.All))%len(actions.All)]
						return
					}
				}
			}
		}
		items = append(items, item{
			row: ui.Rect{X: 20, Y: y - 6, W: 960, H: 34}, left: cycle(-1), right: cycle(1), clicks: clicks, disabled: saverOnly,
			draw: func(p *painter, _, _ bool) {
				p.text("時間到後", colLabel, y, 17, ui.ColorDim, ui.Left)
				for _, d := range draws {
					d(p)
				}
			},
		})
	}

	// 螢幕保護 / 結束螢幕保護後
	{
		y := 632.0
		savers := screensaver.All()
		cycleSaver := func(dir int) func() {
			return func() {
				for i, sv := range savers {
					if sv.Name() == cfg.Screensaver {
						cfg.Screensaver = savers[(i+dir+len(savers))%len(savers)].Name()
						m.saverStart = time.Now()
						return
					}
				}
			}
		}
		clicks, d := selector(colValue, y, 200, func() string { return cfg.Screensaver }, cycleSaver(-1), cycleSaver(1), false)
		previewSaver := func(dst *ebiten.Image, _ int) {
			if sv, ok := screensaver.Get(cfg.Screensaver); ok {
				sv.Draw(dst, screensaver.State{Time: time.Since(m.saverStart).Seconds(), Now: time.Now()})
			}
		}
		items = append(items, item{
			row: ui.Rect{X: 20, Y: y - 6, W: 420, H: 34}, left: cycleSaver(-1), right: cycleSaver(1), clicks: clicks,
			preview: previewSaver,
			draw: func(p *painter, _, _ bool) {
				p.text("螢幕保護", colLabel, y, 17, ui.ColorDim, ui.Left)
				d(p)
			},
		})

		toggleExit := func() {
			if cfg.SaverExit == SaverExitMenu {
				cfg.SaverExit = SaverExitQuit
			} else {
				cfg.SaverExit = SaverExitMenu
			}
		}
		exitClicks, exitDraw := selector(640, y, 200, func() string {
			if cfg.SaverExit == SaverExitQuit {
				return "關閉程式"
			}
			return "回到選單"
		}, toggleExit, toggleExit, false)
		items = append(items, item{
			row: ui.Rect{X: 450, Y: y - 6, W: 530, H: 34}, left: toggleExit, right: toggleExit, clicks: exitClicks,
			preview: previewSaver,
			draw: func(p *painter, _, _ bool) {
				p.text("結束螢幕保護後", 480, y, 17, ui.ColorDim, ui.Left)
				exitDraw(p)
			},
		})
	}

	// 開始 / 結束
	items = append(items,
		button(ui.Rect{X: 620, Y: 676, W: 190, H: 36}, "開始播放　Enter", true, func() { m.start(a) }, false),
		button(ui.Rect{X: 830, Y: 676, W: 130, H: 36}, "結束　Esc", false, func() { m.quit = true }, false),
	)
	return items
}

// ── 互動 ─────────────────────────────────────────────────

func (m *menuScene) start(a *App) {
	if a.cfg.Mode == ModeBurnIn && len(a.cfg.Program()) == 0 {
		m.errMsg = "請至少勾選一個圖樣"
		return
	}
	if err := a.cfg.Save(a.opts.ConfigPath); err != nil {
		log.Printf("無法儲存設定: %v", err)
	}
	if a.cfg.Mode == ModeScreensaver {
		a.goSaver()
		return
	}
	a.goPlay(a.cfg.Program(), a.cfg.Timer())
}

func keyRepeat(k ebiten.Key) bool {
	d := inpututil.KeyPressDuration(k)
	return d == 1 || (d > repeatDelay && (d-repeatDelay)%repeatInterval == 0)
}

func (m *menuScene) Update(a *App) error {
	if m.quit || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	items := m.items(a)
	if len(a.cfg.Program()) > 0 {
		m.errMsg = ""
	}
	moveFocus := func(dir int) {
		for i := 1; i <= len(items); i++ {
			j := (m.focus + dir*i + len(items)) % len(items)
			if !items[j].disabled {
				m.focus = j
				return
			}
		}
	}
	if items[m.focus].disabled {
		moveFocus(1)
	}

	// 鍵盤
	switch {
	case keyRepeat(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyTab) && !ebiten.IsKeyPressed(ebiten.KeyShift):
		moveFocus(1)
	case keyRepeat(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyTab):
		moveFocus(-1)
	case keyRepeat(ebiten.KeyLeft):
		if f := items[m.focus].left; f != nil {
			f()
		}
	case keyRepeat(ebiten.KeyRight):
		if f := items[m.focus].right; f != nil {
			f()
		}
	case inpututil.IsKeyJustPressed(ebiten.KeySpace):
		if f := items[m.focus].activate; f != nil {
			f()
		}
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter):
		m.start(a)
	}

	// 滑鼠：座標換算成邏輯座標
	s, ox, oy := scaleFor(a.screenW, a.screenH)
	cx, cy := ebiten.CursorPosition()
	mx, my := (float64(cx)-ox)/s, (float64(cy)-oy)/s
	m.hover = -1
	for i, it := range items {
		if it.row.Contains(mx, my) && !it.disabled {
			m.hover = i
		}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
	clicked:
		for i, it := range items {
			if it.disabled {
				continue
			}
			for _, c := range it.clicks {
				if c.r.Contains(mx, my) {
					m.focus = i
					c.fn()
					break clicked
				}
			}
			if it.row.Contains(mx, my) {
				m.focus = i
				if it.activate != nil && it.clicks == nil {
					it.activate() // 按鈕
				}
				break
			}
		}
	}
	return nil
}

func scaleFor(w, h int) (s, ox, oy float64) {
	fw, fh := float64(w), float64(h)
	s = min(fw/WindowWidth, fh/WindowHeight)
	return s, (fw - WindowWidth*s) / 2, (fh - WindowHeight*s) / 2
}

// ── 繪製 ─────────────────────────────────────────────────

func (m *menuScene) Draw(a *App, screen *ebiten.Image) {
	screen.Fill(ui.ColorBackground)
	s, ox, oy := uiScale(screen)
	p := &painter{dst: screen, s: s, ox: ox, oy: oy}
	items := m.items(a)

	p.text("go-screen-burn-in-fixer", colLabel, 22, 24, ui.ColorText, ui.Left)
	p.text("設定", 960, 28, 17, ui.ColorDim, ui.Right)
	p.fill(ui.Rect{X: 20, Y: 62, W: 960, H: 1}, ui.ColorBorder)

	saverOnly := a.cfg.Mode == ModeScreensaver
	hdr := color.Color(ui.ColorDim)
	if saverOnly {
		hdr = ui.ColorBorder
	}
	p.text("圖樣（依序輪播）", colLabel, 178, 17, hdr, ui.Left)
	p.text("每段秒數", secondsX+75, 178, 15, hdr, ui.Center)
	p.fill(ui.Rect{X: 20, Y: 540, W: 960, H: 1}, ui.ColorBorder)

	for i, it := range items {
		focused, hovered := i == m.focus, i == m.hover
		if (hovered || focused) && it.clicks != nil && !it.disabled {
			bg := color.Color(ui.ColorPanel)
			if hovered {
				bg = ui.ColorHover
			}
			p.fill(it.row, bg)
		}
		if focused && it.clicks != nil {
			p.fill(ui.Rect{X: it.row.X, Y: it.row.Y, W: 3, H: it.row.H}, ui.ColorAccent)
		}
		it.draw(p, focused, hovered)
	}

	// 一輪合計
	if !saverOnly {
		total := a.cfg.Program().Total()
		p.text(fmt.Sprintf("一輪合計 %s，共 %d 個圖樣", formatDuration(total), len(a.cfg.Program())), previewX, 432, 15, ui.ColorDim, ui.Left)
	}
	if m.errMsg != "" {
		p.text(m.errMsg, colLabel, 684, 17, ui.ColorError, ui.Left)
	}
	p.text("↑↓ 選擇　←→ 調整　空白鍵 勾選", colLabel, 688, 13, ui.ColorBorder, ui.Left)
	m.drawPreview(a, p, items)
}

// drawPreview 在預覽框繪製滑鼠指到或焦點所在的圖樣（螢幕保護模式時預覽螢幕保護畫面）。
func (m *menuScene) drawPreview(a *App, p *painter, items []item) {
	var preview func(*ebiten.Image, int)
	switch {
	case m.hover >= 0 && items[m.hover].preview != nil:
		preview = items[m.hover].preview
	case items[m.focus].preview != nil:
		preview = items[m.focus].preview
	case a.cfg.Mode == ModeScreensaver:
		preview = items[len(a.cfg.Segments)+6].preview // 螢幕保護選擇列
	default:
		for i, seg := range a.cfg.Segments {
			if seg.Enabled {
				preview = items[2+i].preview
				break
			}
		}
	}
	r := p.rect(ui.Rect{X: previewX, Y: previewY, W: previewW, H: previewH})
	w, h := int(r.W), int(r.H)
	if w <= 0 || h <= 0 {
		return
	}
	if m.previewImg == nil || m.previewImg.Bounds().Dx() != w || m.previewImg.Bounds().Dy() != h {
		if m.previewImg != nil {
			m.previewImg.Deallocate()
		}
		m.previewImg = ebiten.NewImage(w, h)
	}
	m.previewImg.Fill(color.Black)
	if preview != nil {
		preview(m.previewImg, a.frame)
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(int(r.X)), float64(int(r.Y)))
	p.dst.DrawImage(m.previewImg, op)
	p.stroke(ui.Rect{X: previewX, Y: previewY, W: previewW, H: previewH}, ui.ColorBorder)
}

func formatSeconds(sec int) string {
	if sec >= 60 && sec%60 == 0 {
		return fmt.Sprintf("%d 分", sec/60)
	}
	return fmt.Sprintf("%d 秒", sec)
}

// shortDesc 縮短說明，讓圖樣列不會太長：括號內的「寬 N 像素」改成「N px」，其餘括號內容省略。
func shortDesc(s string) string {
	base, paren, _ := strings.Cut(s, "（")
	suffix := ""
	var n int
	if _, err := fmt.Sscanf(paren, "寬 %d 像素", &n); err == nil {
		suffix = fmt.Sprintf(" %d px", n)
	}
	r := []rune(base)
	if max := 12 - len([]rune(suffix)); len(r) > max {
		r = append(r[:max-1], '…')
	}
	return string(r) + suffix
}
