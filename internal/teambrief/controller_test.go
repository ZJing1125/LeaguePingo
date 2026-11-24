package teambrief

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"leaguepingo/internal/apperrors"
)

type fakeChat struct {
	mu               sync.Mutex
	room             string
	rooms            []string
	reads            int
	preflightBlocked bool
	posts            int
	bodies           []string
	fail             bool
	entered          chan struct{}
	release          chan struct{}
}

func (f *fakeChat) ChampSelectConversation(context.Context) (string, error) {
	if f.entered != nil {
		select {
		case f.entered <- struct{}{}:
		default:
		}
		<-f.release
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	if len(f.rooms) == 0 || f.reads > len(f.rooms) {
		return f.room, nil
	}
	if room := f.rooms[f.reads-1]; room != "" {
		return room, nil
	}
	return "", apperrors.New(apperrors.DataUnavailable)
}
func (f *fakeChat) SendAnalysisBrief(_ context.Context, _ string, body string, _ string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.preflightBlocked {
		return false, apperrors.New(apperrors.DataUnavailable)
	}
	f.posts++
	f.bodies = append(f.bodies, body)
	if f.fail {
		return true, errors.New("synthetic uncertain send")
	}
	return true, nil
}
func (f *fakeChat) count() int { f.mu.Lock(); defer f.mu.Unlock(); return f.posts }
func testController(t *testing.T) *Controller {
	t.Helper()
	c := New()
	c.wait = 15 * time.Millisecond
	c.chatPoll = 2 * time.Millisecond
	t.Cleanup(c.Stop)
	return c
}
func awaitStatus(t *testing.T, c *Controller, status string) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for {
		if c.Snapshot().Status == status {
			return
		}
		select {
		case <-c.Changes():
		case <-deadline.C:
			t.Fatalf("wanted %s, got %s", status, c.Snapshot().Status)
		}
	}
}

func TestDefaultOffAndEachNewSelectionOnlyOnce(t *testing.T) {
	c := testController(t)
	f := &fakeChat{room: "room-a"}
	frame := Frame{Scope: "connection/account/select-a", Active: true, Ready: true, Rated: 5, Lines: []string{"synthetic analysis"}}
	c.Observe(context.Background(), f, frame)
	if f.count() != 0 || c.Snapshot().Enabled {
		t.Fatal("default sent a message")
	}
	if err := c.Configure(true, "team", true); err != nil {
		t.Fatal(err)
	}
	c.Observe(context.Background(), f, frame)
	awaitStatus(t, c, "sent")
	for i := 0; i < 10; i++ {
		c.Observe(context.Background(), f, frame)
	}
	if f.count() != 1 {
		t.Fatal("updates resent")
	}
	// Another selection can arrive with no observable inactive phase.
	f.mu.Lock()
	f.room = "room-b"
	f.mu.Unlock()
	frame.Scope = "connection/account/select-b"
	c.Observe(context.Background(), f, frame)
	awaitStatus(t, c, "sent")
	if f.count() != 2 {
		t.Fatal("instant next selection did not send")
	}
}

func TestNoReplayAfterUncertainPOSTOrReenable(t *testing.T) {
	c := testController(t)
	f := &fakeChat{room: "room-a", fail: true}
	_ = c.Configure(true, "team", true)
	frame := Frame{Scope: "same", Active: true, Ready: true, Rated: 2, Lines: []string{"synthetic"}}
	c.Observe(context.Background(), f, frame)
	awaitStatus(t, c, "uncertain")
	_ = c.Configure(false, "team", true)
	_ = c.Configure(true, "self", false)
	for i := 0; i < 10; i++ {
		c.Observe(context.Background(), f, frame)
	}
	if f.count() != 1 {
		t.Fatal("uncertain POST replayed")
	}
}

func TestLateRoomCannotSendAfterLeavingSelection(t *testing.T) {
	c := testController(t)
	f := &fakeChat{room: "room-a", entered: make(chan struct{}, 1), release: make(chan struct{})}
	_ = c.Configure(true, "team", true)
	t.Cleanup(func() {
		select {
		case <-f.release:
		default:
			close(f.release)
		}
	})
	frame := Frame{Scope: "same", Active: true, Ready: true, Rated: 2, Lines: []string{"synthetic"}}
	c.Observe(context.Background(), f, frame)
	select {
	case <-f.entered:
	case <-time.After(time.Second):
		t.Fatal("discovery not started")
	}
	frame.Active = false
	c.Observe(context.Background(), f, frame)
	close(f.release)
	c.Stop()
	awaitStatus(t, c, "idle")
	if f.count() != 0 {
		t.Fatal("late room sent in loading/game")
	}
}

