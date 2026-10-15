package main

import (
	"context"
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"leaguepingo/internal/core"
	"leaguepingo/internal/engine"
	"leaguepingo/internal/source"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	var app *application.App
	backend := &AppService{}
	backend.engine = engine.New(source.DiscoverTencent, func(snapshot core.Snapshot) {
		if app != nil {
			app.Event.Emit("leaguepingo:snapshot", snapshot)
		}
	})
	app = application.New(application.Options{
		Name:        "LeaguePingo",
		Description: "低干扰的英雄联盟战绩助手",
		Services:    []application.Service{application.NewService(backend)},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac:         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
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
	menu.Add("退出").OnClick(func(*application.Context) { app.Quit() })
	tray.SetMenu(menu)
	tray.OnClick(func() { mainWindow.Show(); mainWindow.Focus() })
	overlay := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "overlay", Title: "LeaguePingo · 对局快览", Width: 540, Height: 660,
		URL: "/?view=overlay", Hidden: true, Frameless: true, AlwaysOnTop: true,
		BackgroundColour: application.NewRGB(10, 16, 29),
	})
	overlay.SetIgnoreMouseEvents(true)
	backend.overlay = overlay
	if err := app.GlobalShortcut.Register("Ctrl+Shift+P", func() {
		if overlay.IsVisible() {
			overlay.Hide()
		} else {
			overlay.Show()
		}
	}); err != nil {
		log.Printf("无法注册 Ctrl+Shift+P 快捷键: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go backend.engine.Run(ctx)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
