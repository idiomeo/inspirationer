//go:build windows

package tray

import (
	"os"
	"testing"
)

// TestJobUIRestrictionsReport 只输出当前环境的作业对象 UI 限制，用于判断
// 托盘图标失败到底是代码问题还是运行环境限制（例如在受限作业对象里跑测试）。
func TestJobUIRestrictionsReport(t *testing.T) {
	bits, inJob := jobUIRestrictions()
	if !inJob {
		t.Log("当前进程不在作业对象中：普通桌面环境，托盘图标应当可用")
	} else {
		t.Logf("作业对象 UI 限制位 = 0x%02X（%s）", bits, describeJobUI(bits))
	}
	if restricted, known := tokenRestricted(); known {
		t.Logf("令牌是否受限（IsTokenRestricted）= %v", restricted)
		if restricted {
			t.Log("注意：受限令牌属于沙箱环境特征，会阻止 Shell_NotifyIcon，不是代码缺陷。")
		}
	}
	station, desktop := windowStationDesktop()
	t.Logf("窗口站=%s 桌面=%s", station, desktop)
	t.Logf("完整性级别=%s", tokenIntegrityLevel())
	if ok, msg := canReachShell(); ok {
		t.Log("跨进程窗口通信：" + msg)
	} else {
		t.Log("跨进程窗口通信被阻断：" + msg + "（这就是托盘失败的直接原因）")
	}
}

// TestTrayStartInThisEnvironment 在没有 UI 限制的环境下真正建一次托盘图标。
// 受限环境（例如 CI/沙箱）自动跳过，避免误报失败。
func TestTrayStartInThisEnvironment(t *testing.T) {
	if bits, inJob := jobUIRestrictions(); inJob && bits&jobUILimitHandles != 0 {
		t.Skip("当前作业对象带 HANDLES 限制，托盘不可用，跳过")
	}
	if os.Getenv("IH_TRAY_TEST") == "" {
		t.Skip("设置 IH_TRAY_TEST=1 才真正创建托盘图标（否则会在通知区留下图标）")
	}
	tr := New("灵感管理器 · tray test", func() {})
	tr.AddItem("测试项", func() {})
	tr.SetDebugLogger(t.Logf)
	if err := tr.Start(); err != nil {
		t.Fatalf("托盘创建失败: %v（%s）", err, diagnoseTrayFailure())
	}
	tr.Notify("测试", "托盘图标已创建")
	tr.Stop()
}
