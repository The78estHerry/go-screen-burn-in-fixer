package app

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/The78estHerry/go-screen-burn-in-fixer/checks"
	"github.com/The78estHerry/go-screen-burn-in-fixer/internal/i18n"
	"github.com/The78estHerry/go-screen-burn-in-fixer/internal/ui"
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
	if c.GrayMin != 0 || c.GrayMax != 0 || c.GrayFieldN != checks.MinGrayFieldSteps {
		// 直接 Unmarshal 到空的 Config（沒有預設值）：0–0、階數限制到最小值
		t.Errorf("灰階全畫面 = %d–%d、%d 階", c.GrayMin, c.GrayMax, c.GrayFieldN)
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

func TestLoadConfigGrayField(t *testing.T) {
	dir := t.TempDir()
	load := func(js string) Config {
		t.Helper()
		path := filepath.Join(dir, "c.json")
		if err := os.WriteFile(path, []byte(js), 0o644); err != nil {
			t.Fatal(err)
		}
		c, err := LoadConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	// 舊版設定檔沒有灰階全畫面的欄位：使用預設值。
	c := load(`{"mode": "check"}`)
	if c.GrayMin != checks.DefaultGrayMin || c.GrayMax != checks.DefaultGrayMax || c.GrayFieldN != checks.DefaultGrayFieldSteps {
		t.Errorf("預設值錯誤：%d–%d、%d 階", c.GrayMin, c.GrayMax, c.GrayFieldN)
	}
	// 最暗大於最亮時交換；超出範圍時限制。
	c = load(`{"grayMin": 200, "grayMax": 20, "grayFieldSteps": 40}`)
	if c.GrayMin != 20 || c.GrayMax != 200 || c.GrayFieldN != checks.MaxGrayFieldSteps {
		t.Errorf("限制後：%d–%d、%d 階", c.GrayMin, c.GrayMax, c.GrayFieldN)
	}
	c = load(`{"grayMin": 0, "grayMax": 300}`)
	if c.GrayMin != 0 || c.GrayMax != 255 {
		t.Errorf("限制後：%d–%d", c.GrayMin, c.GrayMax)
	}
}

// withLang 在指定語言下執行 f，結束後恢復原本的語言。
func withLang(l i18n.Lang, f func()) {
	defer i18n.Set(i18n.Current())
	i18n.Set(l)
	f()
}

func TestGrayPosition(t *testing.T) {
	ls := []int{13, 43, 72}
	for _, tc := range []struct {
		v      int
		zh, en string
	}{{13, "第 1/3 階", "step 1/3"}, {43, "第 2/3 階", "step 2/3"}, {50, "第 2–3/3 階之間", "between steps 2–3/3"}, {72, "第 3/3 階", "step 3/3"}} {
		withLang(i18n.ZH, func() {
			if got := grayPosition(ls, tc.v); got != tc.zh {
				t.Errorf("grayPosition(%d) = %q，預期 %q", tc.v, got, tc.zh)
			}
		})
		withLang(i18n.EN, func() {
			if got := grayPosition(ls, tc.v); got != tc.en {
				t.Errorf("grayPosition(%d) = %q，預期 %q", tc.v, got, tc.en)
			}
		})
	}
	if v, ok := nextLevel(ls, 50, 1); !ok || v != 72 {
		t.Errorf("50 的下一階 = %d, %v", v, ok)
	}
	if v, ok := nextLevel(ls, 50, -1); !ok || v != 43 {
		t.Errorf("50 的上一階 = %d, %v", v, ok)
	}
	if _, ok := nextLevel(ls, 72, 1); ok {
		t.Error("最後一階之後應該沒有下一階")
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
	cases := map[string]int{"90": 5400, "3h": 10800, "3H": 10800, "2:30": 9000, "2h30": 9000, "2h30m": 9000, "45m": 2700,
		"1:05": 3900, "0:45": 2700, ":30": 1800, "10s": 10, "10S": 10, "3m": 180, "1m30s": 90, "1m30": 90, "1h5m10s": 3910, "1h10s": 3610}
	for in, want := range cases {
		if got, ok := parseTimerInput(in); !ok || got != want {
			t.Errorf("parseTimerInput(%q) = %d, %v，預期 %d", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "h", "m", "s", "0", "0s", "0:00", "abc", "1:x", "h30", "10s5", "1m2h", "1s1s", "1x"} {
		if _, ok := parseTimerInput(in); ok {
			t.Errorf("parseTimerInput(%q) 應該無效", in)
		}
	}
	if got := stepSorted(timerSecondsChoices, 100*60, 1); got != 120*60 {
		t.Errorf("stepSorted(100 分, +1) = %d，預期 7200", got)
	}
	if got := stepSorted(timerSecondsChoices, 100*60, -1); got != 90*60 {
		t.Errorf("stepSorted(100 分, -1) = %d，預期 5400", got)
	}
	if got := stepSorted(timerSecondsChoices, 5*60, -1); got != 60 {
		t.Errorf("5 分鐘的前一個 = %d，預期 60", got)
	}
	if got := stepSorted(timerSecondsChoices, 10, -1); got != 10 {
		t.Errorf("10 秒的前一個 = %d，預期停在 10", got)
	}
}

func TestFormatTimer(t *testing.T) {
	withLang(i18n.ZH, func() {
		for sec, want := range map[int]string{10: "10 秒", 30: "30 秒", 60: "1 分鐘", 90: "1 分 30 秒", 300: "5 分鐘",
			7200: "2 小時", 9000: "2 小時 30 分", 3610: "1 小時 0 分 10 秒", 3910: "1 小時 5 分 10 秒"} {
			if got := formatTimer(sec); got != want {
				t.Errorf("formatTimer(%d) = %q，預期 %q", sec, got, want)
			}
		}
	})
	withLang(i18n.EN, func() {
		for sec, want := range map[int]string{10: "10 s", 60: "1 min", 90: "1 min 30 s", 7200: "2 h", 9000: "2 h 30 min"} {
			if got := formatTimer(sec); got != want {
				t.Errorf("formatTimer(%d) = %q，預期 %q", sec, got, want)
			}
		}
	})
}

// 設定檔沒有 lang 時預設英文；zh-TW 之類的寫法統一成 zh，不認得的值改回預設。
func TestConfigLang(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	for in, want := range map[string]string{`{}`: "en", `{"lang": "zh-TW"}`: "zh", `{"lang": "fr"}`: "en", `{"lang": "en"}`: "en"} {
		if err := os.WriteFile(path, []byte(in), 0o644); err != nil {
			t.Fatal(err)
		}
		c, err := LoadConfig(path)
		if err != nil || c.Lang != want {
			t.Errorf("%s：lang = %q, %v，預期 %q", in, c.Lang, err, want)
		}
	}
	if c := DefaultConfig(); c.Lang != "en" {
		t.Errorf("預設語言 = %q，預期 en", c.Lang)
	}
}

// 清單列的說明：寬度改成「N px」，放不下時截斷；兩種語言都適用。
func TestShortDesc(t *testing.T) {
	for in, suffix := range map[string]string{"隨機顏色的直條紋（寬 4 像素）": " 4 px", "Randomly colored vertical stripes (1 px wide)": " 1 px"} {
		got := shortDesc(in)
		if !strings.HasSuffix(got, suffix) {
			t.Errorf("shortDesc(%q) = %q，應以 %q 結尾", in, got, suffix)
		}
		if w, _ := ui.Measure(got, descSize); w > descMaxW {
			t.Errorf("shortDesc(%q) = %q 寬 %.0f，超過 %.0f", in, got, w, float64(descMaxW))
		}
	}
}

func TestLegacyTimerMinutes(t *testing.T) {
	var c Config
	if err := json.Unmarshal([]byte(`{"timerMinutes": 90}`), &c); err != nil {
		t.Fatal(err)
	}
	c.normalize()
	if c.TimerSeconds != 5400 || c.LegacyTimerMinutes != 0 {
		t.Errorf("舊設定檔 90 分鐘 → %d 秒（舊欄位 %d）", c.TimerSeconds, c.LegacyTimerMinutes)
	}
}

func TestBreath(t *testing.T) {
	if f := breath(0); math.Abs(f-1) > 1e-9 {
		t.Errorf("breath(0) = %v，預期 1", f)
	}
	if f := breath(breathPeriod / 2); math.Abs(f-breathMin) > 1e-9 {
		t.Errorf("breath(半個週期) = %v，預期 %v", f, breathMin)
	}
	if got := dimmed(ui.ColorText, 1); got != ui.ColorText {
		t.Errorf("dimmed(f=1) = %v，預期原色", got)
	}
	if got := dimmed(ui.ColorText, 0); got != ui.ColorBackground {
		t.Errorf("dimmed(f=0) = %v，預期背景色", got)
	}
}
