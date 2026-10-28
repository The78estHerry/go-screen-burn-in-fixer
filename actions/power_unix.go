//go:build darwin || linux

package actions

import (
	"log"
	"os/exec"
	"sync"
)

// macOS 與 Linux 用系統內建的指令防睡眠：KeepAwake(true) 時啟動一個常駐的子行程，
// 它存在期間系統不會進入閒置睡眠；KeepAwake(false) 或程式結束時結束它。

var (
	awakeMu  sync.Mutex
	awakeCmd *exec.Cmd
	awakeEnd func()
)

// KeepAwake 設定是否防止螢幕關閉與系統睡眠。
func KeepAwake(on bool) {
	awakeMu.Lock()
	defer awakeMu.Unlock()
	if on == (awakeCmd != nil) {
		return
	}
	if !on {
		awakeCmd.Process.Kill()
		awakeCmd.Wait()
		awakeCmd = nil
		if awakeEnd != nil {
			awakeEnd()
			awakeEnd = nil
		}
		return
	}
	cmd, end := keepAwakeCmd()
	if cmd == nil {
		return
	}
	if err := cmd.Start(); err != nil {
		log.Printf("無法防止螢幕關閉: %v", err)
		return
	}
	awakeCmd, awakeEnd = cmd, end
}

// run 執行指令，失敗時把輸出附在錯誤訊息裡。
func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil && len(out) > 0 {
		return &cmdError{err: err, out: string(out)}
	}
	return err
}

type cmdError struct {
	err error
	out string
}

func (e *cmdError) Error() string { return e.err.Error() + "：" + e.out }
