// Package teambrief owns the opt-in, one-shot champ-select analysis message.
package teambrief

import (
	"context"
	"sync"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

type Frame struct {
	Scope  string // transient connection/account/selection identity
	Active bool
	Lines  []string
	Ready  bool
	Rated  int
}

type Controller struct {
	mu          sync.Mutex
	state       core.AnalysisBriefState
	frame       Frame
	source      source.AnalysisBriefSource
	scope       string
	sequence    uint64
	cancel      context.CancelFunc
	attempted   bool
	deferred    bool
	result      string
	rooms       map[string]uint64
	changes     chan struct{}
	wake        chan struct{}
	wait, delay time.Duration
	workers     sync.WaitGroup
}

func New() *Controller {
	return &Controller{state: core.AnalysisBriefState{Audience: "team", IncludeSelf: true, Status: "off"}, changes: make(chan struct{}, 1), wake: make(chan struct{}, 1), rooms: map[string]uint64{}, wait: 8 * time.Second, delay: 1500 * time.Millisecond}
}
func (c *Controller) Snapshot() core.AnalysisBriefState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}
func (c *Controller) Changes() <-chan struct{} { return c.changes }

func (c *Controller) statusLocked(status string) {
	if c.state.Status == status {
		return
	}
	c.state.Status, c.state.UpdatedAt = status, time.Now()
	select {
	case c.changes <- struct{}{}:
	default:
	}
}
func (c *Controller) stopLocked() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.sequence++
}
func (c *Controller) Stop() {
	c.mu.Lock()
	c.stopLocked()
	c.mu.Unlock()
	c.workers.Wait()
}

func (c *Controller) Configure(enabled bool, audience string, self bool) error {
	if audience != "team" && audience != "self" {
		return apperrors.New(apperrors.InvalidResponse)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.Enabled == enabled && c.state.Audience == audience && c.state.IncludeSelf == self {
		return nil
	}
	c.stopLocked()
	c.state.Enabled, c.state.Audience, c.state.IncludeSelf = enabled, audience, self
	status := "off"
	if enabled {
		status = "idle"
		if c.attempted {
			status = c.result
		}
	}
	c.statusLocked(status)
	return nil
}

// Observe never performs I/O or invokes an engine callback under either lock.
func (c *Controller) Observe(parent context.Context, src source.AnalysisBriefSource, frame Frame) core.AnalysisBriefState {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frame, c.source = frame, src
	select {
	case c.wake <- struct{}{}:
	default:
	}
	if !frame.Active || parent == nil || parent.Err() != nil || !c.state.Enabled {
		if c.cancel != nil {
			c.stopLocked()
		}
		if c.state.Enabled {
			c.statusLocked("idle")
		}
		return c.state
	}
	if c.scope != frame.Scope {
		c.stopLocked()
		c.scope, c.attempted, c.result, c.deferred = frame.Scope, false, "", false
	}
	if c.deferred && frame.Rated == 0 {
		c.statusLocked("waiting-data")
		return c.state
	}
	if c.attempted || c.cancel != nil {
		return c.state
	}
	if src == nil {
		c.statusLocked("unsupported")
		return c.state
	}
	ctx, cancel := context.WithCancel(parent)
	c.cancel = cancel
	c.sequence++
	token := c.sequence
	resume := c.deferred
	c.deferred = false
	c.statusLocked("waiting")
	c.workers.Add(1)
	go func() { defer c.workers.Done(); c.run(ctx, token, resume) }()
	return c.state
}

func (c *Controller) run(ctx context.Context, token uint64, resume bool) {
	delay, deadline := time.NewTimer(c.delay), time.NewTimer(c.wait)
	defer delay.Stop()
	defer deadline.Stop()
	due, expired := false, false
	for {
		select {
		case <-ctx.Done():
			return
		case <-delay.C:
			due = true
		case <-deadline.C:
			expired = true
		case <-c.wake:
		}
		c.mu.Lock()
		if c.sequence != token || !c.frame.Active || !c.state.Enabled {
			c.mu.Unlock()
			return
		}
		ready := due && (c.frame.Ready || expired || resume)
		c.mu.Unlock()
		if ready {
			c.send(ctx, token)
			return
		}
	}
}

func (c *Controller) send(parent context.Context, token uint64) {
	request, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	c.mu.Lock()
	if c.sequence != token || c.frame.Rated == 0 || len(c.frame.Lines) == 0 {
		if c.sequence == token {
			c.deferred = true
			if c.cancel != nil {
				c.cancel()
				c.cancel = nil
			}
			c.statusLocked("waiting-data")
		}
		c.mu.Unlock()
		return
	}
	src := c.source
	c.mu.Unlock()
	room, err := readConversation(request, src)
	if err != nil {
		c.finish(token, "unavailable", true)
		return
	}
	c.mu.Lock()
	if c.sequence != token || request.Err() != nil || !c.state.Enabled || !c.frame.Active {
		c.mu.Unlock()
		return
	}
	if _, used := c.rooms[room]; used {
		c.mu.Unlock()
		c.finish(token, "skipped", true)
		return
	}
	// Reserve before POST. Even an error or cancellation cannot authorize replay.
	c.rooms[room], c.attempted = token, true
	c.pruneRoomsLocked()
	body, audience := core.AnalysisBriefBody(c.frame.Lines), c.state.Audience
	c.statusLocked("sending")
	c.mu.Unlock()
	err = src.SendAnalysisBrief(request, room, body, audience)
	status := "sent"
	if err != nil {
		status = "uncertain"
	}
	c.finish(token, status, true)
}

func (c *Controller) finish(token uint64, status string, attempted bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sequence != token {
		return
	}
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.attempted, c.result = attempted, status
	c.statusLocked(status)
}

func (c *Controller) pruneRoomsLocked() {
	if len(c.rooms) <= 32 {
		return
	}
	oldest, stamp := "", ^uint64(0)
	for id, value := range c.rooms {
		if value < stamp {
			oldest, stamp = id, value
		}
	}
	delete(c.rooms, oldest)
}

// Only the read-only discovery may retry; the POST below it remains one-shot.
func readConversation(ctx context.Context, src source.AnalysisBriefSource) (string, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(250 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}
		room, err := src.ChampSelectConversation(ctx)
		if err == nil {
			return room, nil
		}
		last = err
	}
	return "", last
}
