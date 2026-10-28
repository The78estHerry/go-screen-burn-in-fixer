// Package ui 提供介面共用的字型、顏色與繪圖輔助函式。
//
// 字型：Noto Sans TC 子集（SIL Open Font License 1.1，見 NotoSansTC-LICENSE.txt），
// 由 tools/fontsubset/subset.py 產生，收錄 Big5 常用字與介面用到的符號。
package ui

import (
	"bytes"
	_ "embed"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

//go:embed NotoSansTC-Subset.otf
var fontData []byte

var fontSource = func() *text.GoTextFaceSource {
	s, err := text.NewGoTextFaceSource(bytes.NewReader(fontData))
	if err != nil {
		panic(err)
	}
	return s
}()

// 介面顏色。
var (
	ColorBackground = color.RGBA{0x15, 0x17, 0x1c, 0xff}
	ColorPanel      = color.RGBA{0x20, 0x23, 0x2a, 0xff}
	ColorHover      = color.RGBA{0x2b, 0x2f, 0x38, 0xff}
	ColorBorder     = color.RGBA{0x3a, 0x3f, 0x4a, 0xff}
	ColorText       = color.RGBA{0xe6, 0xe6, 0xe6, 0xff}
	ColorDim        = color.RGBA{0x8a, 0x8f, 0x98, 0xff}
	ColorAccent     = color.RGBA{0x4a, 0xa3, 0xff, 0xff}
	ColorWarning    = color.RGBA{0xff, 0xc1, 0x4d, 0xff}
	ColorError      = color.RGBA{0xff, 0x6b, 0x6b, 0xff}
	ColorHUD        = color.RGBA{0x00, 0x00, 0x00, 0xc0}
)

// Align 是文字的水平對齊方式。
type Align int

const (
	Left Align = iota
	Center
	Right
)

// Face 回傳指定大小（像素）的字型。
func Face(size float64) *text.GoTextFace {
	return &text.GoTextFace{Source: fontSource, Size: size}
}

// Measure 回傳文字的寬與高。
func Measure(s string, size float64) (w, h float64) {
	return text.Measure(s, Face(size), size*1.4)
}

// Text 在 (x, y) 繪製文字；y 是文字頂端。
func Text(dst *ebiten.Image, s string, x, y, size float64, clr color.Color, align Align) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(clr)
	op.LineSpacing = size * 1.4
	switch align {
	case Center:
		op.PrimaryAlign = text.AlignCenter
	case Right:
		op.PrimaryAlign = text.AlignEnd
	}
	text.Draw(dst, s, Face(size), op)
}

// Rect 是浮點數矩形。
type Rect struct{ X, Y, W, H float64 }

// Contains 回傳點是否在矩形內。
func (r Rect) Contains(x, y float64) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Fill 填滿矩形。
func Fill(dst *ebiten.Image, r Rect, clr color.Color) {
	vector.FillRect(dst, float32(r.X), float32(r.Y), float32(r.W), float32(r.H), clr, false)
}

// Stroke 繪製矩形外框。
func Stroke(dst *ebiten.Image, r Rect, width float64, clr color.Color) {
	vector.StrokeRect(dst, float32(r.X), float32(r.Y), float32(r.W), float32(r.H), float32(width), clr, true)
}

// Circle 繪製圓形；filled 為 false 時只畫外框。
func Circle(dst *ebiten.Image, cx, cy, r, width float64, clr color.Color, filled bool) {
	if filled {
		vector.FillCircle(dst, float32(cx), float32(cy), float32(r), clr, true)
		return
	}
	vector.StrokeCircle(dst, float32(cx), float32(cy), float32(r), float32(width), clr, true)
}
