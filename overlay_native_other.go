//go:build !windows

package main

import (
	"errors"
	"github.com/wailsapp/wails/v3/pkg/application"
	"leaguepingo/internal/settings"
)

type nativeOverlay struct {
	*application.WebviewWindow
	restore func()
}

func (w *nativeOverlay) showHold() error { return errors.New("局内按住快览仅支持 Windows") }
func (w *nativeOverlay) endHold()        {}
func (w *nativeOverlay) prepareManual() {
	if w.restore != nil {
		w.restore()
	}
}
func (w *nativeOverlay) showSidebar(settings.Rect) error {
	return errors.New("客户端侧栏仅支持 Windows")
}
