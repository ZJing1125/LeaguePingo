// Package postgame runs an opt-in, one-shot action after a tracked game ends.
package postgame

import (
	"context"
	"errors"
	"sync"
	"time"

	"leaguepingo/internal/source"
)

type State struct {
	SkipHonor     bool      `json:"skipHonor"`
	Revision      uint64    `json:"revision"`
	ReturnEnabled bool      `json:"returnEnabled"`
	QueueEnabled  bool      `json:"queueEnabled"`
	DelaySeconds  int       `json:"delaySeconds"`
	Status        string    `json:"status"`
	DueAt         time.Time `json:"dueAt"`
}

// Observation is provided only after Engine has frozen the completed review.
// No identities are ever included in State, logs or persisted automation data.
type Observation struct {
	Version        uint64
	Account, Phase string
	GameID         int64
	QueueID        int
	ReviewID       int64
	ReviewState    string
}
type game struct {
	version uint64
	account string
	id      int64
	queue   int
}
type task struct {
	game
	client  source.PostGameSource
	started time.Time
}
type timings struct{ probe, request, total, settlement, honor, returnDelay, lobbyWait, verify time.Duration }

func defaults() timings {
	return timings{time.Second, time.Second, 90 * time.Second, 15 * time.Second, 15 * time.Second, 3 * time.Second, 30 * time.Second, 5 * time.Second}
}

type Controller struct {
	mu        sync.Mutex
	state     State
	version   uint64
	account   string
	candidate *game
	attempted int64
	job       *task
	settled   bool
	cancel    context.CancelFunc
	wake      chan struct{}
	changes   chan State
	timing    timings
}

