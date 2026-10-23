package companion

import (
	"leaguepingo/internal/core"
	"leaguepingo/internal/settings"
	"testing"
	"time"
)

func TestPreviewOnceAcrossReadinessReconnectAndLateData(t *testing.T) {
	now := time.Now()
	s := core.Snapshot{Connected: true, Region: "TEST", GameID: 42, Phase: "GameStart"}
	var p Preview
	s.Phase = "ChampSelect"
	p.Observe(s, now)
	s.Phase = "GameStart"
	p.Observe(s, now)
	if p.Eligible(s, now) {
		t.Fatal("empty loading roster triggered preview")
	}
	s.Participants = []core.Participant{{Key: "synthetic"}}
	if !p.Eligible(s, now.Add(time.Second)) {
		t.Fatal("loading with first roster not eligible")
	}
	p.Consume()
	s.Phase = "InProgress"
	p.Observe(s, now.Add(time.Minute))
	if p.Eligible(s, now.Add(time.Minute)) {
		t.Fatal("gameplay replayed loading preview")
	}
	s.Phase = "Reconnect"
	p.Observe(s, now)
	s.Phase = "InProgress"
	s.ConnectionID++
	p.Observe(s, now)
	if p.Eligible(s, now) {
		t.Fatal("reconnect renewed preview")
	}
	s.GameID++
	p.Observe(s, now)
	if !p.Eligible(s, now) {
		t.Fatal("new game not eligible")
	}
	if p.Eligible(s, now.Add(EntryWindow+time.Second)) {
		t.Fatal("late focus/roster caused surprise popup")
	}
}

func TestPlaceLeftRightOverflowAndNegativeMonitor(t *testing.T) {
	work := settings.Rect{Width: 1920, Height: 1080}
	client := settings.Rect{X: 400, Y: 150, Width: 1100, Height: 720}
	p := Place(client, work, 320, 28, 6, false, false)
	if p.Side != "left" || p.Rail || p.Bounds.X != 74 || p.Bounds.Height != 720 {
		t.Fatalf("left: %+v", p)
	}
	client.X = 20
	p = Place(client, work, 320, 28, 6, false, false)
	if p.Side != "right" || p.Bounds.X != 1126 {
		t.Fatalf("right: %+v", p)
	}
	client.Width = 1880
	if p = Place(client, work, 320, 28, 6, false, false); !p.Rail || p.Bounds.Width != 28 {
		t.Fatalf("overflow rail: %+v", p)
	}
	if p = Place(client, work, 320, 28, 6, false, true); p.Rail || p.Bounds.X != 20 {
		t.Fatalf("inward expansion: %+v", p)
	}
	work.X = -1920
	client = settings.Rect{X: -1400, Y: 60, Width: 1100, Height: 720}
	p = Place(client, work, 400, 35, 8, false, false)
	if p.Side != "left" || p.Bounds.X != -1808 {
		t.Fatalf("negative/high DPI: %+v", p)
	}
	if p = Place(client, work, 400, 35, 8, true, false); !p.Rail || p.Bounds.Width != 35 {
		t.Fatal("manual collapse lost")
	}
}

func TestSidebarUsesAvailableOutsideWidthBeforeTheDecorativeGap(t *testing.T) {
	for _, test := range []struct {
		name          string
		work, client  settings.Rect
		width, gap, x int
		side          string
	}{
		{"centered-1920", settings.Rect{Width: 1920, Height: 1080}, settings.Rect{X: 320, Y: 100, Width: 1280, Height: 720}, 320, 6, 0, "left"},
		{"left-partial-gap-before-roomier-right", settings.Rect{Width: 1920, Height: 1080}, settings.Rect{X: 323, Y: 100, Width: 1100, Height: 720}, 320, 6, 0, "left"},
		{"right-exact", settings.Rect{Width: 1920, Height: 1080}, settings.Rect{X: 0, Y: 100, Width: 1600, Height: 720}, 320, 6, 1600, "right"},
		{"right-partial-gap", settings.Rect{Width: 1920, Height: 1080}, settings.Rect{X: 20, Y: 100, Width: 1577, Height: 720}, 320, 6, 1600, "right"},
		{"configured-280", settings.Rect{Width: 2000, Height: 1080}, settings.Rect{X: 280, Y: 100, Width: 1440, Height: 720}, 280, 6, 0, "left"},
		{"configured-420", settings.Rect{Width: 2120, Height: 1080}, settings.Rect{X: 420, Y: 100, Width: 1280, Height: 720}, 420, 6, 0, "left"},
		{"125-percent-negative-screen", settings.Rect{X: -2400, Width: 2400, Height: 1350}, settings.Rect{X: -2000, Y: 100, Width: 1600, Height: 900}, 400, 7, -2400, "left"},
		{"150-percent", settings.Rect{Width: 2880, Height: 1620}, settings.Rect{X: 480, Y: 100, Width: 1920, Height: 1080}, 480, 9, 0, "left"},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, action := range []struct {
				name     string
				expanded bool
			}{{"automatic", false}, {"reopened", true}} {
				t.Run(action.name, func(t *testing.T) {
					p := Place(test.client, test.work, test.width, 28, test.gap, false, action.expanded)
					outside := p.Bounds.X+p.Bounds.Width <= test.client.X || p.Bounds.X >= test.client.X+test.client.Width
					if p.Rail || !outside || p.Side != test.side || p.Bounds.X != test.x || p.Bounds.Width != test.width {
						t.Fatalf("available outside space was ignored: %+v", p)
					}
				})
			}
		})
	}
}

