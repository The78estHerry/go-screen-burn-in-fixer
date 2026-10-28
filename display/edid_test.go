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

// withExtensions 在 sampleEDID 後加上擴充區塊並更新數量與檢查碼。
func withExtensions(exts ...[]byte) []byte {
	b := sampleEDID()
	b[126] = byte(len(exts))
	fixChecksum(b[:128])
	for _, e := range exts {
		fixChecksum(e)
		b = append(b, e...)
	}
	return b
}

func fixChecksum(b []byte) {
	var sum byte
	for _, v := range b[:127] {
		sum += v
	}
	b[127] = -sum
}

func le(v, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(v >> (8 * i))
	}
	return out
}

func TestParseEDIDExtensions(t *testing.T) {
	// CTA-861：詳細時序 2560×1440@60（像素時脈 241.5 MHz，總畫面 2720×1481）。
	cta := make([]byte, 128)
	cta[0], cta[1], cta[2] = 0x02, 0x03, 4
	d := cta[4:22]
	copy(d[0:2], le(24150, 2))
	d[2], d[3], d[4] = 0x00, 0xa0, 0xa0 // hActive 0xA00, hBlank 0x0A0
	d[5], d[6], d[7] = 0xa0, 0x29, 0x50 // vActive 0x5A0, vBlank 0x029

	// DisplayID 1.3：Type I 詳細時序 5120×1440@120（總畫面 5280×1490）。
	did := make([]byte, 128)
	did[0] = 0x70
	sec := did[1:]
	sec[0], sec[1] = 0x13, 3+20
	blk := sec[4:]
	blk[0], blk[1], blk[2] = 0x03, 0, 20
	tm := blk[3:23]
	clock := 5280 * 1490 * 120 / 10000
	copy(tm[0:3], le(clock-1, 3))
	copy(tm[4:6], le(5120-1, 2))
	copy(tm[6:8], le(160-1, 2))
	copy(tm[12:14], le(1440-1, 2))
	copy(tm[14:16], le(50-1, 2))

	info, err := ParseEDID(withExtensions(cta, did))
	if err != nil {
		t.Fatal(err)
	}
	if info.Native.Width != 5120 || info.Native.Height != 1440 {
		t.Errorf("Native = %v，預期 5120×1440", info.Native)
	}
	if hz := info.Native.RefreshHz; hz < 119.9 || hz > 120.1 {
		t.Errorf("RefreshHz = %v，預期 120", hz)
	}

	info, err = ParseEDID(withExtensions(cta))
	if err != nil {
		t.Fatal(err)
	}
	if info.Native.Width != 2560 || info.Native.Height != 1440 {
		t.Errorf("只有 CTA 時 Native = %v，預期 2560×1440", info.Native)
	}

	// 檢查碼錯誤的擴充區塊略過，仍使用基本區塊。
	b := withExtensions(cta)
	b[200]++
	if info, err := ParseEDID(b); err != nil || info.Native.Width != 1920 {
		t.Errorf("壞掉的擴充區塊應略過: %v %v", info.Native, err)
	}
}
