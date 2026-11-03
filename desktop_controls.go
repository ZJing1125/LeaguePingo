package main

import (
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/hold"
	"leaguepingo/internal/settings"
)

func (r *desktopApplication) connectOverlay() {
	cfg := r.backend.settings.Snapshot()
	r.controller = &overlayController{
		dispatch:      application.InvokeSync,
		state:         OverlayState{Pinned: cfg.Pinned, Shortcut: cfg.Shortcut, Mode: "hidden", HoldEnabled: cfg.HoldEnabled, HoldKey: cfg.HoldKey, SidebarEnabled: cfg.SidebarEnabled, PreviewEnabled: cfg.PreviewEnabled, SidebarWidth: cfg.SidebarWidth},
		readyRequired: true, managed: true, gameForeground: hold.GameForeground,
		currentSnapshot: r.backend.engine.Snapshot, clock: time.Now,
		wake:        func() { wakeDesktop(r.companionWake) },
		holdDesired: func() bool { return r.held.Desired() && hold.GameForeground() },
		emit:        func(state OverlayState) { r.app.Event.Emit("leaguepingo:overlay", state) },
	}
	r.backend.overlay = r.controller
	r.controller.focusClient = func() {
		var own uintptr
		if w, ok := r.controller.window.(*nativeOverlay); ok {
			own = uintptr(w.NativeWindow())
		}
		focusClientWindow(own)
	}
	r.watchGeometry(r.mainWindow, true)
	r.controller.create = r.createCompanionWindow
}

// Geometry changes only update memory. Disk writes occur on explicit setting
// changes or normal exit, never on each drag frame or engine poll.
func (r *desktopApplication) watchGeometry(w *application.WebviewWindow, isMain bool) {
	capture := func(*application.WindowEvent) {
		if r.ctx.Err() != nil {
			return
		}
		application.InvokeSync(func() {
			if r.ctx.Err() != nil || !r.geometryReady || (!isMain && (r.controller.state.Mode != "persistent" || !w.IsVisible())) || w.IsMinimised() || w.IsMaximised() {
				return
			}
			bounds := w.Bounds()
			r.backend.settings.SetBounds(isMain, settings.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: bounds.Height})
		})
	}
	w.OnWindowEvent(events.Common.WindowDidMove, capture)
	w.OnWindowEvent(events.Common.WindowDidResize, capture)
}

func (r *desktopApplication) restoreGeometry(w *application.WebviewWindow, bounds settings.Rect, minWidth, minHeight int) {
	var areas []settings.Rect
	for _, screen := range r.app.Screen.GetAll() {
		a := screen.WorkArea
		areas = append(areas, settings.Rect{X: a.X, Y: a.Y, Width: a.Width, Height: a.Height})
	}
	bounds = settings.Clamp(bounds, areas, minWidth, minHeight)
	w.SetBounds(application.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: bounds.Height})
}

func (r *desktopApplication) createCompanionWindow() overlayWindow {
	cfg := r.backend.settings.Snapshot()
	w := r.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "overlay", Title: "LeaguePingo · 游戏助手", Width: 1100, Height: 640, MinWidth: 28, MinHeight: 200,
		URL: "/?view=companion", Hidden: true, Frameless: true, AlwaysOnTop: cfg.Pinned, BackgroundColour: application.NewRGB(28, 28, 31),
		Windows: application.WindowsWindow{Theme: application.Dark, HiddenOnTaskbar: true},
	})
	r.restoreGeometry(w, cfg.Overlay, 400, 360)
	r.watchGeometry(w, false)
	w.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) { event.Cancel(); r.controller.hide() })
	return &nativeOverlay{WebviewWindow: w, restore: func() { r.restoreGeometry(w, r.backend.settings.Snapshot().Overlay, 400, 360) }}
}

func (r *desktopApplication) configureHold(enabled bool, key string) error {
	r.holdConfigMu.Lock()
	defer r.holdConfigMu.Unlock()
	store := r.backend.settings
	old := store.Snapshot()
	store.SetHold(enabled, key)
	if err := store.Save(); err != nil {
		store.SetHold(old.HoldEnabled, old.HoldKey)
		return err
	}
	r.held.Configure(enabled && r.monitor != nil, hold.Key(key))
	r.controller.update(func() { r.controller.state.HoldEnabled = enabled; r.controller.state.HoldKey = key })
	return nil
}

func (r *desktopApplication) configureShortcut(key string) error {
	r.shortcutMu.Lock()
	defer r.shortcutMu.Unlock()
	if key == r.registeredShortcut {
		return nil
	}
	if err := r.app.GlobalShortcut.Register(key, func() { r.controller.toggle() }); err != nil {
		return apperrors.New(apperrors.ShortcutUnavailable)
	}
	store := r.backend.settings
	old := store.Snapshot().Shortcut
	store.SetShortcut(key)
	if err := store.Save(); err != nil {
		store.SetShortcut(old)
		_ = r.app.GlobalShortcut.Unregister(key)
		return err
	}
	if r.registeredShortcut != "" {
		_ = r.app.GlobalShortcut.Unregister(r.registeredShortcut)
	}
	r.registeredShortcut = key
	r.controller.update(func() {
		r.controller.state.Shortcut = key
		r.controller.state.ShortcutAvailable = true
		r.controller.state.Warning = ""
	})
	return nil
}
