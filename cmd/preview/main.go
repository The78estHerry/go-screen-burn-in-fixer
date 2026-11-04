// preview 是開發用工具：用與正式程式相同的 shader 產生每種畫面的預覽影片。
// 每格固定 1/60 秒（不依賴實際 FPS），以 ReadPixels 取得畫面後透過 stdin 交給 ffmpeg 編碼。
//
//	go run ./cmd/preview [-out previews] [-size 640x480] [-fps 60] [-seconds 3] [-only 名稱]
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/The78estHerry/go-screen-burn-in-fixer/checks"
	"github.com/The78estHerry/go-screen-burn-in-fixer/patterns"
	"github.com/The78estHerry/go-screen-burn-in-fixer/screensaver"
)

// 時鐘預覽使用固定時間，讓每次產生的影片都相同。
var clockBase = time.Date(2026, 1, 1, 10, 8, 0, 0, time.Local)

// 1 像素的彩色細節在 yuv420p（色度只有 1/4 解析度）下會糊掉，這些圖樣改用 yuv444p。
var fullChroma = map[string]bool{
	"rgb-noise":     true,
	"vstripes-fine": true,
	"hstripes-fine": true,
	"cross-fine":    true,
}

// 完全隨機的雜訊幾乎無法壓縮（3 秒可達 60 MB 以上），這些圖樣的預覽縮短成 0.5 秒，
// 畫質不變，讓預覽影片能放進版本控制。
var shortSeconds = map[string]float64{
	"gray-noise": 0.5,
	"rgb-noise":  0.5,
	"cross":      0.5,
	"cross-fine": 0.5,
}

type job struct {
	name    string  // 檔名（不含副檔名）
	pixFmt  string  // 輸出的像素格式
	seconds float64 // 影片長度；0 代表使用 -seconds 參數
	render  func(dst *ebiten.Image, frame int, fps int)
	still   func(out string) error // 設定時輸出 PNG 靜態圖（<name>.png），不產生影片
}

type game struct {
	jobs   []job
	next   int
	outDir string
	w, h   int
	fps    int
	frames int
	canvas *ebiten.Image
	pixels []byte
	failed bool
}

func (g *game) Update() error {
	if g.next >= len(g.jobs) {
		return ebiten.Termination
	}
	j := g.jobs[g.next]
	g.next++
	out := filepath.Join(g.outDir, j.name+".mp4")
	start := time.Now()
	run := func() error { return g.encode(j, out) }
	if j.still != nil {
		out = filepath.Join(g.outDir, j.name+".png")
		run = func() error { return j.still(out) }
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if err := run(); err != nil {
		log.Printf("✗ %s: %v", j.name, err)
		g.failed = true
		return nil
	}
	log.Printf("✓ %s（%.1f 秒）", out, time.Since(start).Seconds())
	return nil
}

// encodeArgs 回傳一個輸出檔的 libx264 編碼參數。
func encodeArgs(pixFmt, out string, extra ...string) []string {
	args := append([]string{"-an"}, extra...)
	return append(args, "-c:v", "libx264", "-preset", "slow", "-crf", "16", "-pix_fmt", pixFmt, "-movflags", "+faststart", out)
}

// encode 逐格繪製並送給 ffmpeg。整支影片在一次 Update 中完成。
// yuv444p 的影片 Windows 內建播放器無法播放，所以另外同時輸出一支放大 2 倍（每個像素變成 2×2）
// 的 yuv420p 版本 <name>-2x.mp4：2×2 區塊剛好對齊色度取樣，顏色不會糊掉。
func (g *game) encode(j job, out string) error {
	args := []string{"-y", "-hide_banner", "-loglevel", "error",
		"-f", "rawvideo", "-pix_fmt", "rgba", "-s", fmt.Sprintf("%dx%d", g.w, g.h), "-r", fmt.Sprint(g.fps), "-i", "-"}
	args = append(args, encodeArgs(j.pixFmt, out)...)
	if j.pixFmt == "yuv444p" {
		out2x := strings.TrimSuffix(out, ".mp4") + "-2x.mp4"
		args = append(args, encodeArgs("yuv420p", out2x, "-vf", "scale=iw*2:ih*2:flags=neighbor")...)
	}
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("無法啟動 ffmpeg（需要安裝 ffmpeg）: %w", err)
	}
	frames := g.frames
	if j.seconds > 0 {
		frames = int(j.seconds * float64(g.fps))
	}
	for i := 0; i < frames; i++ {
		g.canvas.Clear()
		j.render(g.canvas, i, g.fps)
		g.canvas.ReadPixels(g.pixels)
		if _, err := stdin.Write(g.pixels); err != nil {
			stdin.Close()
			cmd.Wait()
			return err
		}
	}
	if err := stdin.Close(); err != nil && err != io.ErrClosedPipe {
		return err
	}
	return cmd.Wait()
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.DrawImage(g.canvas, nil)
}

