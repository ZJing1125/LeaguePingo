package main

import "github.com/wailsapp/wails/v3/pkg/application"

// Keep native resizing, shadows and Windows 11 corners. Only the system title
// bar is replaced; the web title bar uses the pinned Wails window runtime.
func mainWindowOptions(hidden bool) application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Name: "main", Title: "LeaguePingo", Width: 1280, Height: 800,
		MinWidth: 880, MinHeight: 620, URL: "/",
		Hidden: hidden, Frameless: true,
		BackgroundColour: application.NewRGB(242, 241, 235),
		Windows:          application.WindowsWindow{Theme: application.Light},
	}
}
