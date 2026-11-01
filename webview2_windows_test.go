//go:build windows

package main

import "testing"

func TestWebView2PreflightGuidesMissingRuntime(t *testing.T) {
	guided := 0
	if requireWebView2Runtime(func() string { return "123.0.0.0" }, func() { guided++ }) == false || guided != 0 {
		t.Fatal("installed runtime rejected")
	}
	if requireWebView2Runtime(func() string { return "0.0.0.0" }, func() { guided++ }) || guided != 1 {
		t.Fatal("missing runtime did not show one guide")
	}
	if validWebView2Version(" ") || validWebView2Version("0.0.0.0") {
		t.Fatal("invalid runtime version accepted")
	}
}
