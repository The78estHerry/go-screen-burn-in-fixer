<p align="center"><img src="winres/icon.png" width="160" alt="go-screen-burn-in-fixer icon: a monitor showing colored test bars"></p>

# go-screen-burn-in-fixer

[繁體中文](README.md) | **English** | [日本語](README.ja.md)

> The Traditional Chinese README is the primary version. If this translation differs from it, the Chinese version is correct.

[![PayPal](https://img.shields.io/badge/PayPal-paypal.me%2FThe78ester-00457C?logo=paypal&logoColor=white)](https://paypal.me/The78ester)

💖 If this tool helps you, please consider supporting development via [PayPal](https://paypal.me/The78ester).

A small tool that plays "screen repair" patterns full screen in real time: fast-flashing noise, colored stripes, solid color cycles and moving bars, so that every red, green and blue subpixel on the panel is switched between fully on and fully off again and again. When playback ends it can quit, shut down, sleep, or switch to a slow, dark screensaver.

It also includes **test screens**: gray levels, single colors, near-black, gradients, full black / white and an image retention test, to find burn-in, temporary image retention, dead pixels and uniformity problems.

- **Single executable**: nothing to install, just double-click. Windows, macOS (Apple Silicon / Intel) and Linux x64 such as Ubuntu.
- **Generated in real time**: computed by the GPU, sharp at any resolution and refresh rate. No videos to download or convert.
- **Matches your monitor**: uses the current resolution by default and can read the EDID to show the native resolution.
- **Multiple monitors**: play or test on one, several or all monitors at the same time.

## ⚠️ Warning

- **This program contains rapidly flashing images. Do not watch it if you have photosensitive epilepsy or are sensitive to flashing lights.** Avoid staring at the screen during playback.
- **Real OLED burn-in is permanent panel aging and cannot be fixed by any image.** This tool may help reduce **temporary image retention** or wake up **stuck pixels**, but there is no guarantee.
- Playing for a long time at maximum brightness puts extra load on the panel. Choose the playback time with care.
- **Use at your own risk.** This tool is provided "as is" with no guarantee that it works. The author is not responsible for anything that happens after using it, including damage to your monitor or panel, worse image retention or burn-in, changes in brightness or color, data loss caused by the automatic shutdown, or physical discomfort. Read your monitor manufacturer's instructions first (many OLED monitors have a built-in pixel refresh feature, which manufacturers usually recommend using first).

## Screens

A 3-second preview video of every screen is in [`previews/`](previews/README.md).

### Burn-in patterns

| Name | Image |
|---|---|
| `gray-noise` | Gray noise regenerated every frame (1 pixel) |
| `rgb-noise` | Noise where every red, green and blue subpixel is randomly fully on or off |
| `vstripes` / `vstripes-fine` | Randomly colored vertical stripes (4 px / 1 px wide) |
| `hstripes` / `hstripes-fine` | Randomly colored horizontal stripes (4 px / 1 px wide) |
| `cross` / `cross-fine` | Vertical and horizontal stripes combined into a cross grid (4 px / 1 px wide) |
| `solid-cycle` | Full screen red → green → blue → white → black |
| `moving-bar` | A white bar on black, sweeping horizontally and vertically in turn |

### Test screens

The test screens contain **no text at all**; information is shown in an info bar that can be hidden.

| Name | Image | What it shows |
|---|---|---|
| `gray-field` | Full-screen gray, stepping from darkest to brightest (default 13–191 in 7 steps, about 5–75%) | Shadows of burn-in (most visible at 18–35%) |
| `gray-steps` | Gray steps, 2–256 steps | How many gray levels the monitor can distinguish, shadow detail |
| `color-field` | Red, green, blue at 25, 50, 100% | Uneven subpixel aging, which subpixel is faulty |
| `retention-test` | Black and white checkerboard for 10 s, then gray | How long temporary image retention takes to fade |
| `black` | Full black | Bright dots, stuck pixels |
| `white` | Full white | Dark dots, dead pixels, dust |
| `near-black` | Near black 1, 2, 3, 5% | Uneven dark areas (clouding, mura), bright corners |
| `gradient` | Gray and RGB gradients (horizontal / vertical) | Irregular banding or color shifts |

Percentages are fractions of the 8-bit signal value (e.g. 25% = 64/255, 50% ≈ 128), not the actual light output. Monitors have a gamma of about 2.2, so signal 128 emits only about 22% of full white; but human perception of brightness is also non-linear, so the signal fraction is closer to how bright it "looks".

Gray steps are limited to 256: the program outputs an 8-bit signal with 256 levels per color. Testing the shadows or peak brightness of 10-bit panels (1024 levels) or HDR requires a 10-bit or HDR10 signal and is not supported; on such monitors 256 steps can still reveal banding and crushed shadows.

#### Telling temporary image retention from permanent burn-in

1. Find the shadow with `gray-field` (around 25%, gray level 64) and click to mark it.
2. Play the burn-in patterns for 30–60 minutes, or let the monitor stand by (without unplugging it) so the panel runs its pixel refresh.
3. Check again: if the shadow is gone it was **temporary image retention**; if it remains after several refresh cycles it is most likely **permanent burn-in**.

Notes:
- The program outputs a normal SDR (8-bit) image and cannot send HDR high-brightness signals. With HDR enabled in Windows, the brightness depends on the "SDR content brightness" setting; use the same setting every time you test.
- The sides of a curved monitor (e.g. 1000R) are viewed at an angle and may naturally look darker or tinted. This is not burn-in. Sit at the center and turn to face any suspicious area directly.

### Screensavers

Dark, never flashing, always moving, never staying in one place for long.

| Name | Image |
|---|---|
| `drift-clock` | A clock drifting across the whole screen, slowly changing color |
| `aurora` | Flowing dark gradient bands |

## Usage

### Download

Download the file for your system from [Releases](https://github.com/The78estHerry/go-screen-burn-in-fixer/releases):

| System | File |
|---|---|
| Windows x64 | `go-screen-burn-in-fixer-<version>-windows-amd64.exe` |
| macOS (Apple Silicon, M1 or later) | `go-screen-burn-in-fixer-<version>-macos-apple-silicon.zip` |
| macOS (Intel) | `go-screen-burn-in-fixer-<version>-macos-intel.zip` |
| Linux x64 such as Ubuntu | `go-screen-burn-in-fixer-<version>-linux-amd64.tar.gz` |

**Windows**: if "Windows protected your PC" appears the first time, click "More info" → "Run anyway" (the program is not code-signed).

**macOS** (macOS 12 or later): unzip and drag `go-screen-burn-in-fixer.app` to Applications. The app has no Apple developer signature, so the first launch is blocked ("cannot be opened" or "is damaged"). Either:
- Open System Settings → Privacy & Security and click "Open Anyway" near the bottom.
- Or run in Terminal: `xattr -cr /Applications/go-screen-burn-in-fixer.app`

Settings are stored in `~/Library/Application Support/go-screen-burn-in-fixer/`. The first time you use shutdown, macOS asks whether to allow controlling "System Events"; click Allow.

**Linux**:

```
tar xzf go-screen-burn-in-fixer-<version>-linux-amd64.tar.gz
cd go-screen-burn-in-fixer-<version>-linux-amd64
./go-screen-burn-in-fixer
```

Requires a graphical desktop (X11, or XWayland under Wayland) and OpenGL, included in Ubuntu Desktop. During playback `systemd-inhibit` and `xdg-screensaver` keep the screen on; shutdown and sleep use `systemctl poweroff` and `systemctl suspend`.

**Limitations on macOS and Linux**: monitor names and the EDID native resolution are currently only available on Windows. On other systems the menu shows only the monitor number and resolution, and `-list-monitors` is not supported.

**macOS and Linux have not been tested on real hardware yet.** Reports are welcome.

### Double-click to run

1. Run `go-screen-burn-in-fixer.exe`.
2. Read the warning screen and press any key.
3. In the settings menu choose a mode: **Burn-in repair**, **Test screens** or **Screensaver only**, then tick the monitors and items to use. The item under the mouse is previewed live on the right.
4. Press **Start** / **Start test** or Enter to play full screen on the selected monitors. Settings are saved to `go-screen-burn-in-fixer.json` next to the exe and reused next time.

The settings menu can be used entirely from the keyboard: ↑↓ select, ←→ adjust, Space toggle, Enter start, Esc quit.

### Interface language

The interface is available in **English** and **Traditional Chinese**; English is the default. Click "English | 中文" in the top right of the first warning screen or of the settings menu to switch (on the warning screen ← → also work; in the menu press ↑ to move to the top, then ←→). All text changes immediately and the choice is saved in the settings file. On the command line, use `-lang en` or `-lang zh`.

### Multiple monitors

With more than one monitor, **Monitors** in the settings menu has a checkbox for each monitor; every ticked monitor plays full screen at the same time (←→ select a monitor, Space toggle).

- All monitors share the same timer and end together; pressing Esc on any monitor stops all of them.
- Pause, pattern changes and test steps are synchronized across all monitors.
- Shutdown or sleep at the end counts down only once (the other monitors show black during the countdown, and a key on any monitor cancels it); the screensaver plays on every monitor.
- Marks on the test screens are stored separately for each monitor.

Technically, the first ticked monitor is played by the main program and each other monitor gets its own copy of the program (Task Manager shows several `go-screen-burn-in-fixer.exe`), which all exit when the main program exits.

### Shuffle

When **Shuffle** is chosen next to **Patterns**, each pattern's time is cut into short slices interleaved with the other patterns, e.g. gray-noise for 40 s, vstripes for 25 s, then moving-bar… and back to gray-noise.

- The seconds for each pattern in the list become **the total time per round**; shuffling does not change the total.
- **Slice** can be random 15–60 s (default) or fixed 10, 20, 30, 60 s.
- The same pattern never plays twice in a row; when the playback time is longer than one round, every round is shuffled again. If one pattern takes more than half of a round, its slices get longer so it can still be interleaved.
- The info bar shows how long the pattern has played in this round, e.g. "gray-noise   this round 3:20 / 10:00".

### Playback time

**Play time** can be set with ◀▶ to common values (10 s, 30 s, 1 min, 5 min up to 24 h), or typed in directly when the row is selected, up to 72 hours. A number without a unit means minutes:

| Input | Meaning |
|---|---|
| `90` | 90 minutes |
| `10s` | 10 seconds |
| `3m` | 3 minutes |
| `3h` | 3 hours |
| `1m30s` | 1 minute 30 seconds |
| `2:30`, `2h30m` | 2 hours 30 minutes |

Press Enter to confirm, Esc to cancel.

### Keys during burn-in playback

| Key | Action |
|---|---|
| Esc | Back to the settings menu (quits when started with command-line options) |
| Space | Pause / resume |
| ← / → | Previous / next pattern |
| H | Show / hide the info bar (current pattern, remaining time, FPS) |
| Hold S | Show the info bar while held |

### Test screens

- Tick the items to test; they run in order. For items with several steps (e.g. near-black has 4), the seconds are per step.
- Each item can be set to a number of seconds (5–120) or **Manual** (press Enter / Space to go to the next step). **Set all** at the top sets every item to automatic 10 s or manual at once.
- The number of gray steps can be switched with ◀▶ (8, 16, 32, 64, 128, 256), or typed in (2–256) when the row is selected.
- **gray-field** has darkest and brightest values (0–255, default 13 and 191) and a number of steps (4–15, default 7); each field can be typed in when selected.
  - Darkest to brightest is divided evenly into n steps: in automatic mode each step is shown for the set time, in manual mode press Enter to advance. The number of steps never exceeds the gray levels in the range (e.g. 3–4 has only 2 steps; darkest equal to brightest has 1).
  - During the test, **drag left / right with the right mouse button** to adjust the gray level (left darker, right brighter; the full screen width equals darkest to brightest). Dragging switches the item to manual; after that Enter jumps to the next step brighter than the current level.
  - The info bar shows the gray value, percentage and step, e.g. "Gray 128 (50.2%)   item 1/8, step 4/7".
- After all items finish, or Esc is pressed, you return to the test menu.

| Key | Action |
|---|---|
| Enter / Space | Next step (required in manual mode; skips ahead in automatic mode) |
| ← | Previous step |
| Right-drag | gray-field: drag left / right to adjust the gray level (switches to manual) |
| Esc | Cancel, back to the test menu |
| Mouse | Shows a crosshair and pixel coordinates when moved; click to mark a suspicious spot, click again to remove it |
| C | Clear all marks |
| M | Hide / show marks |
| H | Show / hide the info bar |
| Hold S | Show the info bar while held |

Marks are saved and kept the next time, so you can check the same spot again after running the burn-in patterns. They can be cleared in the test menu.

### Info bar

**Info bar** in the settings menu controls the info bar in the top left during playback and tests (shared by both modes):

| Setting | Behavior |
|---|---|
| Off | Hidden; appears only while S is held or after pressing H |
| 3 / 5 / 10 s | Shown for that many seconds when the pattern or step changes or the mouse moves (default 5 s) |
| Always | Always shown |

### Timer and action at the end

| Action | Description |
|---|---|
| `quit` | Quit the program |
| `shutdown` | Shut down after a 60-second countdown (any key cancels) |
| `sleep` | Sleep after a 60-second countdown (any key cancels) |
| `screensaver` | Stop flashing and switch to a screensaver; ends on a key press or mouse movement |

Text on the countdown, warning, message screens and the info bar slowly dims and brightens ("breathing") instead of staying at the same brightness in the same place, so the text itself does not cause burn-in.

### Command-line options

Running with playback options (`-program`, `-timer`, `-after`, `-screensaver`, `-monitor`, `-check`, `-shuffle`) skips the warning screen and settings menu and starts playing right away, which is handy for scheduled tasks. These options apply only to that run and do not change the settings file:

```
go-screen-burn-in-fixer.exe -program "gray-noise:600,vstripes:300,solid-cycle:120" -timer 30m -after screensaver
go-screen-burn-in-fixer.exe -monitor all -shuffle -timer 1h -after sleep
```

| Option | Description |
|---|---|
| `-program` | Program list in `pattern:seconds` format, comma-separated (default: all patterns in turn, 50 minutes total) |
| `-timer` | Timer length, e.g. `30m`, `1h` |
| `-after` | Action when time is up: `quit`, `shutdown`, `sleep`, `screensaver` |
| `-screensaver` | Screensaver name |
| `-monitor` | Monitors to use (from 1): `2`, `1,2` or `all` |
| `-shuffle` | Shuffle playback |
| `-shuffle-slice` | Seconds per slice when shuffling; `0` means random 15–60 s |
| `-lang` | Interface language: `en` (English, default) or `zh` (Traditional Chinese) |
| `-list` | List all patterns and screensavers |
| `-list-monitors` | List monitors (current resolution and EDID native resolution) |
| `-windowed` | Run in a window |
| `-dry-run` | For testing: do not actually shut down or sleep |
| `-countdown` | Countdown before shutdown / sleep (default `60s`) |
| `-check` | Start testing right away, e.g. `"gray-field:10,gray-steps:manual,white"` (seconds per step or `manual`; omitted values use the menu settings). Quits when finished or on Esc |
| `-gray-levels` | Number of gray steps (2–256) |
| `-gray-min` / `-gray-max` | Darkest and brightest value for gray-field (0–255) |
| `-gray-field-steps` | Number of gray-field steps (4–15) |

## Building

Requires Go 1.27 or later. Build the release files for all platforms (Windows exe, macOS .app, Linux tar.gz) into `dist/release/`:

```
python tools/release/build.py
```

Everything is cross-compiled with `CGO_ENABLED=0` on any system (Ebitengine needs no cgo on macOS or Linux). Windows only:

```
go build -trimpath -ldflags "-s -w -H windowsgui" -o dist/go-screen-burn-in-fixer.exe .
```

The exe's icon, version information and manifest come from `winres/`; [go-winres](https://github.com/tc-hib/go-winres) generates `rsrc_windows_*.syso` (committed, used automatically by `go build`). Regenerate after changing the icon or version:

```
go install github.com/tc-hib/go-winres@latest
go run ./tools/icon                      # redraws winres/icon.png and icon16.png (not needed for version changes)
go-winres make --arch amd64,arm64
```

Generate the preview videos (requires ffmpeg):

```
go run ./cmd/preview
```

## Adding patterns

Each pattern is a `.go` and `.kage` (Ebitengine shader) file pair under `patterns/`, registered in `init()`; no other code needs to change. See section 3.3 of [CLAUDE.md](CLAUDE.md) (in Chinese). Screensavers (`screensaver/`) and test screens (`checks/`) work the same way.

## Development status

Current progress, items still awaiting real-hardware testing and known limitations are in [STATUS.md](STATUS.md) (in Chinese).

## License

This project is licensed under the [MIT License](LICENSE).

The embedded font Noto Sans TC (subset) is licensed under the [SIL Open Font License 1.1](internal/ui/NotoSansTC-LICENSE.txt).

If this tool helps you, sponsorship to support development is welcome via [PayPal](https://paypal.me/The78ester).
