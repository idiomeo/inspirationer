package i18n

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 每个 key 都必须有三种语言的文案：新增文案时漏翻译会直接测试失败。
func TestTableCompleteness(t *testing.T) {
	if len(table) == 0 {
		t.Fatal("文案表为空")
	}
	for key, langs := range table {
		for _, lang := range []string{English, Chinese, Japanese} {
			if strings.TrimSpace(langs[lang]) == "" {
				t.Errorf("key %q 缺少 %s 的翻译", key, lang)
			}
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"":          English,
		"en":        English,
		"en-US":     English,
		"zh":        Chinese,
		"zh-Hans":   Chinese,
		"zh-CN":     Chinese,
		"zh-TW":     Chinese,
		"ja":        Japanese,
		"ja-JP":     Japanese,
		"fr":        English, // 不支持的语言回退英文
		"  ZH-cn  ": Chinese,
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestTAndFallback(t *testing.T) {
	// 同一 key 三种语言各不相同
	en := T(English, "err.aiDisabled")
	zh := T(Chinese, "err.aiDisabled")
	ja := T(Japanese, "err.aiDisabled")
	if en == zh || en == ja || zh == ja {
		t.Errorf("三种语言应当互不相同: %q / %q / %q", en, zh, ja)
	}
	// 不支持的语言回退到英文
	if got := T("fr", "err.aiDisabled"); got != en {
		t.Errorf("不支持的语言应回退英文，得到 %q", got)
	}
	// 未知 key 原样返回（便于发现漏配）
	if got := T(Chinese, "no.such.key"); got != "no.such.key" {
		t.Errorf("未知 key 应原样返回，得到 %q", got)
	}
	// 占位符替换
	got := T(English, "err.saveFailed", errors.New("disk full"))
	if !strings.Contains(got, "disk full") {
		t.Errorf("占位符未替换: %q", got)
	}
	gotZh := T(Chinese, "err.saveFailed", errors.New("disk full"))
	if !strings.HasPrefix(gotZh, "保存失败") || !strings.Contains(gotZh, "disk full") {
		t.Errorf("中文参数替换异常: %q", gotZh)
	}
}

// 领域包返回的带码错误应当被本地化，且 errors.As 能穿过 %w 包装。
func TestTranslateError(t *testing.T) {
	err := Errorf("err.emptyTagName")
	if got := Translate(Chinese, err); got != "标签名不能为空" {
		t.Errorf("中文文案 = %q", got)
	}
	if got := Translate(Japanese, err); !strings.Contains(got, "タグ") {
		t.Errorf("日文文案 = %q", got)
	}
	if err.Error() != "Tag name cannot be empty" {
		t.Errorf("Error() 应为英文基准，得到 %q", err.Error())
	}

	wrapped := fmt.Errorf("store: %w", err)
	if got := Translate(Chinese, wrapped); got != "标签名不能为空" {
		t.Errorf("包装后的错误未被识别: %q", got)
	}

	// 普通错误原样返回
	if got := Translate(Chinese, errors.New("plain")); got != "plain" {
		t.Errorf("普通错误应原样返回，得到 %q", got)
	}
	if got := Translate(Chinese, nil); got != "" {
		t.Errorf("nil 应返回空串，得到 %q", got)
	}
}

// 语言协商优先级：X-Lang > ?lang= > Accept-Language > 已保存设置 > 英文。
func TestFromRequest(t *testing.T) {
	mk := func(headers map[string]string, query string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/snippets"+query, nil)
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		return r
	}

	if got := FromRequest(mk(map[string]string{"X-Lang": "ja"}, ""), ""); got != Japanese {
		t.Errorf("X-Lang 未生效: %q", got)
	}
	if got := FromRequest(mk(nil, "?lang=zh-CN"), ""); got != Chinese {
		t.Errorf("?lang= 未生效: %q", got)
	}
	if got := FromRequest(mk(map[string]string{"Accept-Language": "ja-JP,ja;q=0.9,en;q=0.8"}, ""), ""); got != Japanese {
		t.Errorf("Accept-Language 未生效: %q", got)
	}
	// X-Lang 优先于 ?lang=
	if got := FromRequest(mk(map[string]string{"X-Lang": "zh-CN"}, "?lang=ja"), ""); got != Chinese {
		t.Errorf("X-Lang 应优先于 ?lang=，得到 %q", got)
	}
	// 没有任何头时使用已保存的设置
	if got := FromRequest(mk(nil, ""), "ja"); got != Japanese {
		t.Errorf("设置回退未生效: %q", got)
	}
	// 设置是 auto 时回落英文
	if got := FromRequest(mk(nil, ""), "auto"); got != English {
		t.Errorf("auto 应回落英文，得到 %q", got)
	}
	if got := FromRequest(nil, "ja"); got != Japanese {
		t.Errorf("nil 请求应使用 fallback，得到 %q", got)
	}
}

func TestFromSetting(t *testing.T) {
	if got := FromSetting("auto"); got != English {
		t.Errorf("auto → %q", got)
	}
	if got := FromSetting("ja"); got != Japanese {
		t.Errorf("ja → %q", got)
	}
	if got := FromSetting(""); got != English {
		t.Errorf("空值 → %q", got)
	}
}

// 托盘/对话框用到的 key 必须存在，否则界面上会直接显示 key。
func TestUIFacingKeysExist(t *testing.T) {
	required := []string{
		"ui.appName", "ui.versionInfo", "ui.startupFailed", "ui.trayFailedBody",
		"ui.trayFailedConsoleHint", "ui.alreadyRunning", "ui.listenFailed",
		"ui.initFailed", "ui.embedFailed", "ui.devWebInvalid", "ui.devWebNoIndex",
		"tray.open", "tray.openData", "tray.backupNow", "tray.openLog", "tray.quit",
		"tray.balloonTitle", "tray.backupOkTitle", "tray.backupFailTitle",
		"err.plain", "err.dataDir", "warn.titleAiFallback", "warn.titleTruncated",
	}
	for _, key := range required {
		if _, ok := table[key]; !ok {
			t.Errorf("缺少文案 key: %s", key)
		}
	}
}
