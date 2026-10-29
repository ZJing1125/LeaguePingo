package postgame

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"
)

type sequenceFake struct {
	*honorFake
	sequence                               string
	sequenceReads, continues, earlyReturns int
	continueMode                           string
	sequenceGate, sequenceEntered          chan struct{}
}

func sequenceFixture(queue int) *sequenceFake {
	f := &sequenceFake{honorFake: honorFixture(), sequence: "other"}
	f.lobby.QueueID = queue
	return f
}
func (f *sequenceFake) PostGameSequence(context.Context) (string, error) {
	if f.sequenceGate != nil {
		select {
		case f.sequenceEntered <- struct{}{}:
		default:
		}
		<-f.sequenceGate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sequenceReads++
	return f.sequence, nil
}
func (f *sequenceFake) CompleteHonorBallot(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.submits++
	f.completion = 42
	f.sequence = "missions-celebration"
	// The Continue page remains in PreEndOfGame, with the old ballot cached.
	return errors.New("synthetic lost honor response")
}
func (f *sequenceFake) ContinuePostGameProgress(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.continues++
	if f.continueMode != "unknown" {
		f.sequence = ""
		f.phase = "EndOfGame"
	}
	return errors.New("synthetic lost continue response")
}
func (f *sequenceFake) PlayAgain(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.returns++
	if f.phase != "EndOfGame" {
		f.earlyReturns++
		return errors.New("still on Continue page")
	}
	f.phase = "Lobby"
	return nil
}
func (f *sequenceFake) chainCounts() (int, int, int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.submits, f.continues, f.returns, f.searches, f.earlyReturns
}
func TestHonorContinuePlayAgainAndQueueAcrossModes(t *testing.T) {
	for _, queue := range []int{420, 440, 450, 2400} {
		t.Run(strconv.Itoa(queue), func(t *testing.T) {
			f := sequenceFixture(queue)
			c := prepare(t, f.fake, true, true)
			c.ConfigureSkipHonor(true)
			o := observed("PreEndOfGame")
			o.QueueID = queue
			c.Observe(f, o)
			waitStatus(t, c, "queued")
			h, n, r, q, early := f.chainCounts()
			if h != 1 || n != 1 || r != 1 || q != 1 || early != 0 {
				t.Fatal("broken honor/continue/return/search chain", h, n, r, q, early)
			}
		})
	}
}
func TestSkipOnlyDoesNotCompleteProgressOrReturn(t *testing.T) {
	f := sequenceFixture(2400)
	c := prepare(t, f.fake, false, false)
	c.ConfigureSkipHonor(true)
	o := observed("PreEndOfGame")
	o.QueueID = 2400
	c.Observe(f, o)
	waitStatus(t, c, "honor-skipped")
	h, n, r, q, _ := f.chainCounts()
	if h != 1 || n+r+q != 0 {
		t.Fatal("skip-only advanced the other stages")
	}
}
func TestUnknownContinueIsNotRepeatedOrBypassed(t *testing.T) {
	f := sequenceFixture(2400)
	f.continueMode = "unknown"
	c := prepare(t, f.fake, true, true)
	c.ConfigureSkipHonor(true)
	o := observed("PreEndOfGame")
	o.QueueID = 2400
	c.Observe(f, o)
	waitStatus(t, c, "continue-unconfirmed")
	h, n, r, q, _ := f.chainCounts()
	if h != 1 || n != 1 || r+q != 0 {
		t.Fatal("unknown continue repeated or bypassed", h, n, r, q)
	}
}
func TestContinueHonorsAccountAndCancellation(t *testing.T) {
	for _, mode := range []string{"cancel", "disable", "account", "disconnect"} {
		t.Run(mode, func(t *testing.T) {
			f := sequenceFixture(2400)
			f.sequence = "missions-celebration"
			c := prepare(t, f.fake, true, true)
			c.ConfigureSkipHonor(true)
			// Pause a source read so old work can finish after user/connection changes.
			f.sequenceGate = make(chan struct{})
			f.sequenceEntered = make(chan struct{}, 1)
			o := observed("PreEndOfGame")
			o.QueueID = 2400
			c.Observe(f, o)
			select {
			case <-f.sequenceEntered:
			case <-time.After(time.Second):
				t.Fatal("sequence read not entered")
			}
			switch mode {
			case "cancel":
				_, _ = c.Cancel()
			case "disable":
				_ = c.Configure(false, false, 5, false)
			case "account":
				f.mu.Lock()
				f.account = "other"
				f.mu.Unlock()
			case "disconnect":
				c.Observe(nil, Observation{})
			}
			close(f.sequenceGate)
			time.Sleep(40 * time.Millisecond)
			h, n, r, q, _ := f.chainCounts()
			if h+n+r+q != 0 {
				t.Fatal("cancelled/foreign progress caused actions", h, n, r, q)
			}
		})
	}
}
func TestUnknownProgressIsNeverCompleted(t *testing.T) {
	f := sequenceFixture(2400)
	f.sequence = "other"
	f.selected = true
	c := prepare(t, f.fake, true, true)
	c.ConfigureSkipHonor(true)
	o := observed("PreEndOfGame")
	o.QueueID = 2400
	c.Observe(f, o)
	waitStatus(t, c, "honor-unavailable")
	h, n, r, q, _ := f.chainCounts()
	if h+n+r+q != 0 {
		t.Fatal("completed unknown selection/ceremony")
	}
	f.mu.Lock()
	reads := f.sequenceReads
	f.mu.Unlock()
	if reads > maxSequenceReads {
		t.Fatal("unbounded sequence probes")
	}
}

func TestManualHonorStillContinuesWithReturnOption(t *testing.T) {
	f := sequenceFixture(2400)
	f.sequence = "missions-celebration"
	c := prepare(t, f.fake, true, true)
	o := observed("PreEndOfGame")
	o.QueueID = 2400
	c.Observe(f, o)
	waitStatus(t, c, "queued")
	h, n, r, q, early := f.chainCounts()
	if h != 0 || n != 1 || r != 1 || q != 1 || early != 0 {
		t.Fatal("manual-honor return is coupled to skip option", h, n, r, q, early)
	}
}

func TestProgressContinueCanStartFromEndOfGame(t *testing.T) {
	f := sequenceFixture(2400)
	f.phase = "EndOfGame"
	c := prepare(t, f.fake, true, true)
	c.ConfigureSkipHonor(true)
	o := observed("EndOfGame")
	o.QueueID = 2400
	c.Observe(f, o)
	waitStatus(t, c, "queued")
	h, n, r, q, early := f.chainCounts()
	if h != 1 || n != 1 || r != 1 || q != 1 || early != 0 {
		t.Fatal("EndOfGame progress presentation blocked chain", h, n, r, q, early)
	}
}

func TestWrongCompletedGameCannotReleaseUncertainHonor(t *testing.T) {
	f := honorFixture()
	f.phase = "EndOfGame"
	f.mode = "unknown"
	f.completion = 41
	c := prepare(t, f.fake, true, true)
	c.ConfigureSkipHonor(true)
	c.Observe(f, observed("EndOfGame"))
	waitStatus(t, c, "honor-unconfirmed")
	if r, q := counts(f.fake); r+q != 0 {
		t.Fatal("previous game's completion released this game")
	}
}

func TestUnrelatedEndOfGameSequenceKeepsExistingReturnBehavior(t *testing.T) {
	f := sequenceFixture(2400)
	f.phase = "EndOfGame"
	f.sequence = "other"
	c := prepare(t, f.fake, true, true)
	o := observed("EndOfGame")
	o.QueueID = 2400
	c.Observe(f, o)
	waitStatus(t, c, "queued")
	h, n, r, q, early := f.chainCounts()
	if h+n != 0 || r != 1 || q != 1 || early != 0 {
		t.Fatal("unrelated sequence disabled the existing EndOfGame return")
	}
}

func TestLateEmptySequenceCannotReturnForChangedAccount(t *testing.T) {
	f := sequenceFixture(2400)
	f.phase = "EndOfGame"
	f.sequence = ""
	f.sequenceGate = make(chan struct{})
	f.sequenceEntered = make(chan struct{}, 1)
	c := prepare(t, f.fake, true, true)
	o := observed("EndOfGame")
	o.QueueID = 2400
	c.Observe(f, o)
	select {
	case <-f.sequenceEntered:
	case <-time.After(time.Second):
		t.Fatal("sequence read did not start")
	}
	f.mu.Lock()
	f.account = "other"
	f.mu.Unlock()
	close(f.sequenceGate)
	waitStatus(t, c, "cancelled")
	_, n, r, q, _ := f.chainCounts()
	if n+r+q != 0 {
		t.Fatal("late sequence metadata returned or queued another account")
	}
}
