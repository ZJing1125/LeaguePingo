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
}

type overlayController struct {
	window   overlayWindow
	create   func() overlayWindow
	dispatch func(func())
	emit     func(OverlayState)
	state    OverlayState
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
}

func (o *overlayController) toggle() OverlayState {
	return o.update(func() {
		if o.state.Visible {
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
		o.window.Show()
		o.state.Visible = true
	})
}

func (o *overlayController) setPinned(pinned bool) OverlayState {
	return o.update(func() {
		if o.window != nil {
			o.window.SetAlwaysOnTop(pinned)
		}
		o.state.Pinned = pinned
	})
}
