//go:build !windows

package actions

// KeepAwake 在其他平台尚未實作。
func KeepAwake(on bool) {}

func shutdown() error { return ErrUnsupported }

func sleep() error { return ErrUnsupported }