func TestReconnectWithoutIDCannotCreateLatePreview(t *testing.T) {
	now := time.Now()
	var p Preview
	s := core.Snapshot{Connected: true, Phase: "Reconnect"}
	p.Observe(s, now)
	s.Phase = "InProgress"
	p.Observe(s, now)
	// Identity can arrive more than one snapshot after Reconnect.
	p.Observe(s, now)
	s.GameID = 42
	s.Participants = []core.Participant{{Key: "synthetic"}}
	p.Observe(s, now)
	if p.Eligible(s, now) {
		t.Fatal("late reconnect ID renewed preview")
	}
	s.Phase = "Lobby"
	p.Observe(s, now)
	s.GameID = 43
	s.Phase = "GameStart"
	p.Observe(s, now)
	s.Phase = "InProgress"
	p.Observe(s, now)
	if !p.Eligible(s, now) {
		t.Fatal("reconnect suppressed a new real match")
	}
}

func TestLoadingDeadlineSurvivesPhaseAndLateIDChanges(t *testing.T) {
	now := time.Now()
	var p Preview
	s := core.Snapshot{Connected: true, Phase: "GameStart", GameID: 42}
	p.Observe(s, now)
	s.Participants = []core.Participant{{Key: "synthetic"}}
	p.Observe(s, now.Add(30*time.Second))
	if !p.Eligible(s, now.Add(30*time.Second)) {
		t.Fatal("repeated loading snapshot reset eligibility")
	}
	s.Phase = "InProgress"
	p.Observe(s, now.Add(46*time.Second))
	if p.Eligible(s, now.Add(46*time.Second)) {
		t.Fatal("gameplay extended the loading entry deadline")
	}
	var late Preview
	s.Phase = "GameStart"
	s.GameID = 0
	late.Observe(s, now)
	s.GameID = 43
	late.Observe(s, now.Add(time.Minute))
	if late.Eligible(s, now.Add(time.Minute)) {
		t.Fatal("late game ID extended entry window")
	}
}

func TestInGameWindowPhaseIsIndependentOfDataPreparation(t *testing.T) {
	for _, phase := range []string{"Lobby", "ChampSelect", "GameStart", "InProgress", "Reconnect", "EndOfGame", "None"} {
		s := core.Snapshot{Connected: true, Phase: phase}
		want := phase == "GameStart" || phase == "InProgress"
		if InGame(s) != want {
			t.Fatal("wrong window phase", phase)
		}
		s.Connected = false
		if InGame(s) {
			t.Fatal("disconnected window allowed", phase)
		}
	}
}

func TestExplicitLoadingViewBeforeIDPreventsLaterAutomaticPopup(t *testing.T) {
	now := time.Now()
	s := core.Snapshot{Connected: true, Phase: "GameStart", Participants: []core.Participant{{Key: "synthetic"}}}
	var p Preview
	p.Observe(s, now)
	p.Consume()
	s.GameID = 42
	p.Observe(s, now.Add(time.Second))
	if p.Eligible(s, now.Add(time.Second)) {
		t.Fatal("late ID reopened preview after explicit loading action")
	}
	s.Phase = "Lobby"
	p.Observe(s, now.Add(time.Minute))
	s.Phase = "GameStart"
	s.GameID = 43
	p.Observe(s, now.Add(2*time.Minute))
	if !p.Eligible(s, now.Add(2*time.Minute)) {
		t.Fatal("explicit action suppressed the next real match")
	}
}
