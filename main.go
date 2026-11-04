// go-screen-burn-in-fixer：即時全螢幕播放螢幕修復畫面（burn-in fix）與螢幕保護畫面。
//
// 不帶參數執行時顯示警告畫面與設定選單；帶播放相關參數時跳過選單直接播放。
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/The78estHerry/go-screen-burn-in-fixer/actions"
	"github.com/The78estHerry/go-screen-burn-in-fixer/app"
	"github.com/The78estHerry/go-screen-burn-in-fixer/checks"
	"github.com/The78estHerry/go-screen-burn-in-fixer/display"
	"github.com/The78estHerry/go-screen-burn-in-fixer/internal/i18n"
	"github.com/The78estHerry/go-screen-burn-in-fixer/patterns"
	"github.com/The78estHerry/go-screen-burn-in-fixer/program"
	"github.com/The78estHerry/go-screen-burn-in-fixer/screensaver"
)

// 這些參數代表「直接播放」：有任何一個時跳過警告畫面與設定選單。
var playFlags = map[string]bool{"program": true, "timer": true, "after": true, "screensaver": true, "monitor": true,
	"check": true, "shuffle": true, "shuffle-slice": true, "child": true}

// initLang 在定義參數之前決定介面語言（參數說明也要翻譯）：
// 命令列的 -lang 優先，其次是設定檔，都沒有時用預設（英文）。
func initLang() {
	args := os.Args[1:]
	for i, arg := range args {
		name, val, hasVal := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if !strings.HasPrefix(arg, "-") || name != "lang" {
			continue
		}
		if !hasVal && i+1 < len(args) {
			val = args[i+1]
		}
		if l, ok := i18n.Parse(val); ok {
			i18n.Set(l)
			return
		}
	}
	cfg, _ := app.LoadConfig(app.ConfigPath())
	l, _ := i18n.Parse(cfg.Lang)
	i18n.Set(l)
}

