// Package teambrief owns the opt-in, one-shot champ-select analysis message.
package teambrief

import (
	"context"
	"sync"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
	"leaguepingo/internal/source"
)

type Frame struct {
	StartedAt    time.Time
	Cached       int
	Scope        string // transient connection/account/selection identity
	SelectionKey string
	GameID       int64
	QueueID      int
	Active       bool
	Lines        []string
	Ready        bool
	Rated        int
}

type Controller struct {
	mu        sync.Mutex
	state     core.AnalysisBriefState
	frame     Frame
	source    source.AnalysisBriefSource
	scope     string
	sequence  uint64
	cancel    context.CancelFunc
	attempted bool
	result    string
	rooms     map[string]uint64
	changes   chan struct{}
	wake      chan struct{}
	wait      time.Duration
	started   time.Time
	frozen    *Frame
	queued    time.Time
	chatPoll  time.Duration
	workers   sync.WaitGroup
}

func New() *Controller {
	return &Controller{state: core.AnalysisBriefState{Audience: "team", IncludeSelf: true, Status: "off"}, changes: make(chan struct{}, 1), wake: make(chan struct{}, 1), rooms: map[string]uint64{}, wait: 5 * time.Second, chatPoll: 750 * time.Millisecond}
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
	if c.attempted && c.result == "queued" {
		c.result = "skipped"
	}
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
	if frame.Active && c.scope != frame.Scope {
		c.stopLocked()
		c.scope, c.attempted, c.result = frame.Scope, false, ""
		c.started = frame.StartedAt
		if c.started.IsZero() {
			c.started = time.Now()
		}
		c.frozen = nil
		c.state.CachedSeats = 0
		c.state.ReadySeats, c.state.TotalSeats, c.state.FreezeMS, c.state.QueuedToPostMS, c.state.PostMS = 0, 0, 0, 0, 0
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
	c.statusLocked("waiting")
	c.workers.Add(1)
	go func() { defer c.workers.Done(); c.run(ctx, token) }()
	return c.state
}

func (c *Controller) run(ctx context.Context, token uint64) {
	c.mu.Lock()
	remaining := time.Until(c.started.Add(c.wait))
	src := c.source
	c.mu.Unlock()
	deadline := time.NewTimer(max(0, remaining))
	defer deadline.Stop()
	// Chat discovery runs concurrently with history. It never freezes roster data.
	chat := make(chan conversationResult, 1)
	c.workers.Add(1)
	go func() {
		defer c.workers.Done()
		discovery, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		room, err := readConversation(discovery, src, c.chatPoll)
		chat <- conversationResult{room, err}
	}()
	for {
		expired := false
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			expired = true
		case <-c.wake:
		}
		c.mu.Lock()
		if c.sequence != token || !c.frame.Active || !c.state.Enabled {
			c.mu.Unlock()
			return
		}
		freeze := expired || c.frame.Ready
		if freeze {
			frozen := c.frame
			frozen.Lines = append([]string(nil), c.frame.Lines...)
			c.frozen = &frozen
			c.attempted = true
			c.queued = time.Now()
			c.state.ReadySeats = frozen.Rated
			c.state.CachedSeats = frozen.Cached
			c.state.TotalSeats = len(frozen.Lines)
			c.state.FreezeMS = time.Since(c.started).Milliseconds()
			c.result = "queued"
			c.statusLocked("queued")
			diagnostics.Default.Record("/brief/freeze", time.Since(c.started), 200, false)
		}
		c.mu.Unlock()
		if !freeze {
			continue
		}
		if c.skipEmpty(token) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case result := <-chat:
			if result.err != nil {
				c.finish(token, "unavailable", true)
				return
			}
			c.sendFrozen(ctx, token, result.room)
			return
		}
	}
}

type conversationResult struct {
	room string
	err  error
}

func (c *Controller) skipEmpty(token uint64) bool {
	c.mu.Lock()
	empty := c.sequence != token || c.frozen == nil || c.frozen.Rated == 0 || len(c.frozen.Lines) == 0
	c.mu.Unlock()
	if empty {
		c.finish(token, "no-data", true)
	}
	return empty
}

func (c *Controller) sendFrozen(parent context.Context, token uint64, room string) {
	c.mu.Lock()
	if c.sequence != token || !c.state.Enabled || !c.frame.Active || c.frozen == nil {
		c.mu.Unlock()
		return
	}
	if _, used := c.rooms[room]; used {
		c.mu.Unlock()
		c.finish(token, "skipped", true)
		return
	}
	frame := *c.frozen
	c.rooms[room] = token
	c.pruneRoomsLocked()
	body, audience, src := core.AnalysisBriefBody(frame.Lines), c.state.Audience, c.source
	queuedToPost := time.Since(c.queued)
	c.state.QueuedToPostMS = queuedToPost.Milliseconds()
	c.statusLocked("sending")
	c.mu.Unlock()
	request, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	started := time.Now()
	dispatched, err := src.SendAnalysisBrief(request, room, body, audience)
	elapsed := time.Since(started)
	diagnostics.Default.Record("/brief/queue-to-post", queuedToPost, 200, false)
	diagnostics.Default.Record("/brief/post", elapsed, 200, err != nil)
	c.mu.Lock()
	if c.sequence == token {
		c.state.PostMS = elapsed.Milliseconds()
	}
	c.mu.Unlock()
	status := "sent"
	if !dispatched {
		status = "unavailable"
	} else if err != nil {
		status = "uncertain"
	}
	c.finish(token, status, true)
}

func (c *Controller) MuteCurrent() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
	c.attempted, c.result = true, "muted"
	c.statusLocked("muted")
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

// Only discovery retries. Require the same ready room across two observations
// so an open conversation from a reconnecting chat session cannot send a brief.
func readConversation(ctx context.Context, src source.AnalysisBriefSource, interval time.Duration) (string, error) {
	previous := ""
	for attempt := 0; attempt < 8 && ctx.Err() == nil; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}
		room, err := src.ChampSelectConversation(ctx)
		switch apperrors.CodeOf(err) {
		case "":
			if room != "" && room == previous {
				return room, nil
			}
			previous = room
		case apperrors.DataUnavailable, apperrors.ServiceBusy, apperrors.RequestTimeout:
			previous = ""
		default:
			return "", err
		}
	}
	return "", apperrors.New(apperrors.DataUnavailable)
}

// Empty initial metadata belongs to the same observed selection. A changed
// verified key identifies a new selection even if no intervening phase arrived.
func (c *Controller) SelectionChanged(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return key != "" && c.frame.SelectionKey != "" && key != c.frame.SelectionKey
}
