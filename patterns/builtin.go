package patterns

import (
	_ "embed"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

var (
	//go:embed gray_noise.kage
	grayNoiseSrc string
	//go:embed rgb_noise.kage
	rgbNoiseSrc string
	//go:embed stripes.kage
	stripesSrc string
	//go:embed moving_bar.kage
	movingBarSrc string
)

// 預設參數。
const (
	grayNoiseContrast  = 0.8
	grayNoisePixelSize = 1
	stripeWidth        = 4
	fineStripeWidth    = 1
	solidHoldSeconds   = 0.5
	movingBarPeriod    = 1.5
	movingBarFrac      = 0.125
)

func init() {
	Register(newShaderPattern("gray-noise", "每格重新產生的灰階雜訊（1 像素）", grayNoiseSrc, func() map[string]any {
		return map[string]any{"Contrast": float32(grayNoiseContrast), "PixelSize": float32(grayNoisePixelSize)}
	}))
	Register(newShaderPattern("rgb-noise", "紅、綠、藍子像素各自隨機全開或全關的雜訊", rgbNoiseSrc, nil))
	for i, s := range []struct{ name, desc string }{
		{"vstripes", "隨機顏色的直條紋"},
		{"hstripes", "隨機顏色的橫條紋"},
		{"cross", "直條紋與橫條紋交錯成的十字格紋"},
	} {
		mode := float32(i)
		for _, v := range []struct {
			suffix, desc string
			width        float32
		}{
			{"", "（寬 4 像素）", stripeWidth},
			{"-fine", "（寬 1 像素）", fineStripeWidth},
		} {
			width := v.width
			Register(newShaderPattern(s.name+v.suffix, s.desc+v.desc, stripesSrc, func() map[string]any {
				return map[string]any{"StripeWidth": width, "Mode": mode}
			}))
		}
	}
	Register(solidCycle{})
	Register(newShaderPattern("moving-bar", "黑底白色色條，水平、垂直輪流掃過螢幕", movingBarSrc, func() map[string]any {
		return map[string]any{"Period": float32(movingBarPeriod), "BarFrac": float32(movingBarFrac)}
	}))
}

// solidCycle：全螢幕紅 → 綠 → 藍 → 白 → 黑輪播。單色不需要 shader，直接 Fill。
type solidCycle struct{}

var solidColors = []color.RGBA{
	{0xff, 0, 0, 0xff},
	{0, 0xff, 0, 0xff},
	{0, 0, 0xff, 0xff},
	{0xff, 0xff, 0xff, 0xff},
	{0, 0, 0, 0xff},
}

func (solidCycle) Name() string        { return "solid-cycle" }
func (solidCycle) Description() string { return "全螢幕紅 → 綠 → 藍 → 白 → 黑輪播" }

func (solidCycle) Draw(dst *ebiten.Image, st State) {
	i := int(math.Floor(st.Time/solidHoldSeconds)) % len(solidColors)
	dst.Fill(solidColors[i])
}
