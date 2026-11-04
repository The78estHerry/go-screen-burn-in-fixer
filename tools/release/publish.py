"""建立 GitHub 與 Gitea 的 Release、上傳附件、下載回來比對 SHA-256。token 不會印出。
用法：python tools/release/publish.py <tag> <說明檔> [附件資料夾，預設 dist/release]

先建好 annotated tag 並推到兩邊（RELEASING.md）。只使用 Python 標準函式庫。"""
import glob, hashlib, json, os, subprocess, sys, urllib.request, uuid

sys.stdout.reconfigure(encoding="utf-8")  # Windows 主控台預設不是 UTF-8

tag, body_file = sys.argv[1:3]
asset_dir = sys.argv[3] if len(sys.argv) > 3 else "dist/release"
body = open(body_file, encoding="utf-8").read()
assets = sorted(glob.glob(os.path.join(asset_dir, f"*{tag.lstrip('v')}*")))
assets = [a for a in assets if os.path.isfile(a)]
print("附件：", [os.path.basename(a) for a in assets])


def token(host, user):
    out = subprocess.run(["git", "credential", "fill"], input=f"protocol=https\nhost={host}\nusername={user}\n\n",
                         capture_output=True, text=True, timeout=20).stdout
    for line in out.splitlines():
        if line.startswith("password="):
            return line[len("password="):]
    sys.exit(f"找不到 {host} 的 token")


def req(method, url, headers, data=None):
    r = urllib.request.Request(url, data=data, method=method, headers=headers)
    with urllib.request.urlopen(r, timeout=300) as resp:
        return resp.read()


def sha(b):
    return hashlib.sha256(b).hexdigest().upper()


local = {os.path.basename(a): sha(open(a, "rb").read()) for a in assets}

# GitHub
gt = token("github.com", "The78estHerry")
gh = {"Authorization": f"Bearer {gt}", "Accept": "application/vnd.github+json", "User-Agent": "release-script"}
rel = json.loads(req("POST", "https://api.github.com/repos/The78estHerry/go-screen-burn-in-fixer/releases",
                     {**gh, "Content-Type": "application/json"},
                     json.dumps({"tag_name": tag, "name": tag, "body": body}).encode()))
print("GitHub Release id", rel["id"], rel["html_url"])
for a in assets:
    n = os.path.basename(a)
    up = json.loads(req("POST", f"https://uploads.github.com/repos/The78estHerry/go-screen-burn-in-fixer/releases/{rel['id']}/assets?name={n}",
                        {**gh, "Content-Type": "application/octet-stream"}, open(a, "rb").read()))
    got = sha(req("GET", up["url"], {**gh, "Accept": "application/octet-stream"}))
    print("  GitHub", n, "OK" if got == local[n] else "MISMATCH")

# Gitea
tt = token("gitea.littleherry.work", "claude-bot")
gi = {"Authorization": f"token {tt}"}
base = "https://gitea.littleherry.work/api/v1/repos/The78estWorld/go-screen-burn-in-fixer"
rel2 = json.loads(req("POST", f"{base}/releases", {**gi, "Content-Type": "application/json"},
                      json.dumps({"tag_name": tag, "name": tag, "body": body}).encode()))
print("Gitea Release id", rel2["id"], rel2["html_url"])
for a in assets:
    n = os.path.basename(a)
    bd = uuid.uuid4().hex
    data = (f"--{bd}\r\nContent-Disposition: form-data; name=\"attachment\"; filename=\"{n}\"\r\n"
            f"Content-Type: application/octet-stream\r\n\r\n").encode() + open(a, "rb").read() + f"\r\n--{bd}--\r\n".encode()
    up = json.loads(req("POST", f"{base}/releases/{rel2['id']}/assets?name={n}",
                        {**gi, "Content-Type": f"multipart/form-data; boundary={bd}"}, data))
    got = sha(req("GET", up["browser_download_url"], gi))
    print("  Gitea", n, "OK" if got == local[n] else "MISMATCH")

for n, h in local.items():
    print(f"{n}  {h}")
