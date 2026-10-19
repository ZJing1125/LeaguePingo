package main

import "github.com/wailsapp/wails/v3/pkg/application"

// All controller access is serialized on Wails' UI thread, including snapshots.
// Tests supply a synchronous dispatcher and a fake window instead of a WebView.
type overlayWindow interface {
	Show() application.Window
	Hide() application.Window
	SetAlwaysOnTop(bool) application.Window
}

type OverlayState struct {
	Revision          uint64 `json:"revision"`
	Visible           bool   `json:"visible"`
	Pinned            bool   `json:"pinned"`
	ShortcutAvailable bool   `json:"shortcutAvailable"`
	Warning           string `json:"warning"`
	Shortcut          string `json:"shortcut"`
	Mode              string `json:"mode"`
	HoldEnabled       bool   `json:"holdEnabled"`
	HoldKey           string `json:"holdKey"`
	HoldAvailable     bool   `json:"holdAvailable"`
	HoldWarning       string `json:"holdWarning"`
}

type overlayController struct {
	window      overlayWindow
	create      func() overlayWindow
	dispatch    func(func())
	emit        func(OverlayState)
	state       OverlayState
	holdDesired func() bool
}

func (o *overlayController) snapshot() (state OverlayState) {
	o.dispatch(func() { state = o.state })
	return
}

func (o *overlayController) update(edit func()) (state OverlayState) {
	o.dispatch(func() {
		edit()
		o.state.Revision++
		state = o.state
		if o.emit != nil {
			o.emit(state)
		}
	})
	return
}

func (o *overlayController) hide() OverlayState {
	return o.update(func() { o.hideOnUI() })
}

func (o *overlayController) hideOnUI() {
	if o.window != nil {
		o.window.Hide()
	}
	o.state.Visible = false
	o.state.Mode = "hidden"
	if w, ok := o.window.(interface{ endHold() }); ok {
		w.endHold()
	}
}

func (o *overlayController) toggle() OverlayState {
	return o.update(func() {
		if o.state.Visible && o.state.Mode != "hold" {
			o.hideOnUI()
			return
		}
		if o.window == nil && o.create != nil {
			o.window = o.create()
		}
		if o.window == nil {
			o.state.Warning = "快览窗口尚未准备好"
			return
		}
		if w, ok := o.window.(interface{ endHold() }); ok {
			w.endHold()
		}
		o.window.SetAlwaysOnTop(o.state.Pinned)
		o.window.Show()
		o.state.Visible = true
		o.state.Mode = "persistent"
	})
}

func (o *overlayController) setPinned(pinned bool) OverlayState {
	return o.update(func() {
		if o.window != nil && o.state.Mode != "hold" {
			o.window.SetAlwaysOnTop(pinned)
		}
		o.state.Pinned = pinned
	})
}

// Recheck physical state after lazy native/WebView creation to reject late shows.
func (o *overlayController) reconcileHold() OverlayState {
	return o.update(func() {
		desired := o.holdDesired != nil && o.holdDesired()
		if !desired {
			if o.state.Mode == "hold" {
				o.hideOnUI()
			}
			return
		}
		if o.state.Mode == "persistent" {
			return
		}
		if o.window == nil && o.create != nil {
			o.window = o.create()
		}
		if o.holdDesired == nil || !o.holdDesired() {
			return
		}
		if o.state.Mode == "hold" {
			return
		}
		if w, ok := o.window.(interface{ showHold() error }); ok {
			o.state.Mode = "hold"
			if err := w.showHold(); err != nil {
				o.hideOnUI()
				o.state.HoldWarning = err.Error()
				return
			}
			if !o.holdDesired() {
				o.hideOnUI()
				return
			}
			o.state.Visible, o.state.Mode = true, "hold"
			o.state.HoldWarning = ""
		}
	})
}
