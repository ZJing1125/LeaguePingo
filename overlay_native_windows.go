//go:build windows

package main

import (
	"errors"
	"leaguepingo/internal/settings"
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
var overlayDPI = windows.NewLazySystemDLL("shcore.dll")

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
	if w.holding {
		return nil
	}
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
	defer overlayUser.NewProc("UnhookWindowsHookEx").Call(hook)
	w.holding = true
	w.SetMinSize(28, 200)
	if w.Resizable() {
		w.SetResizable(false)
	}
	// Wails SetAlwaysOnTop omits SWP_NOACTIVATE in beta.29. Use the native
	// non-activating operation for both temporary topmost and its restoration.
	overlayUser.NewProc("SetWindowPos").Call(h, ^uintptr(0), 0, 0, 0, 0, 0x1|0x2|0x10)
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
		scale := 1.0
		if proc := overlayDPI.NewProc("GetDpiForMonitor"); proc.Find() == nil {
			var x, y uint32
			if result, _, _ := proc.Call(m, 0, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y))); int32(result) >= 0 && x > 0 {
				scale = float64(x) / 96
			}
		}
		margin := int32(32 * scale)
		width := min(int32(1180*scale), area.R-area.L-margin)
		height := min(int32(640*scale), area.B-area.T-margin)
		if width > 0 && height > 0 {
			if ok, _, _ := overlayUser.NewProc("SetWindowPos").Call(h, ^uintptr(0), uintptr(area.L+(area.R-area.L-width)/2), uintptr(area.T+(area.B-area.T-height)/2), uintptr(width), uintptr(height), 0x10); ok == 0 {
				w.endHold()
				return errors.New("无法定位游戏快览，未显示临时窗口")
			}
		} else {
			w.endHold()
			return errors.New("显示区域不足，未显示临时快览")
		}
	} else {
		w.endHold()
		return errors.New("无法读取游戏显示器，未显示临时窗口")
	}
	w.Show()
	overlayUser.NewProc("ShowWindow").Call(h, 4)
	if visible, _, _ := overlayUser.NewProc("IsWindowVisible").Call(h); visible == 0 {
		w.endHold()
		return errors.New("游戏快览窗口未能显示，可在设置中导出诊断")
	}
	return nil
}

// Automatic sidebar display is non-activating, but explicit clicks may focus
// its controls. Unlike the hold view, it must remain keyboard/mouse accessible.
func (w *nativeOverlay) showSidebar(r settings.Rect) error {
	w.Run()
	w.endHold()
	h := uintptr(w.NativeWindow())
	if h == 0 {
		return errors.New("侧栏窗口尚未准备好")
	}
	activationGuardTarget = h
	hook, _, _ := overlayUser.NewProc("SetWindowsHookExW").Call(5, activationGuard, 0, uintptr(windows.GetCurrentThreadId()))
	if hook == 0 {
		return errors.New("侧栏无法保护客户端焦点")
	}
	defer overlayUser.NewProc("UnhookWindowsHookEx").Call(hook)
	w.SetMinSize(28, 200)
	if w.Resizable() {
		w.SetResizable(false)
	}
	ok, _, _ := overlayUser.NewProc("SetWindowPos").Call(h, ^uintptr(0), uintptr(r.X), uintptr(r.Y), uintptr(r.Width), uintptr(r.Height), 0x10)
	if ok == 0 {
		w.Hide()
		return errors.New("侧栏暂时无法定位")
	}
	w.Show()
	return nil
}
func (w *nativeOverlay) prepareManual() {
	w.SetMinSize(400, 360)
	if !w.Resizable() {
		w.SetResizable(true)
	}
	if w.restore != nil {
		w.restore()
	}
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
