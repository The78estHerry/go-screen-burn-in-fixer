//go:build windows

package display

import "testing"

// 實際查詢這台電腦的顯示器；只檢查能正常回傳，並把結果印出來方便人工確認。
func TestDisplaysSmoke(t *testing.T) {
	ds, err := Displays()
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) == 0 {
		t.Skip("沒有使用中的顯示器（可能在無螢幕環境執行）")
	}
	for i, d := range ds {
		t.Logf("%d: %s [%s]", i, d.Label(), d.DeviceName)
		if d.Current.Width <= 0 || d.Current.Height <= 0 {
			t.Errorf("%d: 解析度無效 %v", i, d.Current)
		}
	}
}
