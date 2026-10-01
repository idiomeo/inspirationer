//go:build windows

package tray

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32Job           = syscall.NewLazyDLL("kernel32.dll")
	procQueryInfoJobObj   = kernel32Job.NewProc("QueryInformationJobObject")
	procIsProcessInJob    = kernel32Job.NewProc("IsProcessInJob")
	procGetCurrentProcess = kernel32Job.NewProc("GetCurrentProcess")
	procGetCurrentThread  = kernel32Job.NewProc("GetCurrentThreadId")

	procGetProcessWindowStation = user32.NewProc("GetProcessWindowStation")
	procGetThreadDesktop        = user32.NewProc("GetThreadDesktop")
	procGetUserObjectInfoW      = user32.NewProc("GetUserObjectInformationW")

	advapi32              = syscall.NewLazyDLL("advapi32.dll")
	procOpenProcessToken  = advapi32.NewProc("OpenProcessToken")
	procIsTokenRestricted = advapi32.NewProc("IsTokenRestricted")
	procCloseHandle       = kernel32Job.NewProc("CloseHandle")

	procGetTokenInformation = advapi32.NewProc("GetTokenInformation")
	procSendMessageTimeoutW = user32.NewProc("SendMessageTimeoutW")
)

// tokenIntegrityLevel 读取当前进程令牌的完整性级别字符串。
// UIPI 规则：完整性级别低于目标窗口所属进程时，窗口消息会被拒绝——
// 这是 Shell_NotifyIcon 返回 ERROR_ACCESS_DENIED 最常见的原因（沙箱/受限环境）。
func tokenIntegrityLevel() string {
	rid, ok := integrityRID()
	if !ok {
		return "未知"
	}
	switch rid {
	case 0x0000:
		return "Untrusted（不可信）"
	case 0x1000:
		return "Low（低）"
	case 0x2000:
		return "Medium（中）"
	case 0x2100:
		return "MediumPlus（中+）"
	case 0x3000:
		return "High（高）"
	case 0x4000:
		return "System（系统）"
	default:
		return fmt.Sprintf("0x%X", rid)
	}
}

// integrityRID 解析令牌完整性级别的 RID。只在自己持有的缓冲区里做偏移解析，
// 不把原生指针转回 Go 指针，避免 unsafe 误用。
func integrityRID() (uint32, bool) {
	cur, _, _ := procGetCurrentProcess.Call()
	var h uintptr
	if r, _, _ := procOpenProcessToken.Call(cur, tokenQuery, uintptr(unsafe.Pointer(&h))); r == 0 {
		return 0, false
	}
	defer procCloseHandle.Call(h)

	var size uint32
	procGetTokenInformation.Call(h, 25 /* TokenIntegrityLevel */, 0, 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return 0, false
	}
	buf := make([]byte, size)
	if r, _, _ := procGetTokenInformation.Call(h, 25,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(size), uintptr(unsafe.Pointer(&size))); r == 0 {
		return 0, false
	}

	// TOKEN_MANDATORY_LABEL { SID_AND_ATTRIBUTES { PSID Sid; DWORD Attributes } }
	base := uintptr(unsafe.Pointer(&buf[0]))
	sidPtr := *(*uintptr)(unsafe.Pointer(&buf[0]))
	off := int(sidPtr - base)
	if off < 0 || off+8 > len(buf) {
		return 0, false
	}
	count := int(buf[off+1]) // SID: Revision(1) SubAuthorityCount(1) IdentifierAuthority(6) SubAuthority[count]
	if count == 0 || off+8+count*4 > len(buf) {
		return 0, false
	}
	last := buf[off+8+(count-1)*4 : off+8+count*4]
	rid := uint32(last[0]) | uint32(last[1])<<8 | uint32(last[2])<<16 | uint32(last[3])<<24
	return rid, true
}

// canReachShell 尝试给资源管理器任务栏窗口发一条空消息，
// 用来判断 UIPI 是否阻断了跨进程窗口通信（托盘失败的根因）。
func canReachShell() (bool, string) {
	cls, err := syscall.UTF16PtrFromString("Shell_TrayWnd")
	if err != nil {
		return false, "类名转换失败"
	}
	hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(cls)), 0)
	if hwnd == 0 {
		return false, "找不到 Shell_TrayWnd（资源管理器未运行）"
	}
	var res uintptr
	const smtoAbortIfHung = 0x0002
	r, _, callErr := procSendMessageTimeoutW.Call(hwnd, 0 /* WM_NULL */, 0, 0,
		smtoAbortIfHung, 1000, uintptr(unsafe.Pointer(&res)))
	if r == 0 {
		return false, fmt.Sprintf("向任务栏发消息被拒绝：%v", callErr)
	}
	return true, "可以向任务栏发消息"
}

// tokenQuery = TOKEN_QUERY
const tokenQuery = 0x0008

