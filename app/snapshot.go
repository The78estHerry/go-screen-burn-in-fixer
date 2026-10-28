package app

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/actions"
)

// Snapshot 把各個畫面畫到離屏影像並存成 PNG（開發用）。不需要操作滑鼠鍵盤，
// 也不會切換全螢幕，用來檢查版面與文字。
func Snapshot(opts Options, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	g := &snapshotGame{a: &App{opts: opts, cfg: opts.Config}, dir: dir}
	if err := ebiten.RunGame(g); err != nil {
		return err
	}
	return g.err
}

type snapshotGame struct {
	a    *App
	dir  string
	done bool
	err  error
}

func (g *snapshotGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	g.done = true
	a := g.a
	a.refreshMonitors()
	a.screenW, a.screenH = WindowWidth, WindowHeight
	a.frame = 90

	menu := newMenuScene(a)
	menu.focus = 2 // 第一個圖樣列：預覽框顯示 gray-noise

	play := newPlayScene(a.cfg.Program(), a.cfg.Timer(), InfoAlways)
	play.frame = 10

	saverCfg := a.cfg
	saverCfg.Mode = ModeScreensaver

	// 檢查模式：第 3 個項目設成手動，示範「全部設為」變成自訂；加兩個標記與十字游標。
	checkCfg := a.cfg
	checkCfg.Mode = ModeCheck
	checkCfg.CheckItems = append([]CheckItemConfig(nil), a.cfg.CheckItems...)
	checkCfg.CheckItems[3].Seconds = Manual
	checkCfg.Marks = []Mark{{X: 300, Y: 260}, {X: 720, Y: 480}}
	checkMenu := newMenuScene(a)
	checkMenu.focus = 3 // 第一個檢查項目
	check := newCheckScene(checkCfg.CheckSequence()[3:4], InfoAlways)
	check.lastCursor = [2]int{560, 400}
	check.cursorUntil = time.Now().Add(time.Minute)

	shots := []struct {
		name  string
		scene scene
		cfg   Config
	}{
		{"warning", warningScene{}, a.cfg},
		{"menu", menu, a.cfg},
		{"menu-check", checkMenu, checkCfg},
		{"menu-screensaver", newMenuScene(a), saverCfg},
		{"play-hud", play, a.cfg},
		{"check-hud", check, checkCfg},
		{"countdown", newCountdownScene(actions.Shutdown, 60*time.Second), a.cfg},
		{"screensaver", newSaverScene("drift-clock"), a.cfg},
	}
	img := ebiten.NewImage(WindowWidth, WindowHeight)
	pix := make([]byte, 4*WindowWidth*WindowHeight)
	for _, s := range shots {
		orig := a.cfg
		a.cfg = s.cfg
		img.Clear()
		s.scene.Draw(a, img)
		a.cfg = orig
		img.ReadPixels(pix)
		if err := savePNG(filepath.Join(g.dir, s.name+".png"), pix); err != nil {
			g.err = err
			return ebiten.Termination
		}
		fmt.Println(filepath.Join(g.dir, s.name+".png"))
	}
	return nil
}

func (g *snapshotGame) Draw(*ebiten.Image) {}

func (g *snapshotGame) Layout(int, int) (int, int) { return WindowWidth, WindowHeight }

func savePNG(path string, pix []byte) error {
	rgba := &image.RGBA{Pix: pix, Stride: 4 * WindowWidth, Rect: image.Rect(0, 0, WindowWidth, WindowHeight)}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, rgba)
}
