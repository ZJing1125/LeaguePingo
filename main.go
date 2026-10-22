package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"leaguepingo/internal/core"
	"leaguepingo/internal/engine"
	"leaguepingo/internal/hold"
	"leaguepingo/internal/settings"
	"leaguepingo/internal/source"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	holdWake := make(chan struct{}, 1)
	held := hold.New(func() {
		select {
		case holdWake <- struct{}{}:
		default:
		}
	})
	var monitor *hold.Monitor
	var mainWindow *application.WebviewWindow
	windowReady := make(chan struct{})
	configRoot, err := os.UserConfigDir()
	if err != nil {
		log.Fatal("无法找到用户设置目录")
	}
	configDir := filepath.Join(configRoot, "LeaguePingo")
	offlineSmoke := os.Getenv("LEAGUEPINGO_OFFLINE_SMOKE") == "1"
	if offlineSmoke {
		configDir = os.Getenv("LEAGUEPINGO_SMOKE_DIR")
		if configDir == "" {
			log.Fatal("离线测试必须指定隔离设置目录")
		}
	}
	store, settingsErr := settings.Open(filepath.Join(configDir, "settings.json"))
	backend := &AppService{settings: store, configDir: configDir}
	backend.configureShortcut = func(string) error { return fmt.Errorf("应用尚未准备好注册快捷键") }
	if settingsErr != nil {
		backend.settingsWarning = settingsErr.Error()
	}
	showMain := func() {
		mainWindow.Show()
		mainWindow.UnMinimise()
		mainWindow.Focus()
	}
	instance := singleInstanceOptions(func() {
		<-windowReady
		showMain()
	})
	if offlineSmoke {
		instance.UniqueID = fmt.Sprintf("com.leaguepingo.offline.p%d", os.Getpid())
	}
	app := application.New(application.Options{
		Name:           "LeaguePingo",
		Icon:           appIcon,
		Description:    "低干扰的英雄联盟战绩助手",
		Services:       []application.Service{application.NewService(backend)},
		Assets:         application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac:            application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		SingleInstance: instance,
		OnShutdown: func() {
			cancel()
			if monitor != nil {
				monitor.Stop()
			}
			if err := store.Save(); err != nil {
				log.Print("无法保存窗口设置")
			}
		},
	})
	// New exits duplicate instances before any LCU worker or tray is created.
	discover := source.DiscoverTencent
	if offlineSmoke {
		discover = func() (source.Source, error) { return nil, source.ErrNotConnected }
	}
	backend.engine = engine.New(discover, func(snapshot core.Snapshot) {
		held.Allow(snapshot.Connected && snapshot.Phase == "InProgress")
		app.Event.Emit("leaguepingo:snapshot", snapshot)
	})
	mainWindow = app.Window.NewWithOptions(mainWindowOptions(offlineSmoke))
	// Closing the main window keeps the lightweight helper available in the tray.
	mainWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		mainWindow.Hide()
		event.Cancel()
	})
	tray := app.SystemTray.New()
	tray.SetIcon(appIcon)
	tray.SetTooltip("LeaguePingo")
	menu := app.NewMenu()
	menu.Add("打开 LeaguePingo").OnClick(func(*application.Context) {
		showMain()
	})
	tray.SetMenu(menu)
	tray.OnClick(showMain)
	controller := &overlayController{
		dispatch:    application.InvokeSync,
		state:       OverlayState{Pinned: store.Snapshot().Pinned, Shortcut: store.Snapshot().Shortcut, Mode: "hidden", HoldEnabled: store.Snapshot().HoldEnabled, HoldKey: store.Snapshot().HoldKey},
		holdDesired: func() bool { return held.Desired() && hold.GameForeground() },
		emit:        func(state OverlayState) { app.Event.Emit("leaguepingo:overlay", state) },
	}
	backend.overlay = controller
	// Geometry changes only update memory. Disk writes occur on explicit setting
	// changes or normal exit, never on each drag frame or engine poll.
	geometryReady := false
	watchGeometry := func(w *application.WebviewWindow, isMain bool) {
		capture := func(*application.WindowEvent) {
			if ctx.Err() != nil {
				return
			}
			application.InvokeSync(func() {
				if ctx.Err() != nil || !geometryReady || (!isMain && (controller.state.Mode == "hold" || !w.IsVisible())) || w.IsMinimised() || w.IsMaximised() {
					return
				}
				r := w.Bounds()
				store.SetBounds(isMain, settings.Rect{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height})
			})
		}
		w.OnWindowEvent(events.Common.WindowDidMove, capture)
		w.OnWindowEvent(events.Common.WindowDidResize, capture)
	}
	restoreGeometry := func(w *application.WebviewWindow, r settings.Rect, mw, mh int) {
		var areas []settings.Rect
		for _, screen := range app.Screen.GetAll() {
			a := screen.WorkArea
			areas = append(areas, settings.Rect{X: a.X, Y: a.Y, Width: a.Width, Height: a.Height})
		}
		r = settings.Clamp(r, areas, mw, mh)
		w.SetBounds(application.Rect{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height})
	}
	watchGeometry(mainWindow, true)
	// No second WebView until the user first requests the overlay. Later hides
	// retain it for quick reopen; no per-frame polling or animated background.
	controller.create = func() overlayWindow {
		cfg := store.Snapshot()
		w := app.Window.NewWithOptions(application.WebviewWindowOptions{
			Name: "overlay", Title: "LeaguePingo · 对局快览", Width: 1100, Height: 640, MinWidth: 400, MinHeight: 360,
			URL: "/?view=overlay", Hidden: true, Frameless: true, AlwaysOnTop: cfg.Pinned, BackgroundColour: application.NewRGB(20, 28, 32),
			Windows: application.WindowsWindow{Theme: application.Dark, HiddenOnTaskbar: true},
		})
		restoreGeometry(w, cfg.Overlay, 400, 360)
		watchGeometry(w, false)
		w.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) { event.Cancel(); controller.hide() })
		return &nativeOverlay{WebviewWindow: w, restore: func() { restoreGeometry(w, store.Snapshot().Overlay, 400, 360) }}
	}
	menu.Add("隐藏对局快览").OnClick(func(*application.Context) { controller.hide() })
	menu.Add("退出").OnClick(func(*application.Context) { app.Quit() })
	// Before Run, Wails only queues registration and cannot report OS conflicts.
	// Register after startup so the UI reflects actual OS results.
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		defer close(windowReady)
		cfg := store.Snapshot()
		restoreGeometry(mainWindow, cfg.Main, 880, 620)
		geometryReady = true
		if offlineSmoke {
			if os.Getenv("LEAGUEPINGO_SMOKE_OPEN_OVERLAY") == "1" {
				controller.window = controller.create()
				controller.hide()
			}
			go func() { time.Sleep(35 * time.Second); app.Quit() }()
			return
		}
		var monitorErr error
		monitor, monitorErr = hold.Start(held)
		controller.update(func() {
			controller.state.HoldAvailable = monitorErr == nil
			if monitorErr != nil {
				controller.state.HoldWarning = monitorErr.Error()
			}
		})
		var holdConfigMu sync.Mutex
		backend.configureHold = func(enabled bool, key string) error {
			holdConfigMu.Lock()
			defer holdConfigMu.Unlock()
			old := store.Snapshot()
			store.SetHold(enabled, key)
			if err := store.Save(); err != nil {
				store.SetHold(old.HoldEnabled, old.HoldKey)
				return err
			}
			held.Configure(enabled && monitor != nil, hold.Key(key))
			controller.update(func() { controller.state.HoldEnabled = enabled; controller.state.HoldKey = key })
			return nil
		}
		held.Configure(cfg.HoldEnabled && monitor != nil, hold.Key(cfg.HoldKey))
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-holdWake:
					if ctx.Err() == nil {
						application.InvokeSync(func() {
							if ctx.Err() == nil {
								controller.reconcileHold()
							}
						})
					}
				}
			}
		}()
		registered := ""
		var shortcutMu sync.Mutex
		backend.configureShortcut = func(key string) error {
			shortcutMu.Lock()
			defer shortcutMu.Unlock()
			if key == registered {
				return nil
			}
			if err := app.GlobalShortcut.Register(key, func() { controller.toggle() }); err != nil {
				return fmt.Errorf("%s 注册失败，可能被占用；原设置保留，可用按钮或托盘控制", key)
			}
			old := store.Snapshot().Shortcut
			store.SetShortcut(key)
			if err := store.Save(); err != nil {
				store.SetShortcut(old)
				_ = app.GlobalShortcut.Unregister(key)
				return err
			}
			if registered != "" {
				_ = app.GlobalShortcut.Unregister(registered)
			}
			registered = key
			controller.update(func() {
				controller.state.Shortcut = key
				controller.state.ShortcutAvailable = true
				controller.state.Warning = ""
			})
			return nil
		}
		if err := backend.configureShortcut(cfg.Shortcut); err != nil {
			controller.update(func() { controller.state.Warning = err.Error() })
		}
	})
	go backend.engine.Run(ctx)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func singleInstanceOptions(activate func()) *application.SingleInstanceOptions {
	return &application.SingleInstanceOptions{
		// Stable across executable names and future releases in this Windows session.
		UniqueID:               "com.leaguepingo.desktop",
		OnSecondInstanceLaunch: func(application.SecondInstanceData) { activate() },
	}
}
