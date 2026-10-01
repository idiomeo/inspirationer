//go:build windows

package tray

import (
	"encoding/binary"
	"testing"
	"unsafe"
)

// Win32 结构体布局一旦算错，Shell_NotifyIcon / CreateWindowEx 会静默失败，
// 所以这里把尺寸与关键字段偏移固定下来当作回归测试。
func TestStructLayoutMatchesWin32(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("仅在 64 位下校验布局")
	}
	sizes := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"WNDCLASSEXW", unsafe.Sizeof(wndClassExW{}), 80},
		{"MSG", unsafe.Sizeof(msgW{}), 48},
		{"NOTIFYICONDATAW", unsafe.Sizeof(notifyIconDataW{}), 976},
	}
	for _, c := range sizes {
		if c.got != c.want {
			t.Errorf("%s 大小 = %d，期望 %d", c.name, c.got, c.want)
		}
	}

	offsets := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"WNDCLASSEXW.lpfnWndProc", unsafe.Offsetof(wndClassExW{}.lpfnWndProc), 8},
		{"WNDCLASSEXW.hInstance", unsafe.Offsetof(wndClassExW{}.hInstance), 24},
		{"WNDCLASSEXW.lpszClassName", unsafe.Offsetof(wndClassExW{}.lpszClassName), 64},
		{"MSG.wParam", unsafe.Offsetof(msgW{}.wParam), 16},
		{"MSG.lParam", unsafe.Offsetof(msgW{}.lParam), 24},
		{"NOTIFYICONDATAW.hWnd", unsafe.Offsetof(notifyIconDataW{}.hWnd), 8},
		{"NOTIFYICONDATAW.uCallbackMessage", unsafe.Offsetof(notifyIconDataW{}.uCallbackMessage), 24},
		{"NOTIFYICONDATAW.hIcon", unsafe.Offsetof(notifyIconDataW{}.hIcon), 32},
		{"NOTIFYICONDATAW.szTip", unsafe.Offsetof(notifyIconDataW{}.szTip), 40},
		{"NOTIFYICONDATAW.szInfo", unsafe.Offsetof(notifyIconDataW{}.szInfo), 304},
		{"NOTIFYICONDATAW.szInfoTitle", unsafe.Offsetof(notifyIconDataW{}.szInfoTitle), 820},
		{"NOTIFYICONDATAW.hBalloonIcon", unsafe.Offsetof(notifyIconDataW{}.hBalloonIcon), 968},
	}
	for _, o := range offsets {
		if o.got != o.want {
			t.Errorf("%s 偏移 = %d，期望 %d", o.name, o.got, o.want)
		}
	}
}

func TestEmbeddedIconIsUsable(t *testing.T) {
	if len(iconData) < 6 {
		t.Fatal("内嵌图标数据为空")
	}
	if binary.LittleEndian.Uint16(iconData[2:4]) != 1 {
		t.Fatal("内嵌文件不是 ICO 格式")
	}
	count := int(binary.LittleEndian.Uint16(iconData[4:6]))
	if count == 0 {
		t.Fatal("ICO 不含任何图帧")
	}
	for _, size := range []int{16, 20, 24, 32, 48} {
		off, n := chooseIconEntry(iconData, size)
		if n == 0 {
			t.Fatalf("找不到 %dpx 的图帧", size)
		}
		// DIB 图帧必须以 40 字节的 BITMAPINFOHEADER 开头
		if sz := binary.LittleEndian.Uint32(iconData[off : off+4]); sz != 40 {
			t.Errorf("%dpx 图帧不是 DIB 格式（首字段 %d）", size, sz)
		}
	}
	if off, n := chooseIconEntry(iconData, 24); n == 0 || off+n > len(iconData) {
		t.Fatal("24px 图帧越界")
	}
}

func TestChooseIconEntryPrefersClosestLarger(t *testing.T) {
	// 构造一个含 16 / 32 / 48 三帧的迷你 ICO（帧内容为 40 字节的假 BITMAPINFOHEADER）
	mk := func(sizes ...int) []byte {
		header := make([]byte, 6)
		binary.LittleEndian.PutUint16(header[2:4], 1)
		binary.LittleEndian.PutUint16(header[4:6], uint16(len(sizes)))
		entries := make([]byte, 16*len(sizes))
		blobs := []byte{}
		offset := 6 + 16*len(sizes)
		for i, s := range sizes {
			blob := make([]byte, 40)
			binary.LittleEndian.PutUint32(blob[0:4], 40)
			blobs = append(blobs, blob...)
			e := entries[i*16 : (i+1)*16]
			e[0], e[1] = byte(s), byte(s)
			binary.LittleEndian.PutUint16(e[4:6], 1)
			binary.LittleEndian.PutUint16(e[6:8], 32)
			binary.LittleEndian.PutUint32(e[8:12], uint32(len(blob)))
			binary.LittleEndian.PutUint32(e[12:16], uint32(offset))
			offset += len(blob)
		}
		return append(append(header, entries...), blobs...)
	}
	// 反查某个偏移对应目录项里的宽度
	widthAt := func(data []byte, off int) int {
		count := int(binary.LittleEndian.Uint16(data[4:6]))
		for i := 0; i < count; i++ {
			e := data[6+i*16 : 6+(i+1)*16]
			if int(binary.LittleEndian.Uint32(e[12:16])) == off {
				return int(e[0])
			}
		}
		return -1
	}

	data := mk(16, 32, 48)
	if off, n := chooseIconEntry(data, 20); n == 0 || widthAt(data, off) != 32 {
		t.Errorf("目标 20px 应选 32px 帧，实际 %d", widthAt(data, off))
	}
	if off, n := chooseIconEntry(data, 16); n == 0 || widthAt(data, off) != 16 {
		t.Errorf("目标 16px 应选 16px 帧，实际 %d", widthAt(data, off))
	}
	// 目标尺寸超过所有帧时应回退到最大帧
	if off, n := chooseIconEntry(data, 256); n == 0 || widthAt(data, off) != 48 {
		t.Errorf("目标 256px 应回退到 48px 帧，实际 %d", widthAt(data, off))
	}
	if _, n := chooseIconEntry([]byte{0, 0}, 16); n != 0 {
		t.Error("非法数据应返回空帧")
	}
}
