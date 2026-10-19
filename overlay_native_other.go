//go:build !windows

package main

import (
	"errors"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type nativeOverlay struct {
	*application.WebviewWindow
	restore func()
}

func (w *nativeOverlay) showHold() error { return errors.New("局内按住快览仅支持 Windows") }
func (w *nativeOverlay) endHold()        {}
