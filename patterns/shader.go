package patterns

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/The78estHerry/go-screen-burn-in-fixer/internal/gfx"
	"github.com/The78estHerry/go-screen-burn-in-fixer/internal/i18n"
)

// shaderPattern 是以 Kage shader 繪製的圖樣。每個 shader 都會收到
// Resolution、Frame（已取模的影格編號）、Time，再加上 uniforms 回傳的參數。
type shaderPattern struct {
	name     string
	desc     i18n.Text
	shader   *gfx.Shader
	uniforms func() map[string]any
}

func newShaderPattern(name string, desc i18n.Text, body string, uniforms func() map[string]any) *shaderPattern {
	return &shaderPattern{name: name, desc: desc, shader: gfx.NewShader(name, body), uniforms: uniforms}
}

func (p *shaderPattern) Name() string        { return p.name }
func (p *shaderPattern) Description() string { return p.desc.String() }

func (p *shaderPattern) Draw(dst *ebiten.Image, st State) {
	u := map[string]any{
		"Frame": gfx.FrameSeed(st.Frame),
		"Time":  float32(st.Time),
	}
	if p.uniforms != nil {
		for k, v := range p.uniforms() {
			u[k] = v
		}
	}
	p.shader.Draw(dst, u)
}
