package main

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type fakeOverlay struct{ visible, pinned bool }

func (f *fakeOverlay) Show() application.Window                 { f.visible = true; return nil }
func (f *fakeOverlay) Hide() application.Window                 { f.visible = false; return nil }
func (f *fakeOverlay) SetAlwaysOnTop(b bool) application.Window { f.pinned = b; return nil }

func TestOverlayControlsAndRecovery(t *testing.T) {
	w := &fakeOverlay{pinned: true}
	var published OverlayState
	o := &overlayController{window: w, dispatch: func(fn func()) { fn() }, emit: func(state OverlayState) { published = state }, state: OverlayState{Pinned: true}}
	if o.snapshot().Visible {
		t.Fatal("overlay must start hidden and interactive")
	}
	if !o.toggle().Visible || !w.visible {
		t.Fatal("show failed")
	}
	if o.setPinned(false).Pinned || w.pinned {
		t.Fatal("unpin failed")
	}
	if state := o.hide(); state.Visible || w.visible {
		t.Fatal("hide failed")
	}
	if state := o.toggle(); !state.Visible {
		t.Fatal("reopen is not interactive")
	}
	if state := o.toggle(); state.Visible {
		t.Fatal("toggle did not hide")
	}
	if published != o.snapshot() || published.Revision == 0 {
		t.Fatal("state changes not published")
	}
}

func TestStableSingleInstanceConfiguration(t *testing.T) {
	activated := false
	options := singleInstanceOptions(func() { activated = true })
	if options.UniqueID != "com.leaguepingo.desktop" || options.ExitCode != 0 {
		t.Fatal("single instance ID must not depend on executable name")
	}
	options.OnSecondInstanceLaunch(application.SecondInstanceData{})
	if !activated {
		t.Fatal("duplicate launch did not activate the first instance")
	}
}

func TestOverlayIsCreatedOnlyOnFirstShow(t *testing.T) {
	count := 0
	w := &fakeOverlay{}
	o := &overlayController{dispatch: func(fn func()) { fn() }, create: func() overlayWindow { count++; return w }}
	o.hide()
	o.setPinned(true)
	if count != 0 {
		t.Fatal("hidden controls created unused WebView")
	}
	o.toggle()
	o.hide()
	o.toggle()
	if count != 1 || !w.visible {
		t.Fatal("overlay creation was not lazy and reusable")
	}
}
