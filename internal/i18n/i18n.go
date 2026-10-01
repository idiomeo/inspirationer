// Package i18n 提供后端（HTTP 接口、系统托盘、原生对话框）的多语言文案。
//
// 设计要点：
//   - 英文是基准语言，任何缺失的翻译都会回退到英文，再回退到 key 本身；
//   - 领域包（ai / webdav / store）用 i18n.Errorf("code", args...) 返回带错误码的错误，
//     这样服务端就能按请求语言本地化，而日志与 API 默认输出仍是英文；
//   - 语言来自请求头 X-Lang，其次 ?lang= 参数，最后取已保存的设置。
package i18n

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// 支持的语言。
const (
	English  = "en"
	Chinese  = "zh-CN"
	Japanese = "ja"
	Auto     = "auto"
)

// Supported 返回可选语言（用于设置界面）。
func Supported() []string { return []string{Auto, English, Chinese, Japanese} }

// Normalize 把任意语言标签（zh、zh-Hans、ja-JP、en-US…）归一化为受支持的语言。
func Normalize(tag string) string {
	t := strings.ToLower(strings.TrimSpace(tag))
	switch {
	case t == "":
		return English
	case strings.HasPrefix(t, "zh"):
		return Chinese
	case strings.HasPrefix(t, "ja"):
		return Japanese
	case strings.HasPrefix(t, "en"):
		return English
	}
	for _, l := range []string{English, Chinese, Japanese} {
		if strings.ToLower(l) == t {
			return l
		}
	}
	return English
}

// FromSetting 把设置里的语言值（可能是 "auto"）解析成实际语言。
func FromSetting(v string) string {
	if strings.TrimSpace(v) == "" || strings.EqualFold(strings.TrimSpace(v), Auto) {
		return English
	}
	return Normalize(v)
}

// FromRequest 按 X-Lang 请求头 → ?lang= 查询参数 → fallback 的顺序确定语言。
func FromRequest(r *http.Request, fallback string) string {
	if r != nil {
		if v := r.Header.Get("X-Lang"); strings.TrimSpace(v) != "" {
			return Normalize(v)
		}
		if v := r.URL.Query().Get("lang"); strings.TrimSpace(v) != "" {
			return Normalize(v)
		}
		if v := r.Header.Get("Accept-Language"); strings.TrimSpace(v) != "" {
			// 只取第一个语言标签，忽略 q 权重
			first := strings.Split(v, ",")[0]
			first = strings.Split(first, ";")[0]
			if strings.TrimSpace(first) != "" {
				return Normalize(first)
			}
		}
	}
	if strings.TrimSpace(fallback) != "" {
		return FromSetting(fallback)
	}
	return English
}

// Error 是带错误码的错误，Error() 返回英文文案（供日志与默认 API 输出）。
type Error struct {
	Code string
	Args []interface{}
}

func (e *Error) Error() string { return render(English, e.Code, e.Args...) }

// Errorf 生成带错误码的错误。
func Errorf(code string, args ...interface{}) *Error {
	return &Error{Code: code, Args: args}
}

// Translate 把任意 error 渲染成指定语言的文案。
func Translate(lang string, err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	// errors.As 兼容 fmt.Errorf("...: %w", err) 的包装形式
	if errors.As(err, &e) {
		return render(lang, e.Code, e.Args...)
	}
	return err.Error()
}

// T 按键取文案；参数里的 error 会先被本地化，便于拼出「保存失败：xxx」这类句子。
func T(lang, key string, args ...interface{}) string {
	return render(lang, key, args...)
}

func render(lang, key string, args ...interface{}) string {
	msgs, ok := table[key]
	if !ok {
		return key
	}
	s := msgs[Normalize(lang)]
	if s == "" {
		s = msgs[English]
	}
	if s == "" {
		s = key
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, localizeArgs(lang, args)...)
}

