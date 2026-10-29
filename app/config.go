package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	Monitors     []int           `json:"monitors"` // 要播放的螢幕（從 0 開始，依選單中的順序）
	Segments     []SegmentConfig `json:"segments"`
	Shuffle      bool            `json:"shuffle"`      // Burn-In 圖樣洗牌播放
	ShuffleSlice int             `json:"shuffleSlice"` // 洗牌時每段的秒數；program.RandomSlice（0）代表隨機
	TimerMinutes int             `json:"timerMinutes"`
	After        actions.After   `json:"after"`
	Screensaver  string          `json:"screensaver"`
	SaverExit    SaverExit       `json:"saverExit"`

	CheckItems   []CheckItemConfig `json:"checkItems"`
	GrayLevels   int               `json:"grayLevels"`     // 灰階分階的階數
	GrayMin      int               `json:"grayMin"`        // 灰階全畫面的最暗值（0–255）
	GrayMax      int               `json:"grayMax"`        // 灰階全畫面的最亮值（0–255）
	GrayFieldN   int               `json:"grayFieldSteps"` // 灰階全畫面手動時的階數
	MonitorMarks map[string][]Mark `json:"monitorMarks"`   // 每個螢幕的標記，以 monitor.key 區分

	// 舊版設定檔的欄位（單一螢幕）：讀取後轉換成 Monitors、MonitorMarks。
	LegacyMonitor int    `json:"monitor,omitempty"`
	LegacyMarks   []Mark `json:"marks,omitempty"`

	InfoBar int `json:"infoBar"` // 資訊列：InfoHidden、InfoAlways 或顯示秒數
}

// 選單中可選擇的數值。
var (
	segmentSecondsChoices = []int{10, 15, 20, 30, 45, 60, 90, 120, 180, 300, 600}
	timerMinutesChoices   = []int{5, 10, 15, 20, 30, 45, 50, 60, 90, 120, 180, 240, 300, 360, 480, 600, 720, 1440}
	checkSecondsChoices   = []int{Manual, 5, 10, 15, 20, 30, 45, 60, 120}
	infoBarChoices        = []int{InfoHidden, 3, 5, 10, InfoAlways}
	shuffleSliceChoices   = []int{program.RandomSlice, 10, 20, 30, 60}
)

// MaxTimerMinutes 是播放時間的上限（選單可以直接輸入任意分鐘數）。
const MaxTimerMinutes = 72 * 60

// DefaultCheckSeconds 是檢查項目「自動」時的預設秒數。
const DefaultCheckSeconds = 10

// DefaultConfig 回傳預設設定：全部圖樣各 60 秒循環、播放 50 分鐘、結束後切換成螢幕保護；
// 檢查項目全部自動 10 秒；資訊列顯示 5 秒。
func DefaultConfig() Config {
	c := Config{
		Mode:         ModeBurnIn,
		Monitors:     []int{0},
		TimerMinutes: 50,
		After:        actions.Screensaver,
		Screensaver:  screensaver.All()[0].Name(),
		SaverExit:    SaverExitMenu,
		GrayLevels:   checks.DefaultGrayLevels,
		GrayMin:      checks.DefaultGrayMin,
		GrayMax:      checks.DefaultGrayMax,
		GrayFieldN:   checks.DefaultGrayFieldSteps,
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

// ConfigPath 回傳設定檔路徑：exe 旁邊。macOS 的 .app 裡不應寫入檔案（放在「應用程式」資料夾時也沒有權限），
// 所以在 .app 裡執行時改用 ~/Library/Application Support/go-screen-burn-in-fixer/。
func ConfigPath() string {
	const name = "go-screen-burn-in-fixer.json"
	exe, err := os.Executable()
	if err != nil {
		return name
	}
	if strings.Contains(filepath.ToSlash(exe), ".app/Contents/MacOS/") {
		if dir, err := os.UserConfigDir(); err == nil {
			dir = filepath.Join(dir, "go-screen-burn-in-fixer")
			if err := os.MkdirAll(dir, 0o755); err == nil {
				return filepath.Join(dir, name)
			}
		}
	}
	return filepath.Join(filepath.Dir(exe), name)
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
	c.GrayMin, c.GrayMax, c.GrayFieldN = checks.ClampGrayField(c.GrayMin, c.GrayMax, c.GrayFieldN)
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
	if c.TimerMinutes <= 0 || c.TimerMinutes > MaxTimerMinutes {
		c.TimerMinutes = def.TimerMinutes
	}
	c.ShuffleSlice = nearest(shuffleSliceChoices, c.ShuffleSlice)
	if c.LegacyMonitor > 0 && (len(c.Monitors) == 0 || len(c.Monitors) == 1 && c.Monitors[0] == 0) {
		c.Monitors = []int{c.LegacyMonitor}
	}
	c.LegacyMonitor = 0
	var mons []int
	for _, m := range c.Monitors {
		if m >= 0 && !contains(mons, m) {
			mons = append(mons, m)
		}
	}
	if len(mons) == 0 {
		mons = []int{0}
	}
	c.Monitors = mons

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

// MarkCount 回傳所有螢幕的標記總數。
func (c Config) MarkCount() int {
	n := len(c.LegacyMarks)
	for _, ms := range c.MonitorMarks {
		n += len(ms)
	}
	return n
}

// ClearMarks 清除所有螢幕的標記。
func (c *Config) ClearMarks() {
	c.MonitorMarks, c.LegacyMarks = nil, nil
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

// applyChecks 把灰階分階的階數與灰階全畫面的範圍套用到檢查項目。
func (c Config) applyChecks() {
	for _, ck := range checks.All() {
		switch ck := ck.(type) {
		case *checks.GraySteps:
			ck.Levels = c.GrayLevels
		case *checks.GrayField:
			ck.Min, ck.Max, ck.N = checks.ClampGrayField(c.GrayMin, c.GrayMax, c.GrayFieldN)
		}
	}
}

// stepCount 回傳檢查項目的階數；灰階全畫面是依設定算出的灰階階數。
func (c Config) stepCount(ck checks.Check) int {
	if _, ok := ck.(*checks.GrayField); ok {
		g := checks.GrayField{}
		g.Min, g.Max, g.N = checks.ClampGrayField(c.GrayMin, c.GrayMax, c.GrayFieldN)
		return len(g.Levels())
	}
	return len(ck.Steps())
}

// CheckSequence 依勾選的項目展開成依序顯示的每一階，並套用灰階分階與灰階全畫面的設定。
func (c Config) CheckSequence() []CheckStep {
	c.applyChecks()
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

// CheckSpec 回傳勾選的檢查項目，格式與 -check 參數相同（例如 "gray-field:10,white:manual"）。
func (c Config) CheckSpec() string {
	var parts []string
	for _, it := range c.CheckItems {
		if !it.Enabled {
			continue
		}
		sec := "manual"
		if it.Seconds != Manual {
			sec = strconv.Itoa(it.Seconds)
		}
		parts = append(parts, it.Check+":"+sec)
	}
	return strings.Join(parts, ",")
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
