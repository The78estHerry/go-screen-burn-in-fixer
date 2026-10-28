// Package program 定義節目表：依序（或洗牌後）播放的圖樣與各自的秒數，播完一輪後循環。
package program

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/patterns"
)

// DefaultSegment 是每個圖樣預設播放的時間。
const DefaultSegment = 60 * time.Second

// Segment 是節目表中的一段。
type Segment struct {
	Pattern  string
	Duration time.Duration

	// 洗牌後的段落才有：這個圖樣在這一輪中、這一段之前已播放的時間，以及一輪中的總時間。
	Played       time.Duration
	PatternTotal time.Duration
}

// Program 是依序播放、循環的節目表。
type Program []Segment

// Default 回傳全部圖樣依註冊順序、各播 DefaultSegment 的節目表。
func Default() Program {
	var p Program
	for _, pt := range patterns.All() {
		p = append(p, Segment{Pattern: pt.Name(), Duration: DefaultSegment})
	}
	return p
}

// Parse 解析 "gray-noise:600,vstripes:300" 格式（秒數）的節目表。
func Parse(s string) (Program, error) {
	var p Program
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		name, secs, ok := strings.Cut(item, ":")
		if !ok {
			return nil, fmt.Errorf("節目表格式錯誤 %q：應為 圖樣:秒數", item)
		}
		if _, ok := patterns.Get(name); !ok {
			return nil, fmt.Errorf("沒有名為 %q 的圖樣", name)
		}
		n, err := strconv.ParseFloat(secs, 64)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("圖樣 %s 的秒數無效：%q", name, secs)
		}
		p = append(p, Segment{Pattern: name, Duration: time.Duration(n * float64(time.Second))})
	}
	if len(p) == 0 {
		return nil, errors.New("節目表是空的")
	}
	return p, nil
}

// String 回傳可以給 Parse 解析的字串。
func (p Program) String() string {
	parts := make([]string, len(p))
	for i, s := range p {
		parts[i] = s.Pattern + ":" + strconv.FormatFloat(s.Duration.Seconds(), 'f', -1, 64)
	}
	return strings.Join(parts, ",")
}

// Total 回傳播完一輪的時間。
func (p Program) Total() time.Duration {
	var t time.Duration
	for _, s := range p {
		t += s.Duration
	}
	return t
}

// At 回傳經過 elapsed 時間後正在播放的段落索引，以及在該段落中已播放的時間（循環播放）。
func (p Program) At(elapsed time.Duration) (idx int, offset time.Duration) {
	total := p.Total()
	if total <= 0 {
		return 0, 0
	}
	if elapsed < 0 {
		elapsed = 0
	}
	elapsed %= total
	for i, s := range p {
		if elapsed < s.Duration {
			return i, elapsed
		}
		elapsed -= s.Duration
	}
	return len(p) - 1, 0
}

// Start 回傳第 idx 段在一輪中的開始時間。
func (p Program) Start(idx int) time.Duration {
	var t time.Duration
	for i := 0; i < idx && i < len(p); i++ {
		t += p[i].Duration
	}
	return t
}
