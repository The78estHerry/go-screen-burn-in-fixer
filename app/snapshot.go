package app

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/The78estHerry/go-screen-burn-in-fixer/actions"
	"github.com/The78estHerry/go-screen-burn-in-fixer/program"
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
	menu.focus = 4 // 第一個圖樣列（前面是模式、螢幕、順序、每段長度）：預覽框顯示 gray-noise

	// 洗牌播放的資訊列（顯示這個圖樣在這一輪已播放的時間）。
	play := newPlayScene(program.Shuffle(a.cfg.Program(), program.RandomSlice, 1, a.cfg.Timer()), a.cfg.Timer(), InfoAlways)
	play.frame = 10
	play.shift = 95 * time.Second

	saverCfg := a.cfg
	saverCfg.Mode = ModeScreensaver

	// 檢查模式：第 3 個項目設成手動，示範「全部設為」變成自訂；加兩個標記與十字游標。
	checkCfg := a.cfg
	checkCfg.Mode = ModeCheck
	checkCfg.CheckItems = append([]CheckItemConfig(nil), a.cfg.CheckItems...)
	checkCfg.CheckItems[3].Seconds = Manual
	checkCfg.MonitorMarks = map[string][]Mark{a.playKey(): {{X: 300, Y: 260}, {X: 720, Y: 480}}}
	checkMenu := newMenuScene(a)
	checkMenu.focus = 2 // 「全部設為」：檢查反白範圍不會蓋住文字
	check := newCheckScene(checkCfg.CheckSequence()[3:4], InfoAlways)
	check.lastCursor = [2]int{560, 400}
	check.cursorUntil = time.Now().Add(time.Minute)

	// 灰階全畫面：右鍵拖曳到兩階之間（切換成手動）。
	grayCheck := newCheckScene(checkCfg.CheckSequence()[0:1], InfoAlways)
	grayCheck.setGray(50)

	// 多螢幕的選單：加上兩個假的螢幕示範勾選框，並選擇洗牌（只畫選單，不會真的播放）。
	multiMons := append(append([]monitor(nil), a.monitors...),
		monitor{short: "2：DELL U2720Q 3840×2160", tiny: "2：3840×2160", key: "fake-2"},
		monitor{short: "3：LG 27GR95QE 2560×1440", tiny: "3：2560×1440", key: "fake-3"})
	multiCfg := a.cfg
	multiCfg.Monitors = []int{0, 2}
	multiCfg.Shuffle = true
	multiMenu := newMenuScene(a)
	multiMenu.focus = 16 // 播放時間：示範直接輸入
	multiMenu.editKey, multiMenu.editing = "timer", "2:30"

	shots := []struct {
		name  string
		scene scene
		cfg   Config
		mons  []monitor
	}{
		{"warning", warningScene{}, a.cfg, nil},
		{"menu", menu, a.cfg, nil},
		{"menu-multi", multiMenu, multiCfg, multiMons},
		{"menu-check", checkMenu, checkCfg, nil},
		{"menu-screensaver", newMenuScene(a), saverCfg, nil},
		{"play-hud", play, a.cfg, nil},
		{"check-hud", check, checkCfg, nil},
		{"check-hud-gray", grayCheck, checkCfg, nil},
		{"countdown", newCountdownScene(actions.Shutdown, 60*time.Second), a.cfg, nil},
		{"screensaver", newSaverScene("drift-clock"), a.cfg, nil},
	}
	img := ebiten.NewImage(WindowWidth, WindowHeight)
	pix := make([]byte, 4*WindowWidth*WindowHeight)
	for _, s := range shots {
		orig, origMons := a.cfg, a.monitors
		a.cfg = s.cfg
		if s.mons != nil {
			a.monitors = s.mons
		}
		img.Clear()
		s.scene.Draw(a, img)
		a.cfg, a.monitors = orig, origMons
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
