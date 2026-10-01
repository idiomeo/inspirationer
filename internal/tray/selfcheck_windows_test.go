//go:build windows

package tray

import (
	"strings"
	"testing"

	"inspirationer/internal/platform"
)

// TestWin32ProcsResolve 保证所有 Win32 函数名与所属 DLL 都正确。
// 这条测试可以直接捕获"函数放到错误的 DLL 里"这类会在运行时 panic 的低级错误。
func TestWin32ProcsResolve(t *testing.T) {
	if err := SelfCheck(); err != nil {
		t.Fatalf("tray 包 Win32 函数解析失败：%v", err)
	}
	if err := platform.SelfCheck(); err != nil {
		t.Fatalf("platform 包 Win32 函数解析失败：%v", err)
	}
}

// TestDiagnoseTrayFailure 确认诊断信息包含关键字段，便于用户排查。
func TestDiagnoseTrayFailure(t *testing.T) {
	msg := diagnoseTrayFailure()
	for _, want := range []string{"integrity=", "window station=", "desktop="} {
		if !strings.Contains(msg, want) {
			t.Errorf("诊断信息缺少 %q：%s", want, msg)
		}
	}
}
