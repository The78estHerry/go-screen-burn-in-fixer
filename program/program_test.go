package program

import (
	"testing"
	"time"
)

func TestParseAndString(t *testing.T) {
	p, err := Parse(" gray-noise:600, vstripes:1.5 ,")
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 2 || p[0].Pattern != "gray-noise" || p[0].Duration != 600*time.Second || p[1].Duration != 1500*time.Millisecond {
		t.Fatalf("Parse 結果錯誤: %+v", p)
	}
	if got := p.String(); got != "gray-noise:600,vstripes:1.5" {
		t.Errorf("String() = %q", got)
	}
	if p.Total() != 601500*time.Millisecond {
		t.Errorf("Total() = %v", p.Total())
	}
}

func TestParseErrors(t *testing.T) {
	for _, s := range []string{"", "gray-noise", "no-such:10", "gray-noise:0", "gray-noise:abc"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) 應該要回傳錯誤", s)
		}
	}
}

func TestAt(t *testing.T) {
	p := Program{{Pattern: "gray-noise", Duration: 10 * time.Second}, {Pattern: "vstripes", Duration: 5 * time.Second}}
	cases := []struct {
		elapsed time.Duration
		idx     int
		offset  time.Duration
	}{
		{0, 0, 0},
		{9 * time.Second, 0, 9 * time.Second},
		{10 * time.Second, 1, 0},
		{14 * time.Second, 1, 4 * time.Second},
		{15 * time.Second, 0, 0}, // 循環
		{27 * time.Second, 1, 2 * time.Second},
	}
	for _, c := range cases {
		idx, off := p.At(c.elapsed)
		if idx != c.idx || off != c.offset {
			t.Errorf("At(%v) = (%d, %v), want (%d, %v)", c.elapsed, idx, off, c.idx, c.offset)
		}
	}
	if p.Start(1) != 10*time.Second {
		t.Errorf("Start(1) = %v", p.Start(1))
	}
}

func TestDefault(t *testing.T) {
	p := Default()
	if len(p) != 10 {
		t.Errorf("Default() 有 %d 段，預期 10 段", len(p))
	}
}
