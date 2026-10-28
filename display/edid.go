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
	Native       Mode   // 原生解析度：所有詳細時序中像素最多的一組（同解析度取最高更新率）
}

var edidHeader = []byte{0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x00}

// ParseEDID 解析 EDID 基本區塊（前 128 位元組）與其後的擴充區塊。
//
// 原生解析度不一定在基本區塊：超寬或高解析度螢幕（例如 5120×1440）常為了相容性，
// 在基本區塊放較小的模式，真正的原生模式放在 CTA-861 或 DisplayID 擴充區塊。
// 因此收集所有區塊中的詳細時序，取像素最多的一組。
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
	if !checksumOK(b[:128]) {
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
	var modes []Mode
	for i := 0; i < 4; i++ {
		d := b[54+18*i : 54+18*(i+1)]
		if m, ok := parseDTD(d); ok {
			modes = append(modes, m)
			continue
		}
		// 顯示器描述區塊：0xFC 是顯示器名稱。
		if d[3] == 0xfc {
			info.Name = descriptorText(d[5:18])
		}
	}

	// 擴充區塊：位元組 126 是數量。讀不到或檢查碼錯誤的區塊略過。
	for i := 1; i <= int(b[126]) && len(b) >= 128*(i+1); i++ {
		ext := b[128*i : 128*(i+1)]
		if !checksumOK(ext) {
			continue
		}
		switch ext[0] {
		case 0x02: // CTA-861
			modes = append(modes, ctaModes(ext)...)
		case 0x70: // DisplayID
			modes = append(modes, displayIDModes(ext[1:])...)
		}
	}

	if len(modes) == 0 {
		return info, errors.New("EDID 沒有詳細時序")
	}
	info.Native = modes[0]
	for _, m := range modes[1:] {
		a, best := m.Width*m.Height, info.Native.Width*info.Native.Height
		if a > best || a == best && m.RefreshHz > info.Native.RefreshHz {
			info.Native = m
		}
	}
	return info, nil
}

func checksumOK(b []byte) bool {
	var sum byte
	for _, v := range b {
		sum += v
	}
	return sum == 0
}

// parseDTD 解析 18 位元組的 Detailed Timing Descriptor；像素時脈為 0 代表不是時序（是顯示器描述）。
func parseDTD(d []byte) (Mode, bool) {
	pixelClock := int(d[0]) | int(d[1])<<8 // 單位 10 kHz
	if pixelClock == 0 {
		return Mode{}, false
	}
	hActive := int(d[2]) | int(d[4]&0xf0)<<4
	hBlank := int(d[3]) | int(d[4]&0x0f)<<8
	vActive := int(d[5]) | int(d[7]&0xf0)<<4
	vBlank := int(d[6]) | int(d[7]&0x0f)<<8
	return newMode(hActive, vActive, hBlank, vBlank, float64(pixelClock)*10000), hActive > 0 && vActive > 0
}

func newMode(hActive, vActive, hBlank, vBlank int, clockHz float64) Mode {
	m := Mode{Width: hActive, Height: vActive}
	if total := (hActive + hBlank) * (vActive + vBlank); total > 0 {
		m.RefreshHz = clockHz / float64(total)
	}
	return m
}

// ctaModes 回傳 CTA-861 擴充區塊中的詳細時序（位元組 2 是詳細時序開始的位置）。
func ctaModes(ext []byte) []Mode {
	var modes []Mode
	start := int(ext[2])
	if start < 4 {
		return nil
	}
	for off := start; off+18 <= 127; off += 18 {
		m, ok := parseDTD(ext[off : off+18])
		if !ok {
			break
		}
		modes = append(modes, m)
	}
	return modes
}

// displayIDModes 回傳 DisplayID 區段中的詳細時序（Type I：1.x 版 tag 0x03；Type VII：2.x 版 tag 0x22）。
// sec 從 DisplayID 區段標頭開始：版本、資料長度、產品類型、擴充數量，之後是資料區塊。
func displayIDModes(sec []byte) []Mode {
	if len(sec) < 4 {
		return nil
	}
	end := min(4+int(sec[1]), len(sec))
	var modes []Mode
	for off := 4; off+3 <= end; {
		tag, n := sec[off], int(sec[off+2])
		payload := sec[off+3 : min(off+3+n, end)]
		off += 3 + n
		if tag != 0x03 && tag != 0x22 {
			continue
		}
		for t := 0; t+20 <= len(payload); t += 20 {
			d := payload[t : t+20]
			clock := float64(int(d[0])|int(d[1])<<8|int(d[2])<<16) + 1
			if tag == 0x03 {
				clock *= 10000 // Type I：10 kHz
			} else {
				clock *= 1000 // Type VII：1 kHz
			}
			u16 := func(i int) int { return int(d[i]) | int(d[i+1])<<8 }
			modes = append(modes, newMode(u16(4)+1, u16(12)+1, u16(6)+1, u16(14)+1, clock))
		}
	}
	return modes
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
