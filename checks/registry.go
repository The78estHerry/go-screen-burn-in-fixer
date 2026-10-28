// Package checks 定義檢查畫面：用來找出烙印、暫時性殘影、壞點與不均勻。
// 每個檢查項目包含一或多「階」（例如灰階 5%、10%…），依序顯示。
// 擴充方式與 patterns 相同：在 init() 呼叫 Register。
package checks

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
)

// Check 是一個檢查項目。
type Check interface {
	Name() string        // 參數與檔名使用的名稱，小寫加連字號
	Description() string // 選單使用的一句說明
	Hint() string        // 檢查時顯示的說明：這個畫面要看什麼
	Steps() []string     // 每一階的名稱，例如「灰階 25%」
	// Draw 繪製第 step 階；t 是這一階已經顯示的秒數（殘影測試用來切換畫面）。
	Draw(dst *ebiten.Image, step int, t float64)
}

// Statuser 是可選的介面：檢查畫面上不放文字，需要顯示的狀態（例如倒數）
// 由 Status 提供，顯示在可隱藏的資訊列。
type Statuser interface {
	Status(step int, t float64) string
}

var (
	registry = map[string]Check{}
	order    []string
)

// Register 註冊檢查項目；名稱重複代表開發錯誤，直接中止。
func Register(c Check) {
	if _, dup := registry[c.Name()]; dup {
		panic(fmt.Sprintf("檢查項目名稱重複: %s", c.Name()))
	}
	registry[c.Name()] = c
	order = append(order, c.Name())
}

// Get 依名稱取得檢查項目。
func Get(name string) (Check, bool) {
	c, ok := registry[name]
	return c, ok
}

// All 依註冊順序回傳所有檢查項目。
func All() []Check {
	cs := make([]Check, len(order))
	for i, n := range order {
		cs[i] = registry[n]
	}
	return cs
}
