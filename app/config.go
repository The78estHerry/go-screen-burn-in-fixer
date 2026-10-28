package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/actions"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/checks"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/patterns"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/program"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/screensaver"
)

// Mode 是播放模式。
type Mode string

const (
	ModeBurnIn      Mode = "burnin"      // 播放 Burn-In 圖樣，時間到執行後續動作
	ModeCheck       Mode = "check"       // 依序顯示檢查畫面
	ModeScreensaver Mode = "screensaver" // 只播螢幕保護畫面
)

// Manual 是檢查項目秒數的特殊值：手動，按 Enter 或空白鍵才進行下一階。
const Manual = 0

// 資訊列顯示設定的特殊值；其他正數代表顯示的秒數。
const (
	InfoHidden = 0  // 不顯示（按住 S 或按 H 才出現）
	InfoAlways = -1 // 一直顯示
)

// CheckItemConfig 是設定選單中的一個檢查項目。
type CheckItemConfig struct {
	Check   string `json:"check"`
	Enabled bool   `json:"enabled"`
	Seconds int    `json:"seconds"` // 每一階的秒數；Manual（0）代表手動
}

// Mark 是使用者在檢查畫面上標記的可疑位置（螢幕像素座標）。
type Mark struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// SaverExit 是螢幕保護畫面結束後的動作。
type SaverExit string

const (
	SaverExitMenu SaverExit = "menu"
	SaverExitQuit SaverExit = "quit"
)

// SegmentConfig 是設定選單中的一個圖樣。
type SegmentConfig struct {
	Pattern string `json:"pattern"`
	Enabled bool   `json:"enabled"`
	Seconds int    `json:"seconds"`
}

// Config 是使用者設定，存在 exe 旁邊的 go-screen-burn-in-fixer.json。
type Config struct {
	Mode         Mode            `json:"mode"`
	Monitor      int             `json:"monitor"`
	Segments     []SegmentConfig `json:"segments"`
	TimerMinutes int             `json:"timerMinutes"`
	After        actions.After   `json:"after"`
	Screensaver  string          `json:"screensaver"`
	SaverExit    SaverExit       `json:"saverExit"`

	CheckItems []CheckItemConfig `json:"checkItems"`
	GrayLevels int               `json:"grayLevels"` // 灰階分階的階數
	Marks      []Mark            `json:"marks"`

	InfoBar int `json:"infoBar"` // 資訊列：InfoHidden、InfoAlways 或顯示秒數
}

// 選單中可選擇的數值。
var (
	segmentSecondsChoices = []int{10, 15, 20, 30, 45, 60, 90, 120, 180, 300, 600}
	timerMinutesChoices   = []int{5, 10, 15, 20, 30, 45, 50, 60, 90, 120, 180, 240}
	checkSecondsChoices   = []int{Manual, 5, 10, 15, 20, 30, 45, 60, 120}
	infoBarChoices        = []int{InfoHidden, 3, 5, 10, InfoAlways}
)

// DefaultCheckSeconds 是檢查項目「自動」時的預設秒數。
const DefaultCheckSeconds = 10

// DefaultConfig 回傳預設設定：全部圖樣各 60 秒循環、播放 50 分鐘、結束後切換成螢幕保護；
// 檢查項目全部自動 10 秒；資訊列顯示 5 秒。
func DefaultConfig() Config {
	c := Config{
		Mode:         ModeBurnIn,
		TimerMinutes: 50,
		After:        actions.Screensaver,
		Screensaver:  screensaver.All()[0].Name(),
		SaverExit:    SaverExitMenu,
		GrayLevels:   checks.DefaultGrayLevels,
		InfoBar:      5,
	}
	for _, p := range patterns.All() {
		c.Segments = append(c.Segments, SegmentConfig{Pattern: p.Name(), Enabled: true, Seconds: int(program.DefaultSegment.Seconds())})
	}
	for _, ck := range checks.All() {
		c.CheckItems = append(c.CheckItems, CheckItemConfig{Check: ck.Name(), Enabled: true, Seconds: DefaultCheckSeconds})
	}
	return c
}

// ConfigPath 回傳設定檔路徑（exe 旁邊）。
func ConfigPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "go-screen-burn-in-fixer.json"
	}
	return filepath.Join(filepath.Dir(exe), "go-screen-burn-in-fixer.json")
}

// LoadConfig 讀取設定檔；檔案不存在或格式錯誤時回傳預設設定。
// 回傳的設定一定經過 normalize，可以直接使用。
func LoadConfig(path string) (Config, error) {
	c := DefaultConfig()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err == nil {
		err = json.Unmarshal(b, &c)
	}
	if err != nil {
		return DefaultConfig(), err
	}
	c.normalize()
	return c, nil
}

