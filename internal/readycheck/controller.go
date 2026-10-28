// Package readycheck owns a local, opt-in action lane. It never reads player data.
package readycheck

import (
	"context"
	"errors"
	"sync"
	"time"

	"leaguepingo/internal/source"
)

type State struct {
	Revision     uint64    `json:"revision"`
	Enabled      bool      `json:"enabled"`
	DelaySeconds int       `json:"delaySeconds"`
	Status       string    `json:"status"`
	DueAt        time.Time `json:"dueAt"`
}

type Controller struct {
	mu               sync.Mutex
	state            State
	client           source.ReadyCheckSource
	version          uint64
	phase            string
	observedAt       time.Time
	epoch            uint64
	skipped          bool
	cancel           context.CancelFunc
	wake             chan struct{}
	terminalResponse string
	changes          chan State
}

func New() *Controller {
	return &Controller{state: State{Status: "off", DelaySeconds: 1}, wake: make(chan struct{}, 1), changes: make(chan State, 1)}
}
func ValidDelay(seconds int) bool {
	return seconds == 0 || seconds == 1 || seconds == 3 || seconds == 5
}
func (c *Controller) Snapshot() State       { c.mu.Lock(); defer c.mu.Unlock(); return c.state }
func (c *Controller) Changes() <-chan State { return c.changes }
func (c *Controller) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}
func (c *Controller) stopLocked() {
	c.epoch++
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.signal()
}
func (c *Controller) publishLocked(status string, due time.Time) {
	c.state.Status, c.state.DueAt = status, due
	c.state.Revision++
	select {
	case <-c.changes:
	default:
	}
	c.changes <- c.state
}
func (c *Controller) idleLocked() {
	status := "armed"
	if !c.state.Enabled {
		status = "off"
	} else if c.version == 0 {
		status = "waiting"
	} else if c.client == nil {
		status = "unsupported"
	} else if c.skipped {
		status = "skipped"
	}
	c.publishLocked(status, time.Time{})
}
func (c *Controller) Configure(enabled bool, seconds int) error {
	if !ValidDelay(seconds) {
		return errors.New("接受延迟仅支持 0、1、3、5 秒")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.Enabled == enabled && c.state.DelaySeconds == seconds {
		return nil
	}
	// Disabling during an invitation also disarms it for a later re-enable.
	if !enabled && c.phase == "ReadyCheck" {
		c.skipped = true
	}
	c.state.Enabled, c.state.DelaySeconds = enabled, seconds
	c.stopLocked()
	c.idleLocked()
	return nil
}
func (c *Controller) Skip() (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.Status != "countdown" {
		return c.state, errors.New("当前没有可跳过的自动接受任务")
	}
	c.skipped = true
	c.stopLocked()
	c.idleLocked()
	return c.state, nil
}

// Observe is called for verified connection changes and phase hints. Hints only
// wake/cancel work: the worker verifies phase and response before every POST.
func (c *Controller) Observe(client source.ReadyCheckSource, version uint64, phase string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.observeLocked(client, version, phase, time.Now())
}

// ObserveAt rejects an older HTTP phase response that arrives after a newer
// phase event/probe, so it cannot clear a skip or renew the same invitation.
func (c *Controller) ObserveAt(client source.ReadyCheckSource, version uint64, phase string, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.observeLocked(client, version, phase, at)
}
func (c *Controller) observeLocked(client source.ReadyCheckSource, version uint64, phase string, at time.Time) {
	if c.version == version && at.Before(c.observedAt) {
		return
	}
	c.observedAt = at
	if c.version == version && c.phase == phase {
		return
	}
	c.client, c.version, c.phase = client, version, phase
	c.skipped = false
	c.terminalResponse = ""
	c.stopLocked()
	c.idleLocked()
}
func active(phase string) bool { return phase == "Matchmaking" || phase == "ReadyCheck" }

func (c *Controller) Run(ctx context.Context) {
	defer func() { c.mu.Lock(); c.stopLocked(); c.publishLocked("off", time.Time{}); c.mu.Unlock() }()
	for {
		if ctx.Err() != nil {
			return
		}
		c.mu.Lock()
		client, epoch, phase := c.client, c.epoch, c.phase
		enabled := c.state.Enabled && client != nil && active(phase)
		var runCtx context.Context
		if enabled {
			var cancel context.CancelFunc
			runCtx, cancel = context.WithCancel(ctx)
			c.cancel = cancel
		}
		c.mu.Unlock()
		if enabled {
			c.runSession(runCtx, client, epoch)
			c.mu.Lock()
			if c.epoch == epoch && c.cancel != nil {
				c.cancel()
				c.cancel = nil
			}
			c.mu.Unlock()
		}
		select {
		case <-ctx.Done():
			return
		case <-c.wake:
		}
	}
}
func (c *Controller) current(epoch uint64) bool { return c.epoch == epoch && c.state.Enabled }
func (c *Controller) status(epoch uint64, status string, due time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.current(epoch) {
		return false
	}
	c.publishLocked(status, due)
	return true
}
func (c *Controller) runSession(ctx context.Context, client source.ReadyCheckSource, epoch uint64) {
	eventWake := make(chan struct{}, 1)
	notify := func() {
		select {
		case eventWake <- struct{}{}:
		default:
		}
	}
	// At most one scoped stream in this session. Failure leaves polling active;
	// it is not repeatedly reopened while the same phase remains unchanged.
	watchCtx, stopWatch := context.WithCancel(ctx)
	defer stopWatch()
	if stream, ok := client.(source.ReadyCheckEventSource); ok {
		go func() { _ = stream.WatchReadyCheck(watchCtx, notify) }()
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	var due, started, nextProbe time.Time
	attempts := 0
	finished := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		case <-eventWake:
		}
		if remaining := time.Until(nextProbe); remaining > 0 {
			timer.Reset(remaining)
			continue
		}
		nextProbe = time.Now().Add(250 * time.Millisecond)
		if !finished && !started.IsZero() && time.Since(started) > 15*time.Second {
			c.status(epoch, "failed", time.Time{})
			finished = true
		}
		c.mu.Lock()
		valid := c.current(epoch)
		skipped, delay := c.skipped, c.state.DelaySeconds
		c.mu.Unlock()
		if !valid {
			return
		}
		reqCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
		phaseStarted := time.Now()
		phase, err := client.Phase(reqCtx)
		cancel()
		if ctx.Err() != nil {
			return
		}
		if err != nil { // Never act on a stale phase. Queue fallback continues.
			timer.Reset(time.Second)
			continue
		}
		c.mu.Lock()
		if !c.current(epoch) {
			c.mu.Unlock()
			return
		}
		if phase != c.phase {
			previous := c.epoch
			c.observeLocked(client, c.version, phase, phaseStarted)
			changed := previous != c.epoch
			c.mu.Unlock()
			if changed {
				return
			}
			timer.Reset(250 * time.Millisecond)
			continue
		}
		if phaseStarted.After(c.observedAt) {
			c.observedAt = phaseStarted
		}
		c.mu.Unlock()
		if phase != "ReadyCheck" {
			timer.Reset(time.Second)
			continue
		}
		reqCtx, cancel = context.WithTimeout(ctx, 800*time.Millisecond)
		ready, err := client.ReadyCheck(reqCtx)
		cancel()
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			timer.Reset(500 * time.Millisecond)
			continue
		}
		c.mu.Lock()
		if !c.current(epoch) {
			c.mu.Unlock()
			return
		}
		// Some clients can enter another pop between phase polls. A verified
		// terminal response followed by a pending response is a new invitation.
		if ready.Pending() && c.terminalResponse != "" {
			c.terminalResponse = ""
			c.skipped = false
			c.stopLocked()
			c.idleLocked()
			c.mu.Unlock()
			return
		}
		if ready.PlayerResponse == "Accepted" || ready.PlayerResponse == "Declined" {
			c.terminalResponse = ready.PlayerResponse
		}
		c.mu.Unlock()
		if ready.PlayerResponse == "Accepted" {
			if !finished {
				c.status(epoch, "accepted", time.Time{})
			}
			finished = true
			timer.Reset(time.Second)
			continue
		}
		if ready.PlayerResponse == "Declined" {
			if !finished {
				c.status(epoch, "manual", time.Time{})
			}
			finished = true
			timer.Reset(time.Second)
			continue
		}
		if skipped || finished {
			timer.Reset(time.Second)
			continue
		}
		if !ready.Pending() {
			timer.Reset(500 * time.Millisecond)
			continue
		}
		if started.IsZero() {
			started = time.Now()
			due = started.Add(time.Duration(delay) * time.Second)
			c.status(epoch, "countdown", due)
		}
		if time.Now().Before(due) {
			timer.Reset(min(250*time.Millisecond, time.Until(due)))
			continue
		}
		// Linearize skip/disable against dispatch; only one POST can be in flight.
		c.mu.Lock()
		if !c.current(epoch) || c.skipped {
			c.mu.Unlock()
			return
		}
		c.publishLocked("accepting", time.Time{})
		c.mu.Unlock()
		reqCtx, cancel = context.WithTimeout(ctx, 800*time.Millisecond)
		_ = client.AcceptReadyCheck(reqCtx)
		cancel()
		attempts++
		if ctx.Err() != nil {
			return
		}
		// A successful HTTP response alone is not a confirmed acceptance. On timeout
		// this GET is also the only way to decide whether another POST is permitted.
		reqCtx, cancel = context.WithTimeout(ctx, 800*time.Millisecond)
		confirmed, verifyErr := client.ReadyCheck(reqCtx)
		cancel()
		if ctx.Err() != nil {
			return
		}
		if verifyErr == nil && (confirmed.PlayerResponse == "Accepted" || confirmed.PlayerResponse == "Declined") {
			c.mu.Lock()
			if c.current(epoch) {
				c.terminalResponse = confirmed.PlayerResponse
			}
			c.mu.Unlock()
		}
		switch {
		case verifyErr == nil && confirmed.PlayerResponse == "Accepted":
			c.status(epoch, "accepted", time.Time{})
			finished = true
		case verifyErr == nil && confirmed.PlayerResponse == "Declined":
			c.status(epoch, "manual", time.Time{})
			finished = true
		case verifyErr == nil && confirmed.Pending() && attempts < 3:
			// The next attempt rechecks phase and pending response after bounded backoff.
		default:
			c.status(epoch, "failed", time.Time{})
			finished = true
		}
		timer.Reset(500 * time.Millisecond)
	}
}
