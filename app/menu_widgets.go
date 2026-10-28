package app

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/internal/ui"
)

// 選單的共用小元件。版面以 WindowWidth × WindowHeight 的邏輯座標描述，繪製時再依視窗大小縮放。

// 鍵盤長按時的重複間隔（影格）。
const (
	repeatDelay    = 24
	repeatInterval = 4
)

type clickArea struct {
	r  ui.Rect
	fn func()
}

// item 是選單中可取得焦點的一列。
type item struct {
	row      ui.Rect // 整列範圍：滑鼠指到時反白，點一下取得焦點
	left     func()  // ← 鍵
	right    func()  // → 鍵
	activate func()  // 空白鍵；按鈕點一下也會觸發
	clicks   []clickArea
	disabled bool
	preview  func(dst *ebiten.Image, frame int) // 焦點或滑鼠在這一列時，預覽框顯示的內容
	draw     func(p *painter, focused, hovered bool)
	isButton bool
}

// painter 把邏輯座標轉成畫面座標後繪製。
type painter struct {
	dst       *ebiten.Image
	s, ox, oy float64
}

func (p *painter) rect(r ui.Rect) ui.Rect {
	return ui.Rect{X: p.ox + r.X*p.s, Y: p.oy + r.Y*p.s, W: r.W * p.s, H: r.H * p.s}
}

func (p *painter) text(str string, x, y, size float64, clr color.Color, align ui.Align) {
	ui.Text(p.dst, str, p.ox+x*p.s, p.oy+y*p.s, size*p.s, clr, align)
}

func (p *painter) fill(r ui.Rect, clr color.Color)   { ui.Fill(p.dst, p.rect(r), clr) }
func (p *painter) stroke(r ui.Rect, clr color.Color) { ui.Stroke(p.dst, p.rect(r), 1.5*p.s, clr) }

func (p *painter) circle(cx, cy, r float64, clr color.Color, filled bool) {
	ui.Circle(p.dst, p.ox+cx*p.s, p.oy+cy*p.s, r*p.s, 1.5*p.s, clr, filled)
}

func textColor(disabled bool) color.Color {
	if disabled {
		return ui.ColorBorder
	}
	return ui.ColorText
}

func dimColor(disabled bool) color.Color {
	if disabled {
		return ui.ColorBorder
	}
	return ui.ColorDim
}

// selector：◀ 值 ▶
func selector(x, y, w float64, value func() string, left, right func(), disabled bool) ([]clickArea, func(p *painter)) {
	l := ui.Rect{X: x, Y: y - 4, W: 28, H: 28}
	r := ui.Rect{X: x + w - 28, Y: y - 4, W: 28, H: 28}
	draw := func(p *painter) {
		arrow := color.Color(ui.ColorAccent)
		if disabled {
			arrow = ui.ColorBorder
		}
		p.text("◀", l.X+14, y, 16, arrow, ui.Center)
		p.text(value(), x+w/2, y, 17, textColor(disabled), ui.Center)
		p.text("▶", r.X+14, y, 16, arrow, ui.Center)
	}
	return []clickArea{{l, left}, {r, right}}, draw
}

// radio：○ 標籤。回傳點擊範圍、佔用寬度與繪製函式。
func radio(x, y float64, label string, selected func() bool, fn func(), disabled bool) (clickArea, float64, func(p *painter)) {
	w, _ := ui.Measure(label, 17)
	area := ui.Rect{X: x - 4, Y: y - 4, W: w + 36, H: 28}
	draw := func(p *painter) {
		p.circle(x+8, y+11, 8, dimColor(disabled), false)
		if selected() {
			dot := color.Color(ui.ColorAccent)
			if disabled {
				dot = ui.ColorBorder
			}
			p.circle(x+8, y+11, 4.5, dot, true)
		}
		p.text(label, x+24, y, 17, textColor(disabled), ui.Left)
	}
	return clickArea{area, fn}, w + 52, draw
}

func checkbox(p *painter, x, y float64, checked, disabled bool) {
	box := ui.Rect{X: x, Y: y + 3, W: 18, H: 18}
	if checked && !disabled {
		p.fill(box, ui.ColorAccent)
		p.text("✓", x+9, y, 15, ui.ColorBackground, ui.Center)
		return
	}
	p.stroke(box, ui.ColorDim)
	if checked {
		p.text("✓", x+9, y, 15, ui.ColorDim, ui.Center)
	}
}

func button(r ui.Rect, label string, primary bool, fn func(), disabled bool) item {
	return item{
		row: r, activate: fn, disabled: disabled, isButton: true,
		draw: func(p *painter, focused, hovered bool) {
			bg := color.Color(ui.ColorPanel)
			if primary {
				bg = ui.ColorAccent
			}
			if hovered && !disabled {
				bg = ui.ColorHover
				if primary {
					bg = color.RGBA{0x6a, 0xb6, 0xff, 0xff}
				}
			}
			p.fill(r, bg)
			if focused {
				p.stroke(ui.Rect{X: r.X - 3, Y: r.Y - 3, W: r.W + 6, H: r.H + 6}, ui.ColorText)
			}
			c := color.Color(ui.ColorText)
			if primary {
				c = ui.ColorBackground
			}
			if disabled {
				c = ui.ColorBorder
			}
			p.text(label, r.X+r.W/2, r.Y+r.H/2-12, 17, c, ui.Center)
		},
	}
}

func keyRepeat(k ebiten.Key) bool {
	d := inpututil.KeyPressDuration(k)
	return d == 1 || (d > repeatDelay && (d-repeatDelay)%repeatInterval == 0)
}

func scaleFor(w, h int) (s, ox, oy float64) {
	fw, fh := float64(w), float64(h)
	s = min(fw/WindowWidth, fh/WindowHeight)
	return s, (fw - WindowWidth*s) / 2, (fh - WindowHeight*s) / 2
}

func formatSeconds(sec int) string {
	if sec >= 60 && sec%60 == 0 {
		return fmt.Sprintf("%d 分", sec/60)
	}
	return fmt.Sprintf("%d 秒", sec)
}

// shortDesc 縮短說明，讓清單列不會太長：括號內的「寬 N 像素」改成「N px」，其餘括號內容省略。
func shortDesc(s string) string {
	base, paren, _ := strings.Cut(s, "（")
	suffix := ""
	var n int
	if _, err := fmt.Sscanf(paren, "寬 %d 像素", &n); err == nil {
		suffix = fmt.Sprintf(" %d px", n)
	}
	r := []rune(base)
	if max := 12 - len([]rune(suffix)); len(r) > max {
		r = append(r[:max-1], '…')
	}
	return string(r) + suffix
}
