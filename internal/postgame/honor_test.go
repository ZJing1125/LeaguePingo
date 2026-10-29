package postgame

import (
	"context"
	"errors"
	"testing"
	"time"
)

type honorFake struct {
	*fake
	ballot                    int64
	selected, selectAtAccount bool
	honorReads, submits       int
	mode                      string
	ballotGate, ballotEntered chan struct{}
}

func (f *honorFake) HonorBallot(context.Context) (int64, bool, error) {
	if f.ballotGate != nil {
		select {
		case f.ballotEntered <- struct{}{}:
		default:
		}
		<-f.ballotGate // Deliberately ignore cancellation to exercise late replies.
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.honorReads++
	if f.mode == "unavailable" {
		return 0, false, errors.New("synthetic unsupported")
	}
	return f.ballot, f.selected, nil
}
func (f *honorFake) AccountIdentity(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.selectAtAccount {
		f.selected = true
	}
	return f.account, nil
}
func (f *honorFake) CompleteHonorBallot(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.submits++
	if f.mode != "unknown" {
		f.ballot = 0
		if f.mode != "stuck-phase" {
			f.phase = "EndOfGame"
		}
	}
	return errors.New("synthetic response lost")
}
func (f *honorFake) honorCounts() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.honorReads, f.submits
}
func honorFixture() *honorFake {
	f := &honorFake{fake: fixture(), ballot: 42}
	f.phase = "PreEndOfGame"
	return f
}

func TestSkipHonorIsIndependentAndDefaultOff(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		f := honorFixture()
		c := prepare(t, f.fake, false, false)
		c.ConfigureSkipHonor(enabled)
		c.Observe(f, observed("PreEndOfGame"))
		if enabled {
			waitStatus(t, c, "honor-skipped")
		} else {
			time.Sleep(25 * time.Millisecond)
		}
		reads, submits := f.honorCounts()
		if (!enabled && reads+submits != 0) || (enabled && submits != 1) {
			t.Fatal("skip opt-in/one-shot violated", reads, submits)
		}
		if r, q := counts(f.fake); r+q != 0 {
			t.Fatal("skip-only returned or queued")
		}
		c.Observe(f, observed("EndOfGame"))
		time.Sleep(20 * time.Millisecond)
		_, after := f.honorCounts()
		if after != submits {
			t.Fatal("duplicate ceremony submission")
		}
	}
}

func TestSkipHonorMustFinishBeforeReturnAndQueue(t *testing.T) {
	f := honorFixture()
	c := prepare(t, f.fake, true, true)
	c.ConfigureSkipHonor(true)
	c.Observe(f, observed("PreEndOfGame"))
	waitStatus(t, c, "queued")
	_, n := f.honorCounts()
	r, q := counts(f.fake)
	if n != 1 || r != 1 || q != 1 {
		t.Fatal("incomplete chain", n, r, q)
	}
}

func TestUncertainHonorNeverRepeatedOrBypassed(t *testing.T) {
	for _, mode := range []string{"unknown", "stuck-phase", "unavailable"} {
		t.Run(mode, func(t *testing.T) {
			f := honorFixture()
			f.mode = mode
			c := prepare(t, f.fake, true, true)
			c.ConfigureSkipHonor(true)
			c.Observe(f, observed("PreEndOfGame"))
			status := "honor-unconfirmed"
			if mode == "unavailable" {
				status = "honor-unavailable"
			}
			waitStatus(t, c, status)
			reads, n := f.honorCounts()
			expected := 1
			if mode == "unavailable" {
				expected = 0
			}
			if n != expected || reads > maxHonorReads {
				t.Fatal("unbounded honor attempts", reads, n)
			}
			if r, q := counts(f.fake); r+q != 0 {
				t.Fatal("unknown honor bypassed to return/search")
			}
		})
	}
}

func TestHonorUserSelectionPreservedIncludingFinalRecheck(t *testing.T) {
	for _, late := range []bool{false, true} {
		f := honorFixture()
		f.selected = !late
		f.selectAtAccount = late
		c := prepare(t, f.fake, true, false)
		c.ConfigureSkipHonor(true)
		c.Observe(f, observed("PreEndOfGame"))
		waitStatus(t, c, "waiting-honor")
		if _, n := f.honorCounts(); n != 0 {
			t.Fatal("submitted user's selections")
		}
		f.mu.Lock()
		f.ballot = 0
		f.phase = "EndOfGame"
		f.mu.Unlock()
		waitStatus(t, c, "returned")
		if _, n := f.honorCounts(); n != 0 {
			t.Fatal("manual completion was submitted again")
		}
	}
}