// tokenRestricted 报告当前进程令牌是否被限制。
// 受限令牌（常见于沙箱/作业对象过滤令牌）会缺少必要的组，导致系统拒绝
// 与资源管理器之间的窗口消息交互（Shell_NotifyIcon 返回 ERROR_ACCESS_DENIED）。
func tokenRestricted() (restricted bool, known bool) {
	cur, _, _ := procGetCurrentProcess.Call()
	var h uintptr
	r, _, _ := procOpenProcessToken.Call(cur, tokenQuery, uintptr(unsafe.Pointer(&h)))
	if r == 0 {
		return false, false
	}
	defer procCloseHandle.Call(h)
	tr, _, _ := procIsTokenRestricted.Call(h)
	return tr != 0, true
}

// windowStationDesktop 返回当前进程/线程所在的窗口站与桌面名，
// 用于判断托盘失败是不是"跑在了非交互桌面上"。
func windowStationDesktop() (string, string) {
	name := func(h uintptr) string {
		if h == 0 {
			return "?"
		}
		buf := make([]uint16, 128)
		var need uint32
		r, _, _ := procGetUserObjectInfoW.Call(h, 2,
			uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*2),
			uintptr(unsafe.Pointer(&need)))
		if r == 0 {
			return "?"
		}
		return syscall.UTF16ToString(buf)
	}
	ws, _, _ := procGetProcessWindowStation.Call()
	tid, _, _ := procGetCurrentThread.Call()
	td, _, _ := procGetThreadDesktop.Call(tid)
	return name(ws), name(td)
}

// JobObjectBasicUIRestrictions 信息类
const jobObjectBasicUIRestrictions = 4

// 作业对象 UI 限制位
const (
	jobUILimitHandles          = 0x0001
	jobUILimitReadClipboard    = 0x0002
	jobUILimitWriteClipboard   = 0x0004
	jobUILimitSystemParameters = 0x0008
	jobUILimitDisplaySettings  = 0x0010
	jobUILimitGlobalAtoms      = 0x0020
	jobUILimitDesktop          = 0x0040
	jobUILimitExitWindows      = 0x0080
)

// jobUIRestrictions 返回当前进程所属作业对象的 UI 限制位。
// 第二个返回值为 false 表示进程不在任何作业对象里（普通桌面双击就是这种情况）。
func jobUIRestrictions() (uint32, bool) {
	cur, _, _ := procGetCurrentProcess.Call()
	var inJob int32
	ok, _, _ := procIsProcessInJob.Call(cur, 0, uintptr(unsafe.Pointer(&inJob)))
	if ok == 0 || inJob == 0 {
		return 0, false
	}
	var out uint32
	var ret uint32
	r, _, _ := procQueryInfoJobObj.Call(0, jobObjectBasicUIRestrictions,
		uintptr(unsafe.Pointer(&out)), unsafe.Sizeof(out), uintptr(unsafe.Pointer(&ret)))
	if r == 0 {
		return 0, false
	}
	return out, true
}

// describeJobUI 把限制位翻译成人话。
func describeJobUI(bits uint32) string {
	if bits == 0 {
		return "无限制"
	}
	names := []string{}
	for _, f := range []struct {
		bit  uint32
		name string
	}{
		{jobUILimitHandles, "HANDLES（禁止访问作业外的 USER 句柄/窗口，托盘图标会失败）"},
		{jobUILimitReadClipboard, "READCLIPBOARD"},
		{jobUILimitWriteClipboard, "WRITECLIPBOARD"},
		{jobUILimitSystemParameters, "SYSTEMPARAMETERS"},
		{jobUILimitDisplaySettings, "DISPLAYSETTINGS"},
		{jobUILimitGlobalAtoms, "GLOBALATOMS"},
		{jobUILimitDesktop, "DESKTOP"},
		{jobUILimitExitWindows, "EXITWINDOWS"},
	} {
		if bits&f.bit != 0 {
			names = append(names, f.name)
		}
	}
	return strings.Join(names, " | ")
}

// diagnoseTrayFailure 在托盘创建失败时给出可读原因，方便用户判断是环境问题还是被限制。
func diagnoseTrayFailure() string {
	parts := []string{}
	parts = append(parts, "完整性级别="+tokenIntegrityLevel())
	if restricted, known := tokenRestricted(); known && restricted {
		parts = append(parts, "令牌受限（沙箱特征）")
	}
	bits, inJob := jobUIRestrictions()
	if inJob {
		parts = append(parts, "作业对象 UI 限制："+describeJobUI(bits))
	}
	station, desktop := windowStationDesktop()
	parts = append(parts, "窗口站="+station+" 桌面="+desktop)
	if ok, msg := canReachShell(); !ok {
		parts = append(parts, msg)
	}
	return strings.Join(parts, "；")
}
