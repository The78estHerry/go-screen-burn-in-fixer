package app

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/checks"
)

// 移動滑鼠後十字游標顯示的時間。
const crosshairDuration = 3 * time.Second

// checkScene 依序顯示檢查畫面。畫面上不放任何文字；狀態只顯示在可隱藏的資訊列。
type checkScene struct {
	seq         []CheckStep
	idx         int
	stepStart   time.Time
	info        *infoBar
	lastCursor  [2]int
	cursorUntil time.Time
	marksHidden bool
}

func newCheckScene(seq []CheckStep, infoSetting int) *checkScene {
	cx, cy := ebiten.CursorPosition()
	return &checkScene{seq: seq, stepStart: time.Now(), info: newInfoBar(infoSetting), lastCursor: [2]int{cx, cy}}
}

// stepDuration 回傳目前這一階自動前進前的顯示時間；手動時回傳 0。
func (s *checkScene) stepDuration() time.Duration {
	st := s.seq[s.idx]
	if st.Seconds == Manual {
		return 0
	}
	d := time.Duration(st.Seconds) * time.Second
	if st.Check.Name() == "retention-test" {
		d += checks.RetentionPatternSeconds * time.Second // 格紋階段另外計算
	}
	return d
}

func (s *checkScene) goTo(idx int) {
	s.idx = idx
	s.stepStart = time.Now()
	s.info.flash()
}

// finish 結束檢查（完成或取消）：儲存標記後回到選單；帶參數執行時直接結束。
func (s *checkScene) finish(a *App, cancelled bool) error {
	if cancelled {
		log.Printf("檢查已取消（第 %d/%d 階）", s.idx+1, len(s.seq))
	} else {
		log.Printf("檢查完成，共 %d 階", len(s.seq))
	}
	if !a.opts.SkipMenu {
		if err := a.cfg.Save(a.opts.ConfigPath); err != nil {
			log.Printf("無法儲存設定: %v", err)
		}
	}
	if a.opts.SkipMenu {
		return ebiten.Termination
	}
	a.goMenu()
	return nil
}

func (s *checkScene) Update(a *App) error {
	s.info.update()
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		return s.command(a, "esc")
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter),
		inpututil.IsKeyJustPressed(ebiten.KeySpace):
		if err := s.command(a, "next"); err != nil || a.scene != s {
			return err
		}
	case inpututil.IsKeyJustPressed(ebiten.KeyLeft):
		s.command(a, "prev")
	case inpututil.IsKeyJustPressed(ebiten.KeyC):
		a.setMarks(nil)
	case inpututil.IsKeyJustPressed(ebiten.KeyM):
		s.marksHidden = !s.marksHidden
	}

	cx, cy := ebiten.CursorPosition()
	if cx != s.lastCursor[0] || cy != s.lastCursor[1] {
		s.lastCursor = [2]int{cx, cy}
		s.cursorUntil = time.Now().Add(crosshairDuration)
		s.info.flash()
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		s.toggleMark(a, cx, cy)
		s.cursorUntil = time.Now().Add(crosshairDuration)
	}

	// 多螢幕時只由主程式計時，子程式跟著主程式換階。
	if d := s.stepDuration(); a.up == nil && d > 0 && time.Since(s.stepStart) >= d {
		return s.command(a, "next")
	}
	return nil
}

// command 執行換階指令。子程式把指令交給主程式；主程式執行後把目前的階送給所有子程式。
func (s *checkScene) command(a *App, cmd string) error {
	if a.up != nil {
		a.up.send(cmd)
		return nil
	}
	switch cmd {
	case "esc":
		return s.finish(a, true)
	case "next":
		if s.idx+1 >= len(s.seq) {
			return s.finish(a, false)
		}
		s.goTo(s.idx + 1)
	case "prev":
		s.goTo(max(s.idx-1, 0))
	default:
		return nil
	}
	s.sync(a)
	return nil
}

// sync 把目前的階與已顯示的時間送給所有子程式（殘影測試依時間切換畫面）。
func (s *checkScene) sync(a *App) {
	a.broadcast("step %d %d", s.idx, int64(time.Since(s.stepStart)))
}

