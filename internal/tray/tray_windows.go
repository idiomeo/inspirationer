//go:build windows

// Package tray 用 Win32 Shell_NotifyIcon 实现系统托盘图标与右键菜单，
// 只依赖标准库（syscall），图标资源通过 go:embed 内嵌进二进制。
package tray

import (
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

//go:embed assets/bulb.ico
var iconData []byte

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procCreateIconFromResEx = user32.NewProc("CreateIconFromResourceEx")
	procDestroyIcon         = user32.NewProc("DestroyIcon")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procRegisterWindowMsgW  = user32.NewProc("RegisterWindowMessageW")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	procFindWindowW         = user32.NewProc("FindWindowW")
)

/* ------------------------------------------------------------------ 常量 */
const (
	wmDestroy       = 0x0002
	wmClose         = 0x0010
	wmNull          = 0x0000
	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205
	wmApp           = 0x8000
	wmTrayCallback  = wmApp + 1
	wmTrayBalloon   = wmApp + 2

	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004
	nifInfo    = 0x00000010

	niifInfo = 0x00000001

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100
	tpmNoNotify    = 0x0080

	mfString    = 0x00000000
	mfSeparator = 0x00000800

	csHRedraw = 0x0002
	csVRedraw = 0x0001

	smCXSmIcon = 49

	iconResVersion = 0x00030000
	idiApplication = 32512
)

/* ---------------------------------------------------------------- 结构体 */
// 以下结构体布局必须与 Win32 头文件严格一致（amd64），
// 对应的断言见 tray_windows_test.go。

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type point struct {
	x int32
	y int32
}

type msgW struct {
	hwnd    uintptr
	message uint32
	_       uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	_       uint32
}

type notifyIconDataW struct {
	cbSize           uint32
	_                uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	_                uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     uintptr
}

/* ------------------------------------------------------------------ 类型 */

// Item 是一个托盘菜单项。
type Item struct {
	Label   string
	Handler func()
}

// Tray 表示一个托盘图标及其菜单。
type Tray struct {
	tooltip    string
	onActivate func()
	items      []Item

	mu        sync.Mutex
	hwnd      uintptr
	icon      uintptr
	iconOwn   bool // 图标是否由本包创建（需要 DestroyIcon）
	started   bool
	iconReady bool // 托盘图标是否注册成功
	stopping  bool
	lastOpen  time.Time

	balloonTitle string
	balloonText  string

	debugf func(format string, args ...interface{})

	done chan struct{}
}

// SetDebugLogger 设置托盘内部细节日志输出（可为 nil）。
func (t *Tray) SetDebugLogger(fn func(format string, args ...interface{})) { t.debugf = fn }

func (t *Tray) debug(format string, args ...interface{}) {
	if t.debugf != nil {
		t.debugf(format, args...)
	}
}

// New 创建托盘实例。onActivate 为鼠标左键单击图标时执行的动作（可为 nil）。
func New(tooltip string, onActivate func()) *Tray {
	return &Tray{
		tooltip:    tooltip,
		onActivate: onActivate,
		done:       make(chan struct{}),
	}
}

// AddItem 追加一个菜单项。注意：调用顺序即菜单顺序。
func (t *Tray) AddItem(label string, handler func()) {
	t.items = append(t.items, Item{Label: label, Handler: handler})
}

// AddSeparator 追加一条分隔线。
func (t *Tray) AddSeparator() {
	t.items = append(t.items, Item{})
}

// SetBalloon 设置在图标首次出现时弹出的气泡提示。
func (t *Tray) SetBalloon(title, text string) {
	t.balloonTitle, t.balloonText = title, text
}

// Notify 在任意协程中弹出气泡提示（通过消息投递到托盘窗口所在线程执行）。
func (t *Tray) Notify(title, text string) {
	t.mu.Lock()
	hwnd := t.hwnd
	t.balloonTitle, t.balloonText = title, text
	t.mu.Unlock()
	if hwnd == 0 {
		return
	}
	procPostMessageW.Call(hwnd, wmTrayBalloon, 0, 0)
}

// Start 同步创建隐藏窗口与托盘图标，随后在后台线程跑消息循环。
// 返回错误表示托盘图标注册失败（例如受限沙箱、完整性级别不足、资源管理器未就绪），
// 此时服务仍应继续运行；本方法会在后台继续重试注册。
func (t *Tray) Start() error {
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := t.initWindow(); err != nil {
			ready <- err
			return
		}
		err := t.addIcon(3)
		if err == nil {
			t.mu.Lock()
			t.started = true
			t.iconReady = true
			t.mu.Unlock()
		}
		ready <- err
		if err != nil {
			go t.retryIcon()
		}
		t.messageLoop()
		close(t.done)
	}()
	select {
	case err := <-ready:
		return err
	case <-time.After(8 * time.Second):
		return errors.New("创建托盘图标超时")
	}
}

// Ready 报告托盘图标是否注册成功。
func (t *Tray) Ready() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.iconReady
}

