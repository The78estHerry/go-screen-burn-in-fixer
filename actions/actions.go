// Package actions 定義播放時間到之後的動作，以及電源控制（防止睡眠、關機、睡眠）。
package actions

import (
	"errors"
	"fmt"
)

// ErrUnsupported 代表目前平台不支援這個電源操作。
var ErrUnsupported = errors.New("目前平台不支援這個電源操作")

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

// Label 回傳選單使用的中文名稱。
func (a After) Label() string {
	switch a {
	case Quit:
		return "關閉程式"
	case Shutdown:
		return "關機"
	case Sleep:
		return "睡眠"
	case Screensaver:
		return "螢幕保護"
	}
	return string(a)
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
	return "", fmt.Errorf("不支援的後續動作 %q（可用：quit、shutdown、sleep、screensaver）", s)
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
