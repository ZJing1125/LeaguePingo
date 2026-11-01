//go:build windows

package main

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const webView2DownloadURL = "https://go.microsoft.com/fwlink/p/?LinkId=2124703"

var user32 = windows.NewLazySystemDLL("user32.dll")
var messageBox = user32.NewProc("MessageBoxW")
var shell32 = windows.NewLazySystemDLL("shell32.dll")
var shellExecute = shell32.NewProc("ShellExecuteW")

func ensureWebView2Runtime() bool {
	return requireWebView2Runtime(installedWebView2Version, showWebView2Guide)
}

func requireWebView2Runtime(probe func() string, guide func()) bool {
	if validWebView2Version(probe()) {
		return true
	}
	guide()
	return false
}

func validWebView2Version(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && value != "0.0.0.0"
}

func installedWebView2Version() string {
	const client = `Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`
	for _, candidate := range []struct {
		root registry.Key
		path string
		view uint32
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\` + client, registry.WOW64_64KEY},
		{registry.LOCAL_MACHINE, `SOFTWARE\` + client, registry.WOW64_32KEY},
		{registry.LOCAL_MACHINE, `SOFTWARE\` + client, registry.WOW64_64KEY},
		{registry.CURRENT_USER, `Software\` + client, registry.WOW64_64KEY},
		{registry.CURRENT_USER, `Software\` + client, registry.WOW64_32KEY},
	} {
		key, err := registry.OpenKey(candidate.root, candidate.path, registry.QUERY_VALUE|candidate.view)
		if err != nil {
			continue
		}
		value, _, err := key.GetStringValue("pv")
		key.Close()
		if err == nil && validWebView2Version(value) {
			return value
		}
	}
	return ""
}

func showWebView2Guide() {
	title, _ := windows.UTF16PtrFromString("LeaguePingo 缺少运行组件")
	message, _ := windows.UTF16PtrFromString("LeaguePingo 需要 Microsoft Edge WebView2 Runtime。\n\n是否打开微软官方下载页？安装完成后请重新启动 LeaguePingo。")
	const mbYesNoIconWarning = 0x00000004 | 0x00000030
	result, _, _ := messageBox.Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), mbYesNoIconWarning)
	if result != 6 { // IDYES
		return
	}
	operation, _ := windows.UTF16PtrFromString("open")
	target, _ := windows.UTF16PtrFromString(webView2DownloadURL)
	shellExecute.Call(0, uintptr(unsafe.Pointer(operation)), uintptr(unsafe.Pointer(target)), 0, 0, 1)
}
