# 發新版的步驟

0.3.2 起主要放在 GitHub（`github` remote：`https://github.com/The78estHerry/go-screen-burn-in-fixer`，公開），Gitea（`origin`：`The78estWorld/go-screen-burn-in-fixer`，私人）同步一份。**每次都兩邊推、兩邊發 Release。**

1. 更新 `winres/winres.json` 裡所有版本號（`identity.version`、`file_version`、`product_version`、各語言的 `FileVersion`、`ProductVersion`）→ `go-winres make --arch amd64,arm64`。
2. `go vet ./...`、`go test ./...`。
3. `python tools/release/build.py` → `dist/release/` 四個檔案（版本號取自 winres.json）。
4. 請使用者實測要測的項目（STATUS.md）。
5. commit（claude-bot），兩邊推：
   ```
   git push origin main
   git push github main
   ```
6. 建 annotated tag 並兩邊推：
   ```
   git tag -a v0.3.2 -m "v0.3.2"
   git push origin v0.3.2
   git push github v0.3.2
   ```
7. 兩邊都建 Release（`python tools/release/publish.py v<版本> <說明檔>`：建立 Release、上傳附件、下載回來比對 SHA-256，第 8、9 步也一起做完），標題只寫版本號（例如 `v0.3.2`），說明相同：中文為主，英文、日文放在 `<details>` 折疊區塊。有快速閃爍畫面的警告、macOS／Linux 沒有實機測過時要寫。
8. 上傳 `dist/release/*` 當作附件。**不要上傳原始碼壓縮檔**，兩邊都會自動附上。
9. 兩邊都下載回來比對 SHA-256。Gitea 是私人 repo，下載要帶 token，否則拿到的是登入頁。
10. 更新 STATUS.md 的版本表。

## API

token 一律用 `git credential fill` 取得，不要印出來、不要寫進檔案。JSON 裡有中文時先寫成 UTF-8 檔案再用 `--data-binary @檔案` 送出（Windows 的 bash 直接用 `-d '中文'` 會送出錯誤的編碼）。

- **GitHub**：`host=github.com`、`username=The78estHerry`，header `Authorization: Bearer <token>`。
  - 建 Release：`POST https://api.github.com/repos/The78estHerry/go-screen-burn-in-fixer/releases`，JSON 放 `tag_name`、`name`、`body`。
  - 上傳附件：`POST https://uploads.github.com/repos/The78estHerry/go-screen-burn-in-fixer/releases/<id>/assets?name=<檔名>`，`Content-Type: application/octet-stream`，`--data-binary @檔案`。
- **Gitea**：`host=gitea.littleherry.work`、`username=claude-bot`，header `Authorization: token <token>`。
  - 建 Release：`POST https://gitea.littleherry.work/api/v1/repos/The78estWorld/go-screen-burn-in-fixer/releases`。
  - 上傳附件：`POST .../releases/<id>/assets?name=<檔名>`，multipart 欄位 `attachment`。

## 版本紀錄

| 版本 | 日期 | Release 位置 | 內容 |
|---|---|---|---|
| 0.1.0～0.3.1 | 2026-10-28～29 | Gitea | 見 STATUS.md |
| 0.4.0 | 2026-11-04 | GitHub（id 433649310）、Gitea（id 20） | 英文介面（預設英文，警告畫面與選單右上角切換）、`-lang`、使用風險自負的免責說明 |
| 0.3.2 | 2026-11-04 | GitHub（id 433618822）、Gitea（id 19） | 第一次在 GitHub 發佈：module 路徑改成 GitHub、README 加英文／日文版與圖示、著作權人改為 The78ester、開放 PayPal 贊助。功能和 0.3.1 相同 |
