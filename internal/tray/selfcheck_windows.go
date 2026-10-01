//go:build windows

package tray

import (
	"fmt"
	"syscall"
)

// SelfCheck 校验本包用到的所有 Win32 函数都能从对应 DLL 中解析出来。
// DLL 写错或函数名拼错会在运行时 panic（例如把 shell32 的函数写进 user32），
// 这个自检让问题在测试阶段就暴露。
func SelfCheck() error {
	procs := []struct {
		name string
		proc *syscall.LazyProc
	}{
		{"user32!RegisterClassExW", procRegisterClassExW},
		{"user32!CreateWindowExW", procCreateWindowExW},
		{"user32!DefWindowProcW", procDefWindowProcW},
		{"user32!DestroyWindow", procDestroyWindow},
		{"user32!GetMessageW", procGetMessageW},
		{"user32!TranslateMessage", procTranslateMessage},
		{"user32!DispatchMessageW", procDispatchMessageW},
		{"user32!PostQuitMessage", procPostQuitMessage},
		{"user32!PostMessageW", procPostMessageW},
		{"user32!CreatePopupMenu", procCreatePopupMenu},
		{"user32!AppendMenuW", procAppendMenuW},
		{"user32!TrackPopupMenu", procTrackPopupMenu},
		{"user32!DestroyMenu", procDestroyMenu},
		{"user32!SetForegroundWindow", procSetForegroundWindow},
		{"user32!GetCursorPos", procGetCursorPos},
		{"user32!CreateIconFromResourceEx", procCreateIconFromResEx},
		{"user32!DestroyIcon", procDestroyIcon},
		{"user32!LoadIconW", procLoadIconW},
		{"user32!GetSystemMetrics", procGetSystemMetrics},
		{"user32!RegisterWindowMessageW", procRegisterWindowMsgW},
		{"user32!FindWindowW", procFindWindowW},
		{"user32!GetProcessWindowStation", procGetProcessWindowStation},
		{"user32!GetThreadDesktop", procGetThreadDesktop},
		{"user32!GetUserObjectInformationW", procGetUserObjectInfoW},
		{"user32!SendMessageTimeoutW", procSendMessageTimeoutW},
		{"shell32!Shell_NotifyIconW", procShellNotifyIconW},
		{"kernel32!GetModuleHandleW", procGetModuleHandleW},
		{"kernel32!QueryInformationJobObject", procQueryInfoJobObj},
		{"kernel32!IsProcessInJob", procIsProcessInJob},
		{"kernel32!GetCurrentProcess", procGetCurrentProcess},
		{"kernel32!GetCurrentThreadId", procGetCurrentThread},
		{"advapi32!OpenProcessToken", procOpenProcessToken},
		{"advapi32!IsTokenRestricted", procIsTokenRestricted},
		{"advapi32!GetTokenInformation", procGetTokenInformation},
	}
	for _, p := range procs {
		if err := p.proc.Find(); err != nil {
			return fmt.Errorf("cannot resolve %s: %w", p.name, err)
		}
	}
	return nil
}
