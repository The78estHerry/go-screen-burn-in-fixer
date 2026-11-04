package checks

import (
	_ "embed"
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/The78estHerry/go-screen-burn-in-fixer/internal/gfx"
	"github.com/The78estHerry/go-screen-burn-in-fixer/internal/ui"
)

var (
	//go:embed gradient.kage
	gradientSrc string
	//go:embed gray_steps.kage
	grayStepsSrc string
)

// 灰階分階的階數範圍。輸出是 8 位元畫面，每個顏色最多 256 種值。
const (
	MinGrayLevels     = 2
	MaxGrayLevels     = 256
	DefaultGrayLevels = 32
)

// GrayLevelChoices 是選單 ◀▶ 快速切換的階數；也可以直接輸入其他數字。
var GrayLevelChoices = []int{8, 16, 32, 64, 128, 256}

// RetentionPatternSeconds 是殘影測試中高對比格紋的顯示時間，之後切換成灰色觀察殘影。
const RetentionPatternSeconds = 10

// 灰階全畫面的範圍（8 位元訊號值 0–255）與手動時的階數。預設 13–191 約為 5–75%。
const (
	DefaultGrayMin        = 13
	DefaultGrayMax        = 191
	DefaultGrayFieldSteps = 7
	MinGrayFieldSteps     = 4
	MaxGrayFieldSteps     = 15
)

// 百分比都是 8 位元訊號值的比例（25% = 64/255），不是亮度。
var (
	colorLevels     = []int{25, 50, 100}
	nearBlackLevels = []int{1, 2, 3, 5}
)

func level(percent int) uint8 {
	return uint8(math.Round(float64(percent) * 255 / 100))
}

func init() {
	Register(&GrayField{Min: DefaultGrayMin, Max: DefaultGrayMax, N: DefaultGrayFieldSteps, Value: DefaultGrayMin})

	Register(NewGraySteps(DefaultGrayLevels))

	// 紅綠藍單色
	var colorSteps []string
	var colors []color.Color
	for _, c := range []struct {
		name    string
		r, g, b bool
	}{{"紅", true, false, false}, {"綠", false, true, false}, {"藍", false, false, true}} {
		for _, p := range colorLevels {
			colorSteps = append(colorSteps, fmt.Sprintf("%s %d%%", c.name, p))
			v := level(p)
			pick := func(on bool) uint8 {
				if on {
					return v
				}
				return 0
			}
			colors = append(colors, color.RGBA{pick(c.r), pick(c.g), pick(c.b), 0xff})
		}
	}
	Register(&fillCheck{
		name:  "color-field",
		desc:  "紅綠藍單色 25/50/100%",
		hint:  "子像素老化不平均時，影子常只出現在某一種顏色。100% 畫面上的黑點代表該顏色的子像素壞了。",
		steps: colorSteps, colors: colors,
	})

	Register(retentionTest{})

	Register(&fillCheck{
		name:  "black",
		desc:  "全黑",
		hint:  "找出一直亮著的點（亮點、卡住的像素），可能是紅、綠、藍或白色。",
		steps: []string{"全黑"}, colors: []color.Color{color.Black},
	})
	Register(&fillCheck{
		name:  "white",
		desc:  "全白",
		hint:  "找出不亮的黑點（暗點、死像素），也可以看出螢幕上的灰塵或異物。",
		steps: []string{"全白"}, colors: []color.Color{color.White},
	})

	var nbSteps []string
	var nbColors []color.Color
	for _, p := range nearBlackLevels {
		nbSteps = append(nbSteps, fmt.Sprintf("近黑 %d%%", p))
		v := level(p)
		nbColors = append(nbColors, color.RGBA{v, v, v, 0xff})
	}
	Register(&fillCheck{
		name:  "near-black",
		desc:  "近黑 1–5%",
		hint:  "找出暗部不均勻（雲斑、mura）、四周或角落偏亮偏暗。請在昏暗的環境下觀看。",
		steps: nbSteps, colors: nbColors,
	})

	Register(&gradientCheck{shader: gfx.NewShader("gradient", gradientSrc)})
}

// fillCheck：每一階是一個全畫面單色。
type fillCheck struct {
	name, desc, hint string
	steps            []string
	colors           []color.Color
}

func (c *fillCheck) Name() string        { return c.name }
func (c *fillCheck) Description() string { return c.desc }
func (c *fillCheck) Hint() string        { return c.hint }
func (c *fillCheck) Steps() []string     { return c.steps }

func (c *fillCheck) Draw(dst *ebiten.Image, step int, _ float64) {
	dst.Fill(c.colors[step])
}

// GrayField：全畫面單一灰階，Min–Max 平均分成 N 階，自動或手動依序前進（秒數是每一階的時間）；
// 也可以用滑鼠右鍵拖曳調整。目前顯示的值（Value）由檢查畫面決定。
type GrayField struct {
	Min, Max, N int
	Value       int
}

// ClampGrayField 把範圍限制在 0–255（min 大於 max 時交換）、階數限制在 MinGrayFieldSteps–MaxGrayFieldSteps。
func ClampGrayField(lo, hi, n int) (int, int, int) {
	lo, hi = min(max(lo, 0), 255), min(max(hi, 0), 255)
	if lo > hi {
		lo, hi = hi, lo
	}
	return lo, hi, min(max(n, MinGrayFieldSteps), MaxGrayFieldSteps)
}

