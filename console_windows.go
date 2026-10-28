//go:build windows

package main

import (
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole = kernel32.NewProc("AttachConsole")
	user32            = windows.NewLazySystemDLL("user32.dll")
	procMessageBoxW   = user32.NewProc("MessageBoxW")

	consoleOnce sync.Once
	hasConsole  bool
)

const attachParentProcess = ^uintptr(0) // ATTACH_PARENT_PROCESS = (DWORD)-1

// attachConsole 讓以 -H windowsgui 建置的程式在命令提示字元中執行時，也能輸出文字。
// 回傳是否有可用的主控台。
func attachConsole() bool {
	consoleOnce.Do(func() {
		// 輸出已經被重新導向（管線或檔案），或本來就有主控台時，直接使用原本的輸出。
		if isValidHandle(os.Stdout) {
			hasConsole = true
			return
		}
		if r, _, _ := procAttachConsole.Call(attachParentProcess); r == 0 {
			return
		}
		if out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
			os.Stdout, os.Stderr = out, out
			hasConsole = true
		}
	})
	return hasConsole
}

func isValidHandle(f *os.File) bool {
	if f == nil || f.Fd() == 0 || f.Fd() == uintptr(windows.InvalidHandle) {
		return false
	}
	_, err := f.Stat()
	return err == nil
}

func messageBox(title, msg string) {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(msg)
	const mbIconError = 0x10
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), mbIconError)
}
