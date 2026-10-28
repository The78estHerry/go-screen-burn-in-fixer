package app

import (
	"fmt"
	"image/color"
	"log"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/actions"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/checks"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/internal/ui"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/patterns"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/screensaver"
)

// 設定選單。上方（模式、螢幕）三種模式共用；中間依模式顯示圖樣或檢查項目清單；
// 下方是各模式的設定。每一格都重新建立 items：同一份資料同時用於繪製、滑鼠點擊與鍵盤操作。

const (
	colLabel   = 40.0  // 左側標籤
	colValue   = 160.0 // 設定值
	colRight   = 480.0 // 下方設定右半部的標籤
	rowList    = 212.0 // 清單第一列的 y
	rowHeight  = 30.0  // 清單列高
	secondsX   = 430.0 // 每段秒數選擇器
	previewX   = 640.0 // 預覽框
	previewY   = 176.0
	previewW   = 320.0
	previewH   = 240.0
	rowBottom1 = 544.0 // 下方設定第一列的 y
	rowBottom2 = 582.0
	rowBottom3 = 620.0
)

type menuScene struct {
	focus      int // -1 代表尚未設定（預設放在開始按鈕）
	hover      int
	errMsg     string
	quit       bool
	previewImg *ebiten.Image
	saverStart time.Time

	startIdx      int    // 開始按鈕在 items 中的位置
	levelsIdx     int    // 灰階分階階數列的位置（-1 代表不在畫面上）
	editing       string // 正在輸入的階數
	editingActive bool
	defaultView   func(dst *ebiten.Image, frame int)
}

func newMenuScene(a *App) *menuScene {
	return &menuScene{focus: -1, hover: -1, levelsIdx: -1, saverStart: time.Now()}
}

// ── 版面 ─────────────────────────────────────────────────

func (m *menuScene) items(a *App) []item {
	cfg := &a.cfg
	var items []item
	m.levelsIdx = -1
	m.defaultView = nil

	items = append(items, m.modeRow(cfg), m.monitorRow(a))
	switch cfg.Mode {
	case ModeCheck:
		items = append(items, m.checkItems(cfg)...)
	default:
		items = append(items, m.burnInItems(cfg)...)
	}

	label := "開始播放　Enter"
	if cfg.Mode == ModeCheck {
		label = "開始檢查　Enter"
	}
	m.startIdx = len(items)
	items = append(items,
		button(ui.Rect{X: 620, Y: 672, W: 190, H: 36}, label, true, func() { m.start(a) }, false),
		button(ui.Rect{X: 830, Y: 672, W: 130, H: 36}, "結束　Esc", false, func() { m.quit = true }, false),
	)
	return items
}

func (m *menuScene) modeRow(cfg *Config) item {
	modes := []struct {
		mode  Mode
		label string
	}{{ModeBurnIn, "Burn-In 修復"}, {ModeCheck, "檢查畫面"}, {ModeScreensaver, "只播螢幕保護"}}
	var clicks []clickArea
	var draws []func(*painter)
	x := colValue
	for _, md := range modes {
		c, w, d := radio(x, 80, md.label, func() bool { return cfg.Mode == md.mode }, func() { m.setMode(cfg, md.mode) }, false)
		clicks, draws = append(clicks, c), append(draws, d)
		x += w + 30
	}
	cycle := func(dir int) func() {
		return func() {
			for i, md := range modes {
				if md.mode == cfg.Mode {
					m.setMode(cfg, modes[(i+dir+len(modes))%len(modes)].mode)
					return
				}
			}
		}
	}
	return item{
		row: ui.Rect{X: 20, Y: 74, W: 960, H: 34}, left: cycle(-1), right: cycle(1), clicks: clicks,
		draw: func(p *painter, _, _ bool) {
			p.text("模式", colLabel, 80, 17, ui.ColorDim, ui.Left)
			for _, d := range draws {
				d(p)
			}
		},
	}
}

func (m *menuScene) setMode(cfg *Config, mode Mode) {
	if cfg.Mode != mode {
		cfg.Mode = mode
		m.errMsg = ""
		m.editingActive = false
	}
}

