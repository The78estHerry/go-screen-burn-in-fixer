package display

import (
	"fmt"

	"github.com/The78estHerry/go-screen-burn-in-fixer/internal/i18n"
)

// ErrUnsupported 代表目前平台不支援讀取顯示器資訊。
var ErrUnsupported error = i18n.Error{ZH: "目前平台不支援讀取顯示器資訊", EN: "reading display information is not supported on this platform"}

// Display 是作業系統回報的一個使用中顯示器。
type Display struct {
	DeviceName   string    // 作業系統的裝置名稱，例如 \\.\DISPLAY1
	FriendlyName string    // 顯示器名稱，例如 "DELL U2720Q"（可能是空字串）
	Primary      bool      // 是否為主螢幕
	Current      Mode      // 目前的解析度與更新率
	EDID         *EDIDInfo // EDID 資訊；讀不到時為 nil
}

// Label 回傳選單與 -list-monitors 使用的說明文字。
func (d Display) Label() string {
	s := d.Current.String()
	if d.FriendlyName != "" {
		s = d.FriendlyName + i18n.T("　", " ") + s
	}
	if d.EDID != nil {
		s += fmt.Sprintf(i18n.T("（EDID 原生 %d×%d）", " (EDID native %d×%d)"), d.EDID.Native.Width, d.EDID.Native.Height)
	}
	if d.Primary {
		s += i18n.T("［主螢幕］", " [primary]")
	}
	return s
}

// Displays 回傳使用中的顯示器，主螢幕排在第一個。
func Displays() ([]Display, error) {
	ds, err := platformDisplays()
	if err != nil {
		return nil, err
	}
	for i := range ds {
		if ds[i].Primary && i != 0 {
			ds[0], ds[i] = ds[i], ds[0]
			break
		}
	}
	return ds, nil
}
