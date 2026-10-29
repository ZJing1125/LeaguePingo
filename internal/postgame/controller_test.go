package postgame

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

type fake struct {
	mu                       sync.Mutex
	phase, account           string
	lobby                    core.LobbyControl
	returns, searches, reads int
	returnMode, searchMode   string
	gate, entered            chan struct{}
}

func (f *fake) Phase(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	return f.phase, nil
}
func (f *fake) AccountIdentity(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.account, nil
}
func (f *fake) LobbyControl(context.Context) (core.LobbyControl, error) {
	if f.gate != nil {
		select {
		case f.entered <- struct{}{}:
		default:
		}
		<-f.gate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lobby, nil
}
func (f *fake) PlayAgain(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.returns++
	if f.returnMode != "unknown" {
		f.phase = "Lobby"
	}
	return errors.New("synthetic timeout")
}
func (f *fake) StartSearch(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.searches++
	if f.searchMode != "unknown" {
		f.phase = "Matchmaking"
	}
	return errors.New("synthetic timeout")
}
func fixture() *fake {
	return &fake{phase: "EndOfGame", account: "self", lobby: core.LobbyControl{QueueID: 420, PartyKey: "party", MembersKey: "members", Leader: true, Ready: true}}
}
func observed(phase string) Observation {
	return Observation{Version: 1, Account: "self", Phase: phase, GameID: 42, QueueID: 420, ReviewID: 42, ReviewState: "ready"}
}
func prepare(t *testing.T, f *fake, ret, queue bool) *Controller {
	t.Helper()
	c := New()
	_ = c.Configure(ret, queue, 5)
	c.timing = timings{probe: 5 * time.Millisecond, request: 100 * time.Millisecond, total: time.Second, settlement: 30 * time.Millisecond, honor: 15 * time.Millisecond, returnDelay: 10 * time.Millisecond, lobbyWait: 80 * time.Millisecond, verify: 30 * time.Millisecond}
	// Action tests do not wait five wall-clock seconds. The supported preference
	// range/defaults and cancellable real countdown are covered separately.
	c.state.DelaySeconds = 0
	initial := observed("InProgress")
	initial.QueueID = f.lobby.QueueID
	c.Observe(f, initial)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("worker did not stop")
		}
	})
	return c
}
func waitStatus(t *testing.T, c *Controller, status string) {
	t.Helper()
	until := time.Now().Add(2 * time.Second)
	for time.Now().Before(until) {
		if c.Snapshot().Status == status {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("wanted %s, got %+v", status, c.Snapshot())
}
func counts(f *fake) (int, int) { f.mu.Lock(); defer f.mu.Unlock(); return f.returns, f.searches }
func TestReturnQueueAndUncertainHTTPAreVerified(t *testing.T) {
	f := fixture()
	c := prepare(t, f, true, true)
	c.Observe(f, observed("EndOfGame"))
	waitStatus(t, c, "queued")
	for range 20 {
		c.Observe(f, observed("Lobby"))
	}
	time.Sleep(30 * time.Millisecond)
	r, q := counts(f)
	if r != 1 || q != 1 {
		t.Fatalf("mutations repeated: %d %d", r, q)
	}
	// A manually cancelled search and duplicate lobby updates must not requeue.
	f.mu.Lock()
	f.phase = "Lobby"
	f.mu.Unlock()
	c.Hint(1, "Lobby")
	c.Observe(f, observed("Lobby"))
	time.Sleep(20 * time.Millisecond)
	_, q = counts(f)
	if q != 1 {
		t.Fatal("manual queue cancellation was overridden")
	}
}
func TestPlainLobbyDodgeAndMissingFrozenReviewNeverAct(t *testing.T) {
	f := fixture()
	c := prepare(t, f, true, true)
	o := observed("EndOfGame")
	o.ReviewID = 0
	c.Observe(f, o)
	time.Sleep(25 * time.Millisecond)
	if r, q := counts(f); r+q != 0 {
		t.Fatal("acted before review was frozen")
	}
	c.Observe(f, observed("None"))
	c.Observe(f, observed("Lobby"))
	time.Sleep(25 * time.Millisecond)
	if r, q := counts(f); r+q != 0 {
		t.Fatal("ordinary lobby was auto queued")
	}
}
func TestReturnOnlyWorksForNonLeader(t *testing.T) {
	f := fixture()
	f.lobby.Leader = false
	c := prepare(t, f, true, false)
	c.Observe(f, observed("EndOfGame"))
	waitStatus(t, c, "returned")
	if r, q := counts(f); r != 1 || q != 0 {
		t.Fatal("return-only performed search")
	}
}
func TestTeamMemberNeverStartsSearch(t *testing.T) {
	f := fixture()
	f.lobby.Leader = false
	c := prepare(t, f, true, true)
	c.Observe(f, observed("EndOfGame"))
	waitStatus(t, c, "not-leader")
	if r, q := counts(f); r != 1 || q != 0 {
		t.Fatal("non-leader started matchmaking")
	}
}
func TestQueueOnlyWaitsForManualReturnAndTeamReadiness(t *testing.T) {
	f := fixture()
	f.lobby.Ready = false
	c := prepare(t, f, false, true)
	c.Observe(f, observed("EndOfGame"))
	waitStatus(t, c, "waiting-return")
	f.mu.Lock()
	f.phase = "Lobby"
	f.mu.Unlock()
	c.Observe(f, observed("Lobby"))
	waitStatus(t, c, "waiting-ready")
	f.mu.Lock()
	f.lobby.Ready = true
	f.mu.Unlock()
	waitStatus(t, c, "queued")
	if r, q := counts(f); r != 0 || q != 1 {
		t.Fatal("queue-only did not respect manual return")
	}
}
func TestWaitingStatsDoesNotReturnAndSettlementGetsGrace(t *testing.T) {
	f := fixture()
	f.phase = "WaitingForStats"
	c := prepare(t, f, true, false)
	o := observed("WaitingForStats")
	o.ReviewState = "waiting"
	c.Observe(f, o)
	waitStatus(t, c, "waiting-result")
	time.Sleep(35 * time.Millisecond)
	if r, _ := counts(f); r != 0 {
		t.Fatal("left while client was waiting for stats")
	}
	f.mu.Lock()
	f.phase = "EndOfGame"
	f.mu.Unlock()
	o = observed("EndOfGame")
	o.ReviewState = "waiting"
	c.Observe(f, o)
	waitStatus(t, c, "returned")
}
func TestCancelDisableModePartyAndMemberChanges(t *testing.T) {
	for _, kind := range []string{"cancel", "disable", "mode", "party", "members", "disconnect"} {
		t.Run(kind, func(t *testing.T) {
			f := fixture()
			f.phase = "Lobby"
			c := prepare(t, f, true, true)
			c.state.DelaySeconds = 5
			c.Observe(f, observed("Lobby"))
			waitStatus(t, c, "queue-countdown")
			switch kind {
			case "cancel":
				_, _ = c.Cancel()
			case "disable":
				_ = c.Configure(false, false, 10)
			case "disconnect":
				c.Observe(nil, Observation{})
			default:
				f.mu.Lock()
				switch kind {
				case "mode":
					f.lobby.QueueID = 440
				case "party":
					f.lobby.PartyKey = "new"
				case "members":
					f.lobby.MembersKey = "new"
				}
				f.mu.Unlock()
			}
			if kind == "mode" || kind == "party" || kind == "members" {
				waitStatus(t, c, "changed")
			}
			time.Sleep(25 * time.Millisecond)
			if _, q := counts(f); q != 0 {
				t.Fatal("cancelled/changed lobby was queued")
			}
		})
	}
}
func TestAccountMismatchStopsBeforeReturn(t *testing.T) {
	f := fixture()
	f.account = "other"
	c := prepare(t, f, true, true)
	c.Observe(f, observed("EndOfGame"))
	waitStatus(t, c, "cancelled")
	if r, q := counts(f); r+q != 0 {
		t.Fatal("old account task mutated client")
	}
}
func TestUnknownReturnOrSearchIsNotResubmitted(t *testing.T) {
	for _, kind := range []string{"return", "search"} {
		t.Run(kind, func(t *testing.T) {
			f := fixture()
			if kind == "return" {
				f.returnMode = "unknown"
			} else {
				f.searchMode = "unknown"
			}
			c := prepare(t, f, true, true)
			c.Observe(f, observed("EndOfGame"))
			waitStatus(t, c, "failed")
			time.Sleep(20 * time.Millisecond)
			r, q := counts(f)
			if r != 1 || q > 1 || (kind == "return" && q != 0) {
				t.Fatal("unknown mutation retried")
			}
		})
	}
}
func TestReadinessTimeoutAndLateOldConnection(t *testing.T) {
	f := fixture()
	f.phase = "Lobby"
	f.lobby.Ready = false
	c := prepare(t, f, false, true)
	c.Observe(f, observed("Lobby"))
	waitStatus(t, c, "timeout")
	if _, q := counts(f); q != 0 {
		t.Fatal("unready lobby was queued")
	}
	f2 := fixture()
	f2.phase = "Lobby"
	f2.gate = make(chan struct{})
	f2.entered = make(chan struct{}, 1)
	c2 := prepare(t, f2, false, true)
	c2.Observe(f2, observed("Lobby"))
	select {
	case <-f2.entered:
	case <-time.After(time.Second):
		t.Fatal("read did not start")
	}
	c2.Observe(nil, Observation{})
	close(f2.gate)
	time.Sleep(20 * time.Millisecond)
	if _, q := counts(f2); q != 0 {
		t.Fatal("late response acted on a new connection")
	}
}

func TestDisabledMakesNoRequestsAndNewGameRearms(t *testing.T) {
	f := fixture()
	c := prepare(t, f, false, false)
	c.Observe(f, observed("EndOfGame"))
	time.Sleep(20 * time.Millisecond)
	f.mu.Lock()
	reads := f.reads
	f.mu.Unlock()
	if reads != 0 {
		t.Fatal("disabled task read the client")
	}
	_ = c.Configure(true, true, 5)
	c.mu.Lock()
	c.state.DelaySeconds = 0
	c.mu.Unlock()
	c.Observe(f, observed("EndOfGame"))
	waitStatus(t, c, "queued")
	f.mu.Lock()
	f.phase = "InProgress"
	f.mu.Unlock()
	next := observed("InProgress")
	next.GameID = 43
	next.ReviewID = 42
	c.Observe(f, next)
	f.mu.Lock()
	f.phase = "EndOfGame"
	f.mu.Unlock()
	next.Phase = "EndOfGame"
	next.ReviewID = 43
	c.Observe(f, next)
	waitStatus(t, c, "queued")
	if r, q := counts(f); r != 2 || q != 2 {
		t.Fatal("new game did not get its own one-shot task")
	}
}