// Save 寫入設定檔。
func (c Config) Save(path string) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// normalize 修正無效的值，並補上新版本新增、舊設定檔沒有的圖樣。
func (c *Config) normalize() {
	def := DefaultConfig()
	if c.Mode != ModeBurnIn && c.Mode != ModeCheck && c.Mode != ModeScreensaver {
		c.Mode = def.Mode
	}
	c.GrayLevels = checks.ClampGrayLevels(c.GrayLevels)
	if !contains(infoBarChoices, c.InfoBar) {
		c.InfoBar = def.InfoBar
	}

	oldChecks := map[string]CheckItemConfig{}
	for _, it := range c.CheckItems {
		oldChecks[it.Check] = it
	}
	c.CheckItems = c.CheckItems[:0]
	for _, d := range def.CheckItems {
		it, ok := oldChecks[d.Check]
		if !ok {
			it = d
		}
		it.Seconds = nearest(checkSecondsChoices, it.Seconds)
		c.CheckItems = append(c.CheckItems, it)
	}
	if _, err := actions.ParseAfter(string(c.After)); err != nil {
		c.After = def.After
	}
	if _, ok := screensaver.Get(c.Screensaver); !ok {
		c.Screensaver = def.Screensaver
	}
	if c.SaverExit != SaverExitMenu && c.SaverExit != SaverExitQuit {
		c.SaverExit = def.SaverExit
	}
	c.TimerMinutes = nearest(timerMinutesChoices, c.TimerMinutes)
	if c.Monitor < 0 {
		c.Monitor = 0
	}

	// 依目前註冊的圖樣重建清單：保留使用者的勾選與秒數，移除已不存在的圖樣。
	old := map[string]SegmentConfig{}
	for _, s := range c.Segments {
		old[s.Pattern] = s
	}
	c.Segments = c.Segments[:0]
	for _, d := range def.Segments {
		s, ok := old[d.Pattern]
		if !ok {
			s = d
		}
		s.Seconds = nearest(segmentSecondsChoices, s.Seconds)
		c.Segments = append(c.Segments, s)
	}
}

// Program 回傳勾選的圖樣組成的節目表；沒有勾選任何圖樣時回傳 nil。
func (c Config) Program() program.Program {
	var p program.Program
	for _, s := range c.Segments {
		if s.Enabled {
			p = append(p, program.Segment{Pattern: s.Pattern, Duration: time.Duration(s.Seconds) * time.Second})
		}
	}
	return p
}

// Timer 回傳播放時間。
func (c Config) Timer() time.Duration {
	return time.Duration(c.TimerMinutes) * time.Minute
}

// CheckStep 是檢查流程中的一階。
type CheckStep struct {
	Check   checks.Check
	Step    int
	Seconds int // Manual 代表手動
	Index   int // 第幾個項目（從 0 開始）
	Items   int // 項目總數
}

// CheckSequence 依勾選的項目展開成依序顯示的每一階，並套用灰階分階的階數。
func (c Config) CheckSequence() []CheckStep {
	if g, ok := checks.Get("gray-steps"); ok {
		if gs, ok := g.(*checks.GraySteps); ok {
			gs.Levels = c.GrayLevels
		}
	}
	var items []CheckItemConfig
	for _, it := range c.CheckItems {
		if _, ok := checks.Get(it.Check); ok && it.Enabled {
			items = append(items, it)
		}
	}
	var seq []CheckStep
	for i, it := range items {
		ck, _ := checks.Get(it.Check)
		for s := range ck.Steps() {
			seq = append(seq, CheckStep{Check: ck, Step: s, Seconds: it.Seconds, Index: i, Items: len(items)})
		}
	}
	return seq
}

func contains(choices []int, v int) bool {
	for _, c := range choices {
		if c == v {
			return true
		}
	}
	return false
}

// nearest 回傳 choices 中最接近 v 的值。
func nearest(choices []int, v int) int {
	best := choices[0]
	for _, c := range choices {
		if abs(c-v) < abs(best-v) {
			best = c
		}
	}
	return best
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// step 回傳 choices 中 v 的前一個（dir < 0）或下一個（dir > 0）值，到頭時停住。
func step(choices []int, v, dir int) int {
	i := 0
	for j, c := range choices {
		if c == v {
			i = j
		}
	}
	i += dir
	if i < 0 {
		i = 0
	}
	if i >= len(choices) {
		i = len(choices) - 1
	}
	return choices[i]
}
