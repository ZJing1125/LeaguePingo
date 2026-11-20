package main

import (
	"os"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/companion"
	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
	"leaguepingo/internal/hold"
)

func (r *desktopApplication) observeSnapshot(snapshot core.Snapshot) {
	r.backend.resources.SetActive(snapshot.Connected && snapshot.Phase == "InProgress")
	r.backend.guides.Observe(snapshot)
	r.held.Allow(companion.InGame(snapshot))
	r.app.Event.Emit("leaguepingo:snapshot", snapshot)
	wakeDesktop(r.companionWake)
}

func (r *desktopApplication) forwardGuides() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case state := <-r.backend.guides.Changes():
			if state.Base != nil {
				diagnostics.Responses.Published(state.Base.ResponseID)
			}
			if state.Matchup != nil {
				diagnostics.Responses.Published(state.Matchup.ResponseID)
			}
			r.app.Event.Emit("leaguepingo:guide", state)
		}
	}
}

func (r *desktopApplication) forwardAutomation() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case state := <-r.backend.engine.PostGameChanges():
			r.app.Event.Emit("leaguepingo:post-game", state)
		case state := <-r.backend.engine.AutoAcceptChanges():
			r.app.Event.Emit("leaguepingo:auto-accept", state)
		}
	}
}

func (r *desktopApplication) forwardPlayers() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case player := <-r.backend.engine.PlayerUpdates():
			r.app.Event.Emit("leaguepingo:player-updated", player)
		}
	}
}

func (r *desktopApplication) started() {
	defer close(r.windowReady)
	cfg := r.backend.settings.Snapshot()
	r.restoreGeometry(r.mainWindow, cfg.Main, 880, 620)
	r.geometryReady = true
	if r.offlineSmoke {
		if os.Getenv("LEAGUEPINGO_SMOKE_OPEN_OVERLAY") == "1" {
			r.controller.window = r.controller.create()
			r.controller.hide()
		}
		go func() { time.Sleep(35 * time.Second); r.app.Quit() }()
		return
	}
	go r.checkUpdates()
	r.clientMonitor = startClientWindowMonitor()
	go r.watchCompanion(r.clientMonitor)
	var err error
	r.monitor, err = hold.Start(r.held)
	r.controller.update(func() {
		r.controller.state.HoldAvailable = err == nil
		if err != nil {
			r.controller.state.HoldWarning = apperrors.UserMessage(err, apperrors.HoldUnavailable)
		}
	})
	r.backend.configureHold = r.configureHold
	r.held.Configure(cfg.HoldEnabled && r.monitor != nil, hold.Key(cfg.HoldKey))
	go r.watchHold()
	r.backend.configureShortcut = r.configureShortcut
	if err := r.configureShortcut(cfg.Shortcut); err != nil {
		r.controller.update(func() { r.controller.state.Warning = apperrors.UserMessage(err, apperrors.ShortcutUnavailable) })
	}
}

func (r *desktopApplication) checkUpdates() {
	state := r.updates.Check(r.ctx)
	if state.Status != "available" || r.ctx.Err() != nil {
		return
	}
	application.InvokeSync(func() {
		if r.ctx.Err() != nil {
			return
		}
		r.updateItem.SetLabel("发现新版本 " + state.LatestVersion + "（打开下载页）").SetHidden(false)
	})
	if r.ctx.Err() == nil {
		r.tray.SetTooltip("LeaguePingo · 有新版本 " + state.LatestVersion)
	}
}

func (r *desktopApplication) watchCompanion(monitor *clientWindowMonitor) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	var clientHandle uintptr
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-timer.C:
		case <-r.companionWake:
		case <-monitor.Changes:
			// Collapse a burst of native move/resize events into one read.
			select {
			case <-r.ctx.Done():
				return
			case <-time.After(35 * time.Millisecond):
			}
			select {
			case <-monitor.Changes:
			default:
			}
		}
		if r.ctx.Err() != nil {
			return
		}
		snapshot := r.backend.engine.Snapshot()
		delay := 8 * time.Second
		application.InvokeSync(func() { delay = r.reconcileCompanion(snapshot, monitor, &clientHandle) })
		timer.Reset(delay)
	}
}

func (r *desktopApplication) reconcileCompanion(snapshot core.Snapshot, monitor *clientWindowMonitor, clientHandle *uintptr) time.Duration {
	delay := 8 * time.Second
	if r.ctx.Err() != nil {
		return delay
	}
	frame := clientFrame{}
	if companion.Pregame(snapshot) && r.controller.state.SidebarEnabled {
		delay = 2 * time.Second
		var own uintptr
		if w, ok := r.controller.window.(*nativeOverlay); ok {
			own = uintptr(w.NativeWindow())
		}
		frame = readClientFrame(*clientHandle, own)
		*clientHandle = frame.Handle
		monitor.Watch(*clientHandle)
	} else {
		monitor.Watch(0)
	}
	now := time.Now()
	r.controller.preview.Observe(snapshot, now)
	foreground := false
	if companion.InGame(snapshot) && (r.controller.state.Mode == "preview" || r.controller.preview.Eligible(snapshot, now)) {
		foreground = hold.GameForeground()
		delay = 2 * time.Second
	}
	r.controller.reconcileCompanion(snapshot, frame, foreground, now)
	if !r.controller.previewUntil.IsZero() {
		delay = max(time.Millisecond, min(delay, time.Until(r.controller.previewUntil)))
	}
	return delay
}

func (r *desktopApplication) watchHold() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-r.holdWake:
			if r.ctx.Err() == nil {
				application.InvokeSync(func() {
					if r.ctx.Err() == nil {
						r.controller.reconcileHold()
					}
				})
			}
		}
	}
}
