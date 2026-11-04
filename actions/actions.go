// Package actions 定義播放時間到之後的動作，以及電源控制（防止睡眠、關機、睡眠）。
package actions

import (
	"fmt"

	"github.com/The78estHerry/go-screen-burn-in-fixer/internal/i18n"
)

// ErrUnsupported 代表目前平台不支援這個電源操作。
var ErrUnsupported error = i18n.Error{ZH: "目前平台不支援這個電源操作", EN: "this power action is not supported on this platform"}

// After 是播放時間到之後的動作。
type After string

const (
	Quit        After = "quit"
	Shutdown    After = "shutdown"
	Sleep       After = "sleep"
	Screensaver After = "screensaver"
)

// All 依選單顯示順序列出所有動作。
var All = []After{Quit, Shutdown, Sleep, Screensaver}

// Label 回傳選單使用的名稱。
func (a After) Label() string {
	switch a {
	case Quit:
		return i18n.T("關閉程式", "Quit")
	case Shutdown:
		return i18n.T("關機", "Shut down")
	case Sleep:
		return i18n.T("睡眠", "Sleep")
	case Screensaver:
		return i18n.T("螢幕保護", "Screensaver")
	}
	return string(a)
}

// Verb 回傳放在句子中間的動詞（「無法關機」、"could not shut down"）。
func (a After) Verb() string {
	switch a {
	case Shutdown:
		return i18n.T("關機", "shut down")
	case Sleep:
		return i18n.T("睡眠", "sleep")
	}
	return a.Label()
}

// CountdownTitle 回傳倒數畫面的標題。
func (a After) CountdownTitle() string {
	switch a {
	case Shutdown:
		return i18n.T("即將關機", "Shutting down")
	case Sleep:
		return i18n.T("即將睡眠", "Going to sleep")
	}
	return a.Label()
}

// NeedsCountdown 回傳執行前是否要先倒數（給使用者取消的機會）。
func (a After) NeedsCountdown() bool {
	return a == Shutdown || a == Sleep
}

// ParseAfter 解析 -after 參數。
func ParseAfter(s string) (After, error) {
	for _, a := range All {
		if string(a) == s {
			return a, nil
		}
	}
	return "", fmt.Errorf(i18n.T("不支援的後續動作 %q（可用：quit、shutdown、sleep、screensaver）", "unsupported action %q (available: quit, shutdown, sleep, screensaver)"), s)
}

// Execute 執行關機或睡眠。其他動作由呼叫端處理，這裡直接回傳 nil。
func Execute(a After) error {
	switch a {
	case Shutdown:
		return shutdown()
	case Sleep:
		KeepAwake(false) // 解除防睡眠，否則系統可能拒絕進入睡眠
		return sleep()
	}
	return nil
}
