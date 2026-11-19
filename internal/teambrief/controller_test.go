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
func (f *fakeChat) SendAnalysisBrief(context.Context, string, string, string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.preflightBlocked {
		return false, apperrors.New(apperrors.DataUnavailable)
	}
	f.posts++
	if f.fail {
		return true, errors.New("synthetic uncertain send")
	}
	return true, nil
}
func (f *fakeChat) count() int { f.mu.Lock(); defer f.mu.Unlock(); return f.posts }
func testController(t *testing.T) *Controller {
	t.Helper()
	c := New()
	c.delay = time.Millisecond
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

func TestLateDataResumesWithoutFetchingAndNoBlankMessage(t *testing.T) {
	c := testController(t)
	f := &fakeChat{room: "room-a"}
	_ = c.Configure(true, "team", true)
	frame := Frame{Scope: "same", Active: true, Ready: false, Rated: 0}
	c.Observe(context.Background(), f, frame)
	awaitStatus(t, c, "waiting-data")
	if f.count() != 0 {
		t.Fatal("blank analysis sent")
	}
	frame.Rated, frame.Lines = 1, []string{"one rated player, rest pending"}
	c.Observe(context.Background(), f, frame)
	if f.count() != 0 || c.Snapshot().Status != "waiting-data" {
		t.Fatal("partial analysis used the send opportunity")
	}
	frame.Ready = true
	c.Observe(context.Background(), f, frame)
	awaitStatus(t, c, "sent")
	if f.count() != 1 {
		t.Fatal("late data did not resume once")
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
