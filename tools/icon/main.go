// icon 產生程式圖示 winres/icon.png（256×256）與 winres/icon16.png（16×16）。
//
// 圖案：深色圓角方塊上的一台螢幕，畫面是 Burn-In 修復用的彩色直條紋。
// 用 4×4 超取樣做反鋸齒，只使用標準函式庫。
//
//	go run ./tools/icon
//	go-winres make --arch amd64,arm64
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

const ss = 4 // 超取樣倍率

var stripes = []color.RGBA{
	{0xff, 0x30, 0x30, 0xff}, // 紅
	{0x30, 0xe0, 0x40, 0xff}, // 綠
	{0x30, 0x70, 0xff, 0xff}, // 藍
	{0x20, 0xe0, 0xe0, 0xff}, // 青
	{0xf0, 0x30, 0xe0, 0xff}, // 洋紅
	{0xff, 0xe0, 0x20, 0xff}, // 黃
}

func main() {
	write("winres/icon.png", render(256))
	write("winres/icon16.png", render(16))
}

// shape 回傳邏輯座標 (x, y)（0–1）的顏色。
func shape(x, y float64) color.RGBA {
	transparent := color.RGBA{}
	bg := color.RGBA{0x1c, 0x22, 0x2e, 0xff}
	frame := color.RGBA{0xd8, 0xdd, 0xe6, 0xff}

	if !inRoundRect(x, y, 0.02, 0.02, 0.98, 0.98, 0.18) {
		return transparent
	}
	// 螢幕外框與畫面
	if inRoundRect(x, y, 0.12, 0.17, 0.88, 0.70, 0.05) {
		if inRect(x, y, 0.17, 0.22, 0.83, 0.65) {
			i := int((x - 0.17) / (0.66 / float64(len(stripes))))
			return stripes[min(i, len(stripes)-1)]
		}
		return frame
	}
	// 支架與底座
	if inRect(x, y, 0.45, 0.70, 0.55, 0.80) || inRoundRect(x, y, 0.32, 0.79, 0.68, 0.85, 0.03) {
		return frame
	}
	return bg
}

func inRect(x, y, x0, y0, x1, y1 float64) bool {
	return x >= x0 && x < x1 && y >= y0 && y < y1
}

func inRoundRect(x, y, x0, y0, x1, y1, r float64) bool {
	if !inRect(x, y, x0, y0, x1, y1) {
		return false
	}
	cx := math.Max(x0+r, math.Min(x, x1-r))
	cy := math.Max(y0+r, math.Min(y, y1-r))
	return math.Hypot(x-cx, y-cy) <= r
}

func render(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for py := range size {
		for px := range size {
			var r, g, b, a float64
			for sy := range ss {
				for sx := range ss {
					c := shape((float64(px)+(float64(sx)+0.5)/ss)/float64(size), (float64(py)+(float64(sy)+0.5)/ss)/float64(size))
					fa := float64(c.A) / 255
					r += float64(c.R) * fa
					g += float64(c.G) * fa
					b += float64(c.B) * fa
					a += fa
				}
			}
			if a == 0 {
				continue
			}
			img.SetNRGBA(px, py, color.NRGBA{uint8(r/a + 0.5), uint8(g/a + 0.5), uint8(b/a + 0.5), uint8(a/(ss*ss)*255 + 0.5)})
		}
	}
	return img
}

func write(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
}
