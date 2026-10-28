package program

import (
	"testing"
	"time"
)

func TestShuffle(t *testing.T) {
	base := Program{
		{Pattern: "gray-noise", Duration: 600 * time.Second},
		{Pattern: "vstripes", Duration: 300 * time.Second},
		{Pattern: "moving-bar", Duration: 45 * time.Second},
		{Pattern: "solid-cycle", Duration: 10 * time.Second},
	}
	for _, slice := range []time.Duration{RandomSlice, 10 * time.Second, 30 * time.Second, 60 * time.Second} {
		for seed := uint64(1); seed <= 50; seed++ {
			p := Shuffle(base, slice, seed, 0)
			if p.Total() != base.Total() {
				t.Fatalf("slice %v seed %d：一輪 %v，預期 %v", slice, seed, p.Total(), base.Total())
			}
			sums := map[string]time.Duration{}
			for i, s := range p {
				if i > 0 && p[i-1].Pattern == s.Pattern {
					t.Fatalf("slice %v seed %d：第 %d 段 %s 連續出現", slice, seed, i, s.Pattern)
				}
				if s.Played != sums[s.Pattern] {
					t.Fatalf("第 %d 段 Played = %v，預期 %v", i, s.Played, sums[s.Pattern])
				}
				sums[s.Pattern] += s.Duration
			}
			for _, b := range base {
				if sums[b.Pattern] != b.Duration {
					t.Fatalf("%s 合計 %v，預期 %v", b.Pattern, sums[b.Pattern], b.Duration)
				}
			}
		}
	}
}

func TestShuffleRounds(t *testing.T) {
	base := Program{{Pattern: "gray-noise", Duration: 60 * time.Second}, {Pattern: "vstripes", Duration: 60 * time.Second}}
	p := Shuffle(base, 20*time.Second, 7, 5*time.Minute)
	if p.Total() != 6*time.Minute { // 3 輪
		t.Fatalf("Total() = %v，預期 6m", p.Total())
	}
	for i := 1; i < len(p); i++ {
		if p[i-1].Pattern == p[i].Pattern {
			t.Fatalf("第 %d 段跨輪連續出現", i)
		}
	}
	if q := Shuffle(base, 20*time.Second, 7, 5*time.Minute); q.String() != p.String() {
		t.Error("相同 seed 應產生相同結果")
	}
}

func TestShuffleSinglePattern(t *testing.T) {
	p := Shuffle(Program{{Pattern: "gray-noise", Duration: 100 * time.Second}}, RandomSlice, 3, 0)
	if len(p) != 1 || p[0].Duration != 100*time.Second {
		t.Fatalf("只有一個圖樣時應合併成一段: %+v", p)
	}
}

func TestShuffleRandomSliceRange(t *testing.T) {
	base := Program{
		{Pattern: "gray-noise", Duration: 300 * time.Second},
		{Pattern: "vstripes", Duration: 300 * time.Second},
		{Pattern: "hstripes", Duration: 200 * time.Second},
	}
	for seed := uint64(1); seed <= 50; seed++ {
		for _, s := range Shuffle(base, RandomSlice, seed, 0) {
			if s.Duration < MinRandomSlice || s.Duration > MaxRandomSlice {
				t.Fatalf("seed %d：段長 %v 不在 %v–%v", seed, s.Duration, MinRandomSlice, MaxRandomSlice)
			}
		}
	}
}
