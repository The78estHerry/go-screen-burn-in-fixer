package program

import (
	"math/rand/v2"
	"time"
)

// RandomSlice 是 Shuffle 的特殊段長：每段長度在 MinRandomSlice–MaxRandomSlice 之間隨機。
const RandomSlice = 0

// 隨機段長的範圍。
const (
	MinRandomSlice = 15 * time.Second
	MaxRandomSlice = 60 * time.Second
)

// Shuffle 把每個圖樣的時間切成小段後打散，同一個圖樣不會連續出現。
// 每個圖樣在一輪中的總時間與 p 相同；一輪的長度也與 p 相同。
// 某個圖樣佔一輪一半以上的時間時，它的每一段會比 slice 長，才能與其他圖樣交錯。
//
// 回傳的節目表包含足夠的輪數，總長度至少為 minLength；每一輪重新洗牌。
// 相同的 p、slice、seed、minLength 一定得到相同的結果，多個螢幕可以用同一個 seed 各自產生。
func Shuffle(p Program, slice time.Duration, seed uint64, minLength time.Duration) Program {
	total := p.Total()
	if total <= 0 {
		return p
	}
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))

	// 同名的圖樣合併計算總時間，依第一次出現的順序。
	var names []string
	totals := map[string]time.Duration{}
	for _, s := range p {
		if _, ok := totals[s.Pattern]; !ok {
			names = append(names, s.Pattern)
		}
		totals[s.Pattern] += s.Duration
	}

	rounds := 1
	if minLength > total {
		rounds = int((minLength + total - 1) / total)
	}
	var out Program
	last := ""
	for range rounds {
		r := shuffleRound(names, totals, slice, rng, last)
		last = r[len(r)-1].Pattern
		out = append(out, r...)
	}
	return out
}

// shuffleRound 產生一輪洗牌後的段落。last 是上一輪最後一個圖樣，避免跨輪連續出現。
func shuffleRound(names []string, totals map[string]time.Duration, slice time.Duration, rng *rand.Rand, last string) Program {
	pieces := map[string][]time.Duration{}
	remaining := 0
	for _, n := range names {
		pieces[n] = cut(totals[n], slice, rng)
		remaining += len(pieces[n])
	}
	// 某個圖樣的段數多於其他圖樣合計 + 1 時，一定會連續出現；改切成較少、較長的段，平均穿插在其他圖樣之間。
	for _, n := range names {
		if c, others := len(pieces[n]), remaining-len(pieces[n]); c > others+1 {
			pieces[n] = cut(totals[n], totals[n]/time.Duration(others+1), rng)
			remaining = others + len(pieces[n])
		}
	}

	var out Program
	for remaining > 0 {
		next := pick(names, pieces, remaining, last, rng)
		d := pieces[next][0]
		pieces[next] = pieces[next][1:]
		remaining--
		if len(out) > 0 && out[len(out)-1].Pattern == next {
			out[len(out)-1].Duration += d // 只有一種圖樣時才會發生：合併成同一段
		} else {
			out = append(out, Segment{Pattern: next, Duration: d})
		}
		last = next
	}

	played := map[string]time.Duration{}
	for i := range out {
		out[i].Played = played[out[i].Pattern]
		out[i].PatternTotal = totals[out[i].Pattern]
		played[out[i].Pattern] += out[i].Duration
	}
	return out
}

// pick 選出下一段的圖樣：不和 last 相同，並依剩餘段數加權隨機。
// 某個圖樣的剩餘段數多到不先排它就一定會連續出現時，直接選它。
func pick(names []string, pieces map[string][]time.Duration, remaining int, last string, rng *rand.Rand) string {
	var cands []string
	weight := 0
	for _, n := range names {
		if c := len(pieces[n]); c > 0 && n != last {
			cands = append(cands, n)
			weight += c
		}
	}
	if len(cands) == 0 {
		return last
	}
	for _, n := range cands {
		if c := len(pieces[n]); c > remaining-c {
			return n
		}
	}
	w := rng.IntN(weight)
	for _, n := range cands {
		if w -= len(pieces[n]); w < 0 {
			return n
		}
	}
	return cands[len(cands)-1]
}

// cut 把 total 切成小段：固定段長時平均切成最接近 slice 的段數；RandomSlice 時每段隨機長度（整數秒）。
func cut(total, slice time.Duration, rng *rand.Rand) []time.Duration {
	if slice > 0 {
		n := max(int((total+slice/2)/slice), 1)
		each := total / time.Duration(n)
		ds := make([]time.Duration, n)
		for i := range ds {
			ds[i] = each
		}
		ds[n-1] += total - each*time.Duration(n)
		return ds
	}
	var ds []time.Duration
	for rem := total; rem > 0; {
		if rem <= MaxRandomSlice {
			ds = append(ds, rem)
			break
		}
		span := int((MaxRandomSlice - MinRandomSlice) / time.Second)
		d := MinRandomSlice + time.Duration(rng.IntN(span+1))*time.Second
		if rem-d < MinRandomSlice {
			d = rem - MinRandomSlice
		}
		ds = append(ds, d)
		rem -= d
	}
	return ds
}
