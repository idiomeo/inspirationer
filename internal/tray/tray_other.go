//go:build !windows

// Package tray 的非 Windows 实现：接口保持一致，托盘功能不可用（服务照常运行）。
package tray

import "errors"

// Item 是托盘菜单项。
type Item struct {
	Label   string
	Handler func()
}

// Tray 在非 Windows 系统上是空实现。
type Tray struct {
	items []Item
}

// New 创建托盘实例。
func New(tooltip string, onActivate func()) *Tray { return &Tray{} }

// AddItem 追加菜单项。
func (t *Tray) AddItem(label string, handler func()) {
	t.items = append(t.items, Item{Label: label, Handler: handler})
}

// AddSeparator 追加分隔线。
func (t *Tray) AddSeparator() {}

// SetBalloon 在非 Windows 系统上无效果。
func (t *Tray) SetBalloon(title, text string) {}

// Notify 在非 Windows 系统上无效果。
func (t *Tray) Notify(title, text string) {}

// Start 在非 Windows 系统上总是失败，调用方应据此跳过托盘。
func (t *Tray) Start() error { return errors.New("the system tray is not supported on this platform") }

// Ready 在非 Windows 系统上始终为 false。
func (t *Tray) Ready() bool { return false }

// Stop 在非 Windows 系统上无操作。
func (t *Tray) Stop() {}

// SetDebugLogger 与 Windows 实现保持同样的签名；非 Windows 平台没有托盘细节日志，因此无操作。
func (t *Tray) SetDebugLogger(fn func(format string, args ...interface{})) {}
