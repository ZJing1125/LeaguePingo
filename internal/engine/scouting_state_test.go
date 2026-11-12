package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

func scoutProgressNilForTest(s *scoutPreparation) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.progress == nil
}

func TestScoutStateRoundBudgetIdentityCapAndResume(t *testing.T) {
	var s scoutPreparation
	now := time.Now()
	players := append(fullTestRoster(), core.Participant{PUUID: "extra", IdentityState: core.IdentityKnown})
	if s.queueFor(440) != 440 || s.scopeFor("party", 440, players) != 440 {
		t.Fatal("ranked mode not selected")
	}
	targets, changed := s.targets(players, now)
	if !changed || len(targets) != 10 || players[10].Scout != nil {
		t.Fatal("round exceeded its ten-identity cap")
	}
	ctx, token := s.begin(context.Background(), now)
	defer s.finish("party", token)
	deadline, _ := ctx.Deadline()
	if !deadline.Equal(now.Add(scoutRoundBudget)) {
		t.Fatal("round changed its 180-second deadline")
	}
	s.recordPages(17)
	s.recordPages(16) // A delayed worker cannot lower the shared page budget.
	if next, _ := s.targets(players, now); len(next) != 0 {
		t.Fatal("running round started another batch")
	}
	s.selectQueue(420)
	if s.queueFor(440) != 420 || s.pageCount() != 17 {
		t.Fatal("manual mode change reset the round")
	}
	published := players[0].Scout
	s.stop(players)
	if ctx.Err() != context.Canceled || s.hasTask() || players[0].Scout.State != "partial" || published.State != "loading" {
		t.Fatal("stop did not cancel and retain partial reports")
	}
	s.scopeFor("preparing", 440, players)
	if s.queueFor(440) != 420 || s.pageCount() != 0 {
		t.Fatal("new stage lost manual reference or did not reset task scope")
	}
	_, next := s.begin(context.Background(), now)
	defer s.finish("preparing", next)
	s.finish("party", token)
	if !s.current("preparing", next) || !s.hasTask() {
		t.Fatal("late completion cancelled the next round")
	}
	s.finish("preparing", next)
	s.recordPages(scoutPageLimit)
	if targets, _ := s.targets(players, now); len(targets) != 0 {
		t.Fatal("exhausted page budget admitted more targets")
	}
}

func TestScoutStateProgressCopiesAndFreshness(t *testing.T) {
	var s scoutPreparation
	now := time.Now()
	progress, _ := progressFixture(now, 1)
	c := scoutCursor{player: core.Participant{PUUID: "p"}, matches: progress.matches, seen: progress.seen, offset: progress.offset, oldest: progress.oldest}
	s.rememberProgress(&c, now)
	c.matches[0].GameID = 900
	delete(c.seen, 1)
	resumed := s.resume(c.player, now.Add(time.Minute))
	if resumed.offset != 20 || resumed.matches[0].GameID != 1 || !resumed.seen[1] {
		t.Fatal("saved cursor shared the worker's mutable slices or map")
	}
	resumed.matches[0].GameID = 800
	delete(resumed.seen, 1)
	if again := s.resume(c.player, now); again.matches[0].GameID != 1 || !again.seen[1] {
		t.Fatal("resumed cursor mutated the stored cursor")
	}
	if expired := s.resume(c.player, now.Add(5*time.Minute)); expired.offset != 0 {
		t.Fatal("expired cursor was resumed")
	}
	s.clearProgress()
	if !scoutProgressNilForTest(&s) || s.resume(c.player, now).offset != 0 {
		t.Fatal("finished result retained its pre-result cursor")
	}
}

func TestLateScoutProgressCannotResurrectClearedOrNewScopeCursor(t *testing.T) {
	for _, reset := range []string{"phase", "connection", "result"} {
		t.Run(reset, func(t *testing.T) {
			e := New(nil, nil)
			e.client, e.clientVersion = &fakeClient{}, 1
			e.snapshot = core.Snapshot{Connected: true, Stage: string(StageParty)}
			e.scouting.scopeFor("old", 420, nil)
			_, token := e.scouting.begin(context.Background(), time.Now())
			defer e.scouting.stop(nil)
			progress, _ := progressFixture(time.Now(), 1)
			cursor := scoutCursor{player: core.Participant{PUUID: "p"}, matches: progress.matches, seen: progress.seen, offset: progress.offset}
			e.rememberScoutProgress(1, "old", token, &cursor)
			if e.scouting.resume(cursor.player, time.Now()).offset != 20 {
				t.Fatal("valid cursor not saved")
			}
			e.mu.Lock()
			switch reset {
			case "phase":
				e.phaseHint = PhaseChampSelect
				e.scouting.scopeFor("new", 420, nil)
				next := cursor
				next.offset = 40
				e.scouting.rememberProgress(&next, time.Now())
			case "connection":
				e.invalidateConnectionLocked()
			case "result":
				e.stopScoutingLocked()
				e.scouting.clearProgress()
			}
			e.mu.Unlock()
			e.rememberScoutProgress(1, "old", token, &cursor)
			expect := 0
			if reset == "phase" {
				expect = 40
			}
			if got := e.scouting.resume(cursor.player, time.Now()).offset; got != expect {
				t.Fatalf("late cursor crossed %s reset: offset=%d want=%d", reset, got, expect)
			}
		})
	}
}

func TestScoutStateConcurrentReadersAndResets(t *testing.T) {
	var s scoutPreparation
	now := time.Now()
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				s.current("scope", 1)
				s.pageCount()
				s.hasTask()
				s.resume(core.Participant{PUUID: "p"}, now)
			}
		}()
	}
	for i := 0; i < 100; i++ {
		scope := fmt.Sprint(i)
		s.queueFor(420)
		s.scopeFor(scope, 420, nil)
		_, token := s.begin(context.Background(), now)
		s.recordPages(i)
		s.finish(scope, token)
		s.refresh(nil)
		s.clearProgress()
		s.invalidate()
	}
	wg.Wait()
}

func TestScoutReferenceSurvivesStagesButResetsForNextSelection(t *testing.T) {
	var s scoutPreparation
	s.queueFor(2400)
	s.selectQueue(420)
	s.observeGame(77, PhaseChampSelect)
	s.scopeFor("preparing", 2400, nil)
	s.observeGame(77, PhaseGameStart)
	s.scopeFor("in-game", 2400, nil)
	if s.queueFor(2400) != 420 {
		t.Fatal("phase transition erased reference")
	}
	s.observeGame(0, PhaseChampSelect)
	if s.queueFor(2400) != 2400 {
		t.Fatal("new selection kept previous game's reference")
	}
	s.selectQueue(440)
	if s.queueFor(450) != 450 {
		t.Fatal("real mode change did not restore auto")
	}
	s.selectQueue(0)
	if s.queueFor(99999) != 0 {
		t.Fatal("unknown queue became ranked")
	}
}
