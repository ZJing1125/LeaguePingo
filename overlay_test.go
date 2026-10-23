package main

import (
	"fmt"
	"leaguepingo/internal/core"
	"leaguepingo/internal/settings"
	"testing"
	"time"

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

type fakeCompanionOverlay struct {
	fakeHoldOverlay
	bounds settings.Rect
}

func (w *fakeCompanionOverlay) showSidebar(r settings.Rect) error {
	w.bounds = r
	w.visible = true
	return nil
}

func TestCompanionSidebarPreviewHoldAndEndTransition(t *testing.T) {
	now := time.Now()
	w := &fakeCompanionOverlay{}
	down := false
	o := &overlayController{window: w, dispatch: func(f func()) { f() }, state: OverlayState{SidebarEnabled: true, PreviewEnabled: true, SidebarWidth: 320}, holdDesired: func() bool { return down }}
	s := core.Snapshot{Connected: true, Phase: "ChampSelect", GameID: 42, Participants: []core.Participant{{Key: "synthetic"}}}
	f := clientFrame{Active: true, Scale: 1, Client: settings.Rect{X: 400, Y: 100, Width: 1100, Height: 720}, Work: settings.Rect{Width: 1920, Height: 1080}}
	o.reconcileCompanion(s, f, false, now)
	if o.state.Mode != "sidebar" || w.bounds.X != 74 {
		t.Fatal("left sidebar not attached")
	}
	o.collapse(true)
	o.reconcileCompanion(s, f, false, now)
	if o.state.Mode != "rail" || w.bounds.Width != 28 {
		t.Fatal("manual collapse lost")
	}
	s.Phase = "GameStart"
	o.reconcileCompanion(s, clientFrame{}, true, now)
	if o.state.Mode != "preview" || !w.held {
		t.Fatal("loading must replace sidebar with preview")
	}
	s.Phase = "InProgress"
	o.reconcileCompanion(s, clientFrame{}, true, now.Add(time.Second))
	if o.state.Mode != "preview" || !w.held {
		t.Fatal("gameplay interrupted loading preview")
	}
	o.reconcileCompanion(s, clientFrame{}, true, now.Add(5*time.Second))
	down = true
	o.reconcileHold()
	o.reconcileCompanion(s, clientFrame{}, true, now.Add(7*time.Second))
	if o.state.Mode != "hold" {
		t.Fatal("preview deadline closed held window")
	}
	down = false
	o.reconcileHold()
	o.reconcileCompanion(s, clientFrame{}, true, now.Add(8*time.Second))
	if o.state.Visible {
		t.Fatal("release or late data reopened preview")
	}
	s.GameID = 43
	o.reconcileCompanion(s, clientFrame{}, true, now.Add(2*time.Minute))
	o.reconcileCompanion(s, clientFrame{}, true, now.Add(2*time.Minute+6*time.Second))
	if o.state.Visible {
		t.Fatal("preview did not expire")
	}
	s.Phase = "EndOfGame"
	o.reconcileCompanion(s, clientFrame{}, true, now.Add(3*time.Minute))
	if o.state.Visible {
		t.Fatal("end kept overlay visible")
	}
}

func TestPreviewWaitsForFrontendAndRechecksFocusAndPhase(t *testing.T) {
	now := time.Now()
	w := &fakeHoldOverlay{}
	foreground := true
	s := core.Snapshot{Connected: true, Phase: "InProgress", GameID: 42, Participants: []core.Participant{{Key: "synthetic"}}}
	o := &overlayController{dispatch: func(f func()) { f() }, state: OverlayState{PreviewEnabled: true}, readyRequired: true, gameForeground: func() bool { return foreground }, currentSnapshot: func() core.Snapshot { return s }, create: func() overlayWindow { return w }}
	o.reconcileCompanion(s, clientFrame{}, true, now)
	if o.state.Visible {
		t.Fatal("blank/unmounted frontend shown")
	}
	o.ready = true
	foreground = false
	o.reconcileCompanion(s, clientFrame{}, true, now)
	if o.state.Visible {
		t.Fatal("late creation showed over another application")
	}
	foreground = true
	o.reconcileCompanion(s, clientFrame{}, true, now)
	if !o.state.Visible {
		t.Fatal("first ready foreground display failed")
	}
	o.reconcileCompanion(s, clientFrame{}, false, now.Add(time.Second))
	o.reconcileCompanion(s, clientFrame{}, true, now.Add(2*time.Second))
	if o.state.Visible {
		t.Fatal("returning to game replayed automatic preview")
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

type fakeHoldOverlay struct {
	fakeOverlay
	held bool
	fail bool
}

func (f *fakeHoldOverlay) showHold() error {
	if f.fail {
		return fmt.Errorf("native failure")
	}
	f.held = true
	f.visible = true
	return nil
}
func (f *fakeHoldOverlay) endHold() { f.held = false }
func TestHoldDoesNotHidePersistentOrShowAfterRelease(t *testing.T) {
	down := true
	w := &fakeHoldOverlay{}
	o := &overlayController{dispatch: func(fn func()) { fn() }, window: w, holdDesired: func() bool { return down }}
	if s := o.reconcileHold(); s.Mode != "hold" || !w.held {
		t.Fatal("temporary show")
	}
	down = false
	if s := o.reconcileHold(); s.Visible || w.held {
		t.Fatal("release")
	}
	o.toggle()
	down = true
	o.reconcileHold()
	down = false
	if s := o.reconcileHold(); !s.Visible || s.Mode != "persistent" {
		t.Fatal("release closed persistent view")
	}
	o.hide()
	o.window = nil
	down = true
	o.create = func() overlayWindow { down = false; return w }
	if s := o.reconcileHold(); s.Visible {
		t.Fatal("late initialization showed after release")
	}
	down = true
	w.fail = true
	if s := o.reconcileHold(); s.Visible || s.HoldWarning == "" {
		t.Fatal("native failure must keep hidden")
	}
}

func TestLoadingWaitsForRendererThenShowsOnce(t *testing.T) {
	w := &fakeCompanionOverlay{}
	o := &overlayController{dispatch: func(f func()) { f() }, state: OverlayState{PreviewEnabled: true}, readyRequired: true}
	created := 0
	o.create = func() overlayWindow { created++; return w }
	s := core.Snapshot{Connected: true, Phase: "GameStart", GameID: 42, Participants: []core.Participant{{Key: "synthetic"}}}
	o.reconcileCompanion(s, clientFrame{}, true, time.Now())
	if created != 1 || o.state.Visible || w.visible || w.held {
		t.Fatal("loading warm-up showed a window")
	}
	o.reconcileCompanion(s, clientFrame{}, true, time.Now())
	if created != 1 {
		t.Fatal("warm-up created a second window")
	}
	o.frontendReady()
	now := time.Now()
	o.reconcileCompanion(s, clientFrame{}, true, now)
	if !o.state.Visible || o.state.Mode != "preview" {
		t.Fatal("ready renderer did not display loading preview")
	}
	deadline := o.previewUntil
	s.Phase = "InProgress"
	o.reconcileCompanion(s, clientFrame{}, true, now.Add(time.Second))
	if o.previewUntil != deadline {
		t.Fatal("gameplay renewed preview deadline")
	}
	o.reconcileCompanion(s, clientFrame{}, true, deadline)
	o.reconcileCompanion(s, clientFrame{}, true, deadline.Add(time.Second))
	if o.state.Visible || created != 1 {
		t.Fatal("same game reopened or created another window")
	}
}

func TestLoadingManualAndHoldRemainAvailableAfterAutomaticDeadline(t *testing.T) {
	now := time.Now()
	w := &fakeCompanionOverlay{}
	down := false
	s := core.Snapshot{Connected: true, Phase: "GameStart", GameID: 42, Participants: []core.Participant{{Key: "synthetic"}}}
	o := &overlayController{window: w, dispatch: func(f func()) { f() }, state: OverlayState{PreviewEnabled: true}, currentSnapshot: func() core.Snapshot { return s }, holdDesired: func() bool { return down }}
	o.reconcileCompanion(s, clientFrame{}, false, now)
	o.reconcileCompanion(s, clientFrame{}, true, now.Add(time.Minute))
	if o.state.Visible {
		t.Fatal("late focus reopened automatic loading preview")
	}
	if state := o.toggle(); !state.Visible || state.Mode != "persistent" {
		t.Fatal("loading manual action remained blocked")
	}
	o.hide()
	down = true
	if state := o.reconcileHold(); !state.Visible || state.Mode != "hold" {
		t.Fatal("loading hold action remained blocked")
	}
	down = false
	o.reconcileHold()
	if o.state.Visible {
		t.Fatal("loading release kept hold visible")
	}
}
