// Package screensaver 定義螢幕保護畫面：暗、慢、不閃爍，也不會長時間停在同一個位置。
// 擴充方式與 patterns 相同：在 init() 呼叫 Register。
package screensaver

import (
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// MaxBrightness 是螢幕保護畫面的亮度上限（0–1）。
const MaxBrightness = 0.3

// State 是繪製一格螢幕保護畫面所需的狀態。
type State struct {
	Time float64   // 秒（決定移動與變色）
	Now  time.Time // 時鐘顯示的時間
}

// Saver 是一種螢幕保護畫面。
type Saver interface {
	Name() string
	Description() string
	Draw(dst *ebiten.Image, st State)
}

var (
	registry = map[string]Saver{}
	order    []string
)

// Register 註冊螢幕保護畫面；名稱重複代表開發錯誤，直接中止。
func Register(s Saver) {
	if _, dup := registry[s.Name()]; dup {
		panic(fmt.Sprintf("螢幕保護畫面名稱重複: %s", s.Name()))
	}
	registry[s.Name()] = s
	order = append(order, s.Name())
}

// Get 依名稱取得螢幕保護畫面。
func Get(name string) (Saver, bool) {
	s, ok := registry[name]
	return s, ok
}

// All 依註冊順序回傳所有螢幕保護畫面。
func All() []Saver {
	ss := make([]Saver, len(order))
	for i, n := range order {
		ss[i] = registry[n]
	}
	return ss
}
