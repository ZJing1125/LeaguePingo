// Package readycheck owns a local, opt-in action lane. It never reads player data.
package readycheck

import (
	"context"
	"errors"
	"sync"
	"time"

	"leaguepingo/internal/actionlane"
	"leaguepingo/internal/core"
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
	lane             actionlane.Executor
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
	c.lane.Stop()
	c.signal()
}
func (c *Controller) resetLocked() {
	c.lane.Reset()
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
	} else if c.lane.Blocked() {
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
		c.lane.Block()
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
	c.lane.Block()
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
	c.terminalResponse = ""
	c.resetLocked()
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
		client, phase := c.client, c.phase
		enabled := c.state.Enabled && client != nil && active(phase)
		var runCtx context.Context
		var session actionlane.Session
		if enabled {
			var started bool
			runCtx, session, started = c.lane.Start(ctx, 0)
			enabled = started
		}
		c.mu.Unlock()
		if enabled {
			c.runSession(runCtx, client, session)
			c.mu.Lock()
			c.lane.Finish(session)
			c.mu.Unlock()
		}
		select {
		case <-ctx.Done():
			return
		case <-c.wake:
		}
	}
}
func (c *Controller) current(session actionlane.Session) bool {
	return c.lane.Current(session) && c.state.Enabled
}
func (c *Controller) status(session actionlane.Session, status string, due time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.current(session) {
		return false
	}
	c.publishLocked(status, due)
	return true
}
func (c *Controller) runSession(ctx context.Context, client source.ReadyCheckSource, session actionlane.Session) {
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
	run := readyRun{controller: c, ctx: ctx, client: client, session: session, eventWake: eventWake, timer: timer}
	for run.wait() && run.step() {
	}
}

type readyRun struct {
	controller              *Controller
	ctx                     context.Context
	client                  source.ReadyCheckSource
	session                 actionlane.Session
	eventWake               <-chan struct{}
	timer                   *time.Timer
	due, started, nextProbe time.Time
	finished                bool
}

func (r *readyRun) wait() bool {
	select {
	case <-r.ctx.Done():
		return false
	case <-r.timer.C:
		return true
	case <-r.eventWake:
		return true
	}
}

func (r *readyRun) schedule(delay time.Duration) { r.timer.Reset(delay) }

func (r *readyRun) step() bool {
	if remaining := time.Until(r.nextProbe); remaining > 0 {
		r.schedule(remaining)
		return true
	}
	r.nextProbe = time.Now().Add(250 * time.Millisecond)
	if !r.finished && !r.started.IsZero() && time.Since(r.started) > 15*time.Second {
		r.controller.status(r.session, "failed", time.Time{})
		r.finished = true
	}
	valid, skipped, delay := r.controller.sessionState(r.session)
	if !valid {
		return false
	}
	phase, proceed, keep := r.readPhase()
	if !keep || !proceed {
		return keep
	}
	if phase != "ReadyCheck" {
		r.schedule(time.Second)
		return true
	}
	ready, proceed, keep := r.readReady()
	if !keep || !proceed {
		return keep
	}
	return r.handleReady(ready, skipped, delay)
}

func (c *Controller) sessionState(session actionlane.Session) (bool, bool, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current(session), c.lane.Blocked(), c.state.DelaySeconds
}

func (r *readyRun) readPhase() (string, bool, bool) {
	reqCtx, cancel := context.WithTimeout(r.ctx, 800*time.Millisecond)
	started := time.Now()
	phase, err := r.client.Phase(reqCtx)
	cancel()
	if r.ctx.Err() != nil {
		return "", false, false
	}
	if err != nil {
		r.schedule(time.Second)
		return "", false, true
	}
	if !r.controller.syncPhase(r.session, r.client, phase, started) {
		return "", false, false
	}
	return phase, true, true
}

