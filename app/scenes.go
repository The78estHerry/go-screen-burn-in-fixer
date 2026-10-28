package app

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/actions"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/internal/ui"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/patterns"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/program"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/screensaver"
)

// uiScale 回傳把邏輯座標（WindowWidth × WindowHeight）放進畫面的縮放比例與左上角位移。
func uiScale(screen *ebiten.Image) (s, ox, oy float64) {
	b := screen.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	s = math.Min(w/WindowWidth, h/WindowHeight)
	return s, (w - WindowWidth*s) / 2, (h - WindowHeight*s) / 2
}

// ── 警告畫面 ─────────────────────────────────────────────

type warningScene struct{}

func (warningScene) Update(a *App) error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	if anyInput() {
		a.goMenu()
	}
	return nil
}

func (warningScene) Draw(a *App, screen *ebiten.Image) {
	screen.Fill(ui.ColorBackground)
	s, ox, oy := uiScale(screen)
	cx := ox + WindowWidth/2*s
	ui.Text(screen, "⚠ 警告", cx, oy+150*s, 44*s, ui.ColorWarning, ui.Center)
	lines := []string{
		"本程式內含快速閃爍的畫面。",
		"光敏性癲癇患者或對閃光敏感的人請勿觀看，",
		"播放時也請避免一直盯著螢幕。",
		"",
		"OLED 烙印是永久性的面板老化，無法用任何畫面修復；",
		"本工具可能有助於減輕暫時性殘影或喚醒卡住的像素，但不保證有效。",
	}
	for i, l := range lines {
		ui.Text(screen, l, cx, oy+(250+float64(i)*38)*s, 22*s, ui.ColorText, ui.Center)
	}
	ui.Text(screen, "按任意鍵或點一下繼續　　Esc 結束", cx, oy+560*s, 20*s, ui.ColorDim, ui.Center)
}

// ── 播放 Burn-In ─────────────────────────────────────────

type playScene struct {
	prog  program.Program
	timer time.Duration

	start       time.Time
	paused      bool
	pausedAt    time.Time
	pausedTotal time.Duration
	shift       time.Duration // ←/→ 切換圖樣造成的節目時間位移
	frame       int

	info       *infoBar
	lastIdx    int
	lastCursor [2]int
}

func newPlayScene(p program.Program, timer time.Duration, infoSetting int) *playScene {
	cx, cy := ebiten.CursorPosition()
	return &playScene{prog: p, timer: timer, start: time.Now(), info: newInfoBar(infoSetting), lastCursor: [2]int{cx, cy}}
}

// elapsed 回傳實際播放的時間（不含暫停）。
func (s *playScene) elapsed() time.Duration {
	now := time.Now()
	if s.paused {
		now = s.pausedAt
	}
	return now.Sub(s.start) - s.pausedTotal
}

// position 回傳目前的段落與段落內的時間。
func (s *playScene) position() (int, time.Duration) {
	t := s.elapsed() + s.shift
	if total := s.prog.Total(); t < 0 {
		t = t%total + total
	}
	return s.prog.At(t)
}

func (s *playScene) Update(a *App) error {
	s.info.update()
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		if a.opts.SkipMenu {
			return ebiten.Termination
		}
		a.leave()
		return nil
	case inpututil.IsKeyJustPressed(ebiten.KeySpace):
		if s.paused {
			s.pausedTotal += time.Since(s.pausedAt)
		} else {
			s.pausedAt = time.Now()
		}
		s.paused = !s.paused
	case inpututil.IsKeyJustPressed(ebiten.KeyRight):
		idx, off := s.position()
		s.shift += s.prog[idx].Duration - off
	case inpututil.IsKeyJustPressed(ebiten.KeyLeft):
		idx, off := s.position()
		if off < time.Second { // 段落剛開始時，跳到上一段；否則回到這一段的開頭
			prev := (idx - 1 + len(s.prog)) % len(s.prog)
			off += s.prog[prev].Duration
		}
		s.shift -= off
		s.lastIdx = -1 // 回到同一段的開頭也顯示資訊列
	}
	if cx, cy := ebiten.CursorPosition(); cx != s.lastCursor[0] || cy != s.lastCursor[1] {
		s.lastCursor = [2]int{cx, cy}
		s.info.flash()
	}
	if idx, _ := s.position(); idx != s.lastIdx { // 換圖樣時顯示資訊列
		s.lastIdx = idx
		s.info.flash()
	}
	if !s.paused {
		s.frame++
	}
	if s.elapsed() >= s.timer {
		return a.finishPlay()
	}
	return nil
}

func (s *playScene) Draw(a *App, screen *ebiten.Image) {
	idx, off := s.position()
	if p, ok := patterns.Get(s.prog[idx].Pattern); ok {
		p.Draw(screen, patterns.State{Frame: s.frame, Time: off.Seconds()})
	}
	state := ""
	if s.paused {
		state = "　［暫停］"
	}
	line1 := fmt.Sprintf("%s（%d/%d）　剩餘 %s　%.0f FPS%s",
		s.prog[idx].Pattern, idx+1, len(s.prog), formatDuration(s.timer-s.elapsed()), ebiten.ActualFPS(), state)
	s.info.draw(screen, line1, "Esc 回到選單　空白鍵 暫停　← → 切換圖樣　H 資訊列　按住 S 顯示")
}

