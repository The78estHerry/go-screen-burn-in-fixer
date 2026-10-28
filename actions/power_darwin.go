package actions

import (
	"os"
	"os/exec"
	"strconv"
)

// keepAwakeCmd：caffeinate -d（螢幕不關閉）-i（系統不閒置睡眠）-w（本程式結束時自動結束）。
func keepAwakeCmd() (*exec.Cmd, func()) {
	return exec.Command("caffeinate", "-d", "-i", "-w", strconv.Itoa(os.Getpid())), nil
}

// shutdown 透過「系統事件」關機（第一次執行時 macOS 會詢問是否允許控制「系統事件」）。
func shutdown() error {
	return run("osascript", "-e", `tell application "System Events" to shut down`)
}

func sleep() error { return run("pmset", "sleepnow") }
