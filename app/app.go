// Package app 是程式的主體：以場景（scene）組成的狀態機。
//
//	警告畫面 → 設定選單 → 播放 Burn-In → 後續動作（關閉 / 倒數後關機或睡眠 / 螢幕保護）
package app

import (
	"fmt"
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/actions"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/display"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/program"
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
}

type scene interface {
	Update(a *App) error
	Draw(a *App, screen *ebiten.Image)
}

// monitor 是選單中可選擇的螢幕。
type monitor struct {
	m     *ebiten.MonitorType
	label string
}

// App 實作 ebiten.Game。
type App struct {
	opts     Options
	cfg      Config
	scene    scene
	frame    int // 全域影格計數，給選單預覽動畫使用
	monitors []monitor
	screenW  int
	screenH  int
}

// New 建立程式主體。
func New(opts Options) *App {
	a := &App{opts: opts, cfg: opts.Config}
	a.refreshMonitors()
	switch {
	case opts.SkipMenu && a.cfg.Mode == ModeCheck:
		a.goCheck(a.cfg.CheckSequence())
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
		label := fmt.Sprintf("%d：%d×%d", i+1, pw, ph)
		// Ebitengine 與作業系統都把主螢幕排在第一個；依序找解析度相同、尚未對應的顯示器。
		for j, d := range ds {
			if !used[j] && d.Current.Width == pw && d.Current.Height == ph {
				used[j] = true
				label = fmt.Sprintf("%d：%s", i+1, d.Label())
				break
			}
		}
		a.monitors = append(a.monitors, monitor{m: m, label: label})
	}
	if a.cfg.Monitor >= len(a.monitors) {
		a.cfg.Monitor = 0
	}
}

// setWindowMode 切換成一般視窗（警告畫面、設定選單）。
func (a *App) setWindowMode() {
	ebiten.SetFullscreen(false)
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	actions.KeepAwake(false)
}

// setPlayMode 切換成在選定螢幕上全螢幕播放，並防止螢幕關閉。
func (a *App) setPlayMode() {
	if !a.opts.Windowed {
		if a.cfg.Monitor < len(a.monitors) {
			ebiten.SetMonitor(a.monitors[a.cfg.Monitor].m)
		}
		ebiten.SetFullscreen(true)
		ebiten.SetCursorMode(ebiten.CursorModeHidden)
	}
	actions.KeepAwake(true)
}

func (a *App) goMenu() {
	a.setWindowMode()
	a.refreshMonitors()
	a.scene = newMenuScene(a)
}

func (a *App) goPlay(p program.Program, timer time.Duration) {
	log.Printf("開始播放：%s，播放時間 %v", p, timer)
	a.setPlayMode()
	a.scene = newPlayScene(p, timer, a.cfg.InfoBar)
}

func (a *App) goCheck(seq []CheckStep) {
	log.Printf("開始檢查：共 %d 階", len(seq))
	a.setPlayMode()
	a.scene = newCheckScene(seq, a.cfg.InfoBar)
}

func (a *App) goSaver() {
	log.Printf("螢幕保護：%s", a.cfg.Screensaver)
	a.setPlayMode()
	a.scene = newSaverScene(a.cfg.Screensaver)
}

// finishPlay 在播放時間到時執行後續動作。
func (a *App) finishPlay() error {
	log.Printf("播放時間到，後續動作：%s", a.cfg.After)
	switch a.cfg.After {
	case actions.Quit:
		return ebiten.Termination
	case actions.Shutdown, actions.Sleep:
		a.scene = newCountdownScene(a.cfg.After, a.opts.Countdown)
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
	a.setWindowMode()
	a.scene = &messageScene{msg: msg, until: time.Now().Add(d), next: next}
}

// anyInput 回傳這一格是否有按鍵或滑鼠點擊。
func anyInput() bool {
	return len(inpututil.AppendJustPressedKeys(nil)) > 0 ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
}
