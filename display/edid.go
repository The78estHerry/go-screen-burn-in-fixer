// Package display 提供螢幕列表與 EDID 原生解析度。
package display

import (
	"errors"
	"fmt"
)

// Mode 是一種顯示模式。
type Mode struct {
	Width, Height int
	RefreshHz     float64
}

func (m Mode) String() string {
	return fmt.Sprintf("%d×%d @ %.0fHz", m.Width, m.Height, m.RefreshHz)
}

// EDIDInfo 是從 EDID 解析出的資訊。
type EDIDInfo struct {
	Manufacturer string // 三個字母的廠商代碼，例如 "DEL"
	Name         string // 顯示器名稱描述（可能是空字串）
	Native       Mode   // 第一個 Detailed Timing Descriptor，即原生（偏好）模式
}

var edidHeader = []byte{0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x00}

// ParseEDID 解析 EDID 基本區塊（前 128 位元組）。
func ParseEDID(b []byte) (EDIDInfo, error) {
	var info EDIDInfo
	if len(b) < 128 {
		return info, errors.New("EDID 長度不足 128 位元組")
	}
	for i, v := range edidHeader {
		if b[i] != v {
			return info, errors.New("EDID 標頭錯誤")
		}
	}
	var sum byte
	for _, v := range b[:128] {
		sum += v
	}
	if sum != 0 {
		return info, errors.New("EDID 檢查碼錯誤")
	}

	// 位元組 8–9：廠商代碼，三個 5 位元字母（1 = 'A'）。
	id := uint16(b[8])<<8 | uint16(b[9])
	info.Manufacturer = string([]byte{
		byte('A' - 1 + (id>>10)&0x1f),
		byte('A' - 1 + (id>>5)&0x1f),
		byte('A' - 1 + id&0x1f),
	})

	// 位元組 54–125：四個 18 位元組的描述區塊。
	found := false
	for i := 0; i < 4; i++ {
		d := b[54+18*i : 54+18*(i+1)]
		pixelClock := int(d[0]) | int(d[1])<<8 // 單位 10 kHz
		if pixelClock != 0 {
			if found {
				continue
			}
			hActive := int(d[2]) | int(d[4]&0xf0)<<4
			hBlank := int(d[3]) | int(d[4]&0x0f)<<8
			vActive := int(d[5]) | int(d[7]&0xf0)<<4
			vBlank := int(d[6]) | int(d[7]&0x0f)<<8
			info.Native = Mode{Width: hActive, Height: vActive}
			if total := (hActive + hBlank) * (vActive + vBlank); total > 0 {
				info.Native.RefreshHz = float64(pixelClock) * 10000 / float64(total)
			}
			found = true
			continue
		}
		// 顯示器描述區塊：0xFC 是顯示器名稱。
		if d[3] == 0xfc {
			info.Name = descriptorText(d[5:18])
		}
	}
	if !found {
		return info, errors.New("EDID 沒有 Detailed Timing Descriptor")
	}
	return info, nil
}

// descriptorText 取出描述區塊的文字（以 0x0A 結尾，空白補齊）。
func descriptorText(b []byte) string {
	n := len(b)
	for i, c := range b {
		if c == 0x0a {
			n = i
			break
		}
	}
	for n > 0 && b[n-1] == ' ' {
		n--
	}
	return string(b[:n])
}