// localizeArgs 把参数里的 error 换成对应语言的文案。
func localizeArgs(lang string, args []interface{}) []interface{} {
	out := make([]interface{}, len(args))
	for i, a := range args {
		if err, ok := a.(error); ok {
			out[i] = Translate(lang, err)
			continue
		}
		out[i] = a
	}
	return out
}

// table：key → 语言 → 文案。英文为基准，缺失自动回退。
var table = map[string]map[string]string{
	/* ---------------------------------------------------------- 通用 HTTP */
	"err.unknownEndpoint": {
		English:  "Unknown endpoint",
		Chinese:  "未知接口",
		Japanese: "不明なエンドポイントです",
	},
	"err.methodNotAllowed": {
		English:  "Method not allowed",
		Chinese:  "方法不支持",
		Japanese: "このメソッドには対応していません",
	},
	"err.badBody": {
		English:  "Failed to parse request body: %v",
		Chinese:  "请求体解析失败：%v",
		Japanese: "リクエストボディの解析に失敗しました：%v",
	},
	"err.unknownAction": {
		English:  "Unknown action: %s",
		Chinese:  "未知操作：%s",
		Japanese: "不明な操作です：%s",
	},

	"err.plain": {
		English:  "%v",
		Chinese:  "%v",
		Japanese: "%v",
	},
	/* ---------------------------------------------------------- 灵感 */
	"err.snippetNotFound": {
		English:  "Snippet not found",
		Chinese:  "灵感不存在",
		Japanese: "ひらめきが見つかりません",
	},
	"err.emptyContent": {
		English:  "Content cannot be empty",
		Chinese:  "内容不能为空",
		Japanese: "内容を入力してください",
	},
	"err.saveFailed": {
		English:  "Failed to save: %v",
		Chinese:  "保存失败：%v",
		Japanese: "保存に失敗しました：%v",
	},
	"err.deleteFailed": {
		English:  "Failed to delete: %v",
		Chinese:  "删除失败：%v",
		Japanese: "削除に失敗しました：%v",
	},
	"err.bulkFailed": {
		English:  "Bulk update failed: %v",
		Chinese:  "批量修改失败：%v",
		Japanese: "一括更新に失敗しました：%v",
	},
	"err.noSelection": {
		English:  "No snippets selected",
		Chinese:  "未选择任何灵感",
		Japanese: "ひらめきが選択されていません",
	},
	"err.settingsSaveFailed": {
		English:  "Failed to save settings: %v",
		Chinese:  "保存设置失败：%v",
		Japanese: "設定の保存に失敗しました：%v",
	},

	/* ---------------------------------------------------------- 标签 / 分类 */
	"err.tagNotFound": {
		English:  "Tag not found",
		Chinese:  "标签不存在",
		Japanese: "タグが見つかりません",
	},
	"err.categoryNotFound": {
		English:  "Category not found",
		Chinese:  "分类不存在",
		Japanese: "カテゴリが見つかりません",
	},
	"err.emptyTagName": {
		English:  "Tag name cannot be empty",
		Chinese:  "标签名不能为空",
		Japanese: "タグ名を入力してください",
	},
	"err.emptyCategoryName": {
		English:  "Category name cannot be empty",
		Chinese:  "分类名不能为空",
		Japanese: "カテゴリ名を入力してください",
	},
	"err.invalidTagTarget": {
		English:  "Invalid target tag",
		Chinese:  "目标标签无效",
		Japanese: "マージ先のタグが不正です",
	},

	/* ---------------------------------------------------------- 标题生成 */
	"warn.titleAiFallback": {
		English:  "AI failed to generate a title; used the first characters of the body instead: %v",
		Chinese:  "AI 生成标题失败，已改用正文截取：%v",
		Japanese: "AI によるタイトル生成に失敗したため、本文の先頭を使用しました：%v",
	},
	"warn.titleTruncated": {
		English:  "AI API is not configured; the title was taken from the first %d characters of the body",
		Chinese:  "未配置 AI API，标题取正文前 %d 个字",
		Japanese: "AI API が未設定のため、本文の先頭 %d 文字をタイトルにしました",
	},

	/* ---------------------------------------------------------- AI */
	"err.aiDisabled": {
		English:  "AI features are disabled",
		Chinese:  "AI 功能未启用",
		Japanese: "AI 機能が無効です",
	},
	"err.aiDisabledHint": {
		English:  "AI features are disabled — enable and configure the AI API in Settings → AI first",
		Chinese:  "AI 功能未启用，请先在设置中配置 AI API",
		Japanese: "AI 機能が無効です。先に「設定 → AI」で API を設定してください",
	},
	"err.aiNoKey": {
		English:  "No AI API key configured",
		Chinese:  "未配置 AI API Key",
		Japanese: "AI API キーが設定されていません",
	},
	"err.aiRequestFailed": {
		English:  "AI request failed: %v",
		Chinese:  "请求 AI 接口失败：%v",
		Japanese: "AI API へのリクエストに失敗しました：%v",
	},
	"err.aiBadStatus": {
		English:  "AI API returned %d: %s",
		Chinese:  "AI 接口返回 %d：%s",
		Japanese: "AI API が %d を返しました：%s",
	},
	"err.aiBadResponse": {
		English:  "Failed to parse the AI response: %v",
		Chinese:  "解析 AI 响应失败：%v",
		Japanese: "AI の応答を解析できませんでした：%v",
	},
	"err.aiApiError": {
		English:  "AI API error: %s",
		Chinese:  "AI 接口错误：%s",
		Japanese: "AI API エラー：%s",
	},
	"err.aiEmptyResponse": {
		English:  "The AI API returned no content",
		Chinese:  "AI 接口未返回内容",
		Japanese: "AI API から内容が返されませんでした",
	},

	/* ---------------------------------------------------------- WebDAV */
	"err.webdavNoUrl": {
		English:  "WebDAV URL is not configured",
		Chinese:  "未配置 WebDAV 地址",
		Japanese: "WebDAV の URL が設定されていません",
	},
	"err.webdavDisabled": {
		English:  "WebDAV is disabled or has no URL configured",
		Chinese:  "WebDAV 未启用或未配置地址",
		Japanese: "WebDAV が無効、または URL が未設定です",
	},
	"err.webdavBadUrl": {
		English:  "Invalid WebDAV URL: %v",
		Chinese:  "WebDAV 地址无效：%v",
		Japanese: "WebDAV の URL が不正です：%v",
	},
	"err.webdavMkcol": {
		English:  "Failed to create the remote folder: %v",
		Chinese:  "创建远端目录失败：%v",
		Japanese: "リモートフォルダの作成に失敗しました：%v",
	},
	"err.webdavMkcolStatus": {
		English:  "Failed to create remote folder %s: HTTP %d %s",
		Chinese:  "创建远端目录 %s 失败：HTTP %d %s",
		Japanese: "リモートフォルダ %s の作成に失敗しました：HTTP %d %s",
	},
	"err.webdavUpload": {
		English:  "Upload failed: %v",
		Chinese:  "上传失败：%v",
		Japanese: "アップロードに失敗しました：%v",
	},
	"err.webdavUploadStatus": {
		English:  "Upload failed: HTTP %d %s",
		Chinese:  "上传失败：HTTP %d %s",
		Japanese: "アップロードに失敗しました：HTTP %d %s",
	},
	"err.webdavDownload": {
		English:  "Download failed: %v",
		Chinese:  "下载失败：%v",
		Japanese: "ダウンロードに失敗しました：%v",
	},
	"err.webdavDownloadStatus": {
		English:  "Download failed: HTTP %d %s",
		Chinese:  "下载失败：HTTP %d %s",
		Japanese: "ダウンロードに失敗しました：HTTP %d %s",
	},
	"err.webdavDelete": {
		English:  "Delete failed: HTTP %d %s",
		Chinese:  "删除失败：HTTP %d %s",
		Japanese: "削除に失敗しました：HTTP %d %s",
	},
	"err.webdavList": {
		English:  "Failed to list the remote folder: %v",
		Chinese:  "列目录失败：%v",
		Japanese: "リモートフォルダの一覧取得に失敗しました：%v",
	},
	"err.webdavListStatus": {
		English:  "Failed to list the remote folder: HTTP %d %s",
		Chinese:  "列目录失败：HTTP %d %s",
		Japanese: "リモートフォルダの一覧取得に失敗しました：HTTP %d %s",
	},
	"err.webdavParseList": {
		English:  "Failed to parse the remote listing: %v",
		Chinese:  "解析目录列表失败：%v",
		Japanese: "一覧の解析に失敗しました：%v",
	},
	"webdav.testOk": {
		English:  "Connection OK — %d entries visible at the root",
		Chinese:  "连接成功，根目录可见 %d 个条目",
		Japanese: "接続に成功しました。ルートに %d 件あります",
	},
	"err.webdavBackupFailed": {
		English:  "Backup failed: %v",
		Chinese:  "备份失败：%v",
		Japanese: "バックアップに失敗しました：%v",
	},

	/* ---------------------------------------------------------- 备份 / 恢复 */
	"err.snapshotFailed": {
		English:  "Local snapshot failed: %v",
		Chinese:  "本地快照失败：%v",
		Japanese: "ローカルスナップショットに失敗しました：%v",
	},
	"err.emptyBackup": {
		English:  "The backup data is empty or malformed",
		Chinese:  "备份数据为空或格式不正确",
		Japanese: "バックアップデータが空か、形式が正しくありません",
	},
	"err.importFailed": {
		English:  "Import failed: %v",
		Chinese:  "导入失败：%v",
		Japanese: "インポートに失敗しました：%v",
	},
	"err.restoreFailed": {
		English:  "Restore failed: %v",
		Chinese:  "恢复失败：%v",
		Japanese: "復元に失敗しました：%v",
	},
	"err.noRemoteFile": {
		English:  "No remote file specified",
		Chinese:  "未指定文件",
		Japanese: "ファイルが指定されていません",
	},
	"err.invalidBackupFile": {
		English:  "The remote file is not a valid backup: %v",
		Chinese:  "远端文件不是有效的备份：%v",
		Japanese: "リモートファイルが有効なバックアップではありません：%v",
	},

	/* ---------------------------------------------------------- 存储层 */
	"err.dataDir": {
		English:  "Failed to create the data folder: %v",
		Chinese:  "创建数据目录失败：%v",
		Japanese: "データフォルダの作成に失敗しました：%v",
	},
	"err.readFile": {
		English:  "Failed to read %s: %v",
		Chinese:  "读取 %s 失败：%v",
		Japanese: "%s の読み込みに失敗しました：%v",
	},
	"err.parseFile": {
		English:  "Failed to parse %s: %v",
		Chinese:  "解析 %s 失败：%v",
		Japanese: "%s の解析に失敗しました：%v",
	},

	/* ---------------------------------------------------------- 托盘与对话框 */
	"ui.appName": {
		English:  "Inspirationer",
		Chinese:  "灵感管理器",
		Japanese: "Inspirationer",
	},
	"ui.startupFailed": {
		English:  "Inspirationer failed to start",
		Chinese:  "灵感管理器 启动失败",
		Japanese: "Inspirationer の起動に失敗しました",
	},
	"ui.trayFailedBody": {
		English:  "Registering the tray icon failed, but the service is running normally.\n\nReason: %s\n\nAddress: %s\nData folder: %s%s",
		Chinese:  "托盘图标注册失败，但服务已经正常启动。\n\n原因：%s\n\n访问地址：%s\n数据目录：%s%s",
		Japanese: "タスクトレイのアイコン登録に失敗しましたが、サービスは正常に動作しています。\n\n原因：%s\n\nアドレス：%s\nデータフォルダ：%s%s",
	},
	"ui.trayFailedConsoleHint": {
		English:  "\nA log console window was opened — press Ctrl+C there to quit.",
		Chinese:  "\n已打开日志控制台窗口：可按 Ctrl+C 退出。",
		Japanese: "\nログコンソールを開きました。Ctrl+C で終了できます。",
	},
	"ui.alreadyRunning": {
		English:  "Inspirationer is already running.\n\nLook for the 💡 icon in the system tray (bottom-right).",
		Chinese:  "已经有一个灵感管理器在运行了。\n\n请查看屏幕右下角系统托盘里的 💡 图标（可能需要点开“显示隐藏的图标”）。",
		Japanese: "Inspirationer はすでに起動しています。\n\n画面右下のタスクトレイにある 💡 アイコンを確認してください（「隠れているインジケーターを表示」にある場合があります）。",
	},
	"ui.versionInfo": {
		English:  "Inspirationer v%s",
		Chinese:  "灵感管理器 v%s",
		Japanese: "Inspirationer v%s",
	},
	"ui.listenFailed": {
		English:  "Cannot listen on %s:\n%v\n\nHint: the port may be in use — try -addr 127.0.0.1:<other port>.",
		Chinese:  "无法监听 %s：\n%v\n\n提示：端口可能已被占用，可用 -addr 127.0.0.1:其他端口 指定其它端口。",
		Japanese: "%s をリッスンできません：\n%v\n\nヒント：ポートが使用中の可能性があります。-addr 127.0.0.1:<別のポート> をお試しください。",
	},
	"ui.initFailed": {
		English:  "Failed to initialise the data folder: %v",
		Chinese:  "初始化数据目录失败：%v",
		Japanese: "データフォルダの初期化に失敗しました：%v",
	},
	"ui.embedFailed": {
		English:  "Failed to load the embedded front-end assets: %v",
		Chinese:  "加载内嵌前端资源失败：%v",
		Japanese: "埋め込みフロントエンドの読み込みに失敗しました：%v",
	},
	"ui.devWebInvalid": {
		English:  "Invalid front-end folder: %v",
		Chinese:  "前端目录无效：%v",
		Japanese: "フロントエンドフォルダが不正です：%v",
	},
	"ui.devWebNoIndex": {
		English:  "No index.html found in the front-end folder: %s",
		Chinese:  "前端目录里找不到 index.html：%s",
		Japanese: "フロントエンドフォルダに index.html がありません：%s",
	},

	/* 托盘菜单 */
	"tray.open": {
		English:  "Open Inspirationer",
		Chinese:  "打开灵感管理器",
		Japanese: "Inspirationer を開く",
	},
	"tray.openData": {
		English:  "Open data folder",
		Chinese:  "打开数据目录",
		Japanese: "データフォルダを開く",
	},
	"tray.backupNow": {
		English:  "Back up to WebDAV now",
		Chinese:  "立即备份到 WebDAV",
		Japanese: "今すぐ WebDAV にバックアップ",
	},
	"tray.openLog": {
		English:  "View log file",
		Chinese:  "查看日志文件",
		Japanese: "ログファイルを表示",
	},
	"tray.quit": {
		English:  "Quit",
		Chinese:  "退出",
		Japanese: "終了",
	},
	"tray.balloonTitle": {
		English:  "Inspirationer is running",
		Chinese:  "灵感管理器 已启动",
		Japanese: "Inspirationer が起動しました",
	},
	"tray.backupOkTitle": {
		English:  "Inspirationer: backup succeeded",
		Chinese:  "灵感管理器：备份成功",
		Japanese: "Inspirationer：バックアップ成功",
	},
	"tray.backupFailTitle": {
		English:  "Inspirationer: backup failed",
		Chinese:  "灵感管理器：备份失败",
		Japanese: "Inspirationer：バックアップ失敗",
	},
}
