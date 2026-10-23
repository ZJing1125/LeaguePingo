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
		t.Fatal("loading triggered preview")
	}
	s.Phase = "InProgress"
	p.Observe(s, now.Add(time.Minute))
	if p.Eligible(s, now) {
		t.Fatal("empty roster triggered preview")
	}
	s.Participants = []core.Participant{{Key: "synthetic"}}
	if !p.Eligible(s, now.Add(time.Minute+time.Second)) {
		t.Fatal("first visible roster not eligible")
	}
	p.Consume()
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

func TestReconnectWithoutIDCannotCreateLatePreview(t *testing.T) {
	now := time.Now()
	var p Preview
	s := core.Snapshot{Connected: true, Phase: "Reconnect"}
	p.Observe(s, now)
	s.GameID = 42
	s.Phase = "InProgress"
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

func TestLateIDDoesNotExtendEntryAndLongLoadingDoesNotConsumeIt(t *testing.T) {
	now := time.Now()
	var p Preview
	s := core.Snapshot{Connected: true, Phase: "GameStart", GameID: 42}
	p.Observe(s, now)
	s.Phase = "InProgress"
	s.GameID = 0
	p.Observe(s, now.Add(time.Minute))
	s.GameID = 42
	s.Participants = []core.Participant{{Key: "synthetic"}}
	p.Observe(s, now.Add(time.Minute+time.Second))
	if !p.Eligible(s, now.Add(time.Minute+time.Second)) {
		t.Fatal("long load consumed actual entry time")
	}
	var late Preview
	s.GameID = 0
	late.Observe(s, now)
	s.GameID = 43
	late.Observe(s, now.Add(time.Minute))
	if late.Eligible(s, now.Add(time.Minute)) {
		t.Fatal("late game ID extended entry window")
	}
}
