package main

import (
	"context"
	"embed"
	"log"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"leaguepingo/internal/core"
	"leaguepingo/internal/engine"
	"leaguepingo/internal/source"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	var mainWindow *application.WebviewWindow
	windowReady := make(chan struct{})
	backend := &AppService{}
	app := application.New(application.Options{
		Name:        "LeaguePingo",
		Description: "低干扰的英雄联盟战绩助手",
		Services:    []application.Service{application.NewService(backend)},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac:         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		SingleInstance: singleInstanceOptions(func() {
			<-windowReady
			mainWindow.Show()
			mainWindow.UnMinimise()
			mainWindow.Focus()
		}),
	})
	// New exits duplicate instances before any LCU worker or tray is created.
	backend.engine = engine.New(source.DiscoverTencent, func(snapshot core.Snapshot) {
		app.Event.Emit("leaguepingo:snapshot", snapshot)
	})
	mainWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "main", Title: "LeaguePingo", Width: 1100, Height: 760,
		MinWidth: 880, MinHeight: 620, URL: "/",
		BackgroundColour: application.NewRGB(10, 16, 29),
	})
	// Closing the main window keeps the lightweight helper available in the tray.
	mainWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		mainWindow.Hide()
		event.Cancel()
	})
	tray := app.SystemTray.New()
	tray.SetIcon(makeTrayIcon())
	tray.SetTooltip("LeaguePingo")
	menu := app.NewMenu()
	menu.Add("打开 LeaguePingo").OnClick(func(*application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
	})
	tray.SetMenu(menu)
	tray.OnClick(func() { mainWindow.Show(); mainWindow.Focus() })
	overlay := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "overlay", Title: "LeaguePingo · 对局快览", Width: 540, Height: 660,
		MinWidth: 400, MinHeight: 360,
		URL: "/?view=overlay", Hidden: true, Frameless: true, AlwaysOnTop: true,
		BackgroundColour: application.NewRGB(10, 16, 29),
	})
	controller := &overlayController{
		window: overlay, dispatch: application.InvokeSync,
		state: OverlayState{Pinned: true},
		emit:  func(state OverlayState) { app.Event.Emit("leaguepingo:overlay", state) },
	}
	backend.overlay = controller
	overlay.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		controller.hide()
	})
	menu.Add("隐藏对局快览").OnClick(func(*application.Context) { controller.hide() })
	menu.Add("退出").OnClick(func(*application.Context) { app.Quit() })
	// Before Run, Wails only queues registration and cannot report OS conflicts.
	// Register after startup so the UI reflects actual OS results.
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		defer close(windowReady)
		var warnings []string
		var shortcutAvailable bool
		if err := app.GlobalShortcut.Register("Ctrl+Shift+P", func() {
			controller.toggle()
		}); err != nil {
			log.Printf("无法注册 Ctrl+Shift+P 快捷键: %v", err)
			warnings = append(warnings, "Ctrl+Shift+P 注册失败，请使用主窗口或托盘控制快览；检查是否仍有旧版助手运行")
		} else {
			shortcutAvailable = true
		}
		controller.update(func() {
			controller.state.ShortcutAvailable = shortcutAvailable
			controller.state.Warning = strings.Join(warnings, "；")
		})
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
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
