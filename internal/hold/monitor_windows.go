//go:build windows

package hold

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
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
var setTimer = user.NewProc("SetTimer")
var killTimer = user.NewProc("KillTimer")
var keyState = user.NewProc("GetAsyncKeyState")
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

// Monitor runs a low-level hook on its own message thread. Callbacks only update
// State and signal a coalescing queue; they never call Wails or swallow input.
type Monitor struct {
	State  *State
	thread uint32
	once   sync.Once
}

func Start(state *State) (*Monitor, error) {
	m := &Monitor{State: state}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		m.thread = windows.GetCurrentThreadId()
		type keyEvent struct {
			VK, Scan, Flags, Time uint32
			Extra                 uintptr
		}
		type message struct {
			HWND    uintptr
			ID      uint32
			W, L    uintptr
			Time    uint32
			X, Y    int32
			Private uint32
		}
		var timer uintptr
		var heldVK, heldKey uint32
		gameForeground := GameForeground()
		stopTimer := func() {
			if timer != 0 {
				killTimer.Call(0, timer)
				timer = 0
			}
		}
		keyboard := syscall.NewCallback(func(code int, w uintptr, l unsafe.Pointer) uintptr {
			if code >= 0 {
				e := (*keyEvent)(l)
				down := w == 0x100 || w == 0x104
				key := PhysicalKey(e.VK, e.Scan, e.Flags&1 != 0)
				if e.Flags&0x10 == 0 {
					m.State.Input(key, down, down && gameForeground)
					if key == m.State.CurrentKey() {
						heldVK, heldKey = e.VK, key
						if down && m.State.Desired() && timer == 0 {
							timer, _, _ = setTimer.Call(0, 0, 150, 0)
							if timer == 0 {
								m.State.Release()
							}
						}
						if !down {
							stopTimer()
						}
					}
				}
			}
			r, _, _ := nextHook.Call(0, uintptr(code), w, uintptr(l))
			return r
		})
		h, _, _ := setHook.Call(13, keyboard, 0, 0)
		if h == 0 {
			ready <- errors.New("无法启用局内按住快览；仍可使用手动快览")
			return
		}
		defer unhook.Call(h)
		foreground := foregroundCallback(func() {
			m.State.Release()
			if heldVK != 0 {
				k, _, _ := keyState.Call(uintptr(heldVK))
				if k&0x8000 == 0 {
					m.State.Input(heldKey, false, false)
				}
			}
			gameForeground = GameForeground()
			stopTimer()
		})
		f, _, _ := setEventHook.Call(3, 3, 0, foreground, 0, 0, 0)
		if f == 0 {
			ready <- errors.New("无法监听前台窗口；局内按住快览未启用")
			return
		}
		defer unhookEvent.Call(f)
		// Force message queue creation before publishing readiness/allowing Stop.
		user.NewProc("PeekMessageW").Call(uintptr(unsafe.Pointer(&message{})), 0, 0, 0, 0)
		ready <- nil
		var msg message
		for {
			r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if r == 0 || int32(r) == -1 {
				break
			}
			if msg.ID == 0x113 {
				k, _, _ := keyState.Call(uintptr(heldVK))
				if k&0x8000 == 0 {
					m.State.Input(heldKey, false, false)
				}
				if !m.State.Desired() || k&0x8000 == 0 || !GameForeground() {
					m.State.Release()
					stopTimer()
				}
			}
			dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
		stopTimer()
		m.State.Release()
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	return m, nil
}
func (m *Monitor) Stop() {
	m.once.Do(func() { m.State.Allow(false); postThreadMessage.Call(uintptr(m.thread), 0x12, 0, 0) })
}

// WinEventProc is a void Windows callback, but syscall.NewCallback still
// requires a uintptr-sized Go result. Returning zero satisfies that ABI.
func foregroundCallback(onChange func()) uintptr {
	return syscall.NewCallback(func(_ uintptr, _ uint32, _ uintptr, _ int32, _ int32, _ uint32, _ uint32) uintptr {
		onChange()
		return 0
	})
}
