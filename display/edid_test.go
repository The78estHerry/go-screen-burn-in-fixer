package display

import "testing"

// sampleEDID 產生一份最小的 EDID：廠商 "ABC"、名稱 "TEST MONITOR"、
// 原生模式為 CEA-861 的 1920x1080@60（像素時脈 148.5 MHz，總畫面 2200x1125）。
func sampleEDID() []byte {
	b := make([]byte, 128)
	copy(b, edidHeader)
	id := uint16(1)<<10 | uint16(2)<<5 | uint16(3) // A=1, B=2, C=3
	b[8], b[9] = byte(id>>8), byte(id)

	d := b[54:72]
	d[0], d[1] = 0x02, 0x3a             // 14850 × 10 kHz
	d[2], d[3], d[4] = 0x80, 0x18, 0x71 // hActive 0x780, hBlank 0x118
	d[5], d[6], d[7] = 0x38, 0x2d, 0x40 // vActive 0x438, vBlank 0x02d

	n := b[72:90]
	n[3] = 0xfc
	copy(n[5:], "TEST MONITOR\n")

	var sum byte
	for _, v := range b[:127] {
		sum += v
	}
	b[127] = -sum
	return b
}

func TestParseEDID(t *testing.T) {
	info, err := ParseEDID(sampleEDID())
	if err != nil {
		t.Fatal(err)
	}
	if info.Manufacturer != "ABC" {
		t.Errorf("Manufacturer = %q, want ABC", info.Manufacturer)
	}
	if info.Name != "TEST MONITOR" {
		t.Errorf("Name = %q, want TEST MONITOR", info.Name)
	}
	if info.Native.Width != 1920 || info.Native.Height != 1080 {
		t.Errorf("Native = %dx%d, want 1920x1080", info.Native.Width, info.Native.Height)
	}
	if hz := info.Native.RefreshHz; hz < 59.9 || hz > 60.1 {
		t.Errorf("RefreshHz = %v, want 60", hz)
	}
}

func TestParseEDIDErrors(t *testing.T) {
	short := sampleEDID()[:100]
	badHeader := sampleEDID()
	badHeader[0] = 1
	badSum := sampleEDID()
	badSum[127]++
	for name, b := range map[string][]byte{"short": short, "header": badHeader, "checksum": badSum} {
		if _, err := ParseEDID(b); err == nil {
			t.Errorf("%s: 應該要回傳錯誤", name)
		}
	}
}