func (m *menuScene) monitorRow(a *App) item {
	cfg := &a.cfg
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
	return item{
		row: ui.Rect{X: 20, Y: 116, W: 960, H: 34}, left: cycle(-1), right: cycle(1), clicks: clicks,
		draw: func(p *painter, _, _ bool) {
			p.text("螢幕", colLabel, 122, 17, ui.ColorDim, ui.Left)
			d(p)
		},
	}
}

// listRow 是圖樣或檢查項目清單中的一列：勾選框、名稱、說明、秒數選擇器。
func listRow(i int, name string, desc func() string, enabled *bool, seconds func() string, dec, inc func(),
	disabled bool, preview func(*ebiten.Image, int)) item {
	y := rowList + float64(i)*rowHeight
	toggle := func() { *enabled = !*enabled }
	secClicks, secDraw := selector(secondsX, y+2, 150, seconds, dec, inc, disabled)
	return item{
		row:  ui.Rect{X: 30, Y: y - 1, W: 580, H: rowHeight},
		left: dec, right: inc, activate: toggle, disabled: disabled,
		clicks:  append(secClicks, clickArea{ui.Rect{X: 30, Y: y - 1, W: 390, H: rowHeight}, toggle}),
		preview: preview,
		draw: func(p *painter, _, _ bool) {
			checkbox(p, colLabel, y+2, *enabled, disabled)
			p.text(name, colLabel+30, y+2, 17, textColor(disabled), ui.Left)
			p.text(desc(), colLabel+175, y+4, 14, dimColor(disabled), ui.Left)
			secDraw(p)
		},
	}
}

// listHeader 繪製清單標題與「全選 / 全不選」按鈕。
func listButtons(setAll func(bool) func(), disabled bool) []item {
	return []item{
		button(ui.Rect{X: previewX, Y: 470, W: 150, H: 34}, "全選", false, setAll(true), disabled),
		button(ui.Rect{X: previewX + 170, Y: 470, W: 150, H: 34}, "全不選", false, setAll(false), disabled),
	}
}

// ── Burn-In / 只播螢幕保護 ───────────────────────────────

