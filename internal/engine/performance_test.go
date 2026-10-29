package engine

import (
	"context"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

func TestSelectionTimingStartsBeforeRosterAndCompletesOnlyOnData(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	f := &fakeClient{phase: "ChampSelect", gameID: 42, roster: []core.Participant{{Team: "ally", PUUID: "synthetic-self", Key: "synthetic-self", IdentityState: core.IdentityKnown}}, rosterFn: func() { close(entered); <-release }}
	e := New(func() (Client, error) { return f, nil }, nil)
	done := make(chan struct{})
	go func() { e.poll(context.Background()); close(done) }()
	<-entered
	if m := e.ReadinessMetrics(); m.Started != 1 || !m.Pending || m.Completed != 0 {
		t.Fatal("timer waited for roster", m)
	}
	close(release)
	<-done
	if e.ReadinessMetrics().Completed != 0 {
		t.Fatal("names alone counted as analysis result")
	}
	e.mu.Lock()
	now := time.Now()
	p := &e.snapshot.Participants[0]
	p.HistoryState = core.HistoryReady
	p.HistoryMeta.FetchedAt = &now
	e.publishLocked()
	e.publishLocked()
	e.mu.Unlock()
	if m := e.ReadinessMetrics(); m.Completed != 1 || m.LastMS == nil {
		t.Fatal(m)
	}
	e.mu.Lock()
	e.readiness.Phase("Lobby", time.Now())
	e.readiness.Phase("ChampSelect", time.Now())
	e.disconnectLocked("synthetic disconnect")
	e.mu.Unlock()
	if m := e.ReadinessMetrics(); m.Abandoned != 1 || m.Pending || m.Completed != 1 {
		t.Fatal("disconnect crossed measurement", m)
	}
}
