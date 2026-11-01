//go:build windows

package hold

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/diagnostics"
)

var user = windows.NewLazySystemDLL("user32.dll")
var setHook = user.NewProc("SetWindowsHookExW")
var unhook = user.NewProc("UnhookWindowsHookEx")
var nextHook = user.NewProc("CallNextHookEx")
var getMessage = user.NewProc("GetMessageW")
var dispatchMessage = user.NewProc("DispatchMessageW")
var postThreadMessage = user.NewProc("PostThreadMessageW")
var getForeground = user.NewProc("GetForegroundWindow")
var getPID = user.NewProc("GetWindowThreadProcessId")
var keyState = user.NewProc("GetAsyncKeyState")
var getKeyboardLayout = user.NewProc("GetKeyboardLayout")
var mapVirtualKey = user.NewProc("MapVirtualKeyExW")
var setEventHook = user.NewProc("SetWinEventHook")
var unhookEvent = user.NewProc("UnhookWinEvent")

// GameForeground reads only the foreground executable path with limited query
// rights. It never reads command lines, credentials or game memory.
func GameForeground() bool {
	h, _, _ := getForeground.Call()
	if h == 0 {
		return false
	}
	var pid uint32
	getPID.Call(h, uintptr(unsafe.Pointer(&pid)))
	p, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(p)
	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	if windows.QueryFullProcessImageName(p, 0, &buf[0], &size) != nil {
		return false
	}
	return strings.EqualFold(filepath.Base(windows.UTF16ToString(buf[:size])), "League of Legends.exe")
}

// Monitor runs native hooks on a dedicated message thread. Native callbacks only
// capture the configured key level or invalidate it and signal a coalescing wake.
// Foreground/process/key reads and display notifications run on a separate worker.
type Monitor struct {
	State  *State
	thread uint32
	once   sync.Once
	cancel context.CancelFunc
}

type keyEvent struct {
	VK, Scan, Flags, Time uint32
	Extra                 uintptr
}

func Start(state *State) (*Monitor, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Monitor{State: state, cancel: cancel}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer cancel()
		defer state.Allow(false)
		m.thread = windows.GetCurrentThreadId()
		type message struct {
			HWND    uintptr
			ID      uint32
			W, L    uintptr
			Time    uint32
			X, Y    int32
			Private uint32
		}
		// Resolve the forwarding procedure before installing the hook, so its
		// first input does not have to load a DLL/procedure lazily.
		if nextHook.Find() != nil {
			ready <- apperrors.New(apperrors.HoldUnavailable)
			return
		}
		keyboard := keyboardCallback(state, func(code int, w uintptr, l unsafe.Pointer) uintptr {
			r, _, _ := nextHook.Call(0, uintptr(code), w, uintptr(l))
			return r
		})
		h, _, _ := setHook.Call(13, keyboard, 0, 0)
		if h == 0 {
			ready <- apperrors.New(apperrors.HoldUnavailable)
			return
		}
		defer unhook.Call(h)
		foreground := foregroundCallback(state.interrupt)
		f, _, _ := setEventHook.Call(3, 3, 0, foreground, 0, 0, 0)
		if f == 0 {
			ready <- apperrors.New(apperrors.HoldUnavailable)
			return
		}
		defer unhookEvent.Call(f)
		// Force message queue creation before publishing readiness/allowing Stop.
		user.NewProc("PeekMessageW").Call(uintptr(unsafe.Pointer(&message{})), 0, 0, 0, 0)
		go state.run(ctx, configuredKeyDown, GameForeground)
		ready <- nil
		var msg message
		for {
			r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if r == 0 || int32(r) == -1 {
				break
			}
			dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	return m, nil
}
func (m *Monitor) Stop() {
	m.once.Do(func() {
		m.State.Allow(false)
		m.cancel()
		postThreadMessage.Call(uintptr(m.thread), 0x12, 0, 0)
	})
}

func keyboardCallback(state *State, forward func(int, uintptr, unsafe.Pointer) uintptr) uintptr {
	return syscall.NewCallback(func(code int, w uintptr, l unsafe.Pointer) uintptr {
		if code == 0 && l != nil && (w == 0x100 || w == 0x101 || w == 0x104 || w == 0x105) {
			e := (*keyEvent)(l)
			if e.Flags&0x12 == 0 { // Never use injected/lower-integrity injected input.
				started := time.Now()
				key := PhysicalKey(e.VK, e.Scan, e.Flags&1 != 0)
				if state.capture(key, w == 0x100 || w == 0x104) {
					// Own configured-key work only; downstream hooks are excluded.
					diagnostics.KeyboardCallback.Record(time.Since(started))
				}
			}
		}
		// Always forward the original event unchanged, including ignored events.
		return forward(code, w, l)
	})
}

// Sample the configured physical key outside callbacks. VK_OEM_3 depends on the
// layout; resolve scan 0x29 in the foreground thread's layout for missed-up checks.
func configuredKeyDown(key uint32) bool {
	vk := uintptr(key)
	if key == Key("Backquote") {
		h, _, _ := getForeground.Call()
		thread, _, _ := getPID.Call(h, 0)
		layout, _, _ := getKeyboardLayout.Call(thread)
		vk, _, _ = mapVirtualKey.Call(0x29, 3, layout)
		if vk == 0 {
			return false
		}
	}
	value, _, _ := keyState.Call(vk)
	return value&0x8000 != 0
}

// WinEventProc is a void Windows callback, but syscall.NewCallback still
// requires a uintptr-sized Go result. Returning zero satisfies that ABI.
func foregroundCallback(onChange func()) uintptr {
	return syscall.NewCallback(func(_ uintptr, _ uint32, _ uintptr, _ int32, _ int32, _ uint32, _ uint32) uintptr {
		onChange()
		return 0
	})
}