// Stop 移除托盘图标并结束消息循环。
func (t *Tray) Stop() {
	t.mu.Lock()
	hwnd := t.hwnd
	t.stopping = true
	t.mu.Unlock()
	if hwnd == 0 {
		return
	}
	procPostMessageW.Call(hwnd, wmClose, 0, 0)
	select {
	case <-t.done:
	case <-time.After(3 * time.Second):
	}
	t.mu.Lock()
	t.started = false
	t.mu.Unlock()
}

/* ------------------------------------------------------------ 内部实现 */

// addIcon 注册托盘图标；attempts > 1 时会重试（应对资源管理器尚未就绪）。
func (t *Tray) addIcon(attempts int) error {
	icon := t.loadIcon()
	t.mu.Lock()
	t.icon = icon
	t.mu.Unlock()

	n := t.nid()
	n.uFlags = nifMessage | nifIcon | nifTip
	n.uCallbackMessage = wmTrayCallback
	n.hIcon = icon
	copyUTF16(n.szTip[:], t.tooltip)

	t.debug("NIM_ADD cbSize=%d hwnd=%#x uID=%d hIcon=%#x uFlags=%#x",
		n.cbSize, n.hWnd, n.uID, n.hIcon, n.uFlags)

	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			time.Sleep(time.Duration(attempt-1) * 600 * time.Millisecond)
		}
		ok, _, callErr := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&n)))
		if ok != 0 {
			if attempt > 1 {
				t.debug("NIM_ADD 第 %d 次尝试成功", attempt)
			}
			if t.balloonText != "" {
				t.showBalloon(t.balloonTitle, t.balloonText)
			}
			return nil
		}
		lastErr = callErr
		t.debug("NIM_ADD 第 %d 次失败: %v", attempt, callErr)
	}
	return fmt.Errorf("Shell_NotifyIcon(NIM_ADD) 失败: %v（%s）", lastErr, diagnoseTrayFailure())
}

// retryIcon 后台重试注册托盘图标。适用于：资源管理器稍后才就绪、
// 或运行环境限制后来被解除（例如从受限沙箱切到普通桌面）。
func (t *Tray) retryIcon() {
	for i := 0; i < 8; i++ {
		time.Sleep(15 * time.Second)
		t.mu.Lock()
		stopping := t.stopping
		done := t.iconReady
		t.mu.Unlock()
		if stopping || done {
			return
		}
		if err := t.addIcon(1); err == nil {
			t.mu.Lock()
			t.iconReady = true
			t.mu.Unlock()
			t.debug("托盘图标后台重试成功，托盘已可用")
			return
		}
	}
}

func (t *Tray) initWindow() error {
	className, err := syscall.UTF16PtrFromString("InspirationerTrayWnd")
	if err != nil {
		return err
	}
	title, err := syscall.UTF16PtrFromString("灵感管理器 · 托盘")
	if err != nil {
		return err
	}
	hInst, _, _ := procGetModuleHandleW.Call(0)

	wc := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		style:         csHRedraw | csVRedraw,
		lpfnWndProc:   syscall.NewCallback(t.wndProc),
		hInstance:     hInst,
		lpszClassName: className,
	}
	// 类可能已注册（同进程重复创建），忽略失败
	atom, _, regErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	station, desktop := windowStationDesktop()
	t.debug("RegisterClassExW atom=%d err=%v | 窗口站=%s 桌面=%s", atom, regErr, station, desktop)

	hwnd, _, callErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		0, 0, 0, 0, 0, // 不显示、无样式
		0, 0, hInst, 0)
	if hwnd == 0 {
		return fmt.Errorf("创建托盘消息窗口失败: %v", callErr)
	}
	// 自检：从本进程按类名找回自己的窗口
	found, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(className)), 0)
	t.debug("CreateWindowExW hwnd=%#x err=%v | 自查找 FindWindowW=%#x", hwnd, callErr, found)
	t.mu.Lock()
	t.hwnd = hwnd
	t.mu.Unlock()
	return nil
}

