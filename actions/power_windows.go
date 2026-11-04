//go:build windows

package actions

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/The78estHerry/go-screen-burn-in-fixer/internal/i18n"
)

var (
	kernel32                    = windows.NewLazySystemDLL("kernel32.dll")
	procSetThreadExecutionState = kernel32.NewProc("SetThreadExecutionState")
	powrprof                    = windows.NewLazySystemDLL("powrprof.dll")
	procSetSuspendState         = powrprof.NewProc("SetSuspendState")
)

const (
	esSystemRequired  = 0x00000001
	esDisplayRequired = 0x00000002
	esContinuous      = 0x80000000

	ewxShutdown = 0x00000001
	ewxPowerOff = 0x00000008

	shtdnReasonMajorOther  = 0x00000000
	shtdnReasonFlagPlanned = 0x80000000
)

// SetThreadExecutionState 的設定綁定在呼叫的執行緒上，而 goroutine 會在執行緒之間移動，
// 所以由一個固定在同一個執行緒上的 goroutine 負責設定。
var (
	awakeOnce sync.Once
	awakeCh   chan bool
)

// KeepAwake 設定是否防止螢幕關閉與系統睡眠。
func KeepAwake(on bool) {
	awakeOnce.Do(func() {
		awakeCh = make(chan bool)
		go func() {
			runtime.LockOSThread()
			for on := range awakeCh {
				flags := uintptr(esContinuous)
				if on {
					flags |= esSystemRequired | esDisplayRequired
				}
				procSetThreadExecutionState.Call(flags)
			}
		}()
	})
	awakeCh <- on
}

func shutdown() error {
	if err := enableShutdownPrivilege(); err != nil {
		return err
	}
	if err := windows.ExitWindowsEx(ewxShutdown|ewxPowerOff, shtdnReasonMajorOther|shtdnReasonFlagPlanned); err != nil {
		return fmt.Errorf(i18n.T("關機失敗: %w", "shutdown failed: %w"), err)
	}
	return nil
}

func enableShutdownPrivilege() error {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); err != nil {
		return fmt.Errorf(i18n.T("無法開啟程序權杖: %w", "cannot open process token: %w"), err)
	}
	defer token.Close()
	var id windows.LUID
	name, _ := windows.UTF16PtrFromString("SeShutdownPrivilege")
	if err := windows.LookupPrivilegeValue(nil, name, &id); err != nil {
		return fmt.Errorf(i18n.T("無法查詢關機權限: %w", "cannot look up shutdown privilege: %w"), err)
	}
	tp := windows.Tokenprivileges{PrivilegeCount: 1}
	tp.Privileges[0] = windows.LUIDAndAttributes{Luid: id, Attributes: windows.SE_PRIVILEGE_ENABLED}
	if err := windows.AdjustTokenPrivileges(token, false, &tp, uint32(unsafe.Sizeof(tp)), nil, nil); err != nil {
		return fmt.Errorf(i18n.T("無法取得關機權限: %w", "cannot get shutdown privilege: %w"), err)
	}
	return nil
}

func sleep() error {
	// SetSuspendState(bHibernate=FALSE, bForce=FALSE, bWakeupEventsDisabled=FALSE)
	if r, _, err := procSetSuspendState.Call(0, 0, 0); r == 0 {
		return fmt.Errorf(i18n.T("進入睡眠失敗: %w", "sleep failed: %w"), err)
	}
	return nil
}
