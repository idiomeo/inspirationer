//go:build !windows

// Package platform 的非 Windows 实现：保持相同接口，行为退化为终端输出。
package platform

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// SelfCheck 在非 Windows 系统上无需校验。
func SelfCheck() error { return nil }

// HasConsole 在类 Unix 系统上始终返回 true。
func HasConsole() bool { return true }

// MessageBox 退化为打印到标准错误。
func MessageBox(title, text string) { fmt.Fprintf(os.Stderr, "%s: %s\n", title, text) }

// ErrorBox 退化为打印到标准错误。
func ErrorBox(title, text string) { MessageBox(title, text) }

// OpenURL 用系统默认程序打开 URL。
func OpenURL(target string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", target)
	} else {
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}

// SetDPIAware 在非 Windows 系统上无意义。
func SetDPIAware() {}

// AttachParentConsole 在类 Unix 系统上总是成功。
func AttachParentConsole() bool { return true }

// AllocConsole 在非 Windows 系统上不支持。
func AllocConsole() bool { return false }

// SingleInstance 在非 Windows 系统上不启用。
func SingleInstance(name string) (func(), bool) { return func() {}, false }
