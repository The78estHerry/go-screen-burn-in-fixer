package checks

import (
	"slices"
	"testing"
)

func TestGrayFieldLevels(t *testing.T) {
	for _, tc := range []struct {
		min, max, n int
		want        []int
	}{
		{13, 191, 7, []int{13, 43, 72, 102, 132, 161, 191}},
		{3, 4, 4, []int{3, 4}},        // 範圍內只有 2 個灰階
		{50, 50, 7, []int{50}},        // 最暗等於最亮
		{0, 3, 15, []int{0, 1, 2, 3}}, // 階數多於範圍內的灰階
	} {
		g := GrayField{Min: tc.min, Max: tc.max, N: tc.n}
		if got := g.Levels(); !slices.Equal(got, tc.want) {
			t.Errorf("%d–%d、%d 階：%v，預期 %v", tc.min, tc.max, tc.n, got, tc.want)
		}
	}

	g := GrayField{Min: 0, Max: 255, N: 15}
	ls := g.Levels()
	if len(ls) != 15 || ls[0] != 0 || ls[14] != 255 {
		t.Errorf("0–255、15 階：%v", ls)
	}
}

func TestGrayFieldAt(t *testing.T) {
	g := GrayField{Min: 13, Max: 191}
	for _, tc := range []struct {
		frac float64
		want int
	}{{-1, 13}, {0, 13}, {0.5, 102}, {1, 191}, {2, 191}} {
		if got := g.At(tc.frac); got != tc.want {
			t.Errorf("At(%v) = %d，預期 %d", tc.frac, got, tc.want)
		}
	}
}

func TestClampGrayField(t *testing.T) {
	lo, hi, n := ClampGrayField(300, -5, 99)
	if lo != 0 || hi != 255 || n != MaxGrayFieldSteps {
		t.Errorf("ClampGrayField(300, -5, 99) = %d, %d, %d", lo, hi, n)
	}
	if _, _, n := ClampGrayField(0, 0, 1); n != MinGrayFieldSteps {
		t.Errorf("階數 1 應限制為 %d，得到 %d", MinGrayFieldSteps, n)
	}
}

func TestGrayLabel(t *testing.T) {
	if got := GrayLabel(128); got != "灰階 128（50.2%）" {
		t.Errorf("GrayLabel(128) = %q", got)
	}
}
