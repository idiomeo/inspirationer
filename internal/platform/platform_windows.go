//go:build windows

// Package platform 封装少量 Win32 能力：消息框、打开链接/文件夹、单实例、
// 控制台附着与 DPI 感知。整个包只用标准库 syscall，不引入第三方依赖。
package platform

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	procMessageBoxW        = user32.NewProc("MessageBoxW")
	procShellExecuteW      = shell32.NewProc("ShellExecuteW") // 注意：在 shell32 而不是 user32
	procSetProcessDPIAware = user32.NewProc("SetProcessDPIAware")
	procGetConsoleWindow   = kernel32.NewProc("GetConsoleWindow")
	procAttachConsole      = kernel32.NewProc("AttachConsole")
	procAllocConsole       = kernel32.NewProc("AllocConsole")
	procSetConsoleOutputCP = kernel32.NewProc("SetConsoleOutputCP")
	procCreateMutexW       = kernel32.NewProc("CreateMutexW")
	procCloseHandle        = kernel32.NewProc("CloseHandle")
)

// SelfCheck 校验本包用到的所有 Win32 函数都能从对应 DLL 中解析出来。
// 写错 DLL（例如把 shell32 的 ShellExecuteW 写成 user32）会在调用时 panic，
// 有了这个自检就能在测试阶段直接暴露出来。
func SelfCheck() error {
	procs := []struct {
		name string
		proc *syscall.LazyProc
	}{
		{"user32!MessageBoxW", procMessageBoxW},
		{"shell32!ShellExecuteW", procShellExecuteW},
		{"user32!SetProcessDPIAware", procSetProcessDPIAware},
		{"kernel32!GetConsoleWindow", procGetConsoleWindow},
		{"kernel32!AttachConsole", procAttachConsole},
		{"kernel32!AllocConsole", procAllocConsole},
		{"kernel32!SetConsoleOutputCP", procSetConsoleOutputCP},
		{"kernel32!CreateMutexW", procCreateMutexW},
		{"kernel32!CloseHandle", procCloseHandle},
	}
	for _, p := range procs {
		if err := p.proc.Find(); err != nil {
			return fmt.Errorf("%s 解析失败: %w", p.name, err)
		}
	}
	return nil
}

const (
	mbOK            = 0x00000000
	mbIconError     = 0x00000010
	mbIconInfo      = 0x00000040
	mbTopMost       = 0x00040000
	mbSetForeground = 0x00010000
	swShowNormal    = 1

	errAlreadyExists = syscall.Errno(183) // ERROR_ALREADY_EXISTS
)

// HasConsole 报告当前进程是否有一个可用的控制台窗口。
func HasConsole() bool {
	h, _, _ := procGetConsoleWindow.Call()
	return h != 0
}

// MessageBox 弹出原生提示框（GUI 子系统下没有控制台，用它反馈信息）。
func MessageBox(title, text string) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	procMessageBoxW.Call(0,
		uintptr(unsafe.Pointer(m)),
		uintptr(unsafe.Pointer(t)),
		mbOK|mbIconInfo|mbTopMost|mbSetForeground)
}

// ErrorBox 弹出原生错误框。
func ErrorBox(title, text string) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	procMessageBoxW.Call(0,
		uintptr(unsafe.Pointer(m)),
		uintptr(unsafe.Pointer(t)),
		mbOK|mbIconError|mbTopMost|mbSetForeground)
}

// OpenURL 用系统默认程序打开 URL / 文件夹 / 文件。
// 直接用 ShellExecuteW，既不弹控制台窗口，也没有 cmd 的引号转义问题。
// 内部带 recover：任何 Win32 调用异常都不应该让整个程序崩掉。
func OpenURL(target string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("调用 ShellExecuteW 异常: %v", r)
		}
	}()
	op, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	r, _, callErr := procShellExecuteW.Call(0,
		uintptr(unsafe.Pointer(op)),
		uintptr(unsafe.Pointer(file)),
		0, 0, swShowNormal)
	if r <= 32 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return fmt.Errorf("ShellExecuteW 失败: %v（返回码 %d）", callErr, r)
		}
		return fmt.Errorf("ShellExecuteW 返回错误码 %d", r)
	}
	return nil
}

// SetDPIAware 让托盘图标与窗口在高 DPI 下保持清晰。
func SetDPIAware() {
	procSetProcessDPIAware.Call()
}

// AttachParentConsole 附着到父进程（终端）的控制台并重定向标准流。
func AttachParentConsole() bool {
	if HasConsole() {
		return true
	}
	const attachParentProcess = ^uintptr(0) // (DWORD)-1
	r, _, _ := procAttachConsole.Call(attachParentProcess)
	if r == 0 {
		return false
	}
	rebindStd()
	return true
}

// AllocConsole 新开一个控制台窗口（用于 -console 调试）。
func AllocConsole() bool {
	r, _, _ := procAllocConsole.Call()
	if r == 0 {
		return false
	}
	rebindStd()
	return true
}

func rebindStd() {
	procSetConsoleOutputCP.Call(65001) // UTF-8，避免中文乱码
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout = f
	}
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stderr = f
	}
	if f, err := os.OpenFile("CONIN$", os.O_RDONLY, 0); err == nil {
		os.Stdin = f
	}
}

// SingleInstance 用命名互斥体实现单实例。
// 返回的 release 需在退出时调用；already 为 true 表示已有实例在运行。
func SingleInstance(name string) (release func(), already bool) {
	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return func() {}, false
	}
	h, _, callErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(n)))
	if h == 0 {
		return func() {}, false
	}
	errno, _ := callErr.(syscall.Errno)
	already = errno == errAlreadyExists
	return func() { procCloseHandle.Call(h) }, already
}