func (g *game) Layout(int, int) (int, int) { return g.w, g.h }

func main() {
	outDir := flag.String("out", "previews", "輸出資料夾")
	size := flag.String("size", "640x480", "解析度 WxH")
	fps := flag.Int("fps", 60, "影格率")
	seconds := flag.Float64("seconds", 3, "每支影片的秒數")
	only := flag.String("only", "", "只產生指定名稱的畫面")
	flag.Parse()

	var w, h int
	if _, err := fmt.Sscanf(*size, "%dx%d", &w, &h); err != nil || w <= 0 || h <= 0 {
		log.Fatalf("解析度格式錯誤: %q", *size)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	g := &game{
		jobs:   buildJobs(*only),
		outDir: *outDir,
		w:      w, h: h, fps: *fps,
		frames: int(*seconds * float64(*fps)),
		canvas: ebiten.NewImage(w, h),
		pixels: make([]byte, 4*w*h),
	}
	if len(g.jobs) == 0 {
		log.Fatalf("找不到名稱為 %q 的畫面", *only)
	}

	ebiten.SetWindowTitle("go-screen-burn-in-fixer preview")
	ebiten.SetWindowSize(w, h)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
	if g.failed {
		os.Exit(1)
	}
}

func buildJobs(only string) []job {
	var jobs []job
	for _, p := range patterns.All() {
		if only != "" && only != p.Name() {
			continue
		}
		pixFmt := "yuv420p"
		if fullChroma[p.Name()] {
			pixFmt = "yuv444p"
		}
		jobs = append(jobs, job{name: p.Name(), pixFmt: pixFmt, seconds: shortSeconds[p.Name()], render: func(dst *ebiten.Image, frame, fps int) {
			p.Draw(dst, patterns.State{Frame: frame, Time: float64(frame) / float64(fps)})
		}})
	}
	for _, s := range screensaver.All() {
		if only != "" && only != s.Name() {
			continue
		}
		jobs = append(jobs, job{name: "screensaver-" + s.Name(), pixFmt: "yuv420p", render: func(dst *ebiten.Image, frame, fps int) {
			t := float64(frame) / float64(fps)
			s.Draw(dst, screensaver.State{Time: t, Now: clockBase.Add(time.Duration(t * float64(time.Second)))})
		}})
	}
	for _, c := range checks.All() {
		if only != "" && only != c.Name() {
			continue
		}
		jobs = append(jobs, job{name: "checks/" + c.Name(), still: func(out string) error { return writeCheckSheet(c, out) }})
		if c.Name() == "retention-test" {
			// 殘影測試會隨時間切換畫面，另外輸出實際速度的影片：格紋 10 秒 + 灰色 5 秒。
			jobs = append(jobs, job{name: "checks/" + c.Name(), pixFmt: "yuv420p", seconds: checks.RetentionPatternSeconds + 5,
				render: func(dst *ebiten.Image, frame, fps int) {
					c.Draw(dst, 0, float64(frame)/float64(fps))
				}})
		}
	}
	return jobs
}
