# 💡 Inspirationer · 灵感管理器

**English** | [简体中文](README.zh-CN.md) | [日本語](README.ja.md)

> Create, edit, tag, categorize, search — store your inspiration in a timely manner.
> A **fully local** inspiration manager: Go backend + browser UI, shipped as a single executable.

![Go](https://img.shields.io/badge/Go-1.20%2B-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/Platform-Windows%2010%2F11-0078D7?logo=windows&logoColor=white)
![Languages](https://img.shields.io/badge/UI-English%20%7C%20简体中文%20%7C%20日本語-success)
![Go deps](https://img.shields.io/badge/Go%20dependencies-none-success)
![UI](https://img.shields.io/badge/UI-no%20build%20step-blueviolet)
![License](https://img.shields.io/badge/License-Apache--2.0-blue)

Double-click and you are done: **no console window**, a 💡 icon in the system tray, and your default
browser opens automatically. Everything is stored as JSON on your own disk — Markdown editing,
AI-generated titles, AI-assisted tagging, a keyboard-driven pipeline, full-text search and
scheduled WebDAV backups.

> No Node.js, no database, no network access except when *you* use the AI features.
> The Go side has **zero third-party dependencies** — even the tray icon talks to Win32 directly.

## Screenshots

| Main view (cards, categories, coloured tags) | Markdown editor |
|---|---|
| ![Main view](docs/01-overview.png) | ![Editor](docs/02-editor.png) |

| Pipeline tagging (read-only view, tag, `Alt+J` for next) | Settings: AI / WebDAV / shortcuts |
|---|---|
| ![Pipeline](docs/03-pipeline.png) | ![Settings](docs/05-settings-ai.png) |

The UI ships in **English, 简体中文 and 日本語** — the same screen in Japanese and Chinese:

| 日本語 | 简体中文 |
|---|---|
| ![Japanese UI](docs/ja-01-overview.png) | ![Chinese UI](docs/zh-01-overview.png) |

---

## Table of contents

- [Quick start](#quick-start)
- [System tray & background behaviour](#system-tray--background-behaviour)
- [Features](#features)
- [Internationalization](#internationalization)
- [Keyboard shortcuts](#keyboard-shortcuts)
- [AI configuration](#ai-configuration)
- [WebDAV backups](#webdav-backups)
- [AI tagging & pipeline tagging](#ai-tagging--pipeline-tagging)
- [Search](#search)
- [Where your data lives](#where-your-data-lives)
- [Privacy & security](#privacy--security)
- [Command-line flags](#command-line-flags)
- [REST API](#rest-api)
- [Development & testing](#development--testing)
- [Architecture](#architecture)
- [Third-party components](#third-party-components)
- [FAQ](#faq)
- [License](#license)

---

## Quick start

### Option A — download the executable (recommended)

Grab `inspirationer.exe` from [Releases](../../releases) (or build it, see below) and double-click it:

1. the service starts in the background at `http://127.0.0.1:8420/`;
2. **your default browser opens automatically**;
3. a 💡 icon appears in the system tray — and no console window ever shows up.

### Option B — build from source

```powershell
git clone https://github.com/idiomeo/inspirationer.git
cd inspirationer
.\build.ps1              # GUI build (no console window)
.\inspirationer.exe
```

Without PowerShell:

```powershell
go build -trimpath -ldflags "-s -w -H=windowsgui" -o inspirationer.exe .
.\inspirationer.exe
```

The default port is `8420`; if it is taken the app walks up to `8421`, `8422`… (the real address is
printed in the log).

---

## System tray & background behaviour

| Action | Effect |
|---|---|
| Left-click 💡 | Open the UI in your browser |
| Right-click → Open Inspirationer | Same as above |
| Right-click → Open data folder | Opens the `data` folder in Explorer |
| Right-click → Back up to WebDAV now | Uploads a full backup immediately, reports via balloon tip |
| Right-click → View log file | Opens `data\logs\inspirationer.log` |
| Right-click → Quit | Graceful shutdown (data is flushed to disk) |

Other behaviour:

- **Second launches never start a second service** — starting the exe again detects the running
  instance and simply opens its UI (`-single-instance=false` disables this).
- **Data lives in `data\` next to the executable**; the log is `data\logs\inspirationer.log`
  (rotated to `.1` at 4 MB).
- **If the tray is unavailable** (rare, restricted environments — see [FAQ](#faq)) the app opens a
  log console window and shows a one-time notice instead. The service keeps running; you never end
  up with an invisible process you cannot quit.
- Need live logs? Use `inspirationer-console.exe`, or pass `-console` to the normal exe.

---

## Features

| Area | Details |
|---|---|
| **Snippets** | “＋ New snippet” or `Alt+N`; the title is optional and the body is full Markdown |
| **Markdown editing** | Bundled EasyMDE (CodeMirror + marked): headings, lists, quotes, code blocks, tables, images, preview, fullscreen. Toolbar icons are inline CSS glyphs — no CDN |
| **Automatic titles** | Leave the title empty: with AI configured it is summarised by the model, otherwise the first N characters of the body are used (default 10, configurable). Cards show where the title came from |
| **Tags & categories** | Any number of tags plus one category per snippet, shown as coloured chips right under the title; auto-assigned colours or your own, rename/recolour by right-clicking the sidebar |
| **Search** | Full text / title only / body only, multi-keyword AND, and “search inside one category” by selecting it in the sidebar first |
| **Shortcuts** | Fully configurable (defaults use `Alt` so they never clash with browser shortcuts), enabling a create → write → save pipeline |
| **AI auto-tagging** | Select snippets (or select all) → AI proposes tags and a category per snippet → review, adjust, then apply |
| **Pipeline tagging** | Process the selection one snippet at a time: read, tag, `Alt+J` for the next; every step is saved immediately |
| **Select all** | Toolbar checkbox / `Alt+A` / “Select all suggestions” inside the AI dialog |
| **Organising** | Pin 📌, archive 🗄️, bulk archive/delete, filter by category or tag, three sort orders |
| **Local persistence** | Atomic JSON writes on every change; a local snapshot after every save (30 kept); automatic recovery from the newest snapshot if a file gets corrupted |
| **WebDAV backup** | Scheduled automatic backup, backup now, list remote copies, merge/replace restore, automatic pruning of old backups |
| **Data portability** | Export the whole library as JSON, import it back (merge or replace); a local snapshot is taken before every import/restore |
| **Interface** | Dark/light theme, optional Markdown preview on cards, delete confirmation, live sidebar counters |

---

## Internationalization

The UI ships with **English, Simplified Chinese and Japanese**, and the language is a normal setting.

- **Where to switch:** Settings → 🎨 Appearance → **Language**. The switch applies instantly, no
  reload needed.
- **Follow browser** (default): `auto` picks the best match from `navigator.languages`
  (e.g. `zh-Hans` → 简体中文, `ja-JP` → 日本語) and falls back to English.
- **What is translated:** every label, button, placeholder, toast, dialog, editor toolbar tooltip,
  the relative timestamps (“3 minutes ago” / “3 分前” / “3 分钟前”), the **system tray menu**, the
  Windows dialogs, and the **REST API error messages**.
- **What is not translated:** your own content — snippets, titles, tag and category names are yours.
- **Persistence:** the choice is stored in `data/settings.json` (`ui.language`) and mirrored to
  `localStorage` so the first paint is already in the right language.

### Adding or improving a language

1. **Front-end strings** live in [`web/i18n.js`](web/i18n.js): three flat dictionaries keyed
   `en` / `zh-CN` / `ja`. Copy a block, translate the values, keep the keys and the `{placeholders}`
   (`{n}`, `{name}`, `{time}`…). Missing keys fall back to English automatically, so a partial
   translation is fine.
2. **Back-end strings** (API errors, tray menu, native dialogs) live in
   [`internal/i18n/i18n.go`](internal/i18n/i18n.go) as `key → language → text`.
3. Register the new code in `SUPPORTED_LANGS` (`web/app.js`), `i18n.Supported()`
   (`internal/i18n/i18n.go`), the `Language` select in `web/index.html`, and the `ui.language`
   whitelist in `internal/model/model.go`.

### API language negotiation

Every endpoint honours the request language, in this order:

1. `X-Lang` header — the web UI sends its current language on every request;
2. `?lang=xx` query parameter;
3. `Accept-Language` header;
4. otherwise the saved `ui.language` setting, falling back to English.

```bash
curl -s -X POST http://127.0.0.1:8420/api/ai/suggest \
  -H 'X-Lang: ja' -H 'Content-Type: application/json' -d '{"ids":["nope"]}'
# {"error":"AI 機能が無効です。先に「設定 → AI」で API を設定してください"}
```

---

## Keyboard shortcuts

All defaults use `Alt` combinations that browsers leave alone (avoiding `Ctrl+N/T/W/S/P/F/D`, `F5`,
`Alt+F/E/D/Home/←/→`, …).

| Action | Default | Notes |
|---|---|---|
| New snippet | `Alt+N` | Opens the editor from anywhere |
| Save snippet | `Alt+S` | Saves and closes the editor |
| Focus search | `Alt+K` | Jumps to the search box and selects it |
| Select all / none | `Alt+A` | Selects every snippet in the list |
| Pipeline: next | `Alt+J` | Saves the current snippet and moves on |
| Toggle preview | `Alt+P` | Markdown preview in the editor |
| Open settings | `Alt+O` | |
| Close dialog | `Escape` | Leaves editor fullscreen first |

**Customising:** Settings → ⌨️ Shortcuts → click a box → press the combination you want
(`Backspace` clears it). Duplicates are highlighted in red.

---

## AI configuration

Settings → 🤖 AI:

1. tick **Enable AI features**;
2. pick a **preset** or fill in `API Base URL` + `Model` yourself;
3. paste your `API Key`;
4. hit **🔌 Test connection**.

Built-in presets (anything that speaks the OpenAI `chat/completions` protocol works):

| Service | Base URL | Example model |
|---|---|---|
| OpenAI | `https://api.openai.com/v1` | `gpt-4o-mini` |
| DeepSeek | `https://api.deepseek.com/v1` | `deepseek-chat` |
| Moonshot Kimi | `https://api.moonshot.cn/v1` | `moonshot-v1-8k` |
| Qwen (Alibaba) | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `qwen-plus` |
| Zhipu GLM | `https://open.bigmodel.cn/api/paas/v4` | `glm-4-flash` |
| Ollama (local) | `http://127.0.0.1:11434/v1` | `qwen2.5:7b` |
| One-API / New-API | `http://127.0.0.1:3000/v1` | any |

> Local servers such as Ollama need no key; keys may stay empty when the Base URL contains
> `localhost` / `127.0.0.1`.
> **The API key is only written to your local `data/settings.json`**, which is git-ignored — it is
> never committed and never uploaded anywhere.

**AI is involved in exactly three things:** ① generating a title when the title is empty,
② proposing tags and a category, ③ the “✨ AI title” button inside the editor.

---

## WebDAV backups

Settings → ☁️ WebDAV backup:

| Field | Meaning |
|---|---|
| Enable automatic backup | While running, upload a full backup at the interval below |
| WebDAV URL | e.g. `https://dav.jianguoyun.com/dav/` (Nutstore) |
| Username / password | Use an **app password** for Nutstore, Nextcloud, … rather than your login password |
| Remote folder | Defaults to `inspirationer`; created with MKCOL if missing |
| Backup interval | Minutes, default 60 |
| Keep at most | Old backups beyond this count are deleted remotely (default 10) |

Known-good endpoints:

- **Nutstore (坚果云)**: `https://dav.jianguoyun.com/dav/` (email + app password)
- **Nextcloud / ownCloud**: `https://your-host/remote.php/dav/files/<user>/`
- **Synology**: `https://your-host:5006/<share>/` (WebDAV Server package)
- **Alist**: `http://127.0.0.1:5244/dav/`
- **Anything else** that supports `PROPFIND/PUT/GET/MKCOL/DELETE`

Buttons: **Test connection** (works with unsaved form values), **Back up now**, **Restore from
remote** (per-file “Merge restore” / “Replace restore”, with a local snapshot taken first) and
**Create local snapshot**.

Remote files are named `inspirationer-backup-YYYYMMDD-HHMMSS.json`, plus a `latest.json` that always
points at the newest backup.

---

## AI tagging & pipeline tagging

Both are **explicitly triggered by you** — nothing is ever modified silently.

### 🤖 AI auto-tagging (batch)

1. select snippets (or press `Alt+A`);
2. click “🤖 AI auto-tag” in the toolbar or the bulk bar;
3. review the proposals — title, 2–4 tags, one category and a one-line summary per snippet;
4. untick anything you disagree with, choose append/replace for tags, or “Select all suggestions”;
5. click “Apply selected suggestions”.

The model receives your existing tags and categories, so it **reuses them** (case-insensitively)
instead of inventing near-duplicates; new ones get an automatic colour.

### ⚡ Pipeline tagging (one by one)

1. select snippets → “⚡ Pipeline tagging”;
2. the left side shows the snippet read-only, the right side lets you pick a category and toggle or
   create tags;
3. press `Alt+J` (or “Save and continue”) to save and advance;
4. “Skip” moves on without saving, “← Previous” goes back, “End pipeline” stops at any time.

Progress is shown as `3 / 12`; every snippet is persisted as you go, so closing the window loses
nothing.

---

## Search

The search box (`Alt+K` to focus):

- **Scope:** `Full text` (title + body + tag names + category names), `Title only`, `Body only`;
- **Multiple keywords:** space-separated, all must match (e.g. `markdown writing`);
- **Scoped search:** pick a category or tag in the sidebar first, then search — i.e. “search inside
  one category”;
- case-insensitive, debounced (220 ms), `✕` clears it.

Sidebar quick views: all snippets, uncategorized, pinned, archived.

---

## Where your data lives

The default data folder is `./data` next to the executable (`-data` overrides it; if the working
directory is not writable the app falls back to the executable’s folder).

```
data/
├── snippets.json      # snippets: title, body, tag IDs, category ID, timestamps, title source
├── tags.json          # tags (name + colour)
├── categories.json    # categories (name + colour)
├── settings.json      # AI / WebDAV / shortcuts / UI  (⚠️ may contain your API key and password)
├── runtime.json       # PID + URL, used to find a running instance
├── logs/
│   └── inspirationer.log
└── backups/
    └── snapshot-YYYYMMDD-HHMMSS.json   # last 30 local snapshots
```

- Every write goes through “temp file + atomic rename”, and changes are flushed immediately.
- A snapshot is added on startup if the newest one is older than 6 hours.
- Settings → 💾 Data lets you **export the whole library** (downloaded as JSON) and **import** it
  (merge or replace); both paths take a local snapshot first so you can always roll back.

---

## Privacy & security

- **Your data stays yours.** Everything lives in the local `data/` folder; there is no telemetry,
  no analytics and no phoning home.
- **No network unless you ask.** Requests are only made when you use an AI feature (to the endpoint
  *you* configured) or a WebDAV backup.
- **Secrets stay local.** The AI API key and WebDAV password are written to `data/settings.json`,
  which `.gitignore` excludes — it is never committed. The repository contains no keys and no
  personal paths, and neither does its history.
- **Fully offline front end.** EasyMDE, marked and DOMPurify are embedded in the binary; the page
  references no CDN, and rendered Markdown is filtered through DOMPurify to prevent XSS.
- **The service listens on `127.0.0.1` by default and has no authentication** — it is a
  single-user desktop tool. If you expose it with `-addr 0.0.0.0:8420`, put a reverse proxy with
  authentication in front of it.
- The test scripts only contain throwaway credentials (`mock-user` / `mock-pass`,
  `mock-api-key`) that point at local mock servers on `127.0.0.1`.

---

## Command-line flags

```
inspirationer.exe [flags]

  -addr string              listen address (default "127.0.0.1:8420", walks up if the port is busy)
  -data string              data folder (default "./data", falls back to the exe folder if unwritable)
  -open                     open the browser after a successful start (default true)
  -tray                     show the system tray icon (default true, Windows)
  -console                  additionally show a console window for live logs
  -single-instance          allow only one instance; later launches open the running one (default true)
  -dev-web string           serve the front end from a folder instead of the embedded copy
  -version                  print the version and exit
```

Examples:

```powershell
# different port, data on D:, do not open a browser
.\inspirationer.exe -addr 127.0.0.1:9000 -data D:\inspiration -open=false

# expose to the LAN (no auth — add a proxy before exposing it publicly)
.\inspirationer.exe -addr 0.0.0.0:8420

# watch the logs
.\inspirationer.exe -console
```

---

## REST API

The front end is a plain static page; everything it does is available over REST, which makes
scripting straightforward. All responses are JSON; add `X-Lang: en|zh-CN|ja` for localized errors.

| Method | Path | Notes |
|--------|------|-------|
| GET | `/api/bootstrap` | Settings, tags, categories and counts in one call |
| GET | `/api/snippets` | List / search: `query` `mode` `categoryId` `tagIds` `archive` `sort` `limit` |
| POST | `/api/snippets` | Create (title auto-generated when empty) |
| GET/PUT/DELETE | `/api/snippets/{id}` | Read / replace / delete one snippet |
| PATCH | `/api/snippets/{id}` | Partial update (`title` `content` `tags` `categoryId` `pinned` `archived` `addTags`) |
| POST | `/api/snippets/bulk` | Batch: `delete` `assign` `archive` `unarchive` `pin` `unpin` |
| GET/POST | `/api/tags`, `/api/categories` | List / create |
| PUT/DELETE | `/api/tags/{id}`, `/api/categories/{id}` | Rename, recolour, delete |
| GET/PUT | `/api/settings` | Read / write settings |
| POST | `/api/ai/title` | Generate a title |
| POST | `/api/ai/suggest` | Propose tags and categories for many snippets |
| POST | `/api/ai/apply` | Apply proposals |
| POST | `/api/ai/test` | Check AI connectivity |
| POST | `/api/webdav/test` \| `/api/webdav/backup` \| `/api/webdav/restore` | Test / back up / restore |
| GET | `/api/webdav/list` | List remote backups |
| GET | `/api/backup/export` \| POST `/api/backup/import` \| POST `/api/backup/local` | Export / import / local snapshot |
| GET | `/api/stats` | Counters |

```bash
curl -X POST http://127.0.0.1:8420/api/snippets \
  -H "Content-Type: application/json" \
  -d '{"content":"Note to self: use WebDAV for backups","tags":[],"categoryId":""}'
```

---

## Development & testing

```powershell
.\build.ps1              # GUI build (-H=windowsgui, no console window)
.\build.ps1 -Console     # also produce inspirationer-console.exe (console, for troubleshooting)
.\build.ps1 -Icon        # regenerate the icon and Windows resources (needs python + Pillow + rsrc)
.\dev.ps1                # dev mode: serve the front end from web/, just refresh the browser
.\scripts\make-release.ps1   # build + assemble release/ + zip ready for a GitHub Release
```

> If PowerShell refuses to run these scripts ("not digitally signed"), see the
> [FAQ entry](#q-powershell-says-the-ps1-scripts-are-not-digitally-signed).

**Packaging a release**

```powershell
.\scripts\make-release.ps1            # build and package
.\scripts\make-release.ps1 -SkipBuild # package the existing binaries only
```

`release/` becomes a self-contained, ready-to-upload package:

```
release/
├── inspirationer.exe                            the app (GUI, tray, no console window)
├── inspirationer-console.exe                    console build for troubleshooting
├── start-inspirationer.bat                      double-click launcher (ASCII-only on purpose)
├── README.md  LICENSE                           docs and license
├── HOW-TO-RUN.txt                               trilingual quick start for downloaders
├── inspirationer-v<version>-windows-amd64.zip   ← upload this to a GitHub Release
└── SHA256SUMS.txt                               checksums for the artifacts above
```

**Tests** (start the server first; the AI/WebDAV/UI suites also need the mock services):

```powershell
# terminal 1 — the app (no tray, no browser, so tests are not disturbed)
.\inspirationer.exe -open=false -tray=false

# terminal 2 — mock AI + mock WebDAV (local throwaway credentials only)
node scripts\mock-services.mjs

# terminal 3 — the suites
node scripts\check-i18n.mjs         # translation completeness: 282 keys × en/zh-CN/ja
go test ./...                      # unit tests: tray struct layout, ICO parsing, Win32 resolution
node scripts\api-smoke.mjs         # 62 checks: CRUD, tags, search, bulk ops, backup, i18n errors
node scripts\ai-webdav-test.mjs    # 33 checks: AI titles, AI tagging, WebDAV backup/prune/restore
node scripts\ui-test.mjs           # 61 checks: real UI in a headless browser, incl. language switching
node scripts\auto-backup-test.mjs  # scheduled backup (takes 1–2 minutes)
```

- `ui-test.mjs` finds Edge/Chrome automatically and drives a headless browser over CDP. It verifies
  rendering, every shortcut, the AI dialog, the pipeline, the settings panel, **switching between
  English/Chinese/Japanese** (including that no raw translation key ever leaks into the DOM and that
  the choice survives a reload), and asserts there are **no uncaught JS exceptions or console
  errors**.
- `go test` includes `TestWin32ProcsResolve`, which resolves every Win32 function the app uses —
  a wrong DLL or a typo in a symbol name fails the test instead of panicking at runtime.
- To actually create a tray icon (it is removed again right away):

```powershell
$env:IH_TRAY_TEST=1; go test ./internal/tray/ -run TestTrayStartInThisEnvironment -v
```

---

## Architecture

```
inspirationer/
├── main.go                     # entry point: flags, single instance, tray, logging, graceful exit
├── assets/                     # app.ico + app.manifest (compiled into the exe)
├── rsrc_windows_amd64.syso     # generated by rsrc; go build links it automatically
├── internal/
│   ├── model/model.go          # data structures and defaults
│   ├── store/store.go          # JSON persistence, queries, bulk ops, import/export
│   ├── ai/ai.go                # OpenAI-compatible client (titles, structured tag proposals)
│   ├── webdav/webdav.go        # WebDAV client (MKCOL/PUT/GET/PROPFIND/DELETE)
│   ├── server/server.go        # routing and all HTTP handlers
│   ├── i18n/i18n.go            # back-end message catalogue (en / zh-CN / ja) + error codes
│   ├── platform/               # small Win32 helpers: message box, open URL, single instance, DPI
│   └── tray/                   # tray icon and context menu (pure syscall, no third-party code)
├── web/                        # front end (vanilla JS, no build step)
│   ├── index.html  styles.css  app.js
│   ├── i18n.js                 # UI strings: en / zh-CN / ja
│   └── vendor/                 # EasyMDE + marked + DOMPurify (bundled, works offline)
├── scripts/                    # test suites + icon generator
└── docs/                       # screenshots
```

Design decisions worth knowing:

- **Single-file distribution** — the front end is embedded with `go:embed`, so copying one exe is
  enough; there is no adjacent resource folder to lose.
- **Zero Go dependencies** — tray icon, message boxes, single-instance mutex and “open in browser”
  are implemented against Win32 through the standard library, avoiding CGO and dependency churn.
- **Portable data over a database** — human-readable JSON you can fix by hand or sync with any cloud
  drive; atomic writes, snapshots and corruption recovery keep it safe.
- **English is the base language** — backend errors are English by default (good for API users and
  logs) and translated at the HTTP boundary, while the UI keeps all translations in one JS file.

---

## Third-party components

The front end bundles three libraries locally (no CDN at runtime):

| Component | Version | Purpose | License |
|-----------|---------|---------|---------|
| [EasyMDE](https://github.com/Ionaru/easy-markdown-editor) | 2.18.0 | Markdown editor (bundles CodeMirror + marked) | MIT |
| [marked](https://github.com/markedjs/marked) | 12.0.2 | Markdown rendering for card previews | MIT |
| [DOMPurify](https://github.com/cure53/DOMPurify) | 3.1.6 | Sanitising rendered Markdown | Apache-2.0 / MPL-2.0 |

The Go side has no third-party dependencies; the tray icon, message boxes, single-instance check and
browser launching all call Win32 APIs directly through `syscall`.

---

## FAQ

**Q: I double-clicked it — no console window appeared. Is it running?**
Look for the 💡 icon in the system tray (bottom-right). On Windows 11 it may be tucked into “Show
hidden icons” — drag it out to pin it. The log is at `data\logs\inspirationer.log`.

**Q: How do I quit it?**
Right-click the tray icon → Quit. Alternatively start it with `inspirationer-console.exe` or
`-console` and press `Ctrl+C`, or end the process from Task Manager (data is already on disk).

**Q: The browser did not open.**
Check the log for “asked the default browser to open”; you can always visit the address manually.
With `-open=false` it is intentional.

**Q: Tray registration failed (`Shell_NotifyIcon … Access is denied`).**
That is an integrity-level restriction: Windows UIPI does not let a **low-integrity** process
register a tray icon with Explorer. The log spells it out (`integrity=Low`). It happens in
sandboxes/restricted containers or when something launches the app with a filtered token; on a
normal desktop double-click you will see `tray icon ready` instead. The app falls back to opening a
log console and telling you, and the service itself is unaffected.

**Q: The port is busy.**
The log prints the real address (the app walks up the port range), or pass
`-addr 127.0.0.1:9000`.

**Q: Can I reach it from another machine?**
`-addr 0.0.0.0:8420` works, but the app has **no authentication** — put an authenticating reverse
proxy in front of it before exposing it.

**Q: Will I lose data?**
Every change is written atomically; `data/backups/` keeps 30 snapshots and a corrupted file is
recovered from the newest one at startup. Combined with WebDAV backups the risk is negligible.

**Q: Shortcuts do not react.**
① The browser window must be focused; ② some browsers reserve `Alt` combinations — record something
like `Alt+Shift+X` instead; ③ while a dialog is open only its own shortcuts are active (by design),
and `Escape` always closes it.

**Q: PowerShell says the `.ps1` scripts are "not digitally signed".**
Your execution policy blocks unsigned scripts. Run them in a bypassed session, or unblock the files
once after downloading:

```powershell
powershell -ExecutionPolicy Bypass -File .\build.ps1
# or, for a cloned/downloaded copy:
Get-ChildItem -Recurse -Include *.ps1 | Unblock-File
```

Nothing here *requires* the scripts: the plain `go build` commands are in
[Quick start](#quick-start), and `启动灵感管理器.bat` / `start-inspirationer.bat` are plain cmd files
that are never affected by the PowerShell policy. (The `.ps1` files are saved with a UTF-8 BOM so
that Windows PowerShell 5.1 reads their non-ASCII comments correctly.)

**Q: Does AI tagging cost money?**
Depends on the service you pick; a local Ollama model is free and offline. Nothing is sent anywhere
unless you trigger an AI action.

**Q: How do I start over?**
Tray → Quit, delete the `data/` folder, start again.

---

## License

Licensed under the **Apache License 2.0** — see [LICENSE](LICENSE).

Third-party front-end licenses are listed under [Third-party components](#third-party-components)
(MIT / Apache-2.0 / MPL-2.0); those files are bundled unmodified in `web/vendor/`.