func (m *menuScene) burnInItems(cfg *Config) []item {
	saverOnly := cfg.Mode == ModeScreensaver
	var items []item

	for i := range cfg.Segments {
		seg := &cfg.Segments[i]
		pt, _ := patterns.Get(seg.Pattern)
		preview := func(dst *ebiten.Image, frame int) {
			pt.Draw(dst, patterns.State{Frame: frame, Time: float64(frame) / 60})
		}
		if m.defaultView == nil && seg.Enabled && !saverOnly {
			m.defaultView = preview
		}
		items = append(items, listRow(i, seg.Pattern, func() string { return shortDesc(pt.Description()) }, &seg.Enabled,
			func() string { return formatSeconds(seg.Seconds) },
			func() { seg.Seconds = step(segmentSecondsChoices, seg.Seconds, -1) },
			func() { seg.Seconds = step(segmentSecondsChoices, seg.Seconds, 1) },
			saverOnly, preview))
	}
	items = append(items, listButtons(func(v bool) func() {
		return func() {
			for i := range cfg.Segments {
				cfg.Segments[i].Enabled = v
			}
		}
	}, saverOnly)...)

	// 播放時間 | 資訊列
	{
		y := rowBottom1
		dec := func() { cfg.TimerMinutes = step(timerMinutesChoices, cfg.TimerMinutes, -1) }
		inc := func() { cfg.TimerMinutes = step(timerMinutesChoices, cfg.TimerMinutes, 1) }
		clicks, d := selector(colValue, y, 200, func() string { return fmt.Sprintf("%d 分鐘", cfg.TimerMinutes) }, dec, inc, saverOnly)
		items = append(items, item{
			row: ui.Rect{X: 20, Y: y - 6, W: 430, H: 34}, left: dec, right: inc, clicks: clicks, disabled: saverOnly,
			draw: func(p *painter, _, _ bool) {
				p.text("播放時間", colLabel, y, 17, dimColor(saverOnly), ui.Left)
				d(p)
			},
		})
		items = append(items, infoBarRow(cfg, y, saverOnly))
	}

	// 時間到後
	{
		y := rowBottom2
		var clicks []clickArea
		var draws []func(*painter)
		x := colValue
		for _, act := range actions.All {
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
				p.text("時間到後", colLabel, y, 17, dimColor(saverOnly), ui.Left)
				for _, d := range draws {
					d(p)
				}
			},
		})
	}

	// 螢幕保護 | 結束螢幕保護後
	{
		y := rowBottom3
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
		previewSaver := func(dst *ebiten.Image, _ int) {
			if sv, ok := screensaver.Get(cfg.Screensaver); ok {
				sv.Draw(dst, screensaver.State{Time: time.Since(m.saverStart).Seconds(), Now: time.Now()})
			}
		}
		if saverOnly {
			m.defaultView = previewSaver
		}
		clicks, d := selector(colValue, y, 200, func() string { return cfg.Screensaver }, cycleSaver(-1), cycleSaver(1), false)
		items = append(items, item{
			row: ui.Rect{X: 20, Y: y - 6, W: 430, H: 34}, left: cycleSaver(-1), right: cycleSaver(1), clicks: clicks,
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
		exitClicks, exitDraw := selector(previewX, y, 200, func() string {
			if cfg.SaverExit == SaverExitQuit {
				return "關閉程式"
			}
			return "回到選單"
		}, toggleExit, toggleExit, false)
		items = append(items, item{
			row: ui.Rect{X: 460, Y: y - 6, W: 520, H: 34}, left: toggleExit, right: toggleExit, clicks: exitClicks,
			preview: previewSaver,
			draw: func(p *painter, _, _ bool) {
				p.text("結束螢幕保護後", colRight, y, 17, ui.ColorDim, ui.Left)
				exitDraw(p)
			},
		})
	}
	return items
}

// infoBarRow：資訊列顯示設定（Burn-In 與檢查模式共用，位置相同）。
func infoBarRow(cfg *Config, y float64, disabled bool) item {
	dec := func() { cfg.InfoBar = stepAny(infoBarChoices, cfg.InfoBar, -1) }
	inc := func() { cfg.InfoBar = stepAny(infoBarChoices, cfg.InfoBar, 1) }
	clicks, d := selector(previewX, y, 200, func() string { return infoBarLabel(cfg.InfoBar) }, dec, inc, disabled)
	return item{
		row: ui.Rect{X: 460, Y: y - 6, W: 520, H: 34}, left: dec, right: inc, clicks: clicks, disabled: disabled,
		draw: func(p *painter, _, _ bool) {
			p.text("資訊列", colRight, y, 17, dimColor(disabled), ui.Left)
			d(p)
		},
	}
}

// ── 檢查畫面 ─────────────────────────────────────────────

// checkPreview 在預覽框輪流顯示檢查項目的每一階（殘影測試則依時間顯示格紋與灰色）。
func checkPreview(ck checks.Check, cfg *Config) func(*ebiten.Image, int) {
	return func(dst *ebiten.Image, frame int) {
		if gs, ok := ck.(*checks.GraySteps); ok {
			gs.Levels = cfg.GrayLevels
		}
		n := len(ck.Steps())
		t := float64(frame) / 60
		ck.Draw(dst, (frame/90)%n, mod(t, checks.RetentionPatternSeconds+4))
	}
}

func mod(a, b float64) float64 {
	return a - b*float64(int(a/b))
}

func (m *menuScene) checkItems(cfg *Config) []item {
	var items []item

	// 全部設為：自動 10 秒 / 手動
	{
		allIs := func(sec int) bool {
			for _, it := range cfg.CheckItems {
				if it.Seconds != sec {
					return false
				}
			}
			return true
		}
		setAll := func(sec int) func() {
			return func() {
				for i := range cfg.CheckItems {
					cfg.CheckItems[i].Seconds = sec
				}
			}
		}
		autoC, autoW, autoD := radio(250, 178, fmt.Sprintf("自動 %d 秒", DefaultCheckSeconds), func() bool { return allIs(DefaultCheckSeconds) }, setAll(DefaultCheckSeconds), false)
		manC, _, manD := radio(250+autoW+12, 178, "手動", func() bool { return allIs(Manual) }, setAll(Manual), false)
		toggle := func() {
			if allIs(DefaultCheckSeconds) {
				setAll(Manual)()
			} else {
				setAll(DefaultCheckSeconds)()
			}
		}
		items = append(items, item{
			row: ui.Rect{X: 190, Y: 172, W: 420, H: 32}, left: toggle, right: toggle, clicks: []clickArea{autoC, manC},
			draw: func(p *painter, _, _ bool) {
				p.text("全部設為", 180, 178, 15, ui.ColorDim, ui.Left)
				autoD(p)
				manD(p)
			},
		})
	}

	for i := range cfg.CheckItems {
		it := &cfg.CheckItems[i]
		ck, ok := checks.Get(it.Check)
		if !ok {
			continue
		}
		preview := checkPreview(ck, cfg)
		if m.defaultView == nil && it.Enabled {
			m.defaultView = preview
		}
		desc := func() string { return shortDesc(ck.Description()) }
		if _, ok := ck.(*checks.GraySteps); ok {
			desc = func() string { return fmt.Sprintf("灰階分階 %d 階", cfg.GrayLevels) }
		}
		items = append(items, listRow(i, it.Check, desc, &it.Enabled,
			func() string { return checkSecondsLabel(it.Seconds, len(ck.Steps())) },
			func() { it.Seconds = step(checkSecondsChoices, it.Seconds, -1) },
			func() { it.Seconds = step(checkSecondsChoices, it.Seconds, 1) },
			false, preview))
	}
	items = append(items, listButtons(func(v bool) func() {
		return func() {
			for i := range cfg.CheckItems {
				cfg.CheckItems[i].Enabled = v
			}
		}
	}, false)...)

	// 灰階分階階數 | 資訊列
	{
		y := rowBottom1
		dec := func() { m.commitLevels(cfg); cfg.GrayLevels = stepLevels(cfg.GrayLevels, -1) }
		inc := func() { m.commitLevels(cfg); cfg.GrayLevels = stepLevels(cfg.GrayLevels, 1) }
		value := func() string {
			if m.editingActive {
				return m.editing + "▏ 階"
			}
			return fmt.Sprintf("%d 階", cfg.GrayLevels)
		}
		clicks, d := selector(colValue, y, 200, value, dec, inc, false)
		var preview func(*ebiten.Image, int)
		if gs, ok := checks.Get("gray-steps"); ok {
			preview = checkPreview(gs, cfg)
		}
		m.levelsIdx = len(items) + 2 // 前面還有模式、螢幕兩列
		items = append(items, item{
			row: ui.Rect{X: 20, Y: y - 6, W: 430, H: 34}, left: dec, right: inc, clicks: clicks, preview: preview,
			draw: func(p *painter, focused, _ bool) {
				p.text("灰階分階", colLabel, y, 17, ui.ColorDim, ui.Left)
				d(p)
				if focused {
					p.text(fmt.Sprintf("可直接輸入 %d–%d", checks.MinGrayLevels, checks.MaxGrayLevels), colValue+210, y+3, 13, ui.ColorDim, ui.Left)
				}
			},
		})
		items = append(items, infoBarRow(cfg, y, false))
	}

	// 標記
	{
		y := rowBottom2
		items = append(items, button(ui.Rect{X: colValue + 170, Y: y - 6, W: 150, H: 32}, "清除全部標記", false, func() { cfg.Marks = nil }, len(cfg.Marks) == 0))
		items[len(items)-1].draw = wrapDraw(items[len(items)-1].draw, func(p *painter) {
			p.text("標記", colLabel, y, 17, ui.ColorDim, ui.Left)
			p.text(fmt.Sprintf("已有 %d 個", len(cfg.Marks)), colValue, y, 17, ui.ColorText, ui.Left)
		})
	}
	return items
}

func wrapDraw(f func(*painter, bool, bool), extra func(*painter)) func(*painter, bool, bool) {
	return func(p *painter, focused, hovered bool) {
		extra(p)
		f(p, focused, hovered)
	}
}

func checkSecondsLabel(sec, steps int) string {
	if sec == Manual {
		return "手動"
	}
	if steps > 1 {
		return fmt.Sprintf("%d 秒×%d", sec, steps)
	}
	return formatSeconds(sec)
}

// stepLevels 把灰階階數切換到前一個 / 下一個常用值。
func stepLevels(v, dir int) int {
	cs := checks.GrayLevelChoices
	if dir > 0 {
		for _, c := range cs {
			if c > v {
				return c
			}
		}
		return cs[len(cs)-1]
	}
	for i := len(cs) - 1; i >= 0; i-- {
		if cs[i] < v {
			return cs[i]
		}
	}
	return cs[0]
}

// stepAny 在 choices 中依位置前後切換（choices 不需要排序），到頭時停住。
func stepAny(choices []int, v, dir int) int {
	for i, c := range choices {
		if c == v {
			return choices[min(max(i+dir, 0), len(choices)-1)]
		}
	}
	return choices[0]
}

// commitLevels 套用正在輸入的階數。
func (m *menuScene) commitLevels(cfg *Config) {
	if !m.editingActive {
		return
	}
	if n, err := strconv.Atoi(m.editing); err == nil {
		cfg.GrayLevels = checks.ClampGrayLevels(n)
	}
	m.editingActive, m.editing = false, ""
}

// ── 互動 ─────────────────────────────────────────────────

func (m *menuScene) start(a *App) {
	m.commitLevels(&a.cfg)
	switch a.cfg.Mode {
	case ModeBurnIn:
		if len(a.cfg.Program()) == 0 {
			m.errMsg = "請至少勾選一個圖樣"
			return
		}
	case ModeCheck:
		if len(a.cfg.CheckSequence()) == 0 {
			m.errMsg = "請至少勾選一個檢查項目"
			return
		}
	}
	if err := a.cfg.Save(a.opts.ConfigPath); err != nil {
		log.Printf("無法儲存設定: %v", err)
	}
	switch a.cfg.Mode {
	case ModeScreensaver:
		a.goSaver()
	case ModeCheck:
		a.goCheck(a.cfg.CheckSequence())
	default:
		a.goPlay(a.cfg.Program(), a.cfg.Timer())
	}
}

func (m *menuScene) Update(a *App) error {
	if m.quit || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if m.editingActive { // 輸入中按 Esc 只取消輸入
			m.editingActive, m.editing = false, ""
			m.quit = false
			return nil
		}
		return ebiten.Termination
	}
	items := m.items(a)
	if m.focus < 0 || m.focus >= len(items) {
		m.focus = m.startIdx
	}
	if m.errMsg != "" && (a.cfg.Mode == ModeBurnIn && len(a.cfg.Program()) > 0 || a.cfg.Mode == ModeCheck && len(a.cfg.CheckSequence()) > 0) {
		m.errMsg = ""
	}
	moveFocus := func(dir int) {
		m.commitLevels(&a.cfg)
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

	// 灰階分階階數：焦點在這一列時可以直接輸入數字
	if m.focus == m.levelsIdx {
		for _, r := range ebiten.AppendInputChars(nil) {
			if r >= '0' && r <= '9' && len(m.editing) < 3 {
				m.editing += string(r)
				m.editingActive = true
			}
		}
		if m.editingActive && keyRepeat(ebiten.KeyBackspace) && len(m.editing) > 0 {
			m.editing = m.editing[:len(m.editing)-1]
		}
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
		if m.editingActive { // 輸入中按 Enter 只確定數字
			m.commitLevels(&a.cfg)
		} else {
			m.start(a)
		}
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
					if i != m.focus {
						m.commitLevels(&a.cfg)
					}
					m.focus = i
					c.fn()
					break clicked
				}
			}
			if it.row.Contains(mx, my) {
				if i != m.focus {
					m.commitLevels(&a.cfg)
				}
				m.focus = i
				if it.isButton && it.activate != nil {
					it.activate()
				}
				break
			}
		}
	}
	return nil
}

