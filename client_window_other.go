//go:build !windows

package main

type clientWindowMonitor struct{ Changes chan struct{} }

func startClientWindowMonitor() *clientWindowMonitor {
	return &clientWindowMonitor{Changes: make(chan struct{}, 1)}
}
func (*clientWindowMonitor) Watch(uintptr)         {}
func (*clientWindowMonitor) Stop()                 {}
func readClientFrame(uintptr, uintptr) clientFrame { return clientFrame{} }
func focusClientWindow(uintptr)                    {}
