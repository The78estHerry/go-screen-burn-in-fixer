package actions

import (
	"os/exec"
	"time"
)

// keepAwakeCmd：systemd-inhibit 阻止系統閒置睡眠；桌面環境的螢幕保護 / 關閉螢幕另外每 30 秒用
// xdg-screensaver reset 重設閒置時間（沒有安裝時略過）。
func keepAwakeCmd() (*exec.Cmd, func()) {
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			exec.Command("xdg-screensaver", "reset").Run()
			select {
			case <-stop:
				return
			case <-t.C:
			}
		}
	}()
	end := func() { close(stop) }
	if _, err := exec.LookPath("systemd-inhibit"); err != nil {
		return exec.Command("sleep", "infinity"), end // 沒有 systemd：只靠 xdg-screensaver
	}
	return exec.Command("systemd-inhibit", "--what=idle:sleep", "--who=go-screen-burn-in-fixer",
		"--why=Playing screen repair patterns", "--mode=block", "sleep", "infinity"), end
}

func shutdown() error { return run("systemctl", "poweroff") }

func sleep() error { return run("systemctl", "suspend") }