func TestDeadlineWithoutRatingsSkipsAndNeverResumes(t *testing.T) {
	c := testController(t)
	f := &fakeChat{room: "room-a"}
	_ = c.Configure(true, "team", true)
	frame := Frame{Scope: "same", Active: true, Ready: false, Rated: 0, Lines: []string{"资料待取"}}
	c.Observe(context.Background(), f, frame)
	awaitStatus(t, c, "no-data")
	frame.Rated, frame.Ready, frame.Lines = 5, true, []string{"late"}
	c.Observe(context.Background(), f, frame)
	if f.count() != 0 || c.Snapshot().Status != "no-data" {
		t.Fatal("expired empty selection replayed")
	}
}

func TestTransientChatRecoveryWaitsForSameReadyRoomThenSendsOnce(t *testing.T) {
	c := testController(t)
	f := &fakeChat{room: "new", rooms: []string{"old", "", "", "new", "", "new", "new"}}
	_ = c.Configure(true, "team", true)
	frame := Frame{Scope: "same", Active: true, Ready: true, Rated: 5, Lines: []string{"one short brief"}}
	c.Observe(context.Background(), f, frame)
	awaitStatus(t, c, "sent")
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.posts != 1 || f.reads != 7 {
		t.Fatalf("waited for %d observations and sent %d times", f.reads, f.posts)
	}
}

func TestChatDroppedDuringFinalCheckIsReportedAsUnavailable(t *testing.T) {
	c := testController(t)
	f := &fakeChat{room: "stable", preflightBlocked: true}
	_ = c.Configure(true, "team", true)
	frame := Frame{Scope: "same", Active: true, Ready: true, Rated: 1, Lines: []string{"one short brief"}}
	c.Observe(context.Background(), f, frame)
	awaitStatus(t, c, "unavailable")
	if f.count() != 0 {
		t.Fatal("a preflight failure was treated as a POST")
	}
}

func TestDeadlineFreezesPartialBodyBeforeChatReady(t *testing.T) {
	c := testController(t)
	f := &fakeChat{room: "room-a", entered: make(chan struct{}, 1), release: make(chan struct{})}
	t.Cleanup(func() {
		select {
		case <-f.release:
		default:
			close(f.release)
		}
	})
	_ = c.Configure(true, "team", true)
	frame := Frame{Scope: "partial", Active: true, Ready: false, Rated: 3, Lines: []string{"frozen-three"}}
	started := time.Now()
	c.Observe(context.Background(), f, frame)
	awaitStatus(t, c, "queued")
	frame.Rated, frame.Lines = 5, []string{"late-five"}
	c.Observe(context.Background(), f, frame)
	close(f.release)
	awaitStatus(t, c, "sent")
	f.mu.Lock()
	body := f.bodies[0]
	f.mu.Unlock()
	if body != "frozen-three" {
		t.Fatal("frozen body changed", body)
	}
	if c.Snapshot().ReadySeats != 3 || c.Snapshot().FreezeMS < 10 || time.Since(started) > time.Second {
		t.Fatal("partial body was replaced or frozen late")
	}
}

func TestMutePersistsForCurrentSelectionAndResetsForNext(t *testing.T) {
	c := testController(t)
	f := &fakeChat{room: "room-a"}
	_ = c.Configure(true, "team", true)
	frame := Frame{Scope: "muted", Active: true, Ready: false, Lines: []string{"pending"}}
	c.Observe(context.Background(), f, frame)
	c.MuteCurrent()
	frame.Ready, frame.Rated = true, 5
	c.Observe(context.Background(), f, frame)
	if c.Snapshot().Status != "muted" || f.count() != 0 {
		t.Fatal("muted selection sent")
	}
	frame.Scope = "next"
	c.Observe(context.Background(), f, frame)
	awaitStatus(t, c, "sent")
}

func TestFiveSecondCoverage(t *testing.T) {
	if testing.Short() {
		t.Skip("real five-second deadline")
	}
	for _, rated := range []int{5, 3} {
		c := New()
		f := &fakeChat{room: "coverage"}
		c.chatPoll = time.Millisecond
		_ = c.Configure(true, "team", true)
		frame := Frame{Scope: "coverage", Active: true, Ready: rated == 5, Rated: rated, Lines: []string{"1", "2", "3", "4", "5"}}
		if rated == 5 {
			frame.Cached = 5
		}
		start := time.Now()
		c.Observe(context.Background(), f, frame)
		deadline := time.NewTimer(6 * time.Second)
		for c.Snapshot().Status != "sent" {
			select {
			case <-c.Changes():
			case <-deadline.C:
				t.Fatal("five-second dispatch missed")
			}
		}
		deadline.Stop()
		state := c.Snapshot()
		c.Stop()
		t.Logf("synthetic cached=%d coverage=%d/%d freeze_ms=%d queue_to_post_ms=%d post_ms=%d elapsed_ms=%d", state.CachedSeats, state.ReadySeats, state.TotalSeats, state.FreezeMS, state.QueuedToPostMS, state.PostMS, time.Since(start).Milliseconds())
		if state.ReadySeats != rated || state.TotalSeats != 5 || state.FreezeMS > 5200 {
			t.Fatal("invalid coverage/deadline")
		}
	}
}