func main() {
	initLang()
	langFlag := flag.String("lang", "", i18n.T("介面語言：en（英文）或 zh（繁體中文）；預設用選單設定", "interface language: en (English) or zh (Traditional Chinese); default: menu setting"))
	progFlag := flag.String("program", "", i18n.T(`節目表，例如 "gray-noise:600,vstripes:300"（秒數）`, `program list, e.g. "gray-noise:600,vstripes:300" (seconds)`))
	timerFlag := flag.Duration("timer", 0, i18n.T("播放時間，例如 30m、1h（預設：-program 的一輪長度，或選單設定）", "play time, e.g. 30m, 1h (default: one round of -program, or the menu setting)"))
	afterFlag := flag.String("after", "", i18n.T("時間到之後的動作：quit、shutdown、sleep、screensaver", "action when time is up: quit, shutdown, sleep, screensaver"))
	saverFlag := flag.String("screensaver", "", i18n.T("螢幕保護畫面名稱", "screensaver name"))
	monitorFlag := flag.String("monitor", "1", i18n.T(`使用的螢幕（從 1 開始），多個螢幕用逗號分隔，例如 "1,2"；all 代表全部`, `monitors to use (from 1), comma-separated, e.g. "1,2"; all for every monitor`))
	listFlag := flag.Bool("list", false, i18n.T("列出所有圖樣與螢幕保護畫面", "list all patterns, screensavers and test items"))
	listMonitorsFlag := flag.Bool("list-monitors", false, i18n.T("列出螢幕", "list monitors"))
	windowedFlag := flag.Bool("windowed", false, i18n.T("以視窗模式播放（開發測試用）", "play in a window (for development)"))
	dryRunFlag := flag.Bool("dry-run", false, i18n.T("測試用：時間到時不真的關機或睡眠", "for testing: do not actually shut down or sleep"))
	countdownFlag := flag.Duration("countdown", 60*time.Second, i18n.T("關機 / 睡眠前的倒數時間", "countdown before shutdown / sleep"))
	snapshotFlag := flag.String("snapshot", "", i18n.T("開發用：把各個畫面存成 PNG 到指定資料夾後結束", "for development: save each screen as PNG to the folder and exit"))
	checkFlag := flag.String("check", "", i18n.T(`直接開始檢查，例如 "gray-field:10,gray-steps:manual,white"（每階秒數或 manual；省略時用選單設定）`, `start testing right away, e.g. "gray-field:10,gray-steps:manual,white" (seconds per step or manual; default: menu setting)`))
	grayLevelsFlag := flag.Int("gray-levels", 0, i18n.T("灰階分階的階數（2–256；預設用選單設定）", "number of gray steps (2–256; default: menu setting)"))
	grayMinFlag := flag.Int("gray-min", -1, i18n.T("灰階全畫面的最暗值（0–255；預設用選單設定）", "darkest value of gray-field (0–255; default: menu setting)"))
	grayMaxFlag := flag.Int("gray-max", -1, i18n.T("灰階全畫面的最亮值（0–255；預設用選單設定）", "brightest value of gray-field (0–255; default: menu setting)"))
	grayFieldStepsFlag := flag.Int("gray-field-steps", 0, i18n.T("灰階全畫面的階數（4–15；預設用選單設定）", "number of gray-field steps (4–15; default: menu setting)"))
	shuffleFlag := flag.Bool("shuffle", false, i18n.T("Burn-In 圖樣洗牌播放：每個圖樣切成小段打散，總時間不變", "shuffle burn-in patterns: cut each pattern into slices and interleave them; total time unchanged"))
	sliceFlag := flag.Int("shuffle-slice", -1, i18n.T("洗牌時每段的秒數；0 代表隨機 15–60 秒（預設用選單設定）", "seconds per slice when shuffling; 0 means random 15–60 s (default: menu setting)"))
	seedFlag := flag.Uint64("seed", 0, i18n.T("洗牌的亂數種子（預設每次隨機）", "random seed for shuffling (default: random each time)"))
	childFlag := flag.String("child", "", i18n.T("內部使用：多螢幕播放的子程式（burnin、check、screensaver）", "internal: child process for multi-monitor playback (burnin, check, screensaver)"))
	infoBarFlag := flag.Int("info-bar", -2, i18n.T("內部使用：資訊列設定", "internal: info bar setting"))
	flag.CommandLine.SetOutput(os.Stderr)
	flag.Usage = func() {
		attachConsole()
		fmt.Fprintln(os.Stderr, i18n.T("用法：go-screen-burn-in-fixer [參數]", "Usage: go-screen-burn-in-fixer [options]"))
		flag.PrintDefaults()
	}
	flag.Parse()

	if *listFlag || *listMonitorsFlag {
		attachConsole()
		if *listFlag {
			printList()
		}
		if *listMonitorsFlag {
			printMonitors()
		}
		return
	}

	cfgPath := app.ConfigPath()
	cfg, err := app.LoadConfig(cfgPath)
	if err != nil {
		log.Printf("設定檔格式錯誤，使用預設設定: %v", err)
	}
	if *langFlag != "" {
		l, ok := i18n.Parse(*langFlag)
		if !ok {
			fatal(fmt.Sprintf(i18n.T("-lang 的值無效：%q（可用 en 或 zh）", "invalid -lang value: %q (use en or zh)"), *langFlag))
		}
		cfg.Lang = string(l)
	}
	l, _ := i18n.Parse(cfg.Lang)
	i18n.Set(l)
	opts := app.Options{
		ConfigPath: cfgPath,
		Windowed:   *windowedFlag,
		DryRun:     *dryRunFlag,
		Countdown:  *countdownFlag,
		Seed:       *seedFlag,
		Child:      *childFlag != "",
	}

	flag.Visit(func(f *flag.Flag) {
		if playFlags[f.Name] {
			opts.SkipMenu = true
		}
	})
	if opts.SkipMenu {
		// 直接播放：參數只用於這次執行，不會寫回設定檔。
		opts.Program = cfg.Program()
		if *progFlag != "" {
			if opts.Program, err = program.Parse(*progFlag); err != nil {
				fatal(err.Error())
			}
		}
		if len(opts.Program) == 0 {
			opts.Program = program.Default()
		}
		switch {
		case *timerFlag > 0:
			opts.Timer = *timerFlag
		case *progFlag != "":
			opts.Timer = opts.Program.Total()
		default:
			opts.Timer = cfg.Timer()
		}
		if *afterFlag != "" {
			if cfg.After, err = actions.ParseAfter(*afterFlag); err != nil {
				fatal(err.Error())
			}
		}
		if *saverFlag != "" {
			if _, ok := screensaver.Get(*saverFlag); !ok {
				fatal(fmt.Sprintf(i18n.T("沒有名為 %q 的螢幕保護畫面", "no screensaver named %q"), *saverFlag))
			}
			cfg.Screensaver = *saverFlag
		}
		cfg.Mode = app.ModeBurnIn
		if cfg.Monitors, err = parseMonitors(*monitorFlag); err != nil {
			fatal(err.Error())
		}
		if *shuffleFlag {
			cfg.Shuffle = true
		}
		if *sliceFlag >= 0 {
			cfg.Shuffle = true
			cfg.ShuffleSlice = *sliceFlag
		}
		if *infoBarFlag != -2 {
			cfg.InfoBar = *infoBarFlag
		}
		if *checkFlag != "" {
			if err := applyCheckFlag(&cfg, *checkFlag); err != nil {
				fatal(err.Error())
			}
			cfg.Mode = app.ModeCheck
		}
		if *grayLevelsFlag != 0 {
			cfg.GrayLevels = checks.ClampGrayLevels(*grayLevelsFlag)
		}
		if *grayMinFlag >= 0 {
			cfg.GrayMin = *grayMinFlag
		}
		if *grayMaxFlag >= 0 {
			cfg.GrayMax = *grayMaxFlag
		}
		if *grayFieldStepsFlag != 0 {
			cfg.GrayFieldN = *grayFieldStepsFlag
		}
		cfg.GrayMin, cfg.GrayMax, cfg.GrayFieldN = checks.ClampGrayField(cfg.GrayMin, cfg.GrayMax, cfg.GrayFieldN)
		switch *childFlag {
		case "":
		case string(app.ModeBurnIn), string(app.ModeCheck), string(app.ModeScreensaver):
			cfg.Mode = app.Mode(*childFlag)
		default:
			fatal(fmt.Sprintf(i18n.T("-child 的值無效：%q", "invalid -child value: %q"), *childFlag))
		}
	}
	opts.Config = cfg

	if *snapshotFlag != "" {
		attachConsole()
		ebiten.SetWindowSize(320, 240)
		if err := app.Snapshot(opts, *snapshotFlag); err != nil {
			fatal(err.Error())
		}
		return
	}

	ebiten.SetWindowTitle("go-screen-burn-in-fixer")
	ebiten.SetWindowSize(app.WindowWidth, app.WindowHeight)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(ebiten.SyncWithFPS) // 每次畫面更新都換一格雜訊，配合螢幕的更新率
	a := app.New(opts)
	err = ebiten.RunGame(a)
	a.Close()
	if err != nil {
		fatal(err.Error())
	}
	actions.KeepAwake(false)
	if !opts.SkipMenu {
		if err := a.Config().Save(cfgPath); err != nil {
			log.Printf("無法儲存設定: %v", err)
		}
	}
}