func (s *checkScene) remote(a *App, cmd string, args []string) error {
	if a.up == nil { // 主程式：子程式回報的按鍵
		return s.command(a, cmd)
	}
	if cmd == "step" && len(args) == 2 {
		idx, err1 := strconv.Atoi(args[0])
		el, err2 := strconv.ParseInt(args[1], 10, 64)
		if err1 != nil || err2 != nil || idx < 0 || idx >= len(s.seq) {
			return nil
		}
		if idx != s.idx {
			s.goTo(idx)
		}
		s.stepStart = time.Now().Add(-time.Duration(el))
	}
	return nil
}

// toggleMark 在游標位置新增標記；點在既有標記上則移除它。
func (s *checkScene) toggleMark(a *App, x, y int) {
	r := markRadius(a.screenH) * 1.5
	marks := a.marks()
	for i, mk := range marks {
		if math.Hypot(float64(mk.X-x), float64(mk.Y-y)) <= r {
			a.setMarks(append(marks[:i:i], marks[i+1:]...))
			return
		}
	}
	a.setMarks(append(marks[:len(marks):len(marks)], Mark{X: x, Y: y}))
	s.marksHidden = false
}

func markRadius(screenH int) float64 {
	return 14 * math.Max(float64(screenH)/1080, 0.6)
}

func (s *checkScene) Draw(a *App, screen *ebiten.Image) {
	st := s.seq[s.idx]
	t := time.Since(s.stepStart).Seconds()
	st.Check.Draw(screen, st.Step, t)

	h := screen.Bounds().Dy()
	if !s.marksHidden {
		r := float32(markRadius(h))
		for _, mk := range a.marks() {
			// 黑色外圈加白色內圈：在任何背景上都看得到，中間保持空白不擋住瑕疵。
			vector.StrokeCircle(screen, float32(mk.X), float32(mk.Y), r, 4, color.Black, true)
			vector.StrokeCircle(screen, float32(mk.X), float32(mk.Y), r, 1.5, color.White, true)
		}
	}
	if time.Now().Before(s.cursorUntil) {
		drawCrosshair(screen, s.lastCursor[0], s.lastCursor[1], h)
	}

	steps := st.Check.Steps()
	line1 := fmt.Sprintf("%s（項目 %d/%d，第 %d/%d 階）", steps[st.Step], st.Index+1, st.Items, st.Step+1, len(steps))
	if d := s.stepDuration(); d > 0 {
		line1 += "　剩餘 " + formatDuration(d-time.Since(s.stepStart))
	} else {
		line1 += "　手動：按 Enter／空白鍵 繼續"
	}
	lines := []string{line1}
	if sv, ok := st.Check.(checks.Statuser); ok {
		lines = append(lines, sv.Status(st.Step, t))
	}
	lines = append(lines, st.Check.Hint(),
		fmt.Sprintf("座標 (%d, %d)　標記 %d 個", s.lastCursor[0], s.lastCursor[1], len(a.marks())),
		"Enter／空白鍵 下一階　← 上一階　Esc 取消　點一下 標記／取消標記　C 清除標記　M 隱藏標記　H 資訊列　按住 S 顯示")
	s.info.draw(screen, lines...)
}

// drawCrosshair 繪製十字游標；中心留空，不會擋住要檢查的像素。
func drawCrosshair(screen *ebiten.Image, x, y, screenH int) {
	s := float32(math.Max(float64(screenH)/1080, 0.6))
	fx, fy := float32(x)+0.5, float32(y)+0.5
	gap, length := 6*s, 24*s
	for _, pass := range []struct {
		w float32
		c color.Color
	}{{3, color.Black}, {1, color.White}} {
		vector.StrokeLine(screen, fx-gap-length, fy, fx-gap, fy, pass.w, pass.c, false)
		vector.StrokeLine(screen, fx+gap, fy, fx+gap+length, fy, pass.w, pass.c, false)
		vector.StrokeLine(screen, fx, fy-gap-length, fx, fy-gap, pass.w, pass.c, false)
		vector.StrokeLine(screen, fx, fy+gap, fx, fy+gap+length, pass.w, pass.c, false)
	}
}
