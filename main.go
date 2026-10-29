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

	"leaguepingo/internal/companion"
	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
	"leaguepingo/internal/engine"
	"leaguepingo/internal/guides"
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
	companionWake := make(chan struct{}, 1)
	held := hold.New(func() {
		select {
		case holdWake <- struct{}{}:
		default:
		}
	})
	var monitor *hold.Monitor
	var clientMonitor *clientWindowMonitor
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
			if clientMonitor != nil {
				clientMonitor.Stop()
			}
			if err := store.Save(); err != nil {
				log.Print("无法保存窗口设置")
			}
		},
	})
	// New exits duplicate instances before any LCU worker or tray is created.
	backend.resources = diagnostics.NewResources()
	go backend.resources.Run(ctx)
	discover := source.DiscoverTencent
	if offlineSmoke {
		discover = func() (source.Source, error) { return nil, source.ErrNotConnected }
	}
	backend.guides = guides.New(ctx, filepath.Join(configDir, "guide-cache"), nil)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case state := <-backend.guides.Changes():
				app.Event.Emit("leaguepingo:guide", state)
			}
		}
	}()
	backend.engine, err = engine.NewWithHistoryCache(discover, func(snapshot core.Snapshot) {
		backend.resources.SetActive(snapshot.Connected && snapshot.Phase == "InProgress")
		backend.guides.Observe(snapshot)
		held.Allow(companion.InGame(snapshot))
		app.Event.Emit("leaguepingo:snapshot", snapshot)
		select {
		case companionWake <- struct{}{}:
		default:
		}
	}, filepath.Join(configDir, "history-cache.json"))
	if err != nil {
		log.Print("无法读取本地战绩缓存，将使用本次运行内存缓存")
	}
	_ = backend.engine.ConfigurePostGame(store.Snapshot().AutoReturnEnabled, store.Snapshot().AutoQueueEnabled, store.Snapshot().AutoQueueDelay)
	_ = backend.engine.ConfigureAutoAccept(store.Snapshot().AutoAcceptEnabled, store.Snapshot().AutoAcceptDelay)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case state := <-backend.engine.PostGameChanges():
				app.Event.Emit("leaguepingo:post-game", state)
			case state := <-backend.engine.AutoAcceptChanges():
				app.Event.Emit("leaguepingo:auto-accept", state)
			}
		}
	}()
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
		dispatch:        application.InvokeSync,
		state:           OverlayState{Pinned: store.Snapshot().Pinned, Shortcut: store.Snapshot().Shortcut, Mode: "hidden", HoldEnabled: store.Snapshot().HoldEnabled, HoldKey: store.Snapshot().HoldKey, SidebarEnabled: store.Snapshot().SidebarEnabled, PreviewEnabled: store.Snapshot().PreviewEnabled, SidebarWidth: store.Snapshot().SidebarWidth},
		readyRequired:   true,
		managed:         true,
		gameForeground:  hold.GameForeground,
		currentSnapshot: backend.engine.Snapshot,
		clock:           time.Now,
		wake: func() {
			select {
			case companionWake <- struct{}{}:
			default:
			}
		},
		holdDesired: func() bool { return held.Desired() && hold.GameForeground() },
		emit:        func(state OverlayState) { app.Event.Emit("leaguepingo:overlay", state) },
	}
	backend.overlay = controller
	controller.focusClient = func() {
		var own uintptr
		if w, ok := controller.window.(*nativeOverlay); ok {
			own = uintptr(w.NativeWindow())
		}
		focusClientWindow(own)
	}
	// Geometry changes only update memory. Disk writes occur on explicit setting
	// changes or normal exit, never on each drag frame or engine poll.
	geometryReady := false
	watchGeometry := func(w *application.WebviewWindow, isMain bool) {
		capture := func(*application.WindowEvent) {
			if ctx.Err() != nil {
				return
			}
			application.InvokeSync(func() {
				if ctx.Err() != nil || !geometryReady || (!isMain && (controller.state.Mode != "persistent" || !w.IsVisible())) || w.IsMinimised() || w.IsMaximised() {
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
	// One lazy auxiliary WebView serves both pregame sidebar and in-game glance.
	controller.create = func() overlayWindow {
		cfg := store.Snapshot()
		w := app.Window.NewWithOptions(application.WebviewWindowOptions{
			Name: "overlay", Title: "LeaguePingo · 游戏助手", Width: 1100, Height: 640, MinWidth: 28, MinHeight: 200,
			URL: "/?view=companion", Hidden: true, Frameless: true, AlwaysOnTop: cfg.Pinned, BackgroundColour: application.NewRGB(28, 28, 31),
			Windows: application.WindowsWindow{Theme: application.Dark, HiddenOnTaskbar: true},
		})
		restoreGeometry(w, cfg.Overlay, 400, 360)
		watchGeometry(w, false)
		w.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) { event.Cancel(); controller.hide() })
		return &nativeOverlay{WebviewWindow: w, restore: func() { restoreGeometry(w, store.Snapshot().Overlay, 400, 360) }}
	}
	menu.Add("显示／收起游戏助手").OnClick(func(*application.Context) { controller.toggle() })
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
		clientMonitor = startClientWindowMonitor()
		go func() {
			timer := time.NewTimer(0)
			defer timer.Stop()
			var clientHandle uintptr
			for {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
				case <-companionWake:
				case <-clientMonitor.Changes:
					// Collapse a burst of native move/resize events into one read.
					select {
					case <-ctx.Done():
						return
					case <-time.After(35 * time.Millisecond):
					}
					select {
					case <-clientMonitor.Changes:
					default:
					}
				}
				if ctx.Err() != nil {
					return
				}
				snapshot := backend.engine.Snapshot()
				delay := 8 * time.Second
				application.InvokeSync(func() {
					if ctx.Err() != nil {
						return
					}
					frame := clientFrame{}
					if companion.Pregame(snapshot) && controller.state.SidebarEnabled {
						delay = 2 * time.Second
						var own uintptr
						if w, ok := controller.window.(*nativeOverlay); ok {
							own = uintptr(w.NativeWindow())
						}
						frame = readClientFrame(clientHandle, own)
						clientHandle = frame.Handle
						clientMonitor.Watch(clientHandle)
					} else {
						clientMonitor.Watch(0)
					}
					now := time.Now()
					controller.preview.Observe(snapshot, now)
					foreground := false
					if companion.InGame(snapshot) && (controller.state.Mode == "preview" || controller.preview.Eligible(snapshot, now)) {
						foreground = hold.GameForeground()
						delay = 2 * time.Second
					}
					controller.reconcileCompanion(snapshot, frame, foreground, now)
					if !controller.previewUntil.IsZero() {
						delay = max(time.Millisecond, min(delay, time.Until(controller.previewUntil)))
					}
				})
				timer.Reset(delay)
			}
		}()
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
