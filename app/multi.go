package app

// 多螢幕播放。
//
// Ebitengine 一個程式只能開一個視窗，所以主程式在第一個選定的螢幕上播放，並替其他每個選定的螢幕
// 各啟動一個子程式（同一個 exe 加上 -child 參數），子程式在自己的螢幕上全螢幕播放。
// 兩邊用子程式的標準輸入／輸出，以一行一個指令的文字溝通：
//
//	主程式 → 子程式：pos <節目時間 ns> <0|1 暫停>   Burn-In 播放位置
//	               step <階> <已顯示 ns>          檢查畫面目前的階
//	               gray <0–255>                  灰階全畫面切換成手動後的灰階
//	               saver                         切換成螢幕保護畫面
//	               blank                         顯示全黑（關機 / 睡眠倒數中）
//	               quit                          結束
//	子程式 → 主程式：esc、pause、next、prev        使用者在子程式的螢幕上按的鍵
//	               gray <0–255>                  在子程式的螢幕上用右鍵拖曳調整的灰階
//	               marks <x,y;x,y;…>            檢查畫面的標記（整份）
//
// 計時、換階與後續動作都由主程式決定，子程式只負責顯示並回報按鍵，所以所有螢幕一起前進、一起結束。
// 主程式結束時子程式的標準輸入會關閉，子程式也跟著結束。

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"gitea.littleherry.work/The78estWorld/go-screen-burn-in-fixer/program"
)

// 主程式定期送出播放位置的間隔：補上子程式啟動的延遲，並避免時間慢慢偏移。
const syncInterval = 2 * time.Second

// remoteScene 是可以接收另一個程式送來的指令的場景。
type remoteScene interface {
	remote(a *App, cmd string, args []string) error
}

// ── 主程式端 ─────────────────────────────────────────────

type childProc struct {
	monitor int
	key     string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
}

type childMsg struct {
	from *childProc
	line string
}

type hub struct {
	children []*childProc
	msgs     chan childMsg
	lastSync time.Time
}

// startChildren 替 cfg.Monitors 中第一個以外的每個螢幕啟動子程式。已經在播放（例如從 Burn-In
// 切換成螢幕保護）時不重新啟動。
func (a *App) startChildren(mode Mode, base program.Program, timer time.Duration) {
	if a.opts.Child || a.hub != nil || len(a.cfg.Monitors) < 2 {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		log.Printf("無法啟動其他螢幕的播放: %v", err)
		return
	}
	h := &hub{msgs: make(chan childMsg, 64), lastSync: time.Now()}
	for _, m := range a.cfg.Monitors[1:] {
		c := &childProc{monitor: m}
		if m < len(a.monitors) {
			c.key = a.monitors[m].key
		}
		c.cmd = exec.Command(exe, a.childArgs(mode, m, base, timer)...)
		c.stdin, err = c.cmd.StdinPipe()
		if err != nil {
			log.Printf("螢幕 %d：%v", m+1, err)
			continue
		}
		stdout, err := c.cmd.StdoutPipe()
		if err != nil {
			log.Printf("螢幕 %d：%v", m+1, err)
			continue
		}
		if err := c.cmd.Start(); err != nil {
			log.Printf("螢幕 %d：無法啟動子程式: %v", m+1, err)
			continue
		}
		log.Printf("螢幕 %d：子程式 pid %d", m+1, c.cmd.Process.Pid)
		go func() {
			sc := bufio.NewScanner(stdout)
			sc.Buffer(nil, 1<<20)
			for sc.Scan() {
				h.msgs <- childMsg{c, sc.Text()}
			}
		}()
		h.children = append(h.children, c)
	}
	if len(h.children) > 0 {
		a.hub = h
	}
}

// childArgs 回傳子程式的參數：播放內容全部以參數傳入，不依賴設定檔（帶參數執行時設定檔可能與這次不同）。
func (a *App) childArgs(mode Mode, monitor int, base program.Program, timer time.Duration) []string {
	args := []string{"-child", string(mode), "-monitor", strconv.Itoa(monitor + 1),
		"-screensaver", a.cfg.Screensaver, "-info-bar", strconv.Itoa(a.cfg.InfoBar)}
	if a.opts.Windowed {
		args = append(args, "-windowed")
	}
	switch mode {
	case ModeBurnIn:
		args = append(args, "-program", base.String(), "-timer", timer.String())
		if a.cfg.Shuffle {
			args = append(args, "-shuffle", "-shuffle-slice", strconv.Itoa(a.cfg.ShuffleSlice),
				"-seed", strconv.FormatUint(a.seed, 10))
		}
	case ModeCheck:
		args = append(args, "-check", a.cfg.CheckSpec(), "-gray-levels", strconv.Itoa(a.cfg.GrayLevels),
			"-gray-min", strconv.Itoa(a.cfg.GrayMin), "-gray-max", strconv.Itoa(a.cfg.GrayMax),
			"-gray-field-steps", strconv.Itoa(a.cfg.GrayFieldN))
	}
	return args
}

