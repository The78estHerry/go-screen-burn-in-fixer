// go-screen-burn-in-fixer：即時全螢幕播放螢幕修復畫面（burn-in fix）與螢幕保護畫面。
//
// 不帶參數執行時顯示警告畫面與設定選單；帶播放相關參數時跳過選單直接播放。
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/actions"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/app"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/checks"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/display"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/patterns"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/program"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/screensaver"
)

// 這些參數代表「直接播放」：有任何一個時跳過警告畫面與設定選單。
var playFlags = map[string]bool{"program": true, "timer": true, "after": true, "screensaver": true, "monitor": true,
	"check": true, "shuffle": true, "shuffle-slice": true, "child": true}

func main() {
	progFlag := flag.String("program", "", `節目表，例如 "gray-noise:600,vstripes:300"（秒數）`)
	timerFlag := flag.Duration("timer", 0, "播放時間，例如 30m、1h（預設：-program 的一輪長度，或選單設定）")
	afterFlag := flag.String("after", "", "時間到之後的動作：quit、shutdown、sleep、screensaver")
	saverFlag := flag.String("screensaver", "", "螢幕保護畫面名稱")
	monitorFlag := flag.String("monitor", "1", `使用的螢幕（從 1 開始），多個螢幕用逗號分隔，例如 "1,2"；all 代表全部`)
	listFlag := flag.Bool("list", false, "列出所有圖樣與螢幕保護畫面")
	listMonitorsFlag := flag.Bool("list-monitors", false, "列出螢幕")
	windowedFlag := flag.Bool("windowed", false, "以視窗模式播放（開發測試用）")
	dryRunFlag := flag.Bool("dry-run", false, "測試用：時間到時不真的關機或睡眠")
	countdownFlag := flag.Duration("countdown", 60*time.Second, "關機 / 睡眠前的倒數時間")
	snapshotFlag := flag.String("snapshot", "", "開發用：把各個畫面存成 PNG 到指定資料夾後結束")
	checkFlag := flag.String("check", "", `直接開始檢查，例如 "gray-field:10,gray-steps:manual,white"（每階秒數或 manual；gray-field 是整個漸進的秒數；省略時用選單設定）`)
	grayLevelsFlag := flag.Int("gray-levels", 0, "灰階分階的階數（2–256；預設用選單設定）")
	grayMinFlag := flag.Int("gray-min", -1, "灰階全畫面的最暗值（0–255；預設用選單設定）")
	grayMaxFlag := flag.Int("gray-max", -1, "灰階全畫面的最亮值（0–255；預設用選單設定）")
	grayFieldStepsFlag := flag.Int("gray-field-steps", 0, "灰階全畫面手動時的階數（4–15；預設用選單設定）")
	shuffleFlag := flag.Bool("shuffle", false, "Burn-In 圖樣洗牌播放：每個圖樣切成小段打散，總時間不變")
	sliceFlag := flag.Int("shuffle-slice", -1, "洗牌時每段的秒數；0 代表隨機 15–60 秒（預設用選單設定）")
	seedFlag := flag.Uint64("seed", 0, "洗牌的亂數種子（預設每次隨機）")
	childFlag := flag.String("child", "", "內部使用：多螢幕播放的子程式（burnin、check、screensaver）")
	infoBarFlag := flag.Int("info-bar", -2, "內部使用：資訊列設定")
	flag.CommandLine.SetOutput(os.Stderr)
	flag.Usage = func() {
		attachConsole()
		fmt.Fprintln(os.Stderr, "用法：go-screen-burn-in-fixer [參數]")
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
				fatal(fmt.Sprintf("沒有名為 %q 的螢幕保護畫面", *saverFlag))
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
			fatal(fmt.Sprintf("-child 的值無效：%q", *childFlag))
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
			return nil, fmt.Errorf("-monitor 的值無效：%q（例如 1、1,2 或 all）", s)
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
			return fmt.Errorf("沒有名為 %q 的檢查項目", name)
		}
		it.Enabled = true
		switch {
		case !hasSecs:
		case secs == "manual":
			it.Seconds = app.Manual
		default:
			n, err := strconv.Atoi(secs)
			if err != nil || n <= 0 {
				return fmt.Errorf("檢查項目 %s 的秒數無效：%q（可用正整數或 manual）", name, secs)
			}
			it.Seconds = n
		}
		items = append(items, it)
	}
	if len(items) == 0 {
		return fmt.Errorf("-check 沒有指定任何檢查項目")
	}
	cfg.CheckItems = items
	return nil
}

func printList() {
	fmt.Println("Burn-In 圖樣：")
	for _, p := range patterns.All() {
		fmt.Printf("  %-15s %s\n", p.Name(), p.Description())
	}
	fmt.Println("螢幕保護畫面：")
	for _, s := range screensaver.All() {
		fmt.Printf("  %-15s %s\n", s.Name(), s.Description())
	}
	fmt.Println("檢查項目：")
	for _, c := range checks.All() {
		fmt.Printf("  %-15s %s（%d 階）\n", c.Name(), c.Description(), len(c.Steps()))
	}
}

func printMonitors() {
	ds, err := display.Displays()
	if err != nil {
		fmt.Println("無法取得螢幕資訊:", err)
		return
	}
	for i, d := range ds {
		fmt.Printf("%d：%s\n", i+1, d.Label())
		if d.EDID != nil {
			fmt.Printf("   EDID：廠商 %s，原生 %s\n", d.EDID.Manufacturer, d.EDID.Native)
		}
	}
}

// fatal 顯示錯誤並結束。以 GUI 程式執行（沒有主控台）時改用訊息視窗顯示。
func fatal(msg string) {
	if attachConsole() {
		fmt.Fprintln(os.Stderr, "錯誤：", msg)
	} else {
		messageBox("go-screen-burn-in-fixer", msg)
	}
	os.Exit(1)
}
