//go:build windows

package main

import (
	"errors"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

type nativeOverlay struct {
	*application.WebviewWindow
	savedStyle uintptr
	holding    bool
	restore    func()
}

var overlayUser = windows.NewLazySystemDLL("user32.dll")

// One callback for the lifetime of the process; allocating one on every press
// would exhaust Go's non-reclaimable Windows callback table.
var activationGuardTarget uintptr
var activationGuard = syscall.NewCallback(func(code int, target, detail uintptr) uintptr {
	if code == 5 && target == activationGuardTarget {
		return 1
	}
	r, _, _ := overlayUser.NewProc("CallNextHookEx").Call(0, uintptr(code), target, detail)
	return r
})

func (w *nativeOverlay) showHold() error {
	w.Run()
	h := uintptr(w.NativeWindow())
	if h == 0 {
		return errors.New("快览窗口尚未准备好，请松开后重试")
	}
	get := overlayUser.NewProc("GetWindowLongPtrW")
	set := overlayUser.NewProc("SetWindowLongPtrW")
	exIndex := ^uintptr(19)
	w.savedStyle, _, _ = get.Call(h, exIndex)
	set.Call(h, exIndex, w.savedStyle|0x08000000)
	w.SetIgnoreMouseEvents(true)
	alpha, _, _ := overlayUser.NewProc("SetLayeredWindowAttributes").Call(h, 0, 255, 2)
	actual, _, _ := get.Call(h, exIndex)
	if alpha == 0 || actual&0x08080020 != 0x08080020 {
		w.holding = true
		w.endHold()
		return errors.New("无法启用临时窗口的输入保护，未显示快览")
	}

	// Wails Show makes the WebView controller visible. A thread-local CBT hook
	// denies activation of this HWND during that call; no other process is hooked.
	activationGuardTarget = h
	hook, _, _ := overlayUser.NewProc("SetWindowsHookExW").Call(5, activationGuard, 0, uintptr(windows.GetCurrentThreadId()))
	if hook == 0 {
		w.holding = true
		w.endHold()
		return errors.New("无法保护游戏焦点，未显示临时快览；可用手动快览")
	}
	w.holding = true
	// Wails SetAlwaysOnTop omits SWP_NOACTIVATE in beta.29. Use the native
	// non-activating operation for both temporary topmost and its restoration.
	overlayUser.NewProc("SetWindowPos").Call(h, ^uintptr(0), 0, 0, 0, 0, 0x1|0x2|0x10)
	w.Show()
	overlayUser.NewProc("UnhookWindowsHookEx").Call(hook)
	fg, _, _ := overlayUser.NewProc("GetForegroundWindow").Call()
	m, _, _ := overlayUser.NewProc("MonitorFromWindow").Call(fg, 2)
	type rect struct{ L, T, R, B int32 }
	type monitor struct {
		Size         uint32
		Bounds, Work rect
		Flags        uint32
	}
	info := monitor{}
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, _ := overlayUser.NewProc("GetMonitorInfoW").Call(m, uintptr(unsafe.Pointer(&info))); ok != 0 {
		area := info.Work
		width := min(int32(1180), area.R-area.L-32)
		height := min(int32(640), area.B-area.T-32)
		if width > 0 && height > 0 {
			overlayUser.NewProc("SetWindowPos").Call(h, ^uintptr(0), uintptr(area.L+(area.R-area.L-width)/2), uintptr(area.T+(area.B-area.T-height)/2), uintptr(width), uintptr(height), 0x10|0x40)
		}
	}
	overlayUser.NewProc("ShowWindow").Call(h, 4)
	return nil
}
func (w *nativeOverlay) endHold() {
	if !w.holding {
		return
	}
	w.holding = false
	w.SetIgnoreMouseEvents(false)
	overlayUser.NewProc("SetWindowLongPtrW").Call(uintptr(w.NativeWindow()), ^uintptr(19), w.savedStyle)
	after := ^uintptr(1) // HWND_NOTOPMOST
	if w.savedStyle&8 != 0 {
		after = ^uintptr(0)
	}
	overlayUser.NewProc("SetWindowPos").Call(uintptr(w.NativeWindow()), after, 0, 0, 0, 0, 0x1|0x2|0x10)
	if w.restore != nil {
		w.restore()
	}
}