// parseMonitors 解析 -monitor 參數："1"、"1,2"、"all"（從 1 開始），回傳從 0 開始的編號。
func parseMonitors(s string) ([]int, error) {
	if strings.EqualFold(strings.TrimSpace(s), "all") {
		var ms []int
		for i := range ebiten.AppendMonitors(nil) {
			ms = append(ms, i)
		}
		return ms, nil
	}
	var ms []int
	for _, part := range strings.Split(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n < 1 {
			return nil, fmt.Errorf(i18n.T("-monitor 的值無效：%q（例如 1、1,2 或 all）", "invalid -monitor value: %q (e.g. 1, 1,2 or all)"), s)
		}
		ms = append(ms, n-1)
	}
	return ms, nil
}

// applyCheckFlag 依 -check 參數設定要進行的檢查項目與秒數（依參數中的順序）。
func applyCheckFlag(cfg *app.Config, s string) error {
	byName := map[string]app.CheckItemConfig{}
	for _, it := range cfg.CheckItems {
		byName[it.Check] = it
	}
	var items []app.CheckItemConfig
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, secs, hasSecs := strings.Cut(part, ":")
		it, ok := byName[name]
		if !ok {
			return fmt.Errorf(i18n.T("沒有名為 %q 的檢查項目", "no test item named %q"), name)
		}
		it.Enabled = true
		switch {
		case !hasSecs:
		case secs == "manual":
			it.Seconds = app.Manual
		default:
			n, err := strconv.Atoi(secs)
			if err != nil || n <= 0 {
				return fmt.Errorf(i18n.T("檢查項目 %s 的秒數無效：%q（可用正整數或 manual）", "invalid seconds for test item %s: %q (use a positive integer or manual)"), name, secs)
			}
			it.Seconds = n
		}
		items = append(items, it)
	}
	if len(items) == 0 {
		return errors.New(i18n.T("-check 沒有指定任何檢查項目", "-check does not name any test item"))
	}
	cfg.CheckItems = items
	return nil
}

func printList() {
	fmt.Println(i18n.T("Burn-In 圖樣：", "Burn-in patterns:"))
	for _, p := range patterns.All() {
		fmt.Printf("  %-15s %s\n", p.Name(), p.Description())
	}
	fmt.Println(i18n.T("螢幕保護畫面：", "Screensavers:"))
	for _, s := range screensaver.All() {
		fmt.Printf("  %-15s %s\n", s.Name(), s.Description())
	}
	fmt.Println(i18n.T("檢查項目：", "Test items:"))
	for _, c := range checks.All() {
		fmt.Printf(i18n.T("  %-15s %s（%d 階）\n", "  %-15s %s (%d steps)\n"), c.Name(), c.Description(), len(c.Steps()))
	}
}

func printMonitors() {
	ds, err := display.Displays()
	if err != nil {
		fmt.Println(i18n.T("無法取得螢幕資訊:", "Cannot get monitor information:"), err)
		return
	}
	for i, d := range ds {
		fmt.Printf(i18n.T("%d：%s\n", "%d: %s\n"), i+1, d.Label())
		if d.EDID != nil {
			fmt.Printf(i18n.T("   EDID：廠商 %s，原生 %s\n", "   EDID: manufacturer %s, native %s\n"), d.EDID.Manufacturer, d.EDID.Native)
		}
	}
}

// fatal 顯示錯誤並結束。以 GUI 程式執行（沒有主控台）時改用訊息視窗顯示。
func fatal(msg string) {
	if attachConsole() {
		fmt.Fprintln(os.Stderr, i18n.T("錯誤：", "Error:"), msg)
	} else {
		messageBox("go-screen-burn-in-fixer", msg)
	}
	os.Exit(1)
}
