package main

import _ "embed"

// Wails selects the ICO frame for the native small/large system metrics. Keep
// the runtime tray/window fallback identical to the executable's resources.
//
//go:embed build/windows/icon.ico
var appIcon []byte
