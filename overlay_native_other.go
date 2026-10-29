//go:build !windows

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/settings"
)

type nativeOverlay struct {
	*application.WebviewWindow
	restore func()
}

func (w *nativeOverlay) showHold() error { return apperrors.New(apperrors.OverlayUnavailable) }
func (w *nativeOverlay) endHold()        {}
func (w *nativeOverlay) prepareManual() {
	if w.restore != nil {
		w.restore()
	}
}
func (w *nativeOverlay) showSidebar(settings.Rect) error {
	return apperrors.New(apperrors.OverlayUnavailable)
}
