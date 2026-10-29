package main

import (
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/checks"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/internal/ui"
)

// 檢查畫面預覽：每一階畫成一格（640x480，與 Burn-In 預覽同尺寸、1:1 像素），
// 下方標示階的名稱，全部排成一張 PNG。
const (
	tileW   = 640
	tileH   = 480
	labelH  = 48
	tileGap = 8
)

type sheetTile struct {
	name string
	draw func(dst *ebiten.Image)
}

// grayStepsPreviewLevels 是灰階分階預覽中示範的階數。
var grayStepsPreviewLevels = []int{16, 32, 64, 256}

func writeCheckSheet(c checks.Check, out string) error {
	var steps []sheetTile
	if gf, ok := c.(*checks.GrayField); ok {
		// 灰階全畫面：預設範圍手動時的每一階。
		for _, v := range gf.Levels() {
			steps = append(steps, sheetTile{checks.GrayLabel(v), func(dst *ebiten.Image) { gf.Value = v; gf.Draw(dst, 0, 0) }})
		}
	} else if _, ok := c.(*checks.GraySteps); ok {
		for _, n := range grayStepsPreviewLevels {
			g := checks.NewGraySteps(n)
			steps = append(steps, sheetTile{g.Steps()[0], func(dst *ebiten.Image) { g.Draw(dst, 0, 0) }})
		}
	} else {
		for i, name := range c.Steps() {
			// 殘影測試的靜態圖顯示格紋階段（第 1 秒）。
			steps = append(steps, sheetTile{name, func(dst *ebiten.Image) { c.Draw(dst, i, 1) }})
		}
	}
	cols := len(steps)
	switch {
	case cols == 9:
		cols = 3
	case cols > 4:
		cols = 4
	}
	rows := (len(steps) + cols - 1) / cols
	w := cols*tileW + (cols+1)*tileGap
	h := rows*(tileH+labelH) + (rows+1)*tileGap

	sheet := ebiten.NewImage(w, h)
	defer sheet.Deallocate()
	sheet.Fill(ui.ColorBackground)
	tile := ebiten.NewImage(tileW, tileH)
	defer tile.Deallocate()

	for i, st := range steps {
		name := st.name
		x := tileGap + (i%cols)*(tileW+tileGap)
		y := tileGap + (i/cols)*(tileH+labelH+tileGap)
		tile.Clear()
		st.draw(tile)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x), float64(y))
		sheet.DrawImage(tile, op)
		ui.Stroke(sheet, ui.Rect{X: float64(x), Y: float64(y), W: tileW, H: tileH}, 1, ui.ColorBorder)
		ui.Text(sheet, name, float64(x)+tileW/2, float64(y+tileH)+10, 24, ui.ColorText, ui.Center)
	}

	pix := make([]byte, 4*w*h)
	sheet.ReadPixels(pix)
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	img := &image.RGBA{Pix: pix, Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}
	return png.Encode(f, opaque{img})
}

// opaque 讓 PNG 以不透明 RGB 儲存（檔案比較小）。
type opaque struct{ *image.RGBA }

func (o opaque) Opaque() bool            { return true }
func (o opaque) At(x, y int) color.Color { return o.RGBA.At(x, y) }
func (o opaque) ColorModel() color.Model { return color.RGBAModel }
func (o opaque) Bounds() image.Rectangle { return o.RGBA.Bounds() }