// formatDuration 把時間格式化成 h:mm:ss 或 m:ss。
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	sec := int(d.Round(time.Second).Seconds())
	if sec >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", sec/3600, sec/60%60, sec%60)
	}
	return fmt.Sprintf("%d:%02d", sec/60, sec%60)
}

// ── 關機 / 睡眠前倒數 ────────────────────────────────────

type countdownScene struct {
	action     actions.After
	duration   time.Duration
	start      time.Time
	lastCursor [2]int
}

func newCountdownScene(action actions.After, d time.Duration) *countdownScene {
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	cx, cy := ebiten.CursorPosition()
	return &countdownScene{action: action, duration: d, start: time.Now(), lastCursor: [2]int{cx, cy}}
}

func (s *countdownScene) Update(a *App) error {
	cx, cy := ebiten.CursorPosition()
	moved := math.Hypot(float64(cx-s.lastCursor[0]), float64(cy-s.lastCursor[1])) > 10
	if anyInput() || moved { // 取消
		a.leave()
		return nil
	}
	if time.Since(s.start) < s.duration {
		return nil
	}
	if a.opts.DryRun {
		log.Printf("測試模式：倒數結束，略過「%s」", s.action.Label())
		a.showMessage(fmt.Sprintf("測試模式（-dry-run）：此時會「%s」，實際未執行。", s.action.Label()), 10*time.Second, func() error {
			a.goMenu()
			return nil
		})
		return nil
	}
	if err := actions.Execute(s.action); err != nil {
		a.showMessage(fmt.Sprintf("無法%s：%v", s.action.Label(), err), time.Minute, func() error {
			a.goMenu()
			return nil
		})
		return nil
	}
	if s.action == actions.Shutdown {
		return ebiten.Termination
	}
	// 從睡眠喚醒後回到選單；帶參數執行時直接結束。
	if a.opts.SkipMenu {
		return ebiten.Termination
	}
	a.goMenu()
	return nil
}

func (s *countdownScene) Draw(a *App, screen *ebiten.Image) {
	screen.Fill(ui.ColorBackground)
	sc, ox, oy := uiScale(screen)
	cx := ox + WindowWidth/2*sc
	left := s.duration - time.Since(s.start)
	ui.Text(screen, fmt.Sprintf("即將%s", s.action.Label()), cx, oy+200*sc, 40*sc, ui.ColorText, ui.Center)
	ui.Text(screen, fmt.Sprintf("%d", int(math.Ceil(left.Seconds()))), cx, oy+280*sc, 120*sc, ui.ColorWarning, ui.Center)
	ui.Text(screen, "按任意鍵、點一下或移動滑鼠取消", cx, oy+480*sc, 24*sc, ui.ColorDim, ui.Center)
	if a.opts.DryRun {
		ui.Text(screen, "（測試模式：不會真的執行）", cx, oy+530*sc, 20*sc, ui.ColorDim, ui.Center)
	}
}

// ── 螢幕保護 ─────────────────────────────────────────────

// 螢幕保護開始後這段時間內不理會滑鼠移動，避免切換畫面時的微小移動直接結束。
const saverGrace = time.Second

type saverScene struct {
	saver      screensaver.Saver
	start      time.Time
	lastCursor [2]int
}

func newSaverScene(name string) *saverScene {
	sv, ok := screensaver.Get(name)
	if !ok {
		sv = screensaver.All()[0]
	}
	return &saverScene{saver: sv, start: time.Now()}
}

func (s *saverScene) Update(a *App) error {
	cx, cy := ebiten.CursorPosition()
	if time.Since(s.start) < saverGrace {
		s.lastCursor = [2]int{cx, cy}
		return nil
	}
	moved := math.Hypot(float64(cx-s.lastCursor[0]), float64(cy-s.lastCursor[1])) > 20
	if anyInput() || moved {
		if a.cfg.SaverExit == SaverExitQuit {
			return ebiten.Termination
		}
		a.leave()
	}
	return nil
}

func (s *saverScene) Draw(a *App, screen *ebiten.Image) {
	s.saver.Draw(screen, screensaver.State{Time: time.Since(s.start).Seconds(), Now: time.Now()})
}

// ── 訊息 ─────────────────────────────────────────────────

type messageScene struct {
	msg   string
	until time.Time
	next  func() error
}

func (s *messageScene) Update(a *App) error {
	if anyInput() || time.Now().After(s.until) {
		return s.next()
	}
	return nil
}

func (s *messageScene) Draw(a *App, screen *ebiten.Image) {
	screen.Fill(ui.ColorBackground)
	sc, ox, oy := uiScale(screen)
	cx := ox + WindowWidth/2*sc
	ui.Text(screen, s.msg, cx, oy+300*sc, 24*sc, ui.ColorText, ui.Center)
	ui.Text(screen, "按任意鍵繼續", cx, oy+400*sc, 20*sc, ui.ColorDim, ui.Center)
}
