package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"leaguepingo/internal/companion"
	"leaguepingo/internal/core"
	"leaguepingo/internal/settings"
	"time"
)

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
	SidebarEnabled    bool   `json:"sidebarEnabled"`
	PreviewEnabled    bool   `json:"previewEnabled"`
	SidebarWidth      int    `json:"sidebarWidth"`
	SidebarSide       string `json:"sidebarSide"`
	Collapsed         bool   `json:"collapsed"`
}

type overlayController struct {
	window              overlayWindow
	create              func() overlayWindow
	dispatch            func(func())
	emit                func(OverlayState)
	state               OverlayState
	holdDesired         func() bool
	readyRequired       bool
	ready               bool
	managed             bool
	latest              core.Snapshot
	preview             companion.Preview
	previewUntil        time.Time
	expanded            bool
	lastPlacement       settings.Rect
	wake                func()
	gameForeground      func() bool
	currentSnapshot     func() core.Snapshot
	focusClient         func()
	clock               func() time.Time
	lastGameForeground  bool
	lastClientActive    bool
	lastForegroundCheck time.Time
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
		if o.wake != nil {
			o.wake()
		}
	})
	return
}

func (o *overlayController) hide() OverlayState {
	return o.update(func() {
		if companion.Pregame(o.latest) {
			o.state.Collapsed = true
			o.expanded = false
		}
		o.preview.Consume()
		o.hideOnUI()
	})
}

func (o *overlayController) hideOnUI() {
	if o.window != nil {
		o.window.Hide()
	}
	o.state.Visible = false
	o.state.Mode = "hidden"
	o.previewUntil = time.Time{}
	if w, ok := o.window.(interface{ endHold() }); ok {
		w.endHold()
	}
}

func (o *overlayController) toggle() OverlayState {
	return o.update(func() {
		if o.currentSnapshot != nil {
			o.latest = o.currentSnapshot()
		}
		if o.managed && companion.Pregame(o.latest) {
			o.state.Collapsed = o.state.Visible && o.state.Mode == "sidebar"
			o.expanded = !o.state.Collapsed
			o.hideOnUI()
			if !o.state.Collapsed && o.focusClient != nil {
				o.focusClient()
			}
			return
		}
		if o.managed && !companion.InGame(o.latest) {
			o.state.Warning = "赛前资料在客户端侧栏查看；局内可用快捷键调出快览"
			return
		}
		if o.currentSnapshot != nil {
			o.preview.Observe(o.latest, time.Now())
		}
		o.preview.Consume()
		o.previewUntil = time.Time{}
		if o.state.Visible && o.state.Mode != "hold" {
			o.hideOnUI()
			return
		}
		if o.window == nil && o.create != nil {
			o.state.Mode = "persistent"
			o.window = o.create()
		}
		if o.window == nil {
			o.state.Warning = "快览窗口尚未准备好"
			return
		}
		if w, ok := o.window.(interface{ endHold() }); ok {
			w.endHold()
		}
		if w, ok := o.window.(interface{ prepareManual() }); ok {
			w.prepareManual()
		}
		o.window.SetAlwaysOnTop(o.state.Pinned)
		o.window.Show()
		o.state.Visible = true
		o.state.Mode = "persistent"
	})
}

func (o *overlayController) setPinned(pinned bool) OverlayState {
	return o.update(func() {
		if o.window != nil && o.state.Mode == "persistent" {
			o.window.SetAlwaysOnTop(pinned)
		}
		o.state.Pinned = pinned
	})
}

