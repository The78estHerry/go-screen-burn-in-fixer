package i18n

import "testing"

func TestParse(t *testing.T) {
	for in, want := range map[string]Lang{"en": EN, "EN": EN, "en-US": EN, "zh": ZH, "zh-TW": ZH, "zh_Hant": ZH} {
		if got, ok := Parse(in); !ok || got != want {
			t.Errorf("Parse(%q) = %q, %v，預期 %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "fr", "english"} {
		if got, ok := Parse(in); ok || got != Default {
			t.Errorf("Parse(%q) = %q, %v，預期不認得、回傳預設", in, got, ok)
		}
	}
}

func TestT(t *testing.T) {
	defer Set(Current())
	if Default != EN {
		t.Errorf("預設語言 = %q，使用者指定預設英文", Default)
	}
	Set(ZH)
	if got := T("中", "en"); got != "中" {
		t.Errorf("ZH：T = %q", got)
	}
	if got := (Text{ZH: "中", EN: "en"}).String(); got != "中" {
		t.Errorf("ZH：Text = %q", got)
	}
	Set(EN)
	if got := T("中", "en"); got != "en" {
		t.Errorf("EN：T = %q", got)
	}
	if got := Join(Text{"甲", "a"}, Text{"乙", "b"}).String(); got != "ab" {
		t.Errorf("EN：Join = %q", got)
	}
	Set("fr")
	if Current() != Default {
		t.Errorf("不認得的語言應改用預設，得到 %q", Current())
	}
}
