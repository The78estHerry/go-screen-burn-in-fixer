// Package i18n 決定介面語言。畫面上的文字一律寫成 T("中文", "English")，
// 在繪製（或輸出）的當下才依目前語言選擇，所以在選單切換語言會立刻生效。
//
// 只翻譯使用者看得到的文字；log 與 panic 是給開發者看的，維持中文。
package i18n

import "strings"

// Lang 是介面語言。
type Lang string

const (
	EN Lang = "en" // 英文（預設）
	ZH Lang = "zh" // 繁體中文
)

// Default 是沒有設定時的語言（使用者指定：預設英文）。
const Default = EN

var cur = Default

// Set 設定目前的語言；不認得的值改用預設。
func Set(l Lang) {
	if l != EN && l != ZH {
		l = Default
	}
	cur = l
}

// Current 回傳目前的語言。
func Current() Lang { return cur }

// Parse 解析設定檔或命令列的語言名稱：en、zh（也接受 zh-TW、zh_Hant 等寫法）。
func Parse(s string) (Lang, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case s == "en" || strings.HasPrefix(s, "en-") || strings.HasPrefix(s, "en_"):
		return EN, true
	case s == "zh" || strings.HasPrefix(s, "zh-") || strings.HasPrefix(s, "zh_"):
		return ZH, true
	}
	return Default, false
}

// T 依目前語言回傳其中一個字串。格式字串的參數順序兩種語言要一致（不一致時用 %[2]d 這種寫法）。
func T(zh, en string) string {
	if cur == ZH {
		return zh
	}
	return en
}

// Text 是一段兩種語言的文字，給 init() 時就建立的資料（圖樣說明、檢查項目）使用。
type Text struct{ ZH, EN string }

// String 依目前語言回傳文字。
func (t Text) String() string { return T(t.ZH, t.EN) }

// Join 把多段文字依目前語言串起來。
func Join(parts ...Text) Text {
	var t Text
	for _, p := range parts {
		t.ZH += p.ZH
		t.EN += p.EN
	}
	return t
}

// Error 是兩種語言的錯誤訊息，給套件層級的錯誤值（errors.Is 比對用）使用：
// 訊息在 Error() 被呼叫時才依目前語言決定。
type Error Text

func (e Error) Error() string { return Text(e).String() }
