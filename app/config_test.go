package app

import (
	"encoding/json"
	"testing"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/checks"
)

func TestCheckSequence(t *testing.T) {
	c := DefaultConfig()
	want := 0
	for _, ck := range checks.All() {
		want += len(ck.Steps())
	}
	seq := c.CheckSequence()
	if len(seq) != want {
		t.Fatalf("CheckSequence 有 %d 階，預期 %d 階", len(seq), want)
	}
	last := seq[len(seq)-1]
	if last.Index != last.Items-1 || last.Seconds != DefaultCheckSeconds {
		t.Errorf("最後一階的資訊錯誤: %+v", last)
	}

	c.CheckItems[0].Enabled = false
	if got := len(c.CheckSequence()); got != want-len(checks.All()[0].Steps()) {
		t.Errorf("取消勾選第一項後有 %d 階", got)
	}
}

func TestNormalizeConfig(t *testing.T) {
	// 舊版設定檔：沒有檢查項目、資訊列與階數；有一個不存在的檢查項目與無效秒數。
	var c Config
	if err := json.Unmarshal([]byte(`{
		"mode": "check",
		"checkItems": [{"check": "white", "enabled": false, "seconds": 0},
		               {"check": "gray-field", "enabled": true, "seconds": 7},
		               {"check": "no-such", "enabled": true, "seconds": 10}],
		"grayLevels": 999,
		"infoBar": 4
	}`), &c); err != nil {
		t.Fatal(err)
	}
	c.normalize()
	if c.Mode != ModeCheck {
		t.Errorf("Mode = %q", c.Mode)
	}
	if c.GrayLevels != checks.MaxGrayLevels {
		t.Errorf("GrayLevels = %d，預期 %d", c.GrayLevels, checks.MaxGrayLevels)
	}
	if c.InfoBar != 5 {
		t.Errorf("InfoBar = %d，預期回到預設 5", c.InfoBar)
	}
	if len(c.CheckItems) != len(checks.All()) {
		t.Fatalf("CheckItems 有 %d 項，預期 %d 項", len(c.CheckItems), len(checks.All()))
	}
	for _, it := range c.CheckItems {
		switch it.Check {
		case "white":
			if it.Enabled || it.Seconds != Manual {
				t.Errorf("white 應保留「不勾選、手動」: %+v", it)
			}
		case "gray-field":
			if it.Seconds != 5 { // 7 最接近的選項是 5
				t.Errorf("gray-field 秒數 = %d，預期 5", it.Seconds)
			}
		case "no-such":
			t.Error("不存在的檢查項目應被移除")
		}
	}
}

func TestStepAny(t *testing.T) {
	if got := stepAny(infoBarChoices, 10, 1); got != InfoAlways {
		t.Errorf("10 秒的下一個 = %d，預期一直顯示", got)
	}
	if got := stepAny(infoBarChoices, InfoHidden, -1); got != InfoHidden {
		t.Errorf("不顯示的前一個 = %d，預期停在不顯示", got)
	}
	if got := stepLevels(32, 1); got != 64 {
		t.Errorf("stepLevels(32, 1) = %d", got)
	}
	if got := stepLevels(50, -1); got != 32 {
		t.Errorf("stepLevels(50, -1) = %d", got)
	}
}

func TestNormalizeLegacyMonitor(t *testing.T) {
	var c Config
	if err := json.Unmarshal([]byte(`{"monitor": 2, "marks": [{"x": 1, "y": 2}]}`), &c); err != nil {
		t.Fatal(err)
	}
	c.normalize()
	if len(c.Monitors) != 1 || c.Monitors[0] != 2 || c.LegacyMonitor != 0 {
		t.Errorf("Monitors = %v，預期 [2]", c.Monitors)
	}
	if c.MarkCount() != 1 {
		t.Errorf("MarkCount() = %d，預期保留舊標記", c.MarkCount())
	}
	c.Monitors = []int{3, -1, 3, 0}
	c.normalize()
	if len(c.Monitors) != 2 || c.Monitors[0] != 3 || c.Monitors[1] != 0 {
		t.Errorf("Monitors = %v，預期移除重複與負數", c.Monitors)
	}
}

func TestToggleMonitor(t *testing.T) {
	if got := toggleMonitor([]int{0}, 0); len(got) != 1 {
		t.Errorf("最後一個螢幕不能取消: %v", got)
	}
	if got := toggleMonitor([]int{2}, 0); len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Errorf("toggleMonitor([2], 0) = %v，預期 [0 2]", got)
	}
	if got := toggleMonitor([]int{0, 2}, 0); len(got) != 1 || got[0] != 2 {
		t.Errorf("toggleMonitor([0 2], 0) = %v，預期 [2]", got)
	}
}

func TestCheckSpec(t *testing.T) {
	c := DefaultConfig()
	for i := range c.CheckItems {
		c.CheckItems[i].Enabled = i < 2
	}
	c.CheckItems[1].Seconds = Manual
	want := c.CheckItems[0].Check + ":10," + c.CheckItems[1].Check + ":manual"
	if got := c.CheckSpec(); got != want {
		t.Errorf("CheckSpec() = %q，預期 %q", got, want)
	}
}

func TestParseTimerInput(t *testing.T) {
	cases := map[string]int{"90": 90, "3h": 180, "3H": 180, "2:30": 150, "2h30": 150, "2h30m": 150, "45m": 45, "1:05": 65, "0:45": 45, ":30": 30}
	for in, want := range cases {
		if got, ok := parseTimerInput(in); !ok || got != want {
			t.Errorf("parseTimerInput(%q) = %d, %v，預期 %d", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "h", "m", "0", "0:00", "abc", "1:x", "h30"} {
		if _, ok := parseTimerInput(in); ok {
			t.Errorf("parseTimerInput(%q) 應該無效", in)
		}
	}
	if got := stepSorted(timerMinutesChoices, 100, 1); got != 120 {
		t.Errorf("stepSorted(100, +1) = %d，預期 120", got)
	}
	if got := stepSorted(timerMinutesChoices, 100, -1); got != 90 {
		t.Errorf("stepSorted(100, -1) = %d，預期 90", got)
	}
	if got := formatMinutes(150); got != "2 小時 30 分" {
		t.Errorf("formatMinutes(150) = %q", got)
	}
}
