"""產生內嵌在程式裡的中文字型子集（開發時使用，需要 fonttools）。

用法（在專案根目錄執行；-P 避免從目前目錄載入模組，同時保留使用者安裝的 fonttools）：
    python -P tools/fontsubset/subset.py <NotoSansTC-Regular.otf> internal/ui/NotoSansTC-Subset.otf

來源字型：https://github.com/notofonts/noto-cjk 的 Sans/SubsetOTF/TC/NotoSansTC-Regular.otf
（SIL Open Font License 1.1）。

收錄字元：ASCII、Big5 常用字（第一字面，約 5400 字）、中文標點、全形字元、介面用到的
箭頭與符號，以及專案中所有 .go 檔案裡出現的非 ASCII 字元（例如不在常用字表的「癇」）。
修改介面文字後，若 `go test ./internal/ui` 回報缺字，重新執行本程式即可。
"""

import pathlib
import sys

from fontTools import subset


def source_chars(root):
    chars = set()
    for path in pathlib.Path(root).rglob("*.go"):
        chars.update(c for c in path.read_text(encoding="utf-8") if ord(c) > 0x7F)
    return chars


def big5_level1():
    chars = []
    for hi in range(0xA4, 0xC7):
        for lo in list(range(0x40, 0x7F)) + list(range(0xA1, 0xFF)):
            if hi == 0xC6 and lo > 0x7E:
                break
            try:
                chars.append(bytes([hi, lo]).decode("big5"))
            except UnicodeDecodeError:
                pass
    return chars


def ranges(*pairs):
    return [chr(c) for lo, hi in pairs for c in range(lo, hi + 1)]


def main():
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    src, dst = sys.argv[1], sys.argv[2]
    chars = set(big5_level1())
    chars.update(source_chars(pathlib.Path(__file__).resolve().parents[2]))
    chars.update(ranges(
        (0x0020, 0x007E),  # ASCII
        (0x00A0, 0x00FF),  # Latin-1（×、·、° 等）
        (0x2000, 0x206F),  # 一般標點（…、—、‧）
        (0x2190, 0x21FF),  # 箭頭
        (0x2500, 0x25FF),  # 框線、幾何圖形（◀ ▶ ● ○ ■ □）
        (0x2600, 0x26FF),  # 雜項符號（⚠）
        (0x2700, 0x27BF),  # 雜項符號（✓ ✗）
        (0x3000, 0x303F),  # 中文標點
        (0xFF00, 0xFFEF),  # 全形字元
    ))
    opts = subset.Options()
    opts.layout_features = ["*"]
    opts.name_IDs = ["*"]
    opts.notdef_outline = True
    font = subset.load_font(src, opts)
    sub = subset.Subsetter(opts)
    sub.populate(text="".join(sorted(chars)))
    sub.subset(font)
    subset.save_font(font, dst, opts)
    cmap = font.getBestCmap()
    missing = [c for c in "◀▶✓●○「」：，。（）×" if ord(c) not in cmap]
    print(f"{dst}: {len(cmap)} 個字元" + (f"，缺少 {''.join(missing)}" if missing else ""))


if __name__ == "__main__":
    main()
