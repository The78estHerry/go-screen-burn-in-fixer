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
