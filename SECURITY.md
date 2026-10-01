# Security Policy

**English** | [简体中文](#简体中文) | [日本語](#日本語)

## Supported versions

| Version | Supported |
|---|---|
| 1.2.x | ✅ |
| < 1.2 | ❌ — please upgrade to the latest release |

## Reporting a vulnerability

Please **do not open a public issue** for security problems.

1. **Preferred** — GitHub: open this repository's **Security** tab → **Report a vulnerability**.
   That creates a private security advisory which only the maintainer can read.
2. **Alternative** — email the maintainer; the address is listed on the
   [@idiomeo](https://github.com/idiomeo) profile.

Please include: the affected version or commit, your OS, reproduction steps or a proof of concept,
the impact you believe it has, and any suggested fix. We aim to acknowledge reports within
**7 days** and to ship a fix or a mitigation as soon as it is practical. You will be credited in the
release notes unless you ask us not to.

## Scope

**In scope:** the Go server and its REST API, the browser front end (including Markdown rendering),
the Windows tray icon and native dialogs, the build/release scripts, and the CI workflows.

**Out of scope** (by design — the app is a local, single-user tool, and the README says so):

- another user or process on the same machine reading your `data/` folder. The AI API key and the
  WebDAV password are stored there in plain text, exactly like a normal config file;
- running the service on a public interface. It has **no authentication** — if you expose it with
  `-addr 0.0.0.0:8420`, put an authenticating reverse proxy in front of it;
- vulnerabilities in the bundled front-end libraries (EasyMDE, marked, DOMPurify). Please report
  those upstream, but do tell us as well so we can refresh the vendored copy;
- anything that requires an attacker to already have local code execution or physical access.

## What this project already does

- **Local by default:** no telemetry, no analytics, no phoning home. Network requests happen only
  when *you* trigger an AI feature (to the endpoint *you* configured) or a WebDAV backup.
- **Loopback bind:** the service listens on `127.0.0.1` unless you explicitly change `-addr`.
- **Sanitised rendering:** Markdown reaching the DOM is filtered through DOMPurify.
- **No secrets in the repository:** the AI key and WebDAV password live only in `data/settings.json`,
  which `.gitignore` excludes. The only credential-looking strings in the tree are the throwaway
  mocks (`mock-user` / `mock-pass` / `mock-api-key`) that point at local test servers.
- **Automated checks:** CI runs `gofmt`, `go vet`, `go test`, a translation-completeness check, and
  CodeQL static analysis (CodeQL is free on public repositories; it stays disabled while this
  repository is private without GitHub Advanced Security).

---

## 简体中文

## 安全政策

### 上报漏洞

**请不要开公开 issue。** 首选方式：在本仓库的 **Security** 标签页 → **Report a vulnerability**，
这会创建一个只有维护者可见的私密安全通告；备选方式是给维护者发邮件（地址见
[@idiomeo](https://github.com/idiomeo) 主页）。

请尽量附上：受影响的版本或提交、操作系统、复现步骤或 PoC、你认为的影响范围，以及可能的修复建议。
我们会在 **7 天内**确认收到，并尽快发布修复或缓解措施；除你要求匿名外，我们会在发布说明里致谢。

### 范围

**属于范围内**：Go 服务端与 REST 接口、前端（含 Markdown 渲染）、Windows 托盘与原生对话框、
构建与发布脚本、CI 工作流。

**不属于范围**（这是本地单用户工具的既定设计，README 中已说明）：

- 同一台机器上的其他用户/进程读取你的 `data/` 目录——AI API Key 与 WebDAV 密码以明文存在那里，
  和普通配置文件一样；
- 把服务暴露到公网：它**没有登录鉴权**，需要自己加带认证的反向代理（`-addr 0.0.0.0:8420`）；
- 内置前端库（EasyMDE、marked、DOMPurify）自身的漏洞——请同时上报给上游，并告诉我们以便更新内置副本；
- 需要攻击者已经拥有本机代码执行权限或物理接触的场景。

### 项目已有的安全设计

- **默认全本地**：无遥测、无统计；只有你主动使用 AI 功能（请求发往你自己配置的地址）或 WebDAV 备份时才会联网。
- **默认只监听 `127.0.0.1`**。
- **渲染前消毒**：进入 DOM 的 Markdown 会经 DOMPurify 过滤。
- **仓库内无任何真实密钥**：AI Key 与 WebDAV 密码只存在于被 `.gitignore` 排除的 `data/settings.json`；
  仓库里唯一像凭据的字符串是指向本地测试服务的假凭据（`mock-user` / `mock-pass` / `mock-api-key`）。
- **自动化检查**：CI 会跑 `gofmt`、`go vet`、`go test`、文案完整性检查，以及 CodeQL 静态扫描
  （CodeQL 对公开仓库免费；本仓库在私有且未启用 GitHub Advanced Security 期间会自动跳过）。

---

## 日本語

## セキュリティポリシー

### 脆弱性の報告

**公開 issue は作らないでください。** 第一の方法は、このリポジトリの **Security** タブ →
**Report a vulnerability** です（メンテナだけが閲覧できる非公開のセキュリティアドバイザリが作成されます）。
第二の方法は、メンテナへのメールです（アドレスは [@idiomeo](https://github.com/idiomeo) のプロフィールに記載）。

可能であれば、影響を受けるバージョンまたはコミット、OS、再現手順または PoC、想定される影響、
修正案を添えてください。**7 日以内**に受領の連絡をし、できるだけ早く修正または緩和策をリリースします。
匿名を希望されない限り、リリースノートで謝辞を掲載します。

### 対象範囲

**対象**：Go サーバーと REST API、フロントエンド（Markdown レンダリングを含む）、
Windows のタスクトレイとネイティブダイアログ、ビルド／リリーススクリプト、CI ワークフロー。

**対象外**（ローカル単一ユーザー向けツールという設計上の前提。README にも記載）：

- 同じマシン上の他ユーザーや他プロセスが `data/` を読むこと。AI API キーと WebDAV パスワードは
  通常の設定ファイルと同様に平文で保存されます。
- サービスを公衆網に公開すること。認証機能は**ありません**（`-addr 0.0.0.0:8420` で公開する場合は、
  認証付きリバースプロキシを前段に置いてください）。
- 同梱のフロントエンドライブラリ（EasyMDE、marked、DOMPurify）自体の脆弱性。上流にも報告しつつ、
  同梱版を更新できるよう私たちにもお知らせください。
- すでにローカルでのコード実行権限や物理的なアクセスを持つ攻撃者を前提とするもの。

### 本プロジェクトが既に実施していること

- **既定で完全ローカル**：テレメトリも解析もなく、外部通信は行いません。通信が発生するのは、
  あなたが AI 機能を使ったとき（設定した接続先）と WebDAV バックアップのときだけです。
- **既定で `127.0.0.1` のみ待ち受け**。
- **レンダリングの無害化**：DOM に渡す Markdown は DOMPurify でフィルタします。
- **リポジトリに実鍵はありません**：AI キーと WebDAV パスワードは `.gitignore` で除外された
  `data/settings.json` にのみ存在します。ツリー内で資格情報に見えるものは、ローカルのテスト
  サーバーを指す捨て駒（`mock-user` / `mock-pass` / `mock-api-key`）だけです。
- **自動チェック**：CI で `gofmt`、`go vet`、`go test`、翻訳の完全性チェック、および CodeQL に
  よる静的解析を実行します（CodeQL は公開リポジトリでは無料。GitHub Advanced Security のない
  非公開リポジトリの間は自動的にスキップされます）。