func (c *Controller) syncPhase(session actionlane.Session, client source.ReadyCheckSource, phase string, observedAt time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.current(session) {
		return false
	}
	if phase != c.phase {
		c.observeLocked(client, c.version, phase, observedAt)
		return false
	}
	if observedAt.After(c.observedAt) {
		c.observedAt = observedAt
	}
	return true
}

func (r *readyRun) readReady() (core.ReadyCheck, bool, bool) {
	reqCtx, cancel := context.WithTimeout(r.ctx, 800*time.Millisecond)
	ready, err := r.client.ReadyCheck(reqCtx)
	cancel()
	if r.ctx.Err() != nil {
		return core.ReadyCheck{}, false, false
	}
	if err != nil {
		r.schedule(500 * time.Millisecond)
		return core.ReadyCheck{}, false, true
	}
	return ready, r.controller.recordResponse(r.session, ready), true
}

func (c *Controller) recordResponse(session actionlane.Session, ready core.ReadyCheck) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.current(session) {
		return false
	}
	if ready.Pending() && c.terminalResponse != "" {
		c.terminalResponse = ""
		c.resetLocked()
		c.idleLocked()
		return false
	}
	if ready.PlayerResponse == "Accepted" || ready.PlayerResponse == "Declined" {
		c.terminalResponse = ready.PlayerResponse
	}
	return true
}

func (r *readyRun) handleReady(ready core.ReadyCheck, skipped bool, delay int) bool {
	if ready.PlayerResponse == "Accepted" || ready.PlayerResponse == "Declined" {
		status := "accepted"
		if ready.PlayerResponse == "Declined" {
			status = "manual"
		}
		if !r.finished {
			r.controller.status(r.session, status, time.Time{})
		}
		r.finished = true
		r.schedule(time.Second)
		return true
	}
	if skipped || r.finished {
		r.schedule(time.Second)
		return true
	}
	if !ready.Pending() {
		r.schedule(500 * time.Millisecond)
		return true
	}
	if r.started.IsZero() {
		r.started = time.Now()
		r.due = r.started.Add(time.Duration(delay) * time.Second)
		r.controller.status(r.session, "countdown", r.due)
	}
	if time.Now().Before(r.due) {
		r.schedule(min(250*time.Millisecond, time.Until(r.due)))
		return true
	}
	return r.dispatch()
}

func (r *readyRun) dispatch() bool {
	attempt, ok := r.controller.beginAccept(r.session, r.started)
	if !ok {
		return false
	}
	reqCtx, cancel := context.WithTimeout(r.ctx, 800*time.Millisecond)
	_ = r.client.AcceptReadyCheck(reqCtx)
	cancel()
	if r.ctx.Err() != nil {
		return false
	}
	reqCtx, cancel = context.WithTimeout(r.ctx, 800*time.Millisecond)
	confirmed, err := r.client.ReadyCheck(reqCtx)
	cancel()
	if r.ctx.Err() != nil {
		return false
	}
	r.controller.recordConfirmed(r.session, confirmed, err)
	switch {
	case err == nil && confirmed.PlayerResponse == "Accepted":
		r.controller.status(r.session, "accepted", time.Time{})
		r.finished = true
	case err == nil && confirmed.PlayerResponse == "Declined":
		r.controller.status(r.session, "manual", time.Time{})
		r.finished = true
	case err == nil && confirmed.Pending() && attempt < 3:
	default:
		r.controller.status(r.session, "failed", time.Time{})
		r.finished = true
	}
	r.schedule(500 * time.Millisecond)
	return true
}

func (c *Controller) beginAccept(session actionlane.Session, started time.Time) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	attempt, permitted := c.lane.Permit(session, started, 15*time.Second, 3)
	if !c.current(session) || !permitted {
		return attempt, false
	}
	c.publishLocked("accepting", time.Time{})
	return attempt, true
}

func (c *Controller) recordConfirmed(session actionlane.Session, ready core.ReadyCheck, err error) {
	if err != nil || (ready.PlayerResponse != "Accepted" && ready.PlayerResponse != "Declined") {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current(session) {
		c.terminalResponse = ready.PlayerResponse
	}
}