// ── 繪製 ─────────────────────────────────────────────────

func (m *menuScene) Draw(a *App, screen *ebiten.Image) {
	screen.Fill(ui.ColorBackground)
	s, ox, oy := uiScale(screen)
	p := &painter{dst: screen, s: s, ox: ox, oy: oy}
	items := m.items(a)
	focus := m.focus
	if focus < 0 || focus >= len(items) {
		focus = m.startIdx
	}

	p.text("go-screen-burn-in-fixer", colLabel, 22, 24, ui.ColorText, ui.Left)
	p.text("設定", 960, 28, 17, ui.ColorDim, ui.Right)
	p.fill(ui.Rect{X: 20, Y: 62, W: 960, H: 1}, ui.ColorBorder)

	cfg := &a.cfg
	switch cfg.Mode {
	case ModeCheck:
		p.text("檢查項目", colLabel, 178, 17, ui.ColorDim, ui.Left)
		p.text("每階秒數", secondsX+75, 194, 13, ui.ColorDim, ui.Center)
	default:
		hdr := dimColor(cfg.Mode == ModeScreensaver)
		p.text("圖樣（依序輪播）", colLabel, 178, 17, hdr, ui.Left)
		p.text("每段秒數", secondsX+75, 180, 15, hdr, ui.Center)
	}
	p.fill(ui.Rect{X: 20, Y: 530, W: 960, H: 1}, ui.ColorBorder)

	for i, it := range items {
		focused, hovered := i == focus, i == m.hover
		if (hovered || focused) && !it.isButton && !it.disabled {
			bg := color.Color(ui.ColorPanel)
			if hovered {
				bg = ui.ColorHover
			}
			p.fill(it.row, bg)
		}
		if focused && !it.isButton {
			p.fill(ui.Rect{X: it.row.X, Y: it.row.Y, W: 3, H: it.row.H}, ui.ColorAccent)
		}
		it.draw(p, focused, hovered)
	}

	switch cfg.Mode {
	case ModeBurnIn:
		total := cfg.Program().Total()
		p.text(fmt.Sprintf("一輪合計 %s，共 %d 個圖樣", formatDuration(total), len(cfg.Program())), previewX, 432, 15, ui.ColorDim, ui.Left)
	case ModeCheck:
		p.text(checkSummary(cfg.CheckSequence()), previewX, 432, 15, ui.ColorDim, ui.Left)
	}
	if m.errMsg != "" {
		p.text(m.errMsg, colLabel, 652, 17, ui.ColorError, ui.Left)
	}
	hint := "↑↓ 選擇　←→ 調整　空白鍵 勾選"
	if cfg.Mode == ModeCheck {
		hint = "檢查時：Enter／空白鍵 下一階　← 上一階　Esc 取消　點一下 標記"
	}
	p.text(hint, colLabel, 684, 13, ui.ColorBorder, ui.Left)
	m.drawPreview(a, p, items, focus)
}

// checkSummary 回傳檢查流程的總時間說明。
func checkSummary(seq []CheckStep) string {
	var total time.Duration
	manual := 0
	for _, st := range seq {
		if st.Seconds == Manual {
			manual++
			continue
		}
		total += time.Duration(st.Seconds) * time.Second
		if st.Check.Name() == "retention-test" {
			total += checks.RetentionPatternSeconds * time.Second
		}
	}
	s := fmt.Sprintf("共 %d 階，自動約 %s", len(seq), formatDuration(total))
	if manual > 0 {
		s += fmt.Sprintf("，手動 %d 階", manual)
	}
	return s
}

// drawPreview 在預覽框繪製滑鼠指到或焦點所在的項目；都沒有時顯示該模式的預設內容。
func (m *menuScene) drawPreview(a *App, p *painter, items []item, focus int) {
	preview := m.defaultView
	switch {
	case m.hover >= 0 && items[m.hover].preview != nil:
		preview = items[m.hover].preview
	case items[focus].preview != nil:
		preview = items[focus].preview
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
