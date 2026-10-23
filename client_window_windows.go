//go:build windows

package main

import (
	"golang.org/x/sys/windows"
	"leaguepingo/internal/settings"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var clientUser = windows.NewLazySystemDLL("user32.dll")
var clientDWM = windows.NewLazySystemDLL("dwmapi.dll")

type winRect struct{ L, T, R, B int32 }

func (r winRect) rect() settings.Rect {
	return settings.Rect{X: int(r.L), Y: int(r.T), Width: int(r.R - r.L), Height: int(r.B - r.T)}
}

func clientExecutable(h uintptr) bool {
	var pid uint32
	clientUser.NewProc("GetWindowThreadProcessId").Call(h, uintptr(unsafe.Pointer(&pid)))
	p, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(p)
	buf := make([]uint16, 1024)
	n := uint32(len(buf))
	if windows.QueryFullProcessImageName(p, 0, &buf[0], &n) != nil {
		return false
	}
	base := filepath.Base(windows.UTF16ToString(buf[:n]))
	return strings.EqualFold(base, "LeagueClientUx.exe")
}

// One callback per process, not per geometry event. EnumWindows runs on the
// serialized UI caller; the address passed as LPARAM owns this scan's result.
type clientScan struct {
	handle uintptr
	area   int64
}

var enumerateClient = syscall.NewCallback(func(h uintptr, data unsafe.Pointer) uintptr {
	visible, _, _ := clientUser.NewProc("IsWindowVisible").Call(h)
	if visible == 0 {
		return 1
	}
	if iconic, _, _ := clientUser.NewProc("IsIconic").Call(h); iconic != 0 {
		scan := (*clientScan)(data)
		if scan.handle == 0 && clientExecutable(h) {
			scan.handle, scan.area = h, 1
		}
		return 1
	}
	var r winRect
	if ok, _, _ := clientUser.NewProc("GetWindowRect").Call(h, uintptr(unsafe.Pointer(&r))); ok == 0 || r.R-r.L < 400 || r.B-r.T < 300 {
		return 1
	}
	area := int64(r.R-r.L) * int64(r.B-r.T)
	scan := (*clientScan)(data)
	if area > scan.area && clientExecutable(h) {
		scan.handle, scan.area = h, area
	}
	return 1
})

// Only an explicit expand action may restore/activate the client. Automatic
// docking never calls this, and interaction inside the sidebar keeps its focus.
func focusClientWindow(own uintptr) {
	fg, _, _ := clientUser.NewProc("GetForegroundWindow").Call()
	if own != 0 && fg == own {
		return
	}
	frame := readClientFrame(0, own)
	if frame.Handle != 0 {
		clientUser.NewProc("ShowWindow").Call(frame.Handle, 9)
		clientUser.NewProc("SetForegroundWindow").Call(frame.Handle)
	}
}

func readClientFrame(cached, own uintptr) clientFrame {
	h := cached
	valid, _, _ := clientUser.NewProc("IsWindow").Call(h)
	if h == 0 || valid == 0 || !clientExecutable(h) {
		scan := clientScan{}
		var pin runtime.Pinner
		pin.Pin(&scan)
		clientUser.NewProc("EnumWindows").Call(enumerateClient, uintptr(unsafe.Pointer(&scan)))
		pin.Unpin()
		h = scan.handle
	}
	if h == 0 {
		return clientFrame{}
	}
	out := clientFrame{Handle: h, Scale: 1}
	visible, _, _ := clientUser.NewProc("IsWindowVisible").Call(h)
	iconic, _, _ := clientUser.NewProc("IsIconic").Call(h)
	var cloaked uint32
	clientDWM.NewProc("DwmGetWindowAttribute").Call(h, 14, uintptr(unsafe.Pointer(&cloaked)), 4)
	if visible == 0 || iconic != 0 || cloaked != 0 {
		return out
	}
	fg, _, _ := clientUser.NewProc("GetForegroundWindow").Call()
	root, _, _ := clientUser.NewProc("GetAncestor").Call(fg, 2)
	if fg != own && root != h {
		return out
	}
	var r winRect
	result, _, _ := clientDWM.NewProc("DwmGetWindowAttribute").Call(h, 9, uintptr(unsafe.Pointer(&r)), unsafe.Sizeof(r))
	if int32(result) < 0 {
		clientUser.NewProc("GetWindowRect").Call(h, uintptr(unsafe.Pointer(&r)))
	}
	monitor, _, _ := clientUser.NewProc("MonitorFromWindow").Call(h, 2)
	info := struct {
		Size         uint32
		Bounds, Work winRect
		Flags        uint32
	}{}
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, _ := clientUser.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return out
	}
	if r.R <= r.L || r.B <= r.T || info.Work.R <= info.Work.L || info.Work.B <= info.Work.T {
		return out
	}
	if proc := clientUser.NewProc("GetDpiForWindow"); proc.Find() == nil {
		if dpi, _, _ := proc.Call(h); dpi > 0 {
			out.Scale = float64(dpi) / 96
		}
	}
	// The client may be DPI-unaware; size our readable UI using its monitor's
	// effective DPI rather than inheriting the client's virtualised 96 DPI.
	if proc := overlayDPI.NewProc("GetDpiForMonitor"); proc.Find() == nil {
		var x, y uint32
		if result, _, _ := proc.Call(monitor, 0, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y))); int32(result) >= 0 && x > 0 {
			out.Scale = float64(x) / 96
		}
	}
	out.Client, out.Work, out.Active = r.rect(), info.Work.rect(), true
	return out
}

// Hooks only signal a bounded queue; they never call Wails, inspect players or
// move windows. A low-frequency fallback handles discovery and missed events.
type clientWindowMonitor struct {
	Changes chan struct{}
	target  atomic.Uintptr
	thread  uint32
	once    sync.Once
	done    chan struct{}
}

func startClientWindowMonitor() *clientWindowMonitor {
	m := &clientWindowMonitor{Changes: make(chan struct{}, 1), done: make(chan struct{})}
	ready := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(m.done)
		m.thread = windows.GetCurrentThreadId()
		type message struct {
			HWND    uintptr
			ID      uint32
			W, L    uintptr
			Time    uint32
			X, Y    int32
			Private uint32
		}
		cb := syscall.NewCallback(func(_ uintptr, event uint32, h uintptr, object, child int32, _ uint32, _ uint32) uintptr {
			if event == 3 || (h == m.target.Load() && object == 0 && child == 0) {
				select {
				case m.Changes <- struct{}{}:
				default:
				}
			}
			return 0
		})
		var hooks []uintptr
		for _, pair := range [][2]uintptr{{3, 3}, {0x16, 0x17}, {0x8000, 0x800B}} {
			h, _, _ := clientUser.NewProc("SetWinEventHook").Call(pair[0], pair[1], 0, cb, 0, 0, 0)
			if h != 0 {
				hooks = append(hooks, h)
			}
		}
		defer func() {
			for _, h := range hooks {
				clientUser.NewProc("UnhookWinEvent").Call(h)
			}
		}()
		clientUser.NewProc("PeekMessageW").Call(uintptr(unsafe.Pointer(&message{})), 0, 0, 0, 0)
		close(ready)
		var msg message
		for {
			r, _, _ := clientUser.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if r == 0 || int32(r) == -1 {
				break
			}
			clientUser.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
	<-ready
	return m
}
func (m *clientWindowMonitor) Watch(h uintptr) { m.target.Store(h) }
func (m *clientWindowMonitor) Stop() {
	m.once.Do(func() { clientUser.NewProc("PostThreadMessageW").Call(uintptr(m.thread), 0x12, 0, 0) })
	<-m.done
}
