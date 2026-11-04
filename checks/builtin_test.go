package checks

import (
	"slices"
	"testing"

	"github.com/The78estHerry/go-screen-burn-in-fixer/internal/i18n"
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
	defer i18n.Set(i18n.Current())
	for l, want := range map[i18n.Lang]string{i18n.ZH: "灰階 128（50.2%）", i18n.EN: "Gray 128 (50.2%)"} {
		i18n.Set(l)
		if got := GrayLabel(128); got != want {
			t.Errorf("%s：GrayLabel(128) = %q，預期 %q", l, got, want)
		}
	}
}

// 每個檢查項目在兩種語言下都有說明、提示與各階名稱，英文不可以混入中文。
func TestChecksTranslated(t *testing.T) {
	defer i18n.Set(i18n.Current())
	i18n.Set(i18n.EN)
	for _, c := range All() {
		texts := append([]string{c.Description(), c.Hint()}, c.Steps()...)
		for _, s := range texts {
			if s == "" {
				t.Errorf("%s：有空白的文字", c.Name())
			}
			for _, r := range s {
				if r >= 0x4e00 && r <= 0x9fff {
					t.Errorf("%s：英文介面出現中文 %q", c.Name(), s)
					break
				}
			}
		}
	}
}