// broadcast 把指令送給所有子程式。
func (a *App) broadcast(format string, v ...any) {
	if a.hub == nil {
		return
	}
	line := fmt.Sprintf(format, v...) + "\n"
	for _, c := range a.hub.children {
		io.WriteString(c.stdin, line) // 子程式已結束時寫入失敗，不影響其他螢幕
	}
}

// stopChildren 結束所有子程式。
func (a *App) stopChildren() {
	if a.hub == nil {
		return
	}
	a.broadcast("quit")
	for _, c := range a.hub.children {
		c.stdin.Close()
		go func() {
			done := make(chan struct{})
			go func() { c.cmd.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				c.cmd.Process.Kill()
			}
		}()
	}
	a.hub = nil
}

// Close 在程式結束時呼叫：結束所有子程式。
func (a *App) Close() { a.stopChildren() }

// pollChildren 處理子程式送來的訊息（每一格呼叫一次，不會等待）。
func (a *App) pollChildren() error {
	h := a.hub
	if h == nil {
		return nil
	}
	for {
		select {
		case m := <-h.msgs:
			cmd, rest, _ := strings.Cut(m.line, " ")
			if cmd == "marks" {
				a.setMarksFor(m.from.key, parseMarks(rest))
				continue
			}
			if rs, ok := a.scene.(remoteScene); ok {
				if err := rs.remote(a, cmd, strings.Fields(rest)); err != nil {
					return err
				}
			}
			if a.hub != h { // 已離開播放（例如按了 Esc）
				return nil
			}
		default:
			if time.Since(h.lastSync) >= syncInterval {
				h.lastSync = time.Now()
				if sy, ok := a.scene.(interface{ sync(*App) }); ok {
					sy.sync(a)
				}
			}
			return nil
		}
	}
}

// ── 子程式端 ─────────────────────────────────────────────

type upLink struct {
	msgs chan string // 主程式結束（標準輸入關閉）時關閉
}

func newUpLink() *upLink {
	u := &upLink{msgs: make(chan string, 64)}
	go func() {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			u.msgs <- sc.Text()
		}
		close(u.msgs)
	}()
	return u
}

// send 把訊息送給主程式。
func (u *upLink) send(format string, v ...any) {
	fmt.Fprintf(os.Stdout, format+"\n", v...)
}

// pollParent 處理主程式送來的指令（每一格呼叫一次，不會等待）。
func (a *App) pollParent() error {
	for {
		select {
		case line, ok := <-a.up.msgs:
			if !ok {
				return ebiten.Termination
			}
			cmd, rest, _ := strings.Cut(line, " ")
			switch cmd {
			case "quit":
				return ebiten.Termination
			case "saver":
				a.goSaver()
			case "blank":
				a.scene = &blankScene{}
			default:
				if rs, ok := a.scene.(remoteScene); ok {
					if err := rs.remote(a, cmd, strings.Fields(rest)); err != nil {
						return err
					}
				}
			}
		default:
			return nil
		}
	}
}

// blankScene 是子程式在主程式倒數關機 / 睡眠時顯示的全黑畫面；按任意鍵等同在主程式取消。
type blankScene struct{ sent bool }

func (s *blankScene) Update(a *App) error {
	if anyInput() && !s.sent {
		a.up.send("esc")
		s.sent = true
	}
	return nil
}

func (s *blankScene) Draw(a *App, screen *ebiten.Image) {}

// ── 標記 ─────────────────────────────────────────────────

func (a *App) marks() []Mark { return a.cfg.MonitorMarks[a.playKey()] }

func (a *App) setMarks(ms []Mark) {
	a.setMarksFor(a.playKey(), ms)
	if a.up != nil {
		a.up.send("marks %s", formatMarks(ms))
	}
}

func (a *App) setMarksFor(key string, ms []Mark) {
	if a.cfg.MonitorMarks == nil {
		a.cfg.MonitorMarks = map[string][]Mark{}
	}
	if len(ms) == 0 {
		delete(a.cfg.MonitorMarks, key)
		return
	}
	a.cfg.MonitorMarks[key] = ms
}

// playKey 回傳這個程式播放的螢幕的識別字串（標記依此分開儲存）。
func (a *App) playKey() string {
	if a.playMon < len(a.monitors) {
		return a.monitors[a.playMon].key
	}
	return monitorKey(a.playMon, "")
}

func formatMarks(ms []Mark) string {
	parts := make([]string, len(ms))
	for i, m := range ms {
		parts[i] = fmt.Sprintf("%d,%d", m.X, m.Y)
	}
	return strings.Join(parts, ";")
}

func parseMarks(s string) []Mark {
	var ms []Mark
	for _, p := range strings.Split(s, ";") {
		var m Mark
		if _, err := fmt.Sscanf(p, "%d,%d", &m.X, &m.Y); err == nil {
			ms = append(ms, m)
		}
	}
	return ms
}
