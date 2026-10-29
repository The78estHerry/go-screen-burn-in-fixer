package app

import (
	"fmt"
	"image/color"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/actions"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/checks"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/internal/ui"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/patterns"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/program"
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

	startIdx    int    // 開始按鈕在 items 中的位置
	editKey     string // 正在輸入的列（numEdit.key）；空字串代表沒有在輸入
	editing     string // 正在輸入的文字
	editApply   func(string)
	defaultView func(dst *ebiten.Image, frame int)
	monCursor   int // 螢幕列中鍵盤所在的螢幕
}

func newMenuScene(a *App) *menuScene {
	return &menuScene{focus: -1, hover: -1, saverStart: time.Now()}
}

// ── 版面 ─────────────────────────────────────────────────

func (m *menuScene) items(a *App) []item {
	cfg := &a.cfg
	var items []item
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
		m.cancelEdit()
	}
}

// monitorRow：只有一個螢幕時顯示它的說明；多個螢幕時每個螢幕一個勾選框，可以同時播放。
// 鍵盤用 ←→ 選擇螢幕、空白鍵勾選。
func (m *menuScene) monitorRow(a *App) item {
	cfg := &a.cfg
	const y = 122.0
	row := ui.Rect{X: 20, Y: 116, W: 960, H: 34}
	if len(a.monitors) <= 1 {
		label := "（找不到螢幕）"
		if len(a.monitors) == 1 {
			label = a.monitors[0].label
		}
		return item{row: row, draw: func(p *painter, _, _ bool) {
			p.text("螢幕", colLabel, y, 17, ui.ColorDim, ui.Left)
			p.text(label, colValue, y, 17, ui.ColorText, ui.Left)
		}}
	}

	// 螢幕名稱放不下時只顯示編號與解析度。
	labels := make([]string, len(a.monitors))
	width := 0.0
	for i, mon := range a.monitors {
		labels[i] = mon.short
		w, _ := ui.Measure(mon.short, 17)
		width += w + 60
	}
	if width > 960-colValue {
		for i, mon := range a.monitors {
			labels[i] = mon.tiny
		}
	}
	m.monCursor = min(m.monCursor, len(a.monitors)-1)
	toggle := func(i int) func() {
		return func() {
			m.monCursor = i
			cfg.Monitors = toggleMonitor(cfg.Monitors, i)
		}
	}
	var clicks []clickArea
	var draws []func(p *painter, focused bool)
	x := colValue
	for i := range a.monitors {
		w, _ := ui.Measure(labels[i], 17)
		bx := x
		clicks = append(clicks, clickArea{ui.Rect{X: bx - 4, Y: y - 4, W: w + 36, H: 28}, toggle(i)})
		draws = append(draws, func(p *painter, focused bool) {
			checkbox(p, bx, y, contains(cfg.Monitors, i), false)
			p.text(labels[i], bx+28, y, 17, ui.ColorText, ui.Left)
			if focused && i == m.monCursor {
				p.fill(ui.Rect{X: bx + 28, Y: y + 25, W: w, H: 2}, ui.ColorAccent)
			}
		})
		x += w + 60
	}
	move := func(dir int) func() {
		return func() { m.monCursor = (m.monCursor + dir + len(a.monitors)) % len(a.monitors) }
	}
	return item{
		row: row, left: move(-1), right: move(1), clicks: clicks,
		activate: func() { toggle(m.monCursor)() },
		draw: func(p *painter, focused, _ bool) {
			p.text("螢幕", colLabel, y, 17, ui.ColorDim, ui.Left)
			for _, d := range draws {
				d(p, focused)
			}
		},
	}
}

