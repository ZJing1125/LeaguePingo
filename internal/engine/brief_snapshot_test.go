package engine

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type snapshotBriefClient struct{ *deadlineBriefClient }

func (*snapshotBriefClient) CurrentQueue() int { return 420 }

func briefingReport(games int, score float64) core.ScoutReport {
	state := "sample"
	grade := ""
	if games >= 5 {
		state, grade = "ready", core.HorseTier(score)
	}
	return core.ScoutReport{ModelVersion: core.ScoutStatsVersion, State: "loading", Read: games, FetchedAt: time.Now(), Queues: []core.ScoutQueue{{QueueID: 420, FetchStatus: core.StatusPending, Analysis: &core.PlayerAnalysis{Version: 4, State: state, Grade: grade, Score: &score, ScoredGames: games}}}}
}

func briefingEngine(t *testing.T, fifthGames int) (*Engine, *deadlineBriefClient, time.Time) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	e := New(nil, nil)
	f := &deadlineBriefClient{fakeClient: &fakeClient{phase: "ChampSelect", gameID: 42}, sent: make(chan string, 1)}
	e.client = &snapshotBriefClient{f}
	e.clientVersion = 1
	e.accountIdentity = "p0"
	e.initialMatch.setParent(ctx)
	e.snapshot = core.Snapshot{Connected: true, ConnectionID: 1, SelfKey: "p0", Phase: "ChampSelect", Stage: "preparing", GameID: 42, QueueID: 420, MapID: 11, GameMode: "CLASSIC"}
	for i := range 5 {
		cell := i
		games, score := 15, 60.0
		switch i {
		case 0:
			score = 90
		case 1:
			score = 40
		case 4:
			games = fifthGames
		}
		r := briefingReport(games, score)
		id := fmt.Sprint("p", i)
		e.snapshot.Participants = append(e.snapshot.Participants, core.Participant{ConnectionID: 1, Key: id, PUUID: id, Team: "ally", CellID: &cell, IdentityState: core.IdentityKnown, Scout: &r})
	}
	e.scouting.scope, e.scouting.token = "audit", 1
	e.scouting.cache = map[string]*core.ScoutReport{}
	start := time.Now()
	e.readiness.Phase("ChampSelect", start)
	_ = e.brief.Configure(true, "team", true)
	e.observeAnalysisBriefLocked()
	t.Cleanup(func() { cancel(); e.brief.Stop() })
	return e, f, start
}

func TestBriefUsesReportEvenWhenWailsPublicationIsThrottled(t *testing.T) {
	e, f, start := briefingEngine(t, 3)
	// Move the existing deadline close to expiry without changing its duration.
	e.mu.Lock()
	e.readiness.Phase("", start)
	e.readiness.Phase("ChampSelect", start.Add(-4700*time.Millisecond))
	e.observeAnalysisBriefLocked()
	e.scouting.lastPublish = time.Now()
	e.mu.Unlock()
	if !e.saveScout(1, "audit", 1, "p4", briefingReport(5, 60)) {
		t.Fatal("report rejected")
	}
	select {
	case body := <-f.sent:
		if !strings.Contains(body, "5楼(待定) 评分待齐（60.0分/5场）") {
			t.Fatal("freeze missed committed page", body)
		}
	case <-time.After(time.Second):
		t.Fatal("freeze waited for Wails publication")
	}
	if e.AnalysisBriefStatus().ReadySeats != 5 {
		t.Fatal("incorrect coverage")
	}
}

func TestBriefFourthSecondCellSwapAndFifthSeatAt5100ms(t *testing.T) {
	if testing.Short() {
		t.Skip("real five-second integration")
	}
	e, f, start := briefingEngine(t, 0)
	<-time.NewTimer(time.Until(start.Add(4 * time.Second))).C
	f.roster = e.Snapshot().Participants
	zero, one := 0, 1
	f.roster[0].CellID, f.roster[1].CellID = &one, &zero
	e.refreshChampSelect(context.Background())
	if got := e.Snapshot().Participants[0].CellID; got == nil || *got != 1 {
		t.Fatal("cell-only change lost")
	}
	var body string
	select {
	case body = <-f.sent:
	case <-time.After(2 * time.Second):
		t.Fatal("five-second dispatch missing")
	}
	if !strings.Contains(body, "1楼(待定) 评分待齐（40.0分/15场）") || !strings.Contains(body, "2楼(待定) 评分待齐（90.0分/15场）") || !strings.Contains(body, "5楼(待定) 资料待取") {
		t.Fatal("frozen seats mismatched", body)
	}
	frozen := e.AnalysisBriefStatus()
	if frozen.ReadySeats != 4 || frozen.TotalSeats != 5 || frozen.FreezeMS < 4900 || frozen.FreezeMS > 5200 {
		t.Fatal("invalid deadline", frozen)
	}
	<-time.NewTimer(max(0, time.Until(start.Add(5100*time.Millisecond)))).C
	if !e.saveScout(1, "audit", 1, "p4", briefingReport(15, 60)) {
		t.Fatal("late UI report rejected")
	}
	_, _, rated := core.HorseBriefLines(e.Snapshot(), true)
	if rated != 5 || e.AnalysisBriefStatus().ReadySeats != 4 || f.posts.Load() != 1 {
		t.Fatal("late result replaced or resent frozen brief")
	}
	t.Logf("synthetic swap_ms=4000 late_ms=5100 freeze_ms=%d frozen=4/5 memory=5/5", frozen.FreezeMS)
}

func TestFirstSelectionEventStartsTimerBeforePollConfirmation(t *testing.T) {
	if testing.Short() {
		t.Skip("real five-second event deadline")
	}
	e := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer e.brief.Stop()
	e.client = &deadlineBriefClient{fakeClient: &fakeClient{}}
	e.clientVersion = 1
	e.initialMatch.setParent(ctx)
	e.snapshot = core.Snapshot{Connected: true, ConnectionID: 1, SelfKey: "self", Phase: "Lobby", Stage: "party"}
	_ = e.brief.Configure(true, "team", true)
	_, stop, sequence, ok := e.phaseEvents.begin(ctx, 1, time.Now())
	if !ok {
		t.Fatal("event stream unavailable")
	}
	defer stop()
	e.observePhase(1, sequence, "ChampSelect")
	deadline := time.NewTimer(5500 * time.Millisecond)
	defer deadline.Stop()
	for e.AnalysisBriefStatus().Status != "no-data" {
		select {
		case <-e.brief.Changes():
		case <-deadline.C:
			t.Fatal("event waited for poll confirmation")
		}
	}
	if e.AnalysisBriefStatus().TotalSeats != 5 {
		t.Fatal("lost physical team denominator")
	}
}

func TestRepeatedSelectionEventKeepsLatestSnapshot(t *testing.T) {
	e, f, start := briefingEngine(t, 3)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, stop, sequence, ok := e.phaseEvents.begin(ctx, 1, time.Now())
	if !ok {
		t.Fatal("event stream unavailable")
	}
	defer stop()
	e.mu.Lock()
	e.readiness.Phase("", start)
	e.readiness.Phase("ChampSelect", start.Add(-4700*time.Millisecond))
	e.observeAnalysisBriefLocked()
	e.mu.Unlock()
	e.observePhase(1, sequence, "ChampSelect")
	select {
	case body := <-f.sent:
		if !strings.Contains(body, "5楼(待定) 样本少（3场）") {
			t.Fatal("repeat event erased latest frame", body)
		}
	case <-time.After(time.Second):
		t.Fatal("repeat event consumed or postponed the deadline")
	}
}
