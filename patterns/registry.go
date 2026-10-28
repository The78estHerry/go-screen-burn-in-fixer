// Package patterns 定義 Burn-In 圖樣。每個圖樣在 init() 呼叫 Register 註冊，
// 新增圖樣只要新增檔案，不需要修改其他程式碼。
package patterns

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
)

// State 是繪製一格畫面所需的狀態。畫面只由像素座標、State 與圖樣參數決定，
// 不依賴實際 FPS，所以預覽影片與實際播放完全一致。
type State struct {
	Frame int     // 從 0 開始的影格編號（決定隨機雜訊）
	Time  float64 // 秒（決定移動與輪播）
}

// Pattern 是一種 Burn-In 圖樣。
type Pattern interface {
	Name() string        // 參數與檔名使用的名稱，小寫加連字號
	Description() string // 選單與文件使用的一句說明
	Draw(dst *ebiten.Image, st State)
}

var (
	registry = map[string]Pattern{}
	order    []string
)

// Register 註冊圖樣；名稱重複代表開發錯誤，直接中止。
func Register(p Pattern) {
	if _, dup := registry[p.Name()]; dup {
		panic(fmt.Sprintf("圖樣名稱重複: %s", p.Name()))
	}
	registry[p.Name()] = p
	order = append(order, p.Name())
}

// Get 依名稱取得圖樣。
func Get(name string) (Pattern, bool) {
	p, ok := registry[name]
	return p, ok
}

// All 依註冊順序回傳所有圖樣。
func All() []Pattern {
	ps := make([]Pattern, len(order))
	for i, n := range order {
		ps[i] = registry[n]
	}
	return ps
}
