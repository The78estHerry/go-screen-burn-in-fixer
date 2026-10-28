// go-screen-burn-in-fixer：即時全螢幕播放螢幕修復畫面（burn-in fix）與螢幕保護畫面。
//
// 不帶參數執行時顯示警告畫面與設定選單；帶播放相關參數時跳過選單直接播放。
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/actions"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/app"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/display"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/patterns"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/program"
	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/screensaver"
)

// 這些參數代表「直接播放」：有任何一個時跳過警告畫面與設定選單。
var playFlags = map[string]bool{"program": true, "timer": true, "after": true, "screensaver": true, "monitor": true}

func main() {
	progFlag := flag.String("program", "", `節目表，例如 "gray-noise:600,vstripes:300"（秒數）`)
	timerFlag := flag.Duration("timer", 0, "播放時間，例如 30m、1h（預設：-program 的一輪長度，或選單設定）")
	afterFlag := flag.String("after", "", "時間到之後的動作：quit、shutdown、sleep、screensaver")
	saverFlag := flag.String("screensaver", "", "螢幕保護畫面名稱")
	monitorFlag := flag.Int("monitor", 1, "使用第幾個螢幕（從 1 開始）")
	listFlag := flag.Bool("list", false, "列出所有圖樣與螢幕保護畫面")
	listMonitorsFlag := flag.Bool("list-monitors", false, "列出螢幕")
	windowedFlag := flag.Bool("windowed", false, "以視窗模式播放（開發測試用）")
	dryRunFlag := flag.Bool("dry-run", false, "測試用：時間到時不真的關機或睡眠")
	countdownFlag := flag.Duration("countdown", 60*time.Second, "關機 / 睡眠前的倒數時間")
	snapshotFlag := flag.String("snapshot", "", "開發用：把各個畫面存成 PNG 到指定資料夾後結束")
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
		cfg.Monitor = *monitorFlag - 1
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
	if err := ebiten.RunGame(a); err != nil {
		fatal(err.Error())
	}
	actions.KeepAwake(false)
	if !opts.SkipMenu {
		if err := a.Config().Save(cfgPath); err != nil {
			log.Printf("無法儲存設定: %v", err)
		}
	}
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
