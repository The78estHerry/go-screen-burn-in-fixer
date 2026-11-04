// Package app 是程式的主體：以場景（scene）組成的狀態機。
//
//	警告畫面 → 設定選單 → 播放 Burn-In → 後續動作（關閉 / 倒數後關機或睡眠 / 螢幕保護）
package app

import (
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/The78estHerry/go-screen-burn-in-fixer/actions"
	"github.com/The78estHerry/go-screen-burn-in-fixer/display"
	"github.com/The78estHerry/go-screen-burn-in-fixer/internal/i18n"
	"github.com/The78estHerry/go-screen-burn-in-fixer/program"
)

// 設定選單視窗的邏輯大小（實際大小再乘上螢幕縮放比例）。
const (
	WindowWidth  = 1000
	WindowHeight = 720
)

// Options 是啟動參數。
type Options struct {
	Config     Config
	ConfigPath string
	SkipMenu   bool            // 帶參數執行：跳過警告與選單，直接播放
	Program    program.Program // SkipMenu 時使用的節目表
	Timer      time.Duration   // SkipMenu 時使用的播放時間
	Windowed   bool            // 以視窗模式播放（開發測試用）
	DryRun     bool            // 倒數結束時不真的關機 / 睡眠
	Countdown  time.Duration   // 關機 / 睡眠前的倒數時間
	Child      bool            // 多螢幕播放的子程式：只負責顯示，由主程式控制（見 multi.go）
	Seed       uint64          // 洗牌的亂數種子；0 代表每次播放隨機產生
}

type scene interface {
	Update(a *App) error
	Draw(a *App, screen *ebiten.Image)
}

// monitor 是選單中可選擇的螢幕。說明文字在繪製時才產生（切換語言後立刻改變）。
type monitor struct {
	m    *ebiten.MonitorType
	n    int              // 編號（從 1 開始）
	w, h int              // 解析度（裝置像素）
	disp *display.Display // 作業系統回報的顯示器；對應不到時為 nil
	key  string           // 識別字串：標記依此分開儲存
}

func (mon monitor) prefix() string { return fmt.Sprintf(i18n.T("%d：", "%d: "), mon.n) }

// label 是完整說明（只有一個螢幕時顯示）。
func (mon monitor) label() string {
	if mon.disp != nil {
		return mon.prefix() + mon.disp.Label()
	}
	return mon.tiny()
}

// short 是簡短說明（多個螢幕並排時顯示）。
func (mon monitor) short() string {
	if mon.disp != nil && mon.disp.FriendlyName != "" {
		return fmt.Sprintf("%s%s %d×%d", mon.prefix(), mon.disp.FriendlyName, mon.w, mon.h)
	}
	return mon.tiny()
}

// tiny 是最短的說明：編號與解析度（螢幕名稱放不下時）。
func (mon monitor) tiny() string { return fmt.Sprintf("%s%d×%d", mon.prefix(), mon.w, mon.h) }

// App 實作 ebiten.Game。
type App struct {
	opts     Options
	cfg      Config
	scene    scene
	frame    int // 全域影格計數，給選單預覽動畫使用
	monitors []monitor
	screenW  int
	screenH  int

	playMon int     // 這個程式播放的螢幕（cfg.Monitors 的第一個）
	seed    uint64  // 這次播放的洗牌種子，傳給子程式
	hub     *hub    // 主程式：多螢幕播放中的子程式
	up      *upLink // 子程式：與主程式的連線

	// 開始播放前設定選單視窗所在的螢幕與位置；播放結束回到選單時放回原處。
	menuMon   *ebiten.MonitorType
	menuX     int
	menuY     int
	menuSaved bool
}

// New 建立程式主體。
func New(opts Options) *App {
	a := &App{opts: opts, cfg: opts.Config, seed: opts.Seed}
	if opts.Child {
		a.up = newUpLink()
	}
	a.refreshMonitors()
	switch {
	case opts.SkipMenu && a.cfg.Mode == ModeCheck:
		a.goCheck(a.cfg.CheckSequence())
	case opts.SkipMenu && a.cfg.Mode == ModeScreensaver:
		a.goSaver()
	case opts.SkipMenu:
		a.goPlay(opts.Program, opts.Timer)
	default:
		a.scene = &warningScene{}
	}
	return a
}

// Config 回傳目前的設定（結束時存檔用）。
func (a *App) Config() Config { return a.cfg }

func (a *App) Update() error {
	a.frame++
	if a.up != nil {
		if err := a.pollParent(); err != nil {
			return err
		}
	}
	if err := a.pollChildren(); err != nil {
		return err
	}
	return a.scene.Update(a)
}

func (a *App) Draw(screen *ebiten.Image) {
	a.scene.Draw(a, screen)
}

// Layout 以裝置像素繪製，避免 Windows 縮放比例造成畫面模糊。
func (a *App) Layout(outsideW, outsideH int) (int, int) {
	s := ebiten.Monitor().DeviceScaleFactor()
	a.screenW, a.screenH = int(float64(outsideW)*s), int(float64(outsideH)*s)
	return a.screenW, a.screenH
}

