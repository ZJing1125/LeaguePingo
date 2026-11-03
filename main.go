package main

import (
	"context"
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Check before Wails creates any WebView; a clean Windows installation
	// needs the native recovery guide when WebView2 is missing.
	if !ensureWebView2Runtime() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	desktop, err := newDesktopApplication(ctx, cancel, assets)
	if err != nil {
		log.Fatal(err)
	}
	if err := desktop.app.Run(); err != nil {
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
