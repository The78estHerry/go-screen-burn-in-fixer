//go:build windows

package display

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// 透過 QueryDisplayConfig 取得使用中的顯示器；顯示器的裝置路徑可以對應到登錄檔裡的 EDID，
// 不會讀到曾經接過、但目前沒有使用的舊顯示器。

var (
	user32                          = windows.NewLazySystemDLL("user32.dll")
	procGetDisplayConfigBufferSizes = user32.NewProc("GetDisplayConfigBufferSizes")
	procQueryDisplayConfig          = user32.NewProc("QueryDisplayConfig")
	procDisplayConfigGetDeviceInfo  = user32.NewProc("DisplayConfigGetDeviceInfo")
)

const (
	qdcOnlyActivePaths      = 0x2
	modeInfoTypeSource      = 1
	deviceInfoGetSourceName = 1
	deviceInfoGetTargetName = 2
	invalidModeIdx          = 0xffffffff
)

type luid struct {
	Low  uint32
	High int32
}

type rational struct{ Num, Den uint32 }

// DISPLAYCONFIG_PATH_INFO（72 位元組）
type pathInfo struct {
	SourceAdapterID   luid
	SourceID          uint32
	SourceModeInfoIdx uint32
	SourceStatusFlags uint32

	TargetAdapterID        luid
	TargetID               uint32
	TargetModeInfoIdx      uint32
	TargetOutputTechnology uint32
	TargetRotation         uint32
	TargetScaling          uint32
	TargetRefreshRate      rational
	TargetScanLineOrdering uint32
	TargetAvailable        int32
	TargetStatusFlags      uint32

	Flags uint32
}

// DISPLAYCONFIG_MODE_INFO（64 位元組）；來源模式時 Union 開頭是 width、height、pixelFormat、position。
type modeInfo struct {
	InfoType  uint32
	ID        uint32
	AdapterID luid
	Union     [48]byte
}

type deviceInfoHeader struct {
	Type      uint32
	Size      uint32
	AdapterID luid
	ID        uint32
}

type sourceDeviceName struct {
	Header            deviceInfoHeader
	ViewGdiDeviceName [32]uint16
}

type targetDeviceName struct {
	Header                    deviceInfoHeader
	Flags                     uint32
	OutputTechnology          uint32
	EdidManufactureID         uint16
	EdidProductCodeID         uint16
	ConnectorInstance         uint32
	MonitorFriendlyDeviceName [64]uint16
	MonitorDevicePath         [128]uint16
}

func platformDisplays() ([]Display, error) {
	paths, modes, err := queryDisplayConfig()
	if err != nil {
		return nil, err
	}
	var ds []Display
	for _, p := range paths {
		d := Display{}
		if p.TargetRefreshRate.Den != 0 {
			d.Current.RefreshHz = float64(p.TargetRefreshRate.Num) / float64(p.TargetRefreshRate.Den)
		}
		if idx := p.SourceModeInfoIdx; idx != invalidModeIdx && int(idx) < len(modes) && modes[idx].InfoType == modeInfoTypeSource {
			u := modes[idx].Union[:]
			d.Current.Width = int(le32(u[0:]))
			d.Current.Height = int(le32(u[4:]))
			x, y := int32(le32(u[12:])), int32(le32(u[16:]))
			d.Primary = x == 0 && y == 0
		}

		src := sourceDeviceName{Header: deviceInfoHeader{
			Type: deviceInfoGetSourceName, Size: uint32(unsafe.Sizeof(sourceDeviceName{})),
			AdapterID: p.SourceAdapterID, ID: p.SourceID,
		}}
		if getDeviceInfo(unsafe.Pointer(&src)) == nil {
			d.DeviceName = windows.UTF16ToString(src.ViewGdiDeviceName[:])
		}

		tgt := targetDeviceName{Header: deviceInfoHeader{
			Type: deviceInfoGetTargetName, Size: uint32(unsafe.Sizeof(targetDeviceName{})),
			AdapterID: p.TargetAdapterID, ID: p.TargetID,
		}}
		if getDeviceInfo(unsafe.Pointer(&tgt)) == nil {
			d.FriendlyName = windows.UTF16ToString(tgt.MonitorFriendlyDeviceName[:])
			if b, err := readEDID(windows.UTF16ToString(tgt.MonitorDevicePath[:])); err == nil {
				if info, err := ParseEDID(b); err == nil {
					d.EDID = &info
					if d.FriendlyName == "" {
						d.FriendlyName = info.Name
					}
				}
			}
		}
		ds = append(ds, d)
	}
	return ds, nil
}

func queryDisplayConfig() ([]pathInfo, []modeInfo, error) {
	for range 3 { // 查詢期間顯示器設定可能改變，緩衝區不夠時重試
		var nPath, nMode uint32
		if r, _, _ := procGetDisplayConfigBufferSizes.Call(qdcOnlyActivePaths,
			uintptr(unsafe.Pointer(&nPath)), uintptr(unsafe.Pointer(&nMode))); r != 0 {
			return nil, nil, fmt.Errorf("GetDisplayConfigBufferSizes 失敗: %w", windows.Errno(r))
		}
		if nPath == 0 {
			return nil, nil, nil
		}
		paths := make([]pathInfo, nPath)
		modes := make([]modeInfo, nMode)
		r, _, _ := procQueryDisplayConfig.Call(qdcOnlyActivePaths,
			uintptr(unsafe.Pointer(&nPath)), uintptr(unsafe.Pointer(&paths[0])),
			uintptr(unsafe.Pointer(&nMode)), uintptr(unsafe.Pointer(&modes[0])), 0)
		if windows.Errno(r) == windows.ERROR_INSUFFICIENT_BUFFER {
			continue
		}
		if r != 0 {
			return nil, nil, fmt.Errorf("QueryDisplayConfig 失敗: %w", windows.Errno(r))
		}
		return paths[:nPath], modes[:nMode], nil
	}
	return nil, nil, fmt.Errorf("QueryDisplayConfig 失敗: %w", windows.ERROR_INSUFFICIENT_BUFFER)
}

func getDeviceInfo(p unsafe.Pointer) error {
	if r, _, _ := procDisplayConfigGetDeviceInfo.Call(uintptr(p)); r != 0 {
		return windows.Errno(r)
	}
	return nil
}

// readEDID 由顯示器裝置路徑（\\?\DISPLAY#<型號>#<執行個體>#{GUID}）
// 讀取 HKLM\SYSTEM\CurrentControlSet\Enum\DISPLAY\<型號>\<執行個體>\Device Parameters\EDID。
func readEDID(devicePath string) ([]byte, error) {
	parts := strings.Split(strings.TrimPrefix(devicePath, `\\?\`), "#")
	if len(parts) < 3 {
		return nil, fmt.Errorf("無法解析顯示器裝置路徑: %q", devicePath)
	}
	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Enum\DISPLAY\`+parts[1]+`\`+parts[2]+`\Device Parameters`, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer key.Close()
	b, _, err := key.GetBinaryValue("EDID")
	return b, err
}

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}
