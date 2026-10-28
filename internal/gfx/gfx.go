// Package gfx 提供 Kage shader 的共用工具：組合共用函式、延遲編譯、繪製。
package gfx

import (
	_ "embed"
	"fmt"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

// common.kage 內含所有 shader 共用的函式（hash、色彩轉換）。
// Kage 沒有 include，所以在編譯前把它接在每個 shader 本體前面。
//
//go:embed common.kage
var common string

const header = "//kage:unit pixels\n\npackage main\n\n"

// Shader 是延遲編譯的 Kage shader。本體檔案只需要寫 uniform 與 Fragment，
// 不需要 kage 指示詞與 package 宣告。
type Shader struct {
	name string
	src  []byte

	once   sync.Once
	shader *ebiten.Shader
}

// NewShader 建立 shader；第一次 Draw 時才會編譯（Ebitengine 要求在遊戲迴圈啟動後編譯）。
func NewShader(name, body string) *Shader {
	return &Shader{name: name, src: []byte(header + common + "\n" + body)}
}

// Draw 以 shader 填滿 dst。每個 shader 都會自動收到 Resolution（vec2，像素）。
func (s *Shader) Draw(dst *ebiten.Image, uniforms map[string]any) {
	s.once.Do(func() {
		sh, err := ebiten.NewShader(s.src)
		if err != nil {
			// shader 是內嵌的程式碼，編譯失敗代表開發錯誤，直接中止。
			panic(fmt.Sprintf("shader %s 編譯失敗: %v", s.name, err))
		}
		s.shader = sh
	})
	b := dst.Bounds()
	u := map[string]any{"Resolution": []float32{float32(b.Dx()), float32(b.Dy())}}
	for k, v := range uniforms {
		u[k] = v
	}
	dst.DrawRectShader(b.Dx(), b.Dy(), s.shader, &ebiten.DrawRectShaderOptions{Uniforms: u})
}

// FrameSeed 把影格編號縮到 float32 能精確表示的範圍，避免 hash 在長時間播放後失真。
// 週期 9973 格（60fps 約 166 秒），肉眼看不出重複。
func FrameSeed(frame int) float32 {
	return float32(frame % 9973)
}