// Recheck physical state after lazy native/WebView creation to reject late shows.
func (o *overlayController) reconcileHold() OverlayState {
	return o.update(func() {
		desired := o.holdDesired != nil && o.holdDesired()
		if desired {
			o.lastGameForeground = true
			o.lastForegroundCheck = time.Now()
		}
		if !desired {
			if o.state.Mode == "hold" {
				o.hideOnUI()
			}
			return
		}
		if o.state.Mode == "persistent" {
			return
		}
		if o.currentSnapshot != nil {
			o.preview.Observe(o.currentSnapshot(), time.Now())
		}
		o.preview.Consume()
		if o.window == nil && o.create != nil {
			o.state.Mode = "hold"
			o.window = o.create()
		}
		if o.holdDesired == nil || !o.holdDesired() {
			if !o.state.Visible {
				o.hideOnUI()
			}
			return
		}
		if o.state.Mode == "hold" && o.state.Visible {
			return
		}
		if o.readyRequired && !o.ready {
			return
		}
		if o.state.Mode == "preview" && o.state.Visible {
			o.previewUntil = time.Time{}
			o.state.Mode = "hold"
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

// Called only by the independent auxiliary frontend after its initial render.
func (o *overlayController) frontendReady() {
	o.dispatch(func() { o.ready = true })
	o.reconcileHold()
}

func (o *overlayController) collapse(collapsed bool) OverlayState {
	return o.update(func() {
		o.state.Collapsed = collapsed
		o.expanded = !collapsed
		o.lastPlacement = settings.Rect{}
		if !collapsed && o.focusClient != nil {
			o.focusClient()
		}
	})
}

type clientFrame struct {
	Handle       uintptr
	Client, Work settings.Rect
	Scale        float64
	Active       bool
}

// All state and native geometry changes run on the Wails UI thread. The input
// is cached game state plus window metadata; this path has no Source calls.
func (o *overlayController) reconcileCompanion(s core.Snapshot, frame clientFrame, foreground bool, now time.Time) {
	o.dispatch(func() {
		before := o.state
		o.managed = true
		o.latest = s
		o.preview.Observe(s, now)
		o.lastClientActive = frame.Active
		if companion.InGame(s) && (o.state.Mode == "preview" || o.preview.Eligible(s, now)) {
			o.lastGameForeground, o.lastForegroundCheck = foreground, now
		}
		publish := func() {
			if o.state != before {
				o.state.Revision++
				if o.emit != nil {
					o.emit(o.state)
				}
			}
		}
		defer publish()
		if companion.Pregame(s) && o.state.SidebarEnabled {
			if !frame.Active {
				if o.state.Visible {
					o.hideOnUI()
				}
				return
			}
			scale := frame.Scale
			if scale <= 0 {
				scale = 1
			}
			p := companion.Place(frame.Client, frame.Work, int(float64(o.state.SidebarWidth)*scale), int(28*scale), int(6*scale), o.state.Collapsed, o.expanded)
			mode := "sidebar"
			if p.Rail {
				mode = "rail"
			}
			if o.window == nil && o.create != nil {
				o.state.Mode, o.state.SidebarSide = mode, p.Side
				o.window = o.create()
			}
			if o.window == nil || (o.readyRequired && !o.ready) {
				return
			}
			if o.state.Mode != mode || !o.state.Visible || o.lastPlacement != p.Bounds {
				if w, ok := o.window.(interface{ showSidebar(settings.Rect) error }); ok {
					if err := w.showSidebar(p.Bounds); err != nil {
						o.state.Warning = err.Error()
						o.hideOnUI()
						return
					}
				}
				o.state.Mode, o.state.Visible, o.state.SidebarSide = mode, true, p.Side
				o.lastPlacement = p.Bounds
			}
			return
		}
		if o.state.Mode == "sidebar" || o.state.Mode == "rail" {
			o.hideOnUI()
			o.lastPlacement = settings.Rect{}
		}
		// Warm the one auxiliary WebView in loading even before a roster or
		// foreground is available. Only a ready renderer may become visible.
		if s.Connected && s.Phase == "GameStart" && o.window == nil && o.create != nil && (o.state.HoldEnabled || o.state.PreviewEnabled) {
			o.window = o.create()
		}
		if !companion.InGame(s) {
			if o.state.Visible {
				o.hideOnUI()
			}
			return
		}
		if o.state.Mode == "preview" && o.state.Visible {
			if !o.state.PreviewEnabled || !foreground || !now.Before(o.previewUntil) {
				o.hideOnUI()
			}
			return
		}
		if o.state.Mode == "hold" || o.state.Mode == "persistent" {
			return
		}
		if o.holdDesired != nil && o.holdDesired() {
			return
		}
		if !o.state.PreviewEnabled {
			o.preview.Consume()
			return
		}
		if !foreground || !o.preview.Eligible(s, now) {
			return
		}
		if o.window == nil && o.create != nil {
			o.state.Mode = "preview"
			o.window = o.create()
		}
		if o.window == nil || (o.readyRequired && !o.ready) {
			return
		}
		if o.gameForeground != nil && !o.gameForeground() {
			return
		}
		if o.currentSnapshot != nil {
			current := o.currentSnapshot()
			if !companion.InGame(current) || current.GameID != s.GameID || current.Region != s.Region {
				return
			}
		}
		w, ok := o.window.(interface{ showHold() error })
		if !ok {
			return
		}
		o.preview.Consume()
		if err := w.showHold(); err != nil {
			o.state.HoldWarning = err.Error()
			o.hideOnUI()
			return
		}
		if o.gameForeground != nil && !o.gameForeground() {
			o.hideOnUI()
			return
		}
		if o.currentSnapshot != nil {
			current := o.currentSnapshot()
			if !companion.InGame(current) || current.GameID != s.GameID || current.Region != s.Region {
				o.hideOnUI()
				return
			}
		}
		shownAt := now
		if o.clock != nil {
			shownAt = o.clock()
		}
		o.previewUntil = shownAt.Add(companion.PreviewDuration)
		o.state.Mode, o.state.Visible = "preview", true
		o.state.HoldWarning = ""
	})
}

// No identities, game IDs, geometry, paths or request payloads in diagnostics.
type companionDiagnostics struct {
	Phase               string       `json:"phase"`
	Stage               string       `json:"stage"`
	Participants        int          `json:"participants"`
	FrontendReady       bool         `json:"frontendReady"`
	WindowCreated       bool         `json:"windowCreated"`
	ClientActive        bool         `json:"clientActive"`
	GameForeground      bool         `json:"lastGameForeground"`
	ForegroundCheckedAt time.Time    `json:"foregroundCheckedAt"`
	Overlay             OverlayState `json:"overlay"`
}

func (o *overlayController) diagnostics() (result companionDiagnostics) {
	o.dispatch(func() {
		result = companionDiagnostics{o.latest.Phase, o.latest.Stage, len(o.latest.Participants), o.ready, o.window != nil, o.lastClientActive, o.lastGameForeground, o.lastForegroundCheck, o.state}
	})
	return
}