// chooseIconEntry 从 ICO 目录中挑选最贴近 size 的一帧，返回偏移与长度。
func chooseIconEntry(data []byte, size int) (int, int) {
	if len(data) < 6 {
		return 0, 0
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	bestOff, bestLen, bestScore := 0, 0, -1
	for i := 0; i < count; i++ {
		off := 6 + i*16
		if off+16 > len(data) {
			break
		}
		w := int(data[off])
		if w == 0 {
			w = 256
		}
		n := int(binary.LittleEndian.Uint32(data[off+8 : off+12]))
		imgOff := int(binary.LittleEndian.Uint32(data[off+12 : off+16]))
		if n <= 0 || imgOff+n > len(data) {
			continue
		}
		// 优先不小于目标尺寸的最小者；否则取最大者
		score := 0
		if w >= size {
			score = 10000 - (w - size)
		} else {
			score = w
		}
		if score > bestScore {
			bestScore, bestOff, bestLen = score, imgOff, n
		}
	}
	return bestOff, bestLen
}

func (t *Tray) loadIcon() uintptr {
	// 托盘小图标推荐尺寸随 DPI 变化（16/20/24/32…）
	cx, _, _ := procGetSystemMetrics.Call(smCXSmIcon)
	size := int(cx)
	if size <= 0 {
		size = 16
	}
	if off, n := chooseIconEntry(iconData, size); n > 0 {
		h, _, _ := procCreateIconFromResEx.Call(
			uintptr(unsafe.Pointer(&iconData[off])),
			uintptr(n),
			1, // fIcon
			iconResVersion,
			uintptr(size), uintptr(size), 0)
		if h != 0 {
			t.mu.Lock()
			t.iconOwn = true
			t.mu.Unlock()
			return h
		}
	}
	// 兜底：系统默认应用图标
	h, _, _ := procLoadIconW.Call(0, idiApplication)
	return h
}

func (t *Tray) nid() notifyIconDataW {
	var n notifyIconDataW
	n.cbSize = uint32(unsafe.Sizeof(notifyIconDataW{}))
	n.hWnd = t.hwnd
	n.uID = 1
	return n
}

func copyUTF16(dst []uint16, s string) {
	if len(dst) == 0 {
		return
	}
	src := syscall.StringToUTF16(s)
	n := len(src)
	if n > len(dst) {
		n = len(dst)
	}
	copy(dst[:n], src[:n])
	dst[len(dst)-1] = 0
}

func (t *Tray) showBalloon(title, text string) {
	n := t.nid()
	n.uFlags = nifInfo
	n.dwInfoFlags = niifInfo
	copyUTF16(n.szInfoTitle[:], title)
	copyUTF16(n.szInfo[:], text)
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&n)))
}

func (t *Tray) removeIcon() {
	n := t.nid()
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&n)))
	t.mu.Lock()
	if t.iconOwn && t.icon != 0 {
		procDestroyIcon.Call(t.icon)
	}
	t.icon = 0
	t.hwnd = 0
	t.mu.Unlock()
}

func (t *Tray) messageLoop() {
	var msg msgW
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 { // 0 = WM_QUIT，-1 = 错误
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func (t *Tray) wndProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmTrayCallback:
		switch uint32(lparam) & 0xFFFF {
		case wmLButtonUp, wmLButtonDblClk:
			t.activate()
		case wmRButtonUp:
			t.showMenu()
		}
		return 0

	case wmTrayBalloon:
		t.mu.Lock()
		title, text := t.balloonTitle, t.balloonText
		t.mu.Unlock()
		t.showBalloon(title, text)
		return 0

	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0

	case wmDestroy:
		t.removeIcon()
		procPostQuitMessage.Call(0)
		return 0
	}

	// Explorer 重启后会广播 TaskbarCreated，需要重新注册图标
	if taskbarCreatedMsg != 0 && msg == taskbarCreatedMsg {
		if err := t.addIcon(1); err == nil {
			t.mu.Lock()
			t.iconReady = true
			t.mu.Unlock()
		}
		return 0
	}

	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)
	return r
}

// activate 处理左键点击：600ms 内重复点击只响应一次（避免单击 + 双击连开两次）。
func (t *Tray) activate() {
	if t.onActivate == nil {
		return
	}
	t.mu.Lock()
	if time.Since(t.lastOpen) < 600*time.Millisecond {
		t.mu.Unlock()
		return
	}
	t.lastOpen = time.Now()
	t.mu.Unlock()
	go safeCall(t.onActivate)
}

func (t *Tray) showMenu() {
	pt := point{}
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	// 保持所有 label 的 UTF-16 缓冲存活到菜单显示结束
	labels := make([][]uint16, len(t.items))
	for i, it := range t.items {
		if it.Label == "" && it.Handler == nil {
			procAppendMenuW.Call(menu, mfSeparator, 0, 0)
			continue
		}
		labels[i] = syscall.StringToUTF16(it.Label)
		procAppendMenuW.Call(menu, mfString, uintptr(i+1), uintptr(unsafe.Pointer(&labels[i][0])))
	}
	runtime.KeepAlive(labels)

	procSetForegroundWindow.Call(t.hwnd)
	// 坐标可能为负（多显示器），必须先经 int 做符号扩展再转 uintptr
	cmd, _, _ := procTrackPopupMenu.Call(menu,
		tpmRightButton|tpmReturnCmd|tpmNoNotify,
		uintptr(int(pt.x)), uintptr(int(pt.y)), 0, t.hwnd, 0)
	procPostMessageW.Call(t.hwnd, wmNull, 0, 0)

	if cmd > 0 && int(cmd) <= len(t.items) {
		if h := t.items[cmd-1].Handler; h != nil {
			go safeCall(h)
		}
	}
}

func safeCall(fn func()) {
	defer func() { _ = recover() }()
	fn()
}

// taskbarCreated 由 init 注册，供 wndProc 使用。
var taskbarCreatedMsg uint32

func init() {
	name, err := syscall.UTF16PtrFromString("TaskbarCreated")
	if err != nil {
		return
	}
	r, _, _ := procRegisterWindowMsgW.Call(uintptr(unsafe.Pointer(name)))
	taskbarCreatedMsg = uint32(r)
}