// Levels 回傳每一階：Min 到 Max 平均分成 N 階，但不超過範圍內的灰階數
// （例如 3–4 只有 2 階，Min = Max 只有 1 階）。
func (g *GrayField) Levels() []int {
	k := min(g.N, g.Max-g.Min+1)
	if k <= 1 {
		return []int{g.Min}
	}
	ls := make([]int, k)
	for i := range ls {
		ls[i] = int(math.Round(float64(g.Min) + float64(g.Max-g.Min)*float64(i)/float64(k-1)))
	}
	return ls
}

// GrayLabel 回傳灰階的說明，例如「灰階 128（50.2%）」；百分比是訊號值的比例。
func GrayLabel(v int) string {
	return fmt.Sprintf("灰階 %d（%.1f%%）", v, float64(v)*100/255)
}

func (*GrayField) Name() string        { return "gray-field" }
func (*GrayField) Description() string { return "灰階全畫面（最暗到最亮分階）" }
func (*GrayField) Hint() string {
	return "找出比周圍暗（或亮）的影子，例如工作列、圖示、視窗邊框、遊戲 HUD。烙印在 18–35% 最容易看出來。按住右鍵左右拖曳可調整灰階。"
}
func (*GrayField) Steps() []string { return []string{"灰階全畫面"} }

func (g *GrayField) Draw(dst *ebiten.Image, _ int, _ float64) {
	v := uint8(min(max(g.Value, 0), 255))
	dst.Fill(color.RGBA{v, v, v, 0xff})
}

// GraySteps：畫面由左到右分成 Levels 條灰色色帶（黑到白等距），用來測試螢幕能分辨的灰階數。
// 階數可以由使用者設定（MinGrayLevels–MaxGrayLevels）。
type GraySteps struct {
	Levels int
	shader *gfx.Shader
}

var grayStepsShader = gfx.NewShader("gray-steps", grayStepsSrc)

// NewGraySteps 建立指定階數的灰階分階畫面。
func NewGraySteps(levels int) *GraySteps {
	return &GraySteps{Levels: ClampGrayLevels(levels), shader: grayStepsShader}
}

// ClampGrayLevels 把階數限制在有效範圍內。
func ClampGrayLevels(n int) int {
	return min(max(n, MinGrayLevels), MaxGrayLevels)
}

func (*GraySteps) Name() string        { return "gray-steps" }
func (*GraySteps) Description() string { return "灰階分階（階數可自訂）" }
func (*GraySteps) Hint() string {
	return "相鄰的色帶應該都分得出來。暗部黏在一起代表暗部細節被吃掉；某一段特別寬或出現色偏，代表那一段灰階有問題。"
}
func (g *GraySteps) Steps() []string { return []string{fmt.Sprintf("灰階 %d 階", g.Levels)} }

func (g *GraySteps) Draw(dst *ebiten.Image, _ int, _ float64) {
	g.shader.Draw(dst, map[string]any{"Levels": float32(ClampGrayLevels(g.Levels))})
}

// gradientCheck：灰階與紅綠藍漸層，水平或垂直。
type gradientCheck struct{ shader *gfx.Shader }

func (*gradientCheck) Name() string        { return "gradient" }
func (*gradientCheck) Description() string { return "漸層 水平／垂直" }
func (*gradientCheck) Hint() string {
	return "找出不規則的斷層或色偏（局部老化）。8 位元畫面每個顏色只有 256 階，均勻的細小階梯是正常的。"
}
func (*gradientCheck) Steps() []string { return []string{"漸層 水平", "漸層 垂直"} }

func (c *gradientCheck) Draw(dst *ebiten.Image, step int, _ float64) {
	c.shader.Draw(dst, map[string]any{"Vertical": float32(step)})
}

// retentionTest：先顯示高對比格紋，再切換成 50% 灰，觀察格紋殘影多久消失。
type retentionTest struct{}

func (retentionTest) Name() string        { return "retention-test" }
func (retentionTest) Description() string { return "殘影測試" }
func (retentionTest) Hint() string {
	return fmt.Sprintf("先顯示 %d 秒黑白格紋，再切換成灰色。觀察格紋的殘影要多久才消失：消失得越慢，代表目前的亮度設定越容易留下暫時性殘影。", RetentionPatternSeconds)
}
func (retentionTest) Steps() []string { return []string{"殘影測試"} }

var retentionGray = color.RGBA{0x80, 0x80, 0x80, 0xff}

// Draw 只畫格紋或灰色，畫面上不放任何文字（文字會擋住檢查區域，也可能自己留下殘影）。
// 倒數與經過秒數由 Status 提供，顯示在可隱藏的資訊列。
func (retentionTest) Draw(dst *ebiten.Image, _ int, t float64) {
	if t >= RetentionPatternSeconds {
		dst.Fill(retentionGray)
		return
	}
	b := dst.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	size := h / 6 // 格子邊長
	dst.Fill(color.Black)
	for y := 0; float64(y)*size < h; y++ {
		for x := 0; float64(x)*size < w; x++ {
			if (x+y)%2 == 0 {
				ui.Fill(dst, ui.Rect{X: float64(x) * size, Y: float64(y) * size, W: size, H: size}, color.White)
			}
		}
	}
}

// Status 回傳資訊列顯示的狀態文字。
func (retentionTest) Status(_ int, t float64) string {
	if t < RetentionPatternSeconds {
		return fmt.Sprintf("請注視格紋，%d 秒後切換成灰色", int(math.Ceil(RetentionPatternSeconds-t)))
	}
	return fmt.Sprintf("已切換成灰色 %d 秒：格紋殘影消失了嗎？", int(t-RetentionPatternSeconds))
}
