package screensaver

import (
	"bytes"
	_ "embed"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/internal/gfx"
)

//go:embed aurora.kage
var auroraSrc string

// Speed 是螢幕保護畫面的動畫速度倍率。太慢的畫面停留太久，同樣可能留下殘影。
const Speed = 5.0

func init() {
	Register(driftClock{})
	Register(&shaderSaver{name: "aurora", desc: "流動的深色漸層色帶", shader: gfx.NewShader("aurora", auroraSrc)})
}

type shaderSaver struct {
	name, desc string
	shader     *gfx.Shader
}

func (s *shaderSaver) Name() string        { return s.name }
func (s *shaderSaver) Description() string { return s.desc }

func (s *shaderSaver) Draw(dst *ebiten.Image, st State) {
	s.shader.Draw(dst, map[string]any{
		"Time":          float32(st.Time * Speed),
		"MaxBrightness": float32(MaxBrightness),
	})
}

// driftClock：黑底低亮度時鐘，沿著 Lissajous 曲線在整個螢幕範圍內漂移，色相緩慢變換。
type driftClock struct{}

// clockMargin 是時鐘與螢幕邊緣保留的距離（佔可移動範圍的比例）。
const clockMargin = 0.02

var clockFont = mustFontSource()

func mustFontSource() *text.GoTextFaceSource {
	s, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		panic(err)
	}
	return s
}

func (driftClock) Name() string        { return "drift-clock" }
func (driftClock) Description() string { return "在整個螢幕漂移、慢慢變色的時鐘" }

func (driftClock) Draw(dst *ebiten.Image, st State) {
	dst.Fill(colorBlack)
	b := dst.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	t := st.Time * Speed

	face := &text.GoTextFace{Source: clockFont, Size: h / 6}
	s := st.Now.Format("15:04")
	tw, th := text.Measure(s, face, 0)

	// 在整個螢幕範圍內漂移；兩軸頻率不同，路徑不會很快重複。
	ax, ay := math.Max(0, (w-tw)/2*(1-clockMargin)), math.Max(0, (h-th)/2*(1-clockMargin))
	x := (w-tw)/2 + ax*math.Sin(t*0.07)
	y := (h-th)/2 + ay*math.Sin(t*0.05+1)

	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	r, g, bl := hsv(math.Mod(t*0.01, 1), 0.6, MaxBrightness)
	op.ColorScale.Scale(float32(r), float32(g), float32(bl), 1)
	text.Draw(dst, s, face, op)
}
