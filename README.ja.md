# 💡 ひらめきマネージャー · Inspirationer

[English](README.md) | [简体中文](README.zh-CN.md) | **日本語**

> Create, edit, tag, categorize, search — store your inspiration in a timely manner.
> 思いついたらすぐ記録し、タグを付けて分類し、いつでも検索して取り戻す：**完全にローカルで動作する**ひらめき管理ツールです。

![Go](https://img.shields.io/badge/Go-1.20%2B-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/Platform-Windows%2010%2F11-0078D7?logo=windows&logoColor=white)
![Languages](https://img.shields.io/badge/UI-English%20%7C%20简体中文%20%7C%20日本語-success)
![Go deps](https://img.shields.io/badge/Go%20dependencies-none-success)
![UI](https://img.shields.io/badge/UI-no%20build%20step-blueviolet)
![License](https://img.shields.io/badge/License-Apache--2.0-blue)

ダブルクリックするだけで使えます：**コンソールウィンドウは表示されず**、💡 アイコンがタスクトレイに常駐し、
既定のブラウザが自動的に開きます。すべてはお使いのディスクに JSON として保存されます —— Markdown 編集、
AI によるタイトル生成、AI によるタグ付け支援、キーボード主導のパイプライン、全文検索、
そしてスケジュールされた WebDAV バックアップ。

> Node.js もデータベースも不要で、ネットワークアクセスも*あなたが* AI 機能を使うとき以外は発生しません。
> Go 側は**サードパーティ依存ゼロ** —— タスクトレイのアイコンさえ Win32 を直接呼び出しています。

## 画面プレビュー

| メイン画面（カード、カテゴリ、色付きタグ） | Markdown エディタ |
|---|---|
| ![メイン画面](docs/01-overview.png) | ![エディタ](docs/02-editor.png) |

| パイプラインでのタグ付け（読み取り専用で閲覧、タグ付け、`Alt+J` で次へ） | 設定：AI / WebDAV / ショートカットキー |
|---|---|
| ![パイプライン](docs/03-pipeline.png) | ![設定](docs/05-settings-ai.png) |

UI は **English / 简体中文 / 日本語** の 3 言語に対応しています —— 同じ画面の日本語版と中国語版：

| 日本語 | 简体中文 |
|---|---|
| ![日本語の画面](docs/ja-01-overview.png) | ![中国語の画面](docs/zh-01-overview.png) |

> `node scripts/seed-demo.mjs` を実行するとデモデータ（ひらめき 6 件、カテゴリ 3 件、タグ 6 件）が書き込まれます。
> 不要になったら全選択して削除してください。なお、クローンしたばかりのリポジトリは**空のライブラリ**です（`data/` はリポジトリに含まれません）。

---

## 目次

- [30 秒で使い始める](#30-秒で使い始める)
- [タスクトレイのアイコンとバックグラウンド動作](#タスクトレイのアイコンとバックグラウンド動作)
- [機能一覧](#機能一覧)
- [多言語](#多言語)
- [ショートカットキー](#ショートカットキー)
- [AI の設定](#ai-の設定)
- [WebDAV 自動バックアップ](#webdav-自動バックアップ)
- [AI 自動タグ付け & パイプラインでのタグ付け](#ai-自動タグ付け--パイプラインでのタグ付け)
- [検索](#検索)
- [データの保存場所とバックアップ](#データの保存場所とバックアップ)
- [プライバシーとセキュリティ](#プライバシーとセキュリティ)
- [コマンドライン引数](#コマンドライン引数)
- [REST API](#rest-api)
- [開発とテスト](#開発とテスト)
- [アーキテクチャの説明](#アーキテクチャの説明)
- [サードパーティコンポーネント](#サードパーティコンポーネント)
- [よくある質問](#よくある質問)
- [ライセンス](#ライセンス)

---

## 30 秒で使い始める

### 方法 A：実行ファイルを直接ダウンロードする（推奨）

[Releases](../../releases) から `inspirationer.exe` をダウンロードして（または下記のとおりビルドして）ダブルクリックします。

1. サービスがバックグラウンドで `http://127.0.0.1:8420/` に起動します。
2. **既定のブラウザが自動的に開きます**。
3. 💡 アイコンがタスクトレイに表示されます —— コンソールウィンドウは一切表示されません。

### 方法 B：ソースからビルドする

```powershell
git clone https://github.com/idiomeo/inspirationer.git
cd inspirationer
.\build.ps1              # GUI build (no console window)
.\inspirationer.exe
```

PowerShell がない場合：

```powershell
go build -trimpath -ldflags "-s -w -H=windowsgui" -o inspirationer.exe .
.\inspirationer.exe
```

既定のポートは `8420` です。使用中の場合は `8421`、`8422`… へ繰り上がります（実際のアドレスは
ログに出力されます）。

---

## タスクトレイのアイコンとバックグラウンド動作

| 操作 | 効果 |
|---|---|
| 💡 を左クリック | ブラウザで画面を開く |
| 右クリック → Inspirationer を開く | 同上 |
| 右クリック → データフォルダを開く | エクスプローラーで `data` フォルダを開く |
| 右クリック → 今すぐ WebDAV にバックアップ | 完全バックアップを即座にアップロードし、バルーンで結果を通知 |
| 右クリック → ログファイルを表示 | `data\logs\inspirationer.log` を開く |
| 右クリック → 終了 | 正常終了（データはディスクに書き込み済み） |

その他の動作：

- **2 回目の起動で 2 つ目のサービスが始まることはありません** —— exe をもう一度起動すると、実行中の
  インスタンスを検出してその画面を開くだけです（`-single-instance=false` で無効化できます）。
- **データは実行ファイルの隣の `data\` に保存されます**。ログは `data\logs\inspirationer.log` です
  （4 MB を超えると `.1` へローテーションされます）。
- **タスクトレイが使用できない場合**（まれな制限環境。[よくある質問](#よくある質問)を参照）、アプリは代わりに
  ログコンソールウィンドウを開いて一度だけ通知を表示します。サービスは動作し続けるため、
  終了できない不可視のプロセスが残ることはありません。
- リアルタイムのログが必要な場合は、`inspirationer-console.exe` を使うか、通常の exe に `-console` を渡します。

---

## 機能一覧

| 領域 | 詳細 |
|---|---|
| **ひらめき** | 「＋ 新しいひらめき」または `Alt+N`。タイトルは省略でき、本文は完全な Markdown に対応 |
| **Markdown 編集** | 同梱の EasyMDE（CodeMirror + marked）：見出し、リスト、引用、コードブロック、表、画像、プレビュー、全画面。ツールバーのアイコンはインライン CSS の字形で、CDN に依存しません |
| **自動タイトル** | タイトルを空欄にした場合：AI を設定済みならモデルが要約し、未設定なら本文の先頭 N 文字を使用します（既定 10、変更可能）。カードにはタイトルの出所が表示されます |
| **タグとカテゴリ** | ひらめきごとに任意の数のタグと 1 つのカテゴリを付けられ、タイトルのすぐ下に色付きチップで表示されます。色は自動割り当てまたは任意に選択でき、サイドバーを右クリックして名前と色を変更できます |
| **検索** | 全文 / タイトルのみ / 本文のみ、複数キーワードの AND、先にサイドバーでカテゴリを選べば「カテゴリ内だけの検索」も可能 |
| **ショートカットキー** | すべてカスタマイズ可能（既定ではブラウザのショートカットと衝突しないよう `Alt` を使用）。新規作成 → 入力 → 保存のパイプラインを実現します |
| **AI 自動タグ付け** | ひらめきを選択（または全選択）→ AI が 1 件ずつタグとカテゴリを提案 → 確認・調整してから適用 |
| **パイプライン タグ付け** | 選択したひらめきを 1 件ずつ処理：閲覧、タグ付け、`Alt+J` で次へ。各ステップは即座に保存されます |
| **すべて選択** | ツールバーのチェックボックス / `Alt+A` / AI ダイアログ内の「すべての候補を選択」 |
| **整理** | ピン留め 📌、アーカイブ 🗄️、一括アーカイブ / 削除、カテゴリやタグによる絞り込み、3 種類の並べ替え |
| **ローカル永続化** | 変更のたびに JSON へアトミック書き込み。保存ごとにローカルスナップショットを作成（30 件保持）。ファイルが壊れた場合は起動時に最新のスナップショットから自動復旧します |
| **WebDAV バックアップ** | 定時自動バックアップ、今すぐバックアップ、リモートコピーの一覧、マージ / 置き換えによる復元、古いバックアップの自動削除 |
| **データの可搬性** | ライブラリ全体を JSON でエクスポートし、インポートして戻せます（マージまたは置き換え）。インポート / 復元の前に必ずローカルスナップショットを作成します |
| **インターフェース** | ダーク / ライトテーマ、カードの Markdown プレビュー表示の切り替え、削除時の確認、サイドバーのリアルタイム集計 |

---

## 多言語

UI は **English / 简体中文 / 日本語** に対応しており、言語はごく普通の設定項目です。

- **切り替え場所：** 設定 → 🎨 外観 → **言語**。切り替えは即座に反映され、再読み込みは不要です。
- **ブラウザに合わせる**（既定）：`auto` は `navigator.languages` から最も合うものを選びます
  （例：`zh-Hans` → 简体中文、`ja-JP` → 日本語）。どれにも一致しない場合は英語になります。
- **翻訳されるもの：** すべてのラベル、ボタン、プレースホルダー、トースト、確認ダイアログ、エディタの
  ツールバーのツールチップ、相対時刻（「3 minutes ago」/「3 分前」/「3 分钟前」）、**タスクトレイのメニュー**、
  Windows のネイティブダイアログ、そして **REST API のエラーメッセージ**。
- **翻訳されないもの：** あなた自身のコンテンツ —— ひらめき、タイトル、タグ名、カテゴリ名はあなたのものです。
- **永続化：** 選択内容は `data/settings.json` の `ui.language` に保存し、`localStorage` にも反映されるため、
  初回描画の時点ですでに正しい言語になっています。

### 言語を追加または改善する

1. **フロントエンドの文言**は [`web/i18n.js`](web/i18n.js) にあります：`en` / `zh-CN` / `ja` をキーとする
   3 つのフラットな辞書です。ブロックを 1 つコピーし、値を翻訳し、key と `{プレースホルダー}`
   （`{n}`、`{name}`、`{time}`…）はそのまま残してください。不足した key は自動的に英語へフォールバックするため、
   部分的な翻訳でも問題ありません。
2. **バックエンドの文言**（API エラー、タスクトレイのメニュー、ネイティブダイアログ）は
   [`internal/i18n/i18n.go`](internal/i18n/i18n.go) に `key → 言語 → テキスト` の形であります。
3. 新しい言語コードを `SUPPORTED_LANGS`（`web/app.js`）、`i18n.Supported()`
   （`internal/i18n/i18n.go`）、`web/index.html` の `Language` セレクト、そして
   `internal/model/model.go` の `ui.language` ホワイトリストに登録します。

### API の言語ネゴシエーション

すべてのエンドポイントは次の優先順位でリクエストの言語を尊重します。

1. `X-Lang` ヘッダー —— Web UI はすべてのリクエストで現在の言語を送信します。
2. `?lang=xx` クエリパラメータ。
3. `Accept-Language` ヘッダー。
4. いずれもない場合は保存された `ui.language` 設定、最後に英語へフォールバック。

```bash
curl -s -X POST http://127.0.0.1:8420/api/ai/suggest \
  -H 'X-Lang: ja' -H 'Content-Type: application/json' -d '{"ids":["nope"]}'
# {"error":"AI 機能が無効です。先に「設定 → AI」で API を設定してください"}
```

---

## ショートカットキー

既定値はすべて、ブラウザが使わない `Alt` の組み合わせです（`Ctrl+N/T/W/S/P/F/D`、`F5`、
`Alt+F/E/D/Home/←/→` などを避けています）。

| 動作 | 既定キー | 説明 |
|---|---|---|
| 新しいひらめき | `Alt+N` | どこからでもエディタを開きます |
| ひらめきを保存 | `Alt+S` | 保存してエディタを閉じます |
| 検索にフォーカス | `Alt+K` | 検索ボックスに移動して内容を全選択します |
| すべて選択 / 選択解除 | `Alt+A` | 現在の一覧のひらめきをすべて選択します |
| パイプライン：次へ | `Alt+J` | 現在のひらめきを保存して次へ移動します |
| プレビューを切り替え | `Alt+P` | エディタの Markdown プレビュー |
| 設定を開く | `Alt+O` | |
| ダイアログを閉じる | `Escape` | エディタが全画面の場合は先に全画面を終了します |

**カスタマイズ：** 設定 → ⌨️ ショートカット → 入力欄をクリック → 使いたいキーの組み合わせを押します
（`Backspace` でクリア）。重複したショートカットは赤く表示されます。

---

## AI の設定

設定 → 🤖 AI：

1. **AI 機能を有効にする** にチェックを入れます。
2. **サービスプリセット** を選ぶか、`API Base URL` + `モデル` を自分で入力します。
3. `API Key` を貼り付けます。
4. **🔌 接続テスト** をクリックします。

内蔵プリセット（OpenAI の `chat/completions` プロトコルに対応するサービスならどれでも使用できます）：

| サービス | Base URL | モデル例 |
|---|---|---|
| OpenAI | `https://api.openai.com/v1` | `gpt-4o-mini` |
| DeepSeek | `https://api.deepseek.com/v1` | `deepseek-chat` |
| Moonshot Kimi | `https://api.moonshot.cn/v1` | `moonshot-v1-8k` |
| 阿里通义 | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `qwen-plus` |
| 智谱 GLM | `https://open.bigmodel.cn/api/paas/v4` | `glm-4-flash` |
| Ollama（本地） | `http://127.0.0.1:11434/v1` | `qwen2.5:7b` |
| One-API / New-API | `http://127.0.0.1:3000/v1` | 任意 |

> Ollama などのローカルサービスには Key は不要です。Base URL に `localhost` / `127.0.0.1` が含まれる場合は
> Key を空欄にできます。
> **API Key は本機の `data/settings.json` にのみ書き込まれます**。このディレクトリは git 管理外であり、
> コミットされることも、どこかへアップロードされることもありません。

**AI が関与するのはちょうど 3 つです：** ① タイトルが空のときのタイトル生成、
② タグとカテゴリの提案、③ エディタ内の「✨ AI タイトル」ボタン。

---

## WebDAV 自動バックアップ

設定 → ☁️ WebDAV バックアップ：

| フィールド | 意味 |
|---|---|
| 自動バックアップを有効にする | 実行中は下の間隔で完全バックアップをアップロードします |
| WebDAV URL | 例：`https://dav.jianguoyun.com/dav/`（Nutstore） |
| ユーザー名 / パスワード | Nutstore、Nextcloud などではログインパスワードではなく**アプリパスワード**を使用してください |
| リモートフォルダ | 既定は `inspirationer`。存在しない場合は MKCOL で作成されます |
| バックアップ間隔 | 分単位、既定は 60 |
| リモートに保持する最大件数 | これを超える古いバックアップはリモートから削除されます（既定 10） |

動作確認済みのエンドポイント：

- **Nutstore（坚果云）**：`https://dav.jianguoyun.com/dav/`（メールアドレス + アプリパスワード）
- **Nextcloud / ownCloud**：`https://your-host/remote.php/dav/files/<user>/`
- **Synology**：`https://your-host:5006/<share>/`（WebDAV Server パッケージ）
- **Alist**：`http://127.0.0.1:5244/dav/`
- **その他**：`PROPFIND/PUT/GET/MKCOL/DELETE` に対応する任意のサービス

ボタン：**🔌 接続テスト**（未保存のフォームの値でも動作します）、**☁️ 今すぐバックアップ**、
**📥 リモートから復元**（ファイルごとに「マージ復元」/「置き換え復元」を選べ、先にローカルスナップショットを
作成します）、**💾 ローカルスナップショットを作成**。

リモートのファイル名は `inspirationer-backup-YYYYMMDD-HHMMSS.json` で、常に最新のバックアップを指す
`latest.json` も作成されます。

---

## AI 自動タグ付け & パイプラインでのタグ付け

どちらも**あなたが明示的に実行したとき**だけ動作します —— 何かが黙って変更されることはありません。

### 🤖 AI 自動タグ付け（一括）

1. ひらめきを選択します（または `Alt+A` を押します）。
2. ツールバーまたは一括操作バーの「🤖 AI タグ付け」をクリックします。
3. 提案を確認します —— タイトル、2〜4 個のタグ、1 つのカテゴリ、1 行の要約がひらめきごとに表示されます。
4. 同意できないものはチェックを外し、タグは追加 / 置き換えを選び、「すべての候補を選択」も使えます。
5. 「選択した候補を適用」をクリックします。

モデルには既存のタグとカテゴリが渡されるため、似たものを新しく作るのではなく**それらを再利用**します
（大文字小文字は区別しません）。新しく作られたものには色が自動で割り当てられます。

### ⚡ パイプライン タグ付け（1 件ずつ）

1. ひらめきを選択 → 「⚡ パイプライン」。
2. 左側にひらめきが読み取り専用で表示され、右側でカテゴリを選び、タグを切り替えたり作成したりできます。
3. `Alt+J`（または「保存して次へ進む」）を押して保存し、次へ進みます。
4. 「スキップ」は保存せずに次へ、「← 前へ」は前に戻り、「パイプラインを終了」でいつでも停止できます。

進捗は `3 / 12` のように表示され、各ひらめきは操作のたびに保存されるため、ウィンドウを閉じても失われません。

---

## 検索

検索ボックス（`Alt+K` でフォーカス）：

- **検索範囲：**`全文`（タイトル + 本文 + タグ名 + カテゴリ名）、`タイトルのみ`、`本文のみ`。
- **複数キーワード：** スペース区切りで、すべてに一致する必要があります（例：`markdown 写作`）。
- **スコープ検索：** 先にサイドバーでカテゴリやタグを選んでから検索 —— つまり「カテゴリ内だけの検索」です。
- 大文字小文字は区別せず、デバウンス付き（220 ms）です。`✕` でクリアします。

サイドバーのクイックビュー：すべてのひらめき、未分類、ピン留め、アーカイブ。

---

## データの保存場所とバックアップ

既定のデータフォルダは実行ファイルの隣の `./data` です（`-data` で変更できます。作業ディレクトリが
書き込み不可の場合は実行ファイルのフォルダへフォールバックします）。

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

- すべての書き込みは「一時ファイル + アトミックなリネーム」で行われ、変更は即座に反映されます。
- 最新のスナップショットが 6 時間より古い場合、起動時に 1 件追加されます。
- 設定 → 💾 データ では、**完全バックアップをエクスポート**（JSON としてダウンロード）と
  **インポート**（マージまたは置き換え）ができます。どちらの場合も先にローカルスナップショットを作成するため、
  いつでもロールバックできます。

---

## プライバシーとセキュリティ

- **あなたのデータはあなたのものです。** すべてはローカルの `data/` フォルダにあり、テレメトリも
  解析も外部への送信もありません。
- **あなたが要求しない限りネットワークを使いません。** リクエストが発生するのは、AI 機能（*あなたが*
  設定したエンドポイント）を使うとき、または WebDAV バックアップを行うときだけです。
- **秘密情報はローカルに留まります。** AI API キーと WebDAV パスワードは `data/settings.json` に書き込まれ、
  これは `.gitignore` で除外されているためコミットされません。リポジトリにもその履歴にも、鍵や
  個人のパスは含まれていません。
- **フロントエンドは完全にオフライン。** EasyMDE、marked、DOMPurify はバイナリに埋め込まれており、ページは
  いかなる CDN も参照しません。レンダリングされた Markdown は XSS を防ぐため DOMPurify を通します。
- **サービスは既定で `127.0.0.1` をリッスンし、認証はありません** —— 単一ユーザー向けのデスクトップツールです。
  `-addr 0.0.0.0:8420` で公開する場合は、前に認証付きのリバースプロキシを置いてください。
- テストスクリプトに含まれるのは使い捨ての資格情報（`mock-user` / `mock-pass`、`mock-api-key`）だけで、
  `127.0.0.1` 上のローカルなモックサーバーを指しています。
- **脆弱性の報告**：詳しくは [SECURITY.md](SECURITY.md) を参照してください。公開 issue ではなく、GitHub の非公開の脆弱性報告（Security タブ）をご利用ください。
- リポジトリが公開された後は、`main` へのプッシュごとに CI で CodeQL による静的セキュリティ解析が実行されます。

---

## コマンドライン引数

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

例：

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

フロントエンドは単なる静的ページで、その動作はすべて REST から利用できるため、スクリプト化は簡単です。
すべてのレスポンスは JSON です。ローカライズされたエラーを得るには `X-Lang: en|zh-CN|ja` を付けてください。

| メソッド | パス | 説明 |
|--------|------|------|
| GET | `/api/bootstrap` | 設定、タグ、カテゴリ、件数を一度に取得 |
| GET | `/api/snippets` | 一覧 / 検索：`query` `mode` `categoryId` `tagIds` `archive` `sort` `limit` |
| POST | `/api/snippets` | 新規作成（タイトルが空の場合は自動生成） |
| GET/PUT/DELETE | `/api/snippets/{id}` | 1 件の取得 / 置き換え / 削除 |
| PATCH | `/api/snippets/{id}` | 部分更新（`title` `content` `tags` `categoryId` `pinned` `archived` `addTags`） |
| POST | `/api/snippets/bulk` | 一括：`delete` `assign` `archive` `unarchive` `pin` `unpin` |
| GET/POST | `/api/tags`、`/api/categories` | 一覧 / 作成 |
| PUT/DELETE | `/api/tags/{id}`、`/api/categories/{id}` | 名前と色の変更、削除 |
| GET/PUT | `/api/settings` | 設定の読み取り / 書き込み |
| POST | `/api/ai/title` | タイトルを生成 |
| POST | `/api/ai/suggest` | 複数のひらめきにタグとカテゴリを提案 |
| POST | `/api/ai/apply` | 提案を適用 |
| POST | `/api/ai/test` | AI の接続を確認 |
| POST | `/api/webdav/test` \| `/api/webdav/backup` \| `/api/webdav/restore` | テスト / バックアップ / 復元 |
| GET | `/api/webdav/list` | リモートのバックアップ一覧 |
| GET | `/api/backup/export` \| POST `/api/backup/import` \| POST `/api/backup/local` | エクスポート / インポート / ローカルスナップショット |
| GET | `/api/stats` | 件数 |

```bash
curl -X POST http://127.0.0.1:8420/api/snippets \
  -H "Content-Type: application/json" \
  -d '{"content":"Note to self: use WebDAV for backups","tags":[],"categoryId":""}'
```

---

## 開発とテスト

```powershell
.\build.ps1              # GUI build (-H=windowsgui, no console window)
.\build.ps1 -Console     # also produce inspirationer-console.exe (console, for troubleshooting)
.\build.ps1 -Icon        # regenerate the icon and Windows resources (needs python + Pillow + rsrc)
.\dev.ps1                # dev mode: serve the front end from web/, just refresh the browser
.\scripts\make-release.ps1   # ビルド + release/ の組み立て + GitHub Release にそのまま上げられる zip を生成
```

> PowerShell がこれらのスクリプトを「デジタル署名されていません」として拒否する場合は、
> [よくある質問](#よくある質問)を参照してください。

**リリースのパッケージング**

```powershell
.\scripts\make-release.ps1            # ビルドしてからパッケージング
.\scripts\make-release.ps1 -SkipBuild # 既存の exe だけでパッケージング
```

`release/` はそのままアップロードできる自己完結したパッケージになります：

```
release/
├── inspirationer.exe                            本体（GUI、タスクトレイ、コンソールウィンドウなし）
├── inspirationer-console.exe                    トラブルシューティング用のコンソール版
├── start-inspirationer.bat                      ダブルクリックで起動（意図的に ASCII のみ）
├── README.md  LICENSE                           ドキュメントとライセンス
├── HOW-TO-RUN.txt                               ダウンロードした人向けの 3 言語クイックスタート
├── inspirationer-v<version>-windows-amd64.zip   ← GitHub Release にアップロードするのはこれ
└── SHA256SUMS.txt                               上記ファイルのチェックサム
```

**テスト**（先にサーバーを起動してください。AI / WebDAV / UI のスイートにはモックサービスも必要です）：

```powershell
# terminal 1 — the app (no tray, no browser, so tests are not disturbed)
.\inspirationer.exe -open=false -tray=false

# terminal 2 — mock AI + mock WebDAV (local throwaway credentials only)
node scripts\mock-services.mjs

# terminal 3 — the suites
node scripts\check-i18n.mjs         # 翻訳の完全性チェック：282 key × en/zh-CN/ja
go test ./...                      # 単体テスト：トレイの構造体レイアウト、ICO 解析、Win32 関数の解決
node scripts\api-smoke.mjs         # 62 項目：CRUD、タグ、検索、一括操作、バックアップ、i18n エラー
node scripts\ai-webdav-test.mjs    # 33 項目：AI タイトル、AI タグ付け、WebDAV のバックアップ / 削除 / 復元
node scripts\ui-test.mjs           # 61 項目：ヘッドレスブラウザでの実 UI（言語切り替えを含む）
node scripts\auto-backup-test.mjs  # 定時バックアップ（1〜2 分かかります）
```

- `ui-test.mjs` は Edge/Chrome を自動的に見つけ、CDP 経由でヘッドレスブラウザを操作します。レンダリング、
  すべてのショートカット、AI ダイアログ、パイプライン、設定パネル、**英 / 中 / 日の切り替え**（翻訳 key が
  DOM に漏れていないこと、選択が再読み込み後も保持されることを含む）を検証し、**捕捉されていない JS 例外や
  console エラーがないこと**をアサートします。
- `go test` には `TestWin32ProcsResolve` が含まれており、アプリが使うすべての Win32 関数を解決します ——
  DLL の間違いやシンボル名の綴り間違いは、実行時に panic するのではなくテストの失敗になります。
- 実際にタスクトレイのアイコンを作成する場合（すぐに削除されます）：

```powershell
$env:IH_TRAY_TEST=1; go test ./internal/tray/ -run TestTrayStartInThisEnvironment -v
```

---

## アーキテクチャの説明

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

知っておくと役立つ設計判断：

- **単一ファイルでの配布** —— フロントエンドは `go:embed` で埋め込まれるため、exe を 1 つコピーするだけで十分です。
  失くす心配のある隣接リソースフォルダはありません。
- **Go の依存ゼロ** —— タスクトレイのアイコン、メッセージボックス、単一インスタンスの mutex、
  「ブラウザで開く」はすべて標準ライブラリを通じて Win32 に対して実装されており、CGO や依存関係の
  変動を避けています。
- **データベースより可搬なデータ** —— 手で修正でき、任意のクラウドドライブと同期できる人間が読める JSON です。
  アトミック書き込み、スナップショット、破損時の復旧が安全性を保ちます。
- **英語が基準言語** —— バックエンドのエラーは既定で英語（API 利用者やログにとって好都合）であり、
  HTTP の境界で翻訳されます。一方、UI はすべての翻訳を 1 つの JS ファイルに保持します。

---

## サードパーティコンポーネント

フロントエンドは 3 つのライブラリをローカルに同梱しています（実行時に CDN を使用しません）：

| コンポーネント | バージョン | 用途 | ライセンス |
|-----------|---------|---------|---------|
| [EasyMDE](https://github.com/Ionaru/easy-markdown-editor) | 2.18.0 | Markdown エディタ（CodeMirror + marked を同梱） | MIT |
| [marked](https://github.com/markedjs/marked) | 12.0.2 | カードプレビューの Markdown レンダリング | MIT |
| [DOMPurify](https://github.com/cure53/DOMPurify) | 3.1.6 | レンダリングされた Markdown のサニタイズ | Apache-2.0 / MPL-2.0 |

Go 側にサードパーティ依存はありません。タスクトレイのアイコン、メッセージボックス、単一インスタンスの確認、
ブラウザの起動はすべて `syscall` を通じて Win32 API を直接呼び出しています。

---

## よくある質問

**Q：ダブルクリックしましたが、コンソールウィンドウが出ません。動いているのでしょうか？**
タスクトレイ（右下）の 💡 アイコンを探してください。Windows 11 では「隠れているインジケーターを表示する」に
収められている場合があります —— ドラッグして固定してください。ログは `data\logs\inspirationer.log` です。

**Q：終了するには？**
トレイアイコンを右クリック → 終了。ほかに `inspirationer-console.exe` または `-console` 付きで起動して
`Ctrl+C` を押す方法や、タスクマネージャーでプロセスを終了する方法もあります（データはすでにディスク上にあります）。

**Q：ブラウザが開きませんでした。**
ログに「asked the default browser to open」があるか確認してください。アドレスを手動で開くこともいつでもできます。
`-open=false` の場合は意図的な動作です。

**Q：タスクトレイの登録に失敗しました（`Shell_NotifyIcon … Access is denied`）。**
それは整合性レベルの制限です：Windows の UIPI は**低い整合性レベル**のプロセスがエクスプローラーへ
タスクトレイアイコンを登録することを許可しません。ログにはそれが明記されます（`integrity=Low`）。
サンドボックスや制限付きコンテナ、あるいはフィルタ済みトークンでアプリが起動された場合に発生します。
通常のデスクトップでダブルクリックした場合は代わりに `tray icon ready` と表示されます。アプリは代わりに
ログコンソールを開いて知らせますが、サービス自体は影響を受けません。

**Q：ポートが使用中です。**
ログに実際のアドレスが出力されます（アプリはポート範囲を順に試します）。`-addr 127.0.0.1:9000` を
渡すこともできます。

**Q：別のマシンからアクセスできますか？**
`-addr 0.0.0.0:8420` で可能ですが、このアプリには**認証がありません** —— 公開する前に認証付きの
リバースプロキシを前に置いてください。

**Q：データは失われますか？**
すべての変更はアトミックに書き込まれます。`data/backups/` には 30 件のスナップショットが保持され、
壊れたファイルは起動時に最新のものから復旧されます。WebDAV バックアップと併用すればリスクは無視できる程度です。

**Q：ショートカットが反応しません。**
① ブラウザのウィンドウがフォーカスされている必要があります。② 一部のブラウザは `Alt` の組み合わせを
予約しているため、代わりに `Alt+Shift+X` のような組み合わせを記録してください。③ ダイアログが開いている間は
そのダイアログ自身のショートカットだけが有効です（意図的な仕様）。`Escape` でいつでも閉じられます。

**Q：PowerShell で .ps1 スクリプトが「デジタル署名されていません」と表示される**
実行ポリシーが未署名のスクリプトを止めています。ポリシーを回避したセッションで実行するか、ダウンロード後に一度ファイルのブロックを解除してください：

```powershell
powershell -ExecutionPolicy Bypass -File .\build.ps1
# or, for a cloned/downloaded copy:
Get-ChildItem -Recurse -Include *.ps1 | Unblock-File
```

スクリプトがなくても構築できます：同等の `go build` コマンドは
[30 秒で使い始める](#30-秒で使い始める)にあり、`启动灵感管理器.bat` と `start-inspirationer.bat` は
通常の cmd バッチファイルなので PowerShell のポリシーの影響を受けません。（`.ps1` ファイルは UTF-8 BOM 付きで
保存されているため、Windows PowerShell 5.1 でも日本語や中国語のコメントを正しく読み取れます。）

**Q：AI のタグ付けは有料ですか？**
選んだサービスによります。ローカルの Ollama モデルなら無料でオフラインです。AI の操作を実行しない限り、
どこにも送信されません。

**Q：最初からやり直すには？**
トレイ → 終了 してから `data/` フォルダを削除し、もう一度起動します。

---

## ライセンス

**Apache License 2.0** の下でライセンスされています —— [LICENSE](LICENSE) を参照してください。

サードパーティのフロントエンドライセンスは[サードパーティコンポーネント](#サードパーティコンポーネント)
（MIT / Apache-2.0 / MPL-2.0）に記載されています。これらのファイルは未改変のまま `web/vendor/` に
同梱されています。