func TestHonorScopeAndCancellation(t *testing.T) {
	for _, mode := range []string{"mismatch", "account", "cancel", "disable", "disconnect", "new-game"} {
		t.Run(mode, func(t *testing.T) {
			f := honorFixture()
			c := prepare(t, f.fake, true, true)
			c.ConfigureSkipHonor(true)
			if mode == "mismatch" {
				f.ballot = 43
			}
			if mode == "account" {
				f.account = "other"
			}
			c.Observe(f, observed("PreEndOfGame"))
			switch mode {
			case "mismatch":
				waitStatus(t, c, "honor-mismatch")
			case "account":
				waitStatus(t, c, "cancelled")
			default:
				waitStatus(t, c, "honor-countdown")
				switch mode {
				case "cancel":
					_, _ = c.Cancel()
				case "disable":
					c.ConfigureSkipHonor(false)
				case "disconnect":
					c.Observe(nil, Observation{})
				case "new-game":
					o := observed("InProgress")
					o.GameID = 43
					c.Observe(f, o)
				}
			}
			time.Sleep(25 * time.Millisecond)
			if _, n := f.honorCounts(); n != 0 {
				t.Fatal("cancelled/foreign ceremony submitted")
			}
			if r, q := counts(f.fake); r+q != 0 {
				t.Fatal("cancelled ceremony continued")
			}
		})
	}
}

func TestLateHonorReadCannotActForReplacedConnection(t *testing.T) {
	f := honorFixture()
	f.ballotGate = make(chan struct{})
	f.ballotEntered = make(chan struct{}, 1)
	c := prepare(t, f.fake, true, true)
	c.ConfigureSkipHonor(true)
	c.Observe(f, observed("PreEndOfGame"))
	select {
	case <-f.ballotEntered:
	case <-time.After(time.Second):
		t.Fatal("ballot read did not start")
	}
	c.Observe(nil, Observation{})
	close(f.ballotGate)
	time.Sleep(25 * time.Millisecond)
	if _, n := f.honorCounts(); n != 0 {
		t.Fatal("late honor read caused mutation")
	}
}

func TestHonorManualWaitHasBoundedReads(t *testing.T) {
	f := honorFixture()
	f.selected = true
	c := prepare(t, f.fake, true, true)
	c.ConfigureSkipHonor(true)
	c.Observe(f, observed("PreEndOfGame"))
	waitStatus(t, c, "honor-unavailable")
	reads, n := f.honorCounts()
	if reads != maxHonorReads || n != 0 {
		t.Fatal("unbounded manual ceremony reads", reads, n)
	}
}

func TestSkipHonorNoBallotAndStatsWait(t *testing.T) {
	f := honorFixture()
	f.ballot = 0
	f.phase = "WaitingForStats"
	c := prepare(t, f.fake, false, false)
	c.ConfigureSkipHonor(true)
	c.Observe(f, observed("WaitingForStats"))
	waitStatus(t, c, "waiting-result")
	time.Sleep(20 * time.Millisecond)
	if reads, n := f.honorCounts(); reads+n != 0 {
		t.Fatal("honor read while client waits for stats")
	}
	f.mu.Lock()
	f.phase = "EndOfGame"
	f.mu.Unlock()
	waitStatus(t, c, "honor-not-needed")
	if _, n := f.honorCounts(); n != 0 {
		t.Fatal("submitted a nonexistent ceremony")
	}
	if r, q := counts(f.fake); r+q != 0 {
		t.Fatal("no-ballot skip-only returned/searched")
	}
}

func TestSkipHonorNewGameAndAtomicSettings(t *testing.T) {
	f := honorFixture()
	c := prepare(t, f.fake, false, false)
	if err := c.Configure(false, false, 5, true); err != nil {
		t.Fatal(err)
	}
	c.Observe(f, observed("PreEndOfGame"))
	waitStatus(t, c, "honor-skipped")
	f.mu.Lock()
	f.phase = "InProgress"
	f.ballot = 43
	f.mu.Unlock()
	o := observed("InProgress")
	o.GameID = 43
	c.Observe(f, o)
	f.mu.Lock()
	f.phase = "PreEndOfGame"
	f.mu.Unlock()
	o.Phase = "PreEndOfGame"
	o.ReviewID = 43
	c.Observe(f, o)
	waitStatus(t, c, "honor-skipped")
	if _, n := f.honorCounts(); n != 2 {
		t.Fatal("new game did not receive its own honor action")
	}
}
