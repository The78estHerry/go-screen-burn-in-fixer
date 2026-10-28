package screensaver

import (
	"image/color"
	"math"
)

var colorBlack = color.RGBA{0, 0, 0, 0xff}

// hsv 把 (色相, 飽和度, 明度)（皆為 0–1）轉成 RGB。
func hsv(h, s, v float64) (r, g, b float64) {
	f := func(n float64) float64 {
		k := math.Mod(n+h*6, 6)
		return v - v*s*math.Max(0, math.Min(math.Min(k, 4-k), 1))
	}
	return f(5), f(3), f(1)
}
