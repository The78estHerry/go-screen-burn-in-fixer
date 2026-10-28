"""建置所有平台的發布檔案到 dist/release/。

    python tools/release/build.py

- Windows x64：單一 exe（圖示與版本資訊來自 rsrc_windows_*.syso）
- macOS（Apple Silicon / Intel）：.app 套件壓成 zip（zip 保留執行權限，解壓縮後點兩下即可開啟）
- Linux x64：執行檔與授權檔壓成 tar.gz（保留執行權限）

全部用 CGO_ENABLED=0 交叉編譯：Ebitengine 在 macOS 與 Linux 都不需要 cgo（執行時才載入系統的圖形函式庫）。
版本號取自 winres/winres.json 的 ProductVersion。只使用 Python 標準函式庫。
"""

import io
import json
import os
import shutil
import stat
import struct
import subprocess
import tarfile
import zipfile

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
OUT = os.path.join(ROOT, "dist", "release")
NAME = "go-screen-burn-in-fixer"
BUNDLE_ID = "work.littleherry.go-screen-burn-in-fixer"


def version():
    with open(os.path.join(ROOT, "winres", "winres.json"), encoding="utf-8") as f:
        return json.load(f)["RT_VERSION"]["#1"]["0000"]["info"]["0409"]["ProductVersion"]


def go_build(goos, goarch, out, ldflags="-s -w"):
    env = dict(os.environ, GOOS=goos, GOARCH=goarch, CGO_ENABLED="0")
    subprocess.run(["go", "build", "-trimpath", "-ldflags", ldflags, "-o", out, "."],
                   cwd=ROOT, env=env, check=True)
    print(f"  {goos}/{goarch} → {os.path.relpath(out, ROOT)}")


def icns(png_path):
    """以 256×256 PNG（ic08）組成 .icns；macOS 會自動縮放到其他大小。"""
    with open(png_path, "rb") as f:
        png = f.read()
    entry = b"ic08" + struct.pack(">I", 8 + len(png)) + png
    return b"icns" + struct.pack(">I", 8 + len(entry)) + entry


def info_plist(ver):
    return f"""<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key><string>{NAME}</string>
	<key>CFBundleIdentifier</key><string>{BUNDLE_ID}</string>
	<key>CFBundleName</key><string>{NAME}</string>
	<key>CFBundleDisplayName</key><string>{NAME}</string>
	<key>CFBundleIconFile</key><string>icon.icns</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>{ver}</string>
	<key>CFBundleVersion</key><string>{ver}</string>
	<key>LSMinimumSystemVersion</key><string>12.0</string>
	<key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
"""


def zip_add(z, arcname, data, executable=False):
    info = zipfile.ZipInfo(arcname, date_time=(2026, 1, 1, 0, 0, 0))
    info.create_system = 3  # Unix：讓 external_attr 的權限生效
    mode = 0o755 if executable else 0o644
    info.external_attr = (stat.S_IFREG | mode) << 16
    info.compress_type = zipfile.ZIP_DEFLATED
    z.writestr(info, data)


def mac_app(goarch, ver, label):
    binary = os.path.join(OUT, "tmp", f"darwin-{goarch}", NAME)
    go_build("darwin", goarch, binary)
    app = f"{NAME}.app/Contents"
    out = os.path.join(OUT, f"{NAME}-{ver}-macos-{label}.zip")
    with zipfile.ZipFile(out, "w") as z:
        with open(binary, "rb") as f:
            zip_add(z, f"{app}/MacOS/{NAME}", f.read(), executable=True)
        zip_add(z, f"{app}/Info.plist", info_plist(ver).encode())
        zip_add(z, f"{app}/Resources/icon.icns", icns(os.path.join(ROOT, "winres", "icon.png")))
    return out


def linux_tar(goarch, ver):
    binary = os.path.join(OUT, "tmp", f"linux-{goarch}", NAME)
    go_build("linux", goarch, binary)
    out = os.path.join(OUT, f"{NAME}-{ver}-linux-{goarch}.tar.gz")
    top = f"{NAME}-{ver}-linux-{goarch}"
    with tarfile.open(out, "w:gz") as t:
        def add(path, arcname, mode):
            ti = t.gettarinfo(path, arcname)
            ti.mode, ti.uid, ti.gid, ti.uname, ti.gname = mode, 0, 0, "", ""
            with open(path, "rb") as f:
                t.addfile(ti, f)
        add(binary, f"{top}/{NAME}", 0o755)
        add(os.path.join(ROOT, "LICENSE"), f"{top}/LICENSE", 0o644)
        add(os.path.join(ROOT, "internal", "ui", "NotoSansTC-LICENSE.txt"), f"{top}/NotoSansTC-LICENSE.txt", 0o644)
    return out


def main():
    ver = version()
    shutil.rmtree(OUT, ignore_errors=True)
    os.makedirs(OUT)
    print(f"版本 {ver}")
    outs = []
    exe = os.path.join(OUT, f"{NAME}-{ver}-windows-amd64.exe")
    go_build("windows", "amd64", exe, "-s -w -H windowsgui")
    outs.append(exe)
    outs.append(mac_app("arm64", ver, "apple-silicon"))
    outs.append(mac_app("amd64", ver, "intel"))
    outs.append(linux_tar("amd64", ver))
    shutil.rmtree(os.path.join(OUT, "tmp"))
    print("輸出：")
    for o in outs:
        print(f"  {os.path.relpath(o, ROOT)}  {os.path.getsize(o) / 1e6:.1f} MB")


if __name__ == "__main__":
    main()