// toggleMonitor 勾選或取消勾選螢幕 i（至少保留一個），回傳依編號排序的清單。
// 編號最小的螢幕由主程式播放，其他螢幕各由一個子程式播放。
func toggleMonitor(ms []int, i int) []int {
	var out []int
	found := false
	for _, m := range ms {
		if m == i {
			found = true
			continue
		}
		out = append(out, m)
	}
	if found && len(out) == 0 {
		return ms
	}
	if !found {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
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

	// 順序：依序 / 洗牌 | 每段長度
	{
		const y = 178.0
		seqC, seqW, seqD := radio(95, y, "依序", func() bool { return !cfg.Shuffle }, func() { cfg.Shuffle = false }, saverOnly)
		shC, _, shD := radio(95+seqW, y, "洗牌", func() bool { return cfg.Shuffle }, func() { cfg.Shuffle = true }, saverOnly)
		toggle := func() { cfg.Shuffle = !cfg.Shuffle }
		items = append(items, item{
			row: ui.Rect{X: 85, Y: y - 6, W: 190, H: 32}, left: toggle, right: toggle, activate: toggle,
			clicks: []clickArea{seqC, shC}, disabled: saverOnly,
			draw: func(p *painter, _, _ bool) {
				seqD(p)
				shD(p)
			},
		})

		off := saverOnly || !cfg.Shuffle
		dec := func() { cfg.ShuffleSlice = step(shuffleSliceChoices, cfg.ShuffleSlice, -1) }
		inc := func() { cfg.ShuffleSlice = step(shuffleSliceChoices, cfg.ShuffleSlice, 1) }
		clicks, d := selector(322, y, 140, func() string { return shuffleSliceLabel(cfg.ShuffleSlice) }, dec, inc, off)
		items = append(items, item{
			row: ui.Rect{X: 278, Y: y - 6, W: 188, H: 32}, left: dec, right: inc, clicks: clicks, disabled: off,
			draw: func(p *painter, _, _ bool) {
				p.text("每段", 285, y+2, 15, dimColor(off), ui.Left)
				d(p)
			},
		})
	}

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
		dec := func() { m.commitEdit(); cfg.TimerSeconds = stepSorted(timerSecondsChoices, cfg.TimerSeconds, -1) }
		inc := func() { m.commitEdit(); cfg.TimerSeconds = stepSorted(timerSecondsChoices, cfg.TimerSeconds, 1) }
		value := func() string {
			if m.editKey == "timer" {
				return m.editing + "▏"
			}
			return formatTimer(cfg.TimerSeconds)
		}
		clicks, d := selector(colValue, y, 200, value, dec, inc, saverOnly)
		items = append(items, item{
			row: ui.Rect{X: 20, Y: y - 6, W: 430, H: 34}, left: dec, right: inc, clicks: clicks, disabled: saverOnly,
			edit: &numEdit{
				key: "timer",
				accept: func(buf string, r rune) bool {
					return len(buf) < 9 && (r >= '0' && r <= '9' || strings.ContainsRune(":hHmMsS", r))
				},
				apply: func(buf string) {
					if n, ok := parseTimerInput(buf); ok {
						cfg.TimerSeconds = min(n, MaxTimerSeconds)
					}
				},
				hint: fmt.Sprintf("可直接輸入：90（分鐘）、10s（秒）、3m（分鐘）、3h（小時）、2:30（時:分）、1m30s，最長 %d 小時", MaxTimerSeconds/3600),
			},
			draw: func(p *painter, focused, _ bool) {
				p.text("播放時間", colLabel, y, 17, dimColor(saverOnly), ui.Left)
				d(p)
				if focused {
					p.text("可直接輸入", colValue+210, y+3, 13, ui.ColorDim, ui.Left)
				}
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

// shuffleSliceLabel 回傳洗牌每段長度的說明。
func shuffleSliceLabel(sec int) string {
	if sec == program.RandomSlice {
		return fmt.Sprintf("%d–%d 秒", int(program.MinRandomSlice.Seconds()), int(program.MaxRandomSlice.Seconds()))
	}
	return formatSeconds(sec)
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
	if _, ok := ck.(*checks.GrayField); ok {
		return grayLevelsPreview(cfg)
	}
	return func(dst *ebiten.Image, frame int) {
		cfg.applyChecks()
		n := len(ck.Steps())
		t := float64(frame) / 60
		ck.Draw(dst, (frame/90)%n, mod(t, checks.RetentionPatternSeconds+4))
	}
}

// grayValuePreview 在預覽框顯示單一灰階（選到最暗、最亮欄位時）。
func grayValuePreview(v func() int) func(*ebiten.Image, int) {
	return func(dst *ebiten.Image, _ int) {
		g := uint8(v())
		dst.Fill(color.RGBA{g, g, g, 0xff})
	}
}

// grayLevelsPreview 在預覽框輪流顯示灰階全畫面的每一階。
func grayLevelsPreview(cfg *Config) func(*ebiten.Image, int) {
	return func(dst *ebiten.Image, frame int) {
		cfg.applyChecks()
		if ck, ok := checks.Get("gray-field"); ok {
			g := ck.(*checks.GrayField)
			ls := g.Levels()
			g.Value = ls[(frame/45)%len(ls)]
			g.Draw(dst, 0, 0)
		}
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
			row: ui.Rect{X: 168, Y: 172, W: 290, H: 32}, left: toggle, right: toggle, clicks: []clickArea{autoC, manC},
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
		switch ck.(type) {
		case *checks.GraySteps:
			desc = func() string { return fmt.Sprintf("灰階分階 %d 階", cfg.GrayLevels) }
		case *checks.GrayField:
			desc = func() string { return fmt.Sprintf("灰階 %d–%d", cfg.GrayMin, cfg.GrayMax) }
		}
		items = append(items, listRow(i, it.Check, desc, &it.Enabled,
			func() string { return checkSecondsLabel(it.Seconds, cfg.stepCount(ck)) },
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
		dec := func() { m.commitEdit(); cfg.GrayLevels = stepLevels(cfg.GrayLevels, -1) }
		inc := func() { m.commitEdit(); cfg.GrayLevels = stepLevels(cfg.GrayLevels, 1) }
		value := func() string {
			if m.editKey == "levels" {
				return m.editing + "▏ 階"
			}
			return fmt.Sprintf("%d 階", cfg.GrayLevels)
		}
		clicks, d := selector(colValue, y, 200, value, dec, inc, false)
		var preview func(*ebiten.Image, int)
		if gs, ok := checks.Get("gray-steps"); ok {
			preview = checkPreview(gs, cfg)
		}
		items = append(items, item{
			row: ui.Rect{X: 20, Y: y - 6, W: 430, H: 34}, left: dec, right: inc, clicks: clicks, preview: preview,
			edit: &numEdit{
				key:    "levels",
				accept: func(buf string, r rune) bool { return r >= '0' && r <= '9' && len(buf) < 3 },
				apply: func(buf string) {
					if n, err := strconv.Atoi(buf); err == nil {
						cfg.GrayLevels = checks.ClampGrayLevels(n)
					}
				},
				hint: fmt.Sprintf("灰階分階可直接輸入 %d–%d；Enter 確定", checks.MinGrayLevels, checks.MaxGrayLevels),
			},
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

	items = append(items, m.grayFieldItems(cfg)...)

	// 標記
	{
		y := rowBottom3
		items = append(items, button(ui.Rect{X: colValue + 170, Y: y - 6, W: 150, H: 32}, "清除全部標記", false, func() { cfg.ClearMarks() }, cfg.MarkCount() == 0))
		items[len(items)-1].draw = wrapDraw(items[len(items)-1].draw, func(p *painter) {
			p.text("標記", colLabel, y, 17, ui.ColorDim, ui.Left)
			p.text(fmt.Sprintf("已有 %d 個", cfg.MarkCount()), colValue, y, 17, ui.ColorText, ui.Left)
		})
	}
	return items
}

// grayFieldItems：灰階全畫面的最暗、最亮與手動階數，各自可以 ◀▶ 調整或直接輸入。
func (m *menuScene) grayFieldItems(cfg *Config) []item {
	y := rowBottom2
	field := func(key, label string, x, w float64, get func() int, set func(int), lo, hi int,
		preview func(*ebiten.Image, int)) item {
		apply := func(v int) {
			set(v)
			cfg.GrayMin, cfg.GrayMax, cfg.GrayFieldN = checks.ClampGrayField(cfg.GrayMin, cfg.GrayMax, cfg.GrayFieldN)
		}
		dec := func() { m.commitEdit(); apply(max(get()-1, lo)) }
		inc := func() { m.commitEdit(); apply(min(get()+1, hi)) }
		value := func() string {
			if m.editKey == key {
				return m.editing + "▏"
			}
			return strconv.Itoa(get())
		}
		lw, _ := ui.Measure(label, 15)
		clicks, d := selector(x+lw+8, y, w, value, dec, inc, false)
		return item{
			row: ui.Rect{X: x - 8, Y: y - 6, W: lw + w + 22, H: 34}, left: dec, right: inc, clicks: clicks, preview: preview,
			edit: &numEdit{
				key:    key,
				accept: func(buf string, r rune) bool { return r >= '0' && r <= '9' && len(buf) < 3 },
				apply: func(buf string) {
					if n, err := strconv.Atoi(buf); err == nil {
						apply(min(max(n, lo), hi))
					}
				},
				hint: fmt.Sprintf("灰階全畫面的%s可直接輸入 %d–%d；Enter 確定", label, lo, hi),
			},
			draw: func(p *painter, _, _ bool) {
				p.text(label, x, y+2, 15, ui.ColorDim, ui.Left)
				d(p)
			},
		}
	}
	// 調整最暗超過最亮時，最亮跟著推上去（反之亦然），不會互換。
	setMin := func(v int) { cfg.GrayMin, cfg.GrayMax = v, max(cfg.GrayMax, v) }
	setMax := func(v int) { cfg.GrayMax, cfg.GrayMin = v, min(cfg.GrayMin, v) }
	items := []item{
		field("gmin", "最暗", colValue, 96, func() int { return cfg.GrayMin }, setMin, 0, 255,
			grayValuePreview(func() int { return cfg.GrayMin })),
		field("gmax", "最亮", colValue+150, 96, func() int { return cfg.GrayMax }, setMax, 0, 255,
			grayValuePreview(func() int { return cfg.GrayMax })),
		field("gn", "階數", colValue+300, 86, func() int { return cfg.GrayFieldN }, func(v int) { cfg.GrayFieldN = v },
			checks.MinGrayFieldSteps, checks.MaxGrayFieldSteps, grayLevelsPreview(cfg)),
	}
	items[0].draw = wrapDraw(items[0].draw, func(p *painter) {
		p.text("灰階全畫面", colLabel, y, 17, ui.ColorDim, ui.Left)
		g := checks.GrayField{Min: cfg.GrayMin, Max: cfg.GrayMax, N: cfg.GrayFieldN}
		p.text(fmt.Sprintf("階（實際 %d 階）　%.1f%%–%.1f%%", len(g.Levels()), float64(cfg.GrayMin)*100/255, float64(cfg.GrayMax)*100/255),
			colValue+300+40+86+8, y+3, 14, ui.ColorDim, ui.Left)
	})
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
func stepLevels(v, dir int) int { return stepSorted(checks.GrayLevelChoices, v, dir) }

// stepSorted 回傳遞增的 cs 中比 v 大（dir > 0）或比 v 小（dir < 0）的下一個值，到頭時停住；
// v 可以是不在 cs 中的值（直接輸入的數值）。
func stepSorted(cs []int, v, dir int) int {
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

// commitEdit 套用正在輸入的數值。
func (m *menuScene) commitEdit() {
	if m.editKey != "" && m.editApply != nil {
		m.editApply(m.editing)
	}
	m.cancelEdit()
}

// cancelEdit 放棄正在輸入的數值。
func (m *menuScene) cancelEdit() {
	m.editKey, m.editing, m.editApply = "", "", nil
}

// formatTimer 把播放時間（秒）格式化成「10 秒」「5 分鐘」「1 分 30 秒」「2 小時」「2 小時 30 分」。
func formatTimer(sec int) string {
	h, m, s := sec/3600, sec/60%60, sec%60
	var parts []string
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%d 小時", h))
	}
	switch {
	case m > 0 && h == 0 && s == 0:
		parts = append(parts, fmt.Sprintf("%d 分鐘", m))
	case m > 0 || h > 0 && s > 0:
		parts = append(parts, fmt.Sprintf("%d 分", m))
	}
	if s > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d 秒", s))
	}
	return strings.Join(parts, " ")
}

// parseTimerInput 解析直接輸入的播放時間，回傳秒數：
// "90"（沒有單位是分鐘）、"10s"（秒）、"3m"（分鐘）、"3h"（小時）、"2:30"（小時:分）、
// 單位可以組合，例如 "2h30m"、"1m30s"、"1h5m10s"。單位後面沒有單位的數字是下一個較小的單位
// （"2h30" 是 2 小時 30 分、"1m30" 是 1 分 30 秒）。
func parseTimerInput(s string) (int, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if hs, ms, ok := strings.Cut(s, ":"); ok {
		h, err1 := atoiEmpty(hs)
		m, err2 := atoiEmpty(ms)
		total := h*3600 + m*60
		return total, err1 == nil && err2 == nil && total > 0
	}
	units := map[byte]int{'h': 3600, 'm': 60, 's': 1}
	next := map[int]int{3600: 60, 60: 1}
	total, num, last := 0, "", 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			num += string(c)
			continue
		}
		u, ok := units[c]
		if !ok || num == "" || last != 0 && u >= last { // 單位要由大到小，不可重複
			return 0, false
		}
		n, _ := strconv.Atoi(num)
		total += n * u
		num, last = "", u
	}
	if num != "" {
		u := 60 // 沒有單位：分鐘
		if last != 0 {
			if u = next[last]; u == 0 { // 秒後面不能再接數字
				return 0, false
			}
		}
		n, _ := strconv.Atoi(num)
		total += n * u
	}
	return total, total > 0
}

func atoiEmpty(t string) (int, error) {
	if t == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(t)
	if err == nil && n < 0 {
		err = strconv.ErrRange
	}
	return n, err
}

// ── 互動 ─────────────────────────────────────────────────

func (m *menuScene) start(a *App) {
	m.commitEdit()
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
		if m.editKey != "" { // 輸入中按 Esc 只取消輸入
			m.cancelEdit()
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
		m.commitEdit()
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

	// 可以直接輸入數值的列（播放時間、灰階分階階數）
	if e := items[m.focus].edit; e != nil {
		for _, r := range ebiten.AppendInputChars(nil) {
			buf := ""
			if m.editKey == e.key {
				buf = m.editing
			}
			if !e.accept(buf, r) {
				continue
			}
			if m.editKey != e.key {
				m.commitEdit()
				m.editKey, m.editApply = e.key, e.apply
			}
			m.editing += string(r)
		}
		if m.editKey == e.key && keyRepeat(ebiten.KeyBackspace) && len(m.editing) > 0 {
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
		if m.editKey != "" { // 輸入中按 Enter 只確定數值
			m.commitEdit()
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
						m.commitEdit()
					}
					m.focus = i
					c.fn()
					break clicked
				}
			}
			if it.row.Contains(mx, my) {
				if i != m.focus {
					m.commitEdit()
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
		p.text("每階秒數", secondsX+75, 180, 15, ui.ColorDim, ui.Center)
	default:
		hdr := dimColor(cfg.Mode == ModeScreensaver)
		p.text("圖樣", colLabel, 178, 17, hdr, ui.Left)
		col := "每段秒數"
		if cfg.Shuffle {
			col = "每輪秒數"
		}
		p.text(col, secondsX+75, 180, 15, hdr, ui.Center)
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
		sum := fmt.Sprintf("一輪合計 %s，共 %d 個圖樣", formatDuration(total), len(cfg.Program()))
		if cfg.Shuffle {
			sum += "，洗牌"
		}
		p.text(sum, previewX, 432, 15, ui.ColorDim, ui.Left)
	case ModeCheck:
		p.text(checkSummary(cfg.CheckSequence()), previewX, 432, 15, ui.ColorDim, ui.Left)
	}
	if m.errMsg != "" {
		p.text(m.errMsg, colLabel, 652, 17, ui.ColorError, ui.Left)
	}
	hint, hintColor := "↑↓ 選擇　←→ 調整　空白鍵 勾選", color.Color(ui.ColorBorder)
	if cfg.Mode == ModeCheck {
		hint = "檢查時：Enter／空白鍵 下一階　← 上一階　Esc 取消　點一下 標記　右鍵拖曳 調整灰階"
	}
	if e := items[focus].edit; e != nil && !items[focus].disabled {
		hint, hintColor = e.hint, ui.ColorDim
	}
	// 提示在左下角、開始按鈕的左邊；太長時換行，行數多時往上排（最後一行位置不變）。
	lines := wrapText(hint, 13, hintMaxW)
	for i, l := range lines {
		p.text(l, colLabel, 684-float64(len(lines)-1-i)*hintLineH, 13, hintColor, ui.Left)
	}
	m.drawPreview(a, p, items, focus)
}

// 選單左下角提示的寬度上限（到開始按鈕左邊）與行高。
const (
	hintMaxW  = 620 - 16 - colLabel
	hintLineH = 18.0
)

// wrapText 把文字依寬度換行，優先在標點（、，；）與全形空白之後斷開。
func wrapText(s string, size, maxW float64) []string {
	var segs []string
	start := 0
	for i, r := range s {
		if strings.ContainsRune("、，；　", r) {
			end := i + len(string(r))
			segs = append(segs, s[start:end])
			start = end
		}
	}
	if start < len(s) {
		segs = append(segs, s[start:])
	}
	var lines []string
	cur := ""
	for _, seg := range segs {
		if w, _ := ui.Measure(cur+seg, size); cur != "" && w > maxW {
			lines = append(lines, strings.TrimRight(cur, "　"))
			cur = strings.TrimLeft(seg, "　")
			continue
		}
		cur += seg
	}
	return append(lines, strings.TrimRight(cur, "　"))
}

// checkSummary 回傳檢查流程的總時間說明。
func checkSummary(seq []CheckStep) string {
	var total time.Duration
	manual := 0
	steps := 0
	for _, st := range seq {
		n := 1
		if g, ok := st.Check.(*checks.GrayField); ok {
			n = len(g.Levels())
		}
		steps += n
		if st.Seconds == Manual {
			manual += n
			continue
		}
		total += time.Duration(st.Seconds*n) * time.Second
		if st.Check.Name() == "retention-test" {
			total += checks.RetentionPatternSeconds * time.Second
		}
	}
	s := fmt.Sprintf("共 %d 階，自動約 %s", steps, formatDuration(total))
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