func New() *Controller {
	return &Controller{state: State{Status: "off", DelaySeconds: 10}, wake: make(chan struct{}, 1), changes: make(chan State, 1), timing: defaults()}
}
func ValidDelay(v int) bool                 { return v == 5 || v == 10 || v == 15 || v == 20 }
func (c *Controller) Snapshot() State       { c.mu.Lock(); defer c.mu.Unlock(); return c.state }
func (c *Controller) Changes() <-chan State { return c.changes }
func (c *Controller) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}
func (c *Controller) publishLocked(status string, due time.Time) {
	if c.state.Status == status && c.state.DueAt.Equal(due) {
		return
	}
	c.state.Status, c.state.DueAt = status, due
	c.state.Revision++
	select {
	case <-c.changes:
	default:
	}
	c.changes <- c.state
}
func (c *Controller) idleLocked() {
	s := "armed"
	if !c.state.ReturnEnabled && !c.state.QueueEnabled && !c.state.SkipHonor {
		s = "off"
	}
	c.publishLocked(s, time.Time{})
}
func (c *Controller) stopLocked(status string) {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.job = nil
	c.publishLocked(status, time.Time{})
	c.signal()
}
func (c *Controller) Configure(ret, queue bool, delay int, skipHonor ...bool) error {
	if !ValidDelay(delay) {
		return errors.New("排队延迟仅支持 5、10、15、20 秒")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	skip := c.state.SkipHonor
	if len(skipHonor) > 0 {
		skip = skipHonor[0]
	}
	if c.state.ReturnEnabled == ret && c.state.QueueEnabled == queue && c.state.DelaySeconds == delay && c.state.SkipHonor == skip {
		return nil
	}
	c.stopLocked("cancelled")
	c.state.ReturnEnabled, c.state.QueueEnabled, c.state.DelaySeconds, c.state.SkipHonor = ret, queue, delay, skip
	// Publish config changes even when the visible status stays the same.
	c.state.Status = ""
	c.idleLocked()
	return nil
}
func (c *Controller) ConfigureSkipHonor(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.SkipHonor == enabled {
		return
	}
	c.stopLocked("cancelled")
	c.state.SkipHonor = enabled
	c.state.Status = ""
	c.idleLocked()
}
func (c *Controller) Cancel() (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil {
		return c.state, errors.New("当前没有可取消的赛后自动任务")
	}
	c.stopLocked("cancelled")
	return c.state, nil
}
func completion(phase string) bool {
	return phase == "WaitingForStats" || phase == "PreEndOfGame" || phase == "EndOfGame" || phase == "Lobby"
}
func searching(phase string) bool {
	return phase == "Matchmaking" || phase == "ReadyCheck" || phase == "ChampSelect" || phase == "GameStart"
}
func (c *Controller) Observe(client source.PostGameSource, o Observation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if o.Version != c.version || (o.Account != "" && o.Account != c.account) {
		c.stopLocked("cancelled")
		c.candidate = nil
		c.attempted = 0
		c.version, c.account = o.Version, o.Account
		c.idleLocked()
	}
	if client == nil || o.Version == 0 || o.Account == "" {
		if c.job != nil {
			c.stopLocked("cancelled")
		}
		c.candidate = nil
		return
	}
	if o.Phase == "InProgress" && o.GameID > 0 && o.QueueID > 0 {
		if c.candidate == nil || c.candidate.id != o.GameID {
			c.stopLocked("armed")
			c.candidate = &game{o.Version, o.Account, o.GameID, o.QueueID}
			c.attempted = 0
			c.idleLocked()
		}
		return
	}
	if c.job != nil {
		if !completion(o.Phase) {
			status := "cancelled"
			if searching(o.Phase) {
				status = "queued"
			}
			c.stopLocked(status)
		} else if o.ReviewID == c.job.id {
			c.settled = o.ReviewState == "ready" || o.ReviewState == "unavailable"
		}
		return
	}
	if !completion(o.Phase) {
		c.candidate = nil
		return
	}
	if !c.state.ReturnEnabled && !c.state.QueueEnabled && !c.state.SkipHonor {
		return
	}
	if c.candidate == nil || c.candidate.id == c.attempted || o.ReviewID != c.candidate.id || !completion(o.Phase) {
		return
	}
	c.attempted = c.candidate.id
	c.job = &task{*c.candidate, client, time.Now()}
	c.settled = o.ReviewState == "ready" || o.ReviewState == "unavailable"
	c.publishLocked("waiting-result", time.Time{})
	c.signal()
}

// Hints can cancel/wake a task but cannot create one or bypass review freezing.
func (c *Controller) Hint(version uint64, phase string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if version != c.version {
		return
	}
	if phase == "None" {
		c.candidate = nil
	}
	if c.job == nil {
		return
	}
	if !completion(phase) {
		s := "cancelled"
		if searching(phase) {
			s = "queued"
		}
		c.stopLocked(s)
	} else {
		c.signal()
	}
}
func (c *Controller) Run(ctx context.Context) {
	defer func() { c.mu.Lock(); c.stopLocked("off"); c.mu.Unlock() }()
	for {
		c.mu.Lock()
		job := c.job
		var runCtx context.Context
		if job != nil {
			var cancel context.CancelFunc
			runCtx, cancel = context.WithTimeout(ctx, c.timing.total)
			c.cancel = cancel
		}
		c.mu.Unlock()
		if ctx.Err() != nil {
			return
		}
		if job != nil {
			c.runTask(runCtx, job)
			c.mu.Lock()
			if c.job == job {
				if runCtx.Err() != nil {
					c.publishLocked("timeout", time.Time{})
				}
				if c.cancel != nil {
					c.cancel()
					c.cancel = nil
				}
				c.job = nil
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
func (c *Controller) status(job *task, status string, due time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job != job {
		return false
	}
	c.publishLocked(status, due)
	return true
}
func pause(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
func (c *Controller) phase(ctx context.Context, job *task) (string, error) {
	req, cancel := context.WithTimeout(ctx, c.timing.request)
	defer cancel()
	return job.client.Phase(req)
}
func (c *Controller) sameAccount(ctx context.Context, job *task) bool {
	req, cancel := context.WithTimeout(ctx, c.timing.request)
	defer cancel()
	id, err := job.client.AccountIdentity(req)
	return err == nil && id == job.account && id != "" && ctx.Err() == nil && req.Err() == nil
}
func (c *Controller) runTask(ctx context.Context, job *task) {
	var preAt, returnDue, returnSent, lobbyAt, queueDue time.Time
	honor := honorProgress{}
	party, members := "", ""
	for {
		if ctx.Err() != nil {
			return
		}
		c.mu.Lock()
		valid := c.job == job
		ret, queue, delay, settled := c.state.ReturnEnabled, c.state.QueueEnabled, c.state.DelaySeconds, c.settled
		skip := c.state.SkipHonor
		c.mu.Unlock()
		if !valid {
			return
		}
		phase, err := c.phase(ctx, job)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			c.status(job, "failed", time.Time{})
			return
		}
		if !completion(phase) {
			s := "cancelled"
			if searching(phase) {
				s = "queued"
			}
			c.status(job, s, time.Time{})
			return
		}
		now := time.Now()
		if skip && !honor.done {
			if phase == "WaitingForStats" || (phase != "Lobby" && !settled && now.Sub(job.started) < c.timing.settlement) {
				c.status(job, "waiting-result", time.Time{})
				if !pause(ctx, c.timing.probe) {
					return
				}
				continue
			}
			ready, stop := c.advanceHonor(ctx, job, phase, &honor)
			if stop {
				return
			}
			if !ready {
				if !pause(ctx, c.timing.probe) {
					return
				}
				continue
			}
			// The ceremony request/verification may consume time or change phase.
			continue
		}
		if skip && honor.done && !ret && !queue {
			status := "honor-not-needed"
			if !honor.sent.IsZero() {
				status = "honor-skipped"
			}
			c.status(job, status, time.Time{})
			return
		}
		if phase != "Lobby" {
			if !returnSent.IsZero() {
				if now.Sub(returnSent) > c.timing.verify {
					c.status(job, "failed", time.Time{})
					return
				}
			} else if !ret {
				c.status(job, "waiting-return", time.Time{})
			} else if phase == "WaitingForStats" || (!settled && now.Sub(job.started) < c.timing.settlement) {
				c.status(job, "waiting-result", time.Time{})
			} else {
				if phase == "PreEndOfGame" && !skip {
					if preAt.IsZero() {
						preAt = now
					}
					if now.Sub(preAt) < c.timing.honor {
						c.status(job, "waiting-result", time.Time{})
						if !pause(ctx, c.timing.probe) {
							return
						}
						continue
					}
				}
				if returnDue.IsZero() {
					returnDue = now.Add(c.timing.returnDelay)
				}
				c.status(job, "return-countdown", returnDue)
				if !now.Before(returnDue) {
					if !c.sameAccount(ctx, job) {
						c.status(job, "cancelled", time.Time{})
						return
					}
					fresh, err := c.phase(ctx, job)
					if err != nil {
						c.status(job, "failed", time.Time{})
						return
					}
					if fresh == "Lobby" {
						continue
					}
					if fresh != "PreEndOfGame" && fresh != "EndOfGame" {
						c.status(job, "cancelled", time.Time{})
						return
					}
					if ctx.Err() != nil {
						return
					}
					if !c.status(job, "returning", time.Time{}) {
						return
					}
					req, cancel := context.WithTimeout(ctx, c.timing.request)
					_ = job.client.PlayAgain(req)
					cancel()
					returnSent = time.Now()
					// Never repeat a mutation on an unknown response; observe the actual phase.
					continue
				}
			}
		} else {
			if !queue {
				c.status(job, "returned", time.Time{})
				return
			}
			if lobbyAt.IsZero() {
				lobbyAt = now
			}
			req, cancel := context.WithTimeout(ctx, c.timing.request)
			lobby, err := job.client.LobbyControl(req)
			cancel()
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				c.status(job, "failed", time.Time{})
				return
			}
			if lobby.QueueID != job.queue || lobby.PartyKey == "" || lobby.MembersKey == "" || (party != "" && party != lobby.PartyKey) || (members != "" && members != lobby.MembersKey) {
				c.status(job, "changed", time.Time{})
				return
			}
			party = lobby.PartyKey
			if !lobby.Leader {
				c.status(job, "not-leader", time.Time{})
				return
			}
			if !lobby.Ready {
				queueDue = time.Time{}
				c.status(job, "waiting-ready", time.Time{})
				if now.Sub(lobbyAt) > c.timing.lobbyWait {
					c.status(job, "timeout", time.Time{})
					return
				}
			} else {
				if queueDue.IsZero() {
					members = lobby.MembersKey
					queueDue = now.Add(time.Duration(delay) * time.Second)
				}
				c.status(job, "queue-countdown", queueDue)
				if !now.Before(queueDue) {
					if !c.sameAccount(ctx, job) {
						c.status(job, "cancelled", time.Time{})
						return
					}
					fresh, err := c.phase(ctx, job)
					if err != nil || fresh != "Lobby" {
						c.status(job, "cancelled", time.Time{})
						return
					}
					req, cancel := context.WithTimeout(ctx, c.timing.request)
					latest, err := job.client.LobbyControl(req)
					cancel()
					if err != nil || !latest.Leader || !latest.Ready || latest.QueueID != job.queue || latest.PartyKey != party || latest.MembersKey != members {
						c.status(job, "changed", time.Time{})
						return
					}
					if ctx.Err() != nil {
						return
					}
					if !c.status(job, "queueing", time.Time{}) {
						return
					}
					req, cancel = context.WithTimeout(ctx, c.timing.request)
					_ = job.client.StartSearch(req)
					cancel()
					deadline := time.Now().Add(c.timing.verify)
					for ctx.Err() == nil {
						actual, err := c.phase(ctx, job)
						if err == nil && searching(actual) {
							c.status(job, "queued", time.Time{})
							return
						}
						if err != nil || actual != "Lobby" || time.Now().After(deadline) {
							c.status(job, "failed", time.Time{})
							return
						}
						if !pause(ctx, c.timing.probe) {
							return
						}
					}
					return
				}
			}
		}
		if !pause(ctx, c.timing.probe) {
			return
		}
	}
}