// refreshMonitors 重新取得螢幕列表，並對應作業系統回報的解析度與 EDID 資訊。
func (a *App) refreshMonitors() {
	ms := ebiten.AppendMonitors(nil)
	ds, err := display.Displays()
	if err != nil {
		log.Printf("無法取得顯示器資訊: %v", err)
	}
	used := make([]bool, len(ds))
	a.monitors = a.monitors[:0]
	for i, m := range ms {
		w, h := m.Size()
		s := m.DeviceScaleFactor()
		pw, ph := int(float64(w)*s+0.5), int(float64(h)*s+0.5)
		mon := monitor{m: m, n: i + 1, w: pw, h: ph, key: monitorKey(i, "")}
		// Ebitengine 與作業系統都把主螢幕排在第一個；依序找解析度相同、尚未對應的顯示器。
		for j, d := range ds {
			if !used[j] && d.Current.Width == pw && d.Current.Height == ph {
				used[j] = true
				mon.disp = &ds[j]
				mon.key = monitorKey(i, d.DeviceName+" "+d.FriendlyName)
				break
			}
		}
		a.monitors = append(a.monitors, mon)
	}

	// 移除已經不存在的螢幕；全部都不存在時改用主螢幕。
	if len(a.monitors) > 0 {
		var keep []int
		for _, m := range a.cfg.Monitors {
			if m < len(a.monitors) {
				keep = append(keep, m)
			}
		}
		if len(keep) == 0 {
			keep = []int{0}
		}
		a.cfg.Monitors = keep
	}
	a.playMon = a.cfg.Monitors[0]

	// 舊版設定檔的標記屬於當時選擇的螢幕。
	if len(a.cfg.LegacyMarks) > 0 {
		a.setMarksFor(a.playKey(), append(a.marks(), a.cfg.LegacyMarks...))
		a.cfg.LegacyMarks = nil
	}
}

// monitorKey 回傳螢幕的識別字串：有作業系統的裝置名稱時用裝置名稱，否則用順序。
func monitorKey(i int, device string) string {
	if device != "" && device != " " {
		return device
	}
	return fmt.Sprintf("monitor-%d", i+1)
}

// setWindowMode 切換成一般視窗（警告畫面、設定選單），放回開始播放前的螢幕與位置。
func (a *App) setWindowMode() {
	ebiten.SetFullscreen(false)
	if a.menuSaved {
		if a.menuMon != nil {
			ebiten.SetMonitor(a.menuMon)
		}
		ebiten.SetWindowPosition(a.menuX, a.menuY)
		a.menuSaved = false
	}
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	actions.KeepAwake(false)
}

// setPlayMode 切換成在選定螢幕上全螢幕播放，並防止螢幕關閉。
func (a *App) setPlayMode() {
	a.playMon = a.cfg.Monitors[0]
	if !a.opts.Windowed {
		if !a.menuSaved && !ebiten.IsFullscreen() { // 記住選單視窗的位置（已在播放中切換場景時不覆蓋）
			a.menuMon = ebiten.Monitor()
			a.menuX, a.menuY = ebiten.WindowPosition()
			a.menuSaved = true
		}
		if a.playMon < len(a.monitors) {
			ebiten.SetMonitor(a.monitors[a.playMon].m)
		}
		ebiten.SetFullscreen(true)
		ebiten.SetCursorMode(ebiten.CursorModeHidden)
	}
	actions.KeepAwake(true)
}

func (a *App) goMenu() {
	a.stopChildren()
	a.setWindowMode()
	a.refreshMonitors()
	a.scene = newMenuScene(a)
}

// goPlay 開始播放 Burn-In。base 是依序的節目表；設定洗牌時打散成涵蓋整個播放時間的節目表。
func (a *App) goPlay(base program.Program, timer time.Duration) {
	p := base
	if a.cfg.Shuffle {
		if a.opts.Seed == 0 && !a.opts.Child {
			a.seed = rand.Uint64()
		}
		p = program.Shuffle(base, time.Duration(a.cfg.ShuffleSlice)*time.Second, a.seed, timer)
		log.Printf("開始播放（洗牌，種子 %d）：%s，播放時間 %v", a.seed, base, timer)
	} else {
		log.Printf("開始播放：%s，播放時間 %v", p, timer)
	}
	a.setPlayMode()
	a.scene = newPlayScene(p, timer, a.cfg.InfoBar)
	a.startChildren(ModeBurnIn, base, timer)
}

func (a *App) goCheck(seq []CheckStep) {
	log.Printf("開始檢查：共 %d 階", len(seq))
	a.setPlayMode()
	a.scene = newCheckScene(seq, a.cfg.InfoBar)
	a.startChildren(ModeCheck, nil, 0)
}

// goSaver 切換成螢幕保護畫面；多螢幕播放中時，其他螢幕也一起切換。
func (a *App) goSaver() {
	log.Printf("螢幕保護：%s", a.cfg.Screensaver)
	a.setPlayMode()
	a.scene = newSaverScene(a.cfg.Screensaver)
	a.broadcast("saver")
	a.startChildren(ModeScreensaver, nil, 0)
}

// finishPlay 在播放時間到時執行後續動作。
func (a *App) finishPlay() error {
	log.Printf("播放時間到，後續動作：%s", a.cfg.After)
	switch a.cfg.After {
	case actions.Quit:
		return ebiten.Termination
	case actions.Shutdown, actions.Sleep:
		a.scene = newCountdownScene(a.cfg.After, a.opts.Countdown)
		a.broadcast("blank")
	case actions.Screensaver:
		a.goSaver()
	}
	return nil
}

// leave 從播放、螢幕保護或倒數畫面離開：回到選單。
func (a *App) leave() {
	a.goMenu()
}

// showMessage 顯示訊息，經過 d 時間或按任意鍵後執行 next。
func (a *App) showMessage(msg string, d time.Duration, next func() error) {
	a.stopChildren()
	a.setWindowMode()
	a.scene = &messageScene{msg: msg, until: time.Now().Add(d), next: next}
}

// anyInput 回傳這一格是否有按鍵或滑鼠點擊。
func anyInput() bool {
	return len(inpututil.AppendJustPressedKeys(nil)) > 0 ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
}
