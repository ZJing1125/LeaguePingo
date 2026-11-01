// Package postgame runs an opt-in, one-shot action after a tracked game ends.
package postgame

import (
	"context"
	"errors"
	"sync"
	"time"

	"leaguepingo/internal/actionlane"
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
	session actionlane.Session
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
	lane      actionlane.Executor
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
	c.lane.Stop()
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
	c.job = &task{game: *c.candidate, client: client, started: time.Now()}
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
		var session actionlane.Session
		if job != nil {
			var started bool
			runCtx, session, started = c.lane.Start(ctx, c.timing.total)
			if !started {
				job = nil
			} else {
				job.session = session
			}
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
				c.lane.Finish(session)
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
	if c.job != job || !c.lane.Current(job.session) {
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
	run := postGameRun{controller: c, ctx: ctx, job: job}
	for ctx.Err() == nil {
		config, ok := c.taskState(job)
		if !ok {
			return
		}
		phase, err := c.phase(ctx, job)
		if ctx.Err() != nil {
			return
		}
		if err != nil || !completion(phase) {
			run.finishPhaseError(phase, err)
			return
		}
		result := run.advancePreamble(phase, time.Now(), config)
		if result == stepProceed {
			if phase == "Lobby" {
				result = run.advanceLobby(time.Now(), config)
			} else {
				result = run.advanceReturn(phase, time.Now(), config)
			}
		}
		if result == stepStop {
			return
		}
		if result == stepRepeat {
			continue
		}
		if !pause(ctx, c.timing.probe) {
			return
		}
	}
}

type taskConfig struct {
	ret, queue, skip bool
	delay            int
	settled          bool
}

type stepResult uint8

const (
	stepProceed stepResult = iota
	stepRepeat
	stepStop
)

type postGameRun struct {
	controller                                      *Controller
	ctx                                             context.Context
	job                                             *task
	preAt, returnDue, returnSent, lobbyAt, queueDue time.Time
	honor                                           honorProgress
	sequence                                        sequenceProgress
	party, members                                  string
}

func (c *Controller) taskState(job *task) (taskConfig, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	config := taskConfig{c.state.ReturnEnabled, c.state.QueueEnabled, c.state.SkipHonor, c.state.DelaySeconds, c.settled}
	return config, c.job == job && c.lane.Current(job.session)
}

func (r *postGameRun) finishPhaseError(phase string, err error) {
	status := "failed"
	if err == nil {
		status = "cancelled"
		if searching(phase) {
			status = "queued"
		}
	}
	r.controller.status(r.job, status, time.Time{})
}

func (r *postGameRun) advancePreamble(phase string, now time.Time, config taskConfig) stepResult {
	if config.skip && !r.honor.done {
		if phase == "WaitingForStats" || (phase != "Lobby" && !config.settled && now.Sub(r.job.started) < r.controller.timing.settlement) {
			r.controller.status(r.job, "waiting-result", time.Time{})
			if !pause(r.ctx, r.controller.timing.probe) {
				return stepStop
			}
			return stepRepeat
		}
		ready, stop := r.controller.advanceHonor(r.ctx, r.job, phase, &r.honor, &r.sequence)
		if stop {
			return stepStop
		}
		if !ready && !pause(r.ctx, r.controller.timing.probe) {
			return stepStop
		}
		return stepRepeat
	}
	if !r.sequence.sent.IsZero() && !r.sequence.done {
		ready, stop := r.controller.advanceContinue(r.ctx, r.job, phase, &r.sequence)
		if stop {
			return stepStop
		}
		if !ready {
			if !pause(r.ctx, r.controller.timing.probe) {
				return stepStop
			}
			return stepRepeat
		}
	}
	if config.skip && r.honor.done && !config.ret && !config.queue {
		status := "honor-not-needed"
		if !r.honor.sent.IsZero() {
			status = "honor-skipped"
		}
		r.controller.status(r.job, status, time.Time{})
		return stepStop
	}
	return stepProceed
}

func (r *postGameRun) advanceReturn(phase string, now time.Time, config taskConfig) stepResult {
	if !r.returnSent.IsZero() {
		if now.Sub(r.returnSent) > r.controller.timing.verify {
			r.controller.status(r.job, "failed", time.Time{})
			return stepStop
		}
		return stepProceed
	}
	if !config.ret {
		r.controller.status(r.job, "waiting-return", time.Time{})
		return stepProceed
	}
	if phase == "WaitingForStats" || (!config.settled && now.Sub(r.job.started) < r.controller.timing.settlement) {
		r.controller.status(r.job, "waiting-result", time.Time{})
		return stepProceed
	}
	if phase == "PreEndOfGame" && !config.skip {
		if r.preAt.IsZero() {
			r.preAt = now
		}
		if now.Sub(r.preAt) < r.controller.timing.honor {
			r.controller.status(r.job, "waiting-result", time.Time{})
			if !pause(r.ctx, r.controller.timing.probe) {
				return stepStop
			}
			return stepRepeat
		}
	}
	if r.returnDue.IsZero() {
		r.returnDue = now.Add(r.controller.timing.returnDelay)
	}
	r.controller.status(r.job, "return-countdown", r.returnDue)
	if now.Before(r.returnDue) {
		return stepProceed
	}
	return r.sendReturn()
}

func (r *postGameRun) sendReturn() stepResult {
	phase, result := r.freshReturnPhase()
	if result != stepProceed || phase == "Lobby" {
		return result
	}
	if !r.sequence.done {
		ready, stop := r.controller.advanceContinue(r.ctx, r.job, phase, &r.sequence)
		if stop {
			return stepStop
		}
		if !ready {
			if !pause(r.ctx, r.controller.timing.probe) {
				return stepStop
			}
			return stepRepeat
		}
		if !r.sequence.sent.IsZero() {
			return stepRepeat
		}
		phase, result = r.freshReturnPhase()
		if result != stepProceed || phase == "Lobby" {
			return result
		}
	}
	if r.ctx.Err() != nil || !r.controller.status(r.job, "returning", time.Time{}) {
		return stepStop
	}
	req, cancel := context.WithTimeout(r.ctx, r.controller.timing.request)
	_ = r.job.client.PlayAgain(req)
	cancel()
	r.returnSent = time.Now()
	return stepRepeat
}

func (r *postGameRun) freshReturnPhase() (string, stepResult) {
	if !r.controller.sameAccount(r.ctx, r.job) {
		r.controller.status(r.job, "cancelled", time.Time{})
		return "", stepStop
	}
	phase, err := r.controller.phase(r.ctx, r.job)
	if err != nil {
		r.controller.status(r.job, "failed", time.Time{})
		return "", stepStop
	}
	if phase == "Lobby" {
		return phase, stepRepeat
	}
	if phase != "PreEndOfGame" && phase != "EndOfGame" {
		r.controller.status(r.job, "cancelled", time.Time{})
		return "", stepStop
	}
	return phase, stepProceed
}

func (r *postGameRun) advanceLobby(now time.Time, config taskConfig) stepResult {
	if !config.queue {
		r.controller.status(r.job, "returned", time.Time{})
		return stepStop
	}
	if r.lobbyAt.IsZero() {
		r.lobbyAt = now
	}
	req, cancel := context.WithTimeout(r.ctx, r.controller.timing.request)
	lobby, err := r.job.client.LobbyControl(req)
	cancel()
	if r.ctx.Err() != nil {
		return stepStop
	}
	if err != nil {
		r.controller.status(r.job, "failed", time.Time{})
		return stepStop
	}
	if lobby.QueueID != r.job.queue || lobby.PartyKey == "" || lobby.MembersKey == "" || (r.party != "" && r.party != lobby.PartyKey) || (r.members != "" && r.members != lobby.MembersKey) {
		r.controller.status(r.job, "changed", time.Time{})
		return stepStop
	}
	r.party = lobby.PartyKey
	if !lobby.Leader {
		r.controller.status(r.job, "not-leader", time.Time{})
		return stepStop
	}
	if !lobby.Ready {
		r.queueDue = time.Time{}
		r.controller.status(r.job, "waiting-ready", time.Time{})
		if now.Sub(r.lobbyAt) > r.controller.timing.lobbyWait {
			r.controller.status(r.job, "timeout", time.Time{})
			return stepStop
		}
		return stepProceed
	}
	if r.queueDue.IsZero() {
		r.members = lobby.MembersKey
		r.queueDue = now.Add(time.Duration(config.delay) * time.Second)
	}
	r.controller.status(r.job, "queue-countdown", r.queueDue)
	if now.Before(r.queueDue) {
		return stepProceed
	}
	return r.sendQueue()
}

func (r *postGameRun) sendQueue() stepResult {
	if !r.controller.sameAccount(r.ctx, r.job) {
		r.controller.status(r.job, "cancelled", time.Time{})
		return stepStop
	}
	phase, err := r.controller.phase(r.ctx, r.job)
	if err != nil || phase != "Lobby" {
		r.controller.status(r.job, "cancelled", time.Time{})
		return stepStop
	}
	req, cancel := context.WithTimeout(r.ctx, r.controller.timing.request)
	latest, err := r.job.client.LobbyControl(req)
	cancel()
	if err != nil || !latest.Leader || !latest.Ready || latest.QueueID != r.job.queue || latest.PartyKey != r.party || latest.MembersKey != r.members {
		r.controller.status(r.job, "changed", time.Time{})
		return stepStop
	}
	if r.ctx.Err() != nil || !r.controller.status(r.job, "queueing", time.Time{}) {
		return stepStop
	}
	req, cancel = context.WithTimeout(r.ctx, r.controller.timing.request)
	_ = r.job.client.StartSearch(req)
	cancel()
	return r.verifyQueue()
}

func (r *postGameRun) verifyQueue() stepResult {
	deadline := time.Now().Add(r.controller.timing.verify)
	for r.ctx.Err() == nil {
		phase, err := r.controller.phase(r.ctx, r.job)
		if err == nil && searching(phase) {
			r.controller.status(r.job, "queued", time.Time{})
			return stepStop
		}
		if err != nil || phase != "Lobby" || time.Now().After(deadline) {
			r.controller.status(r.job, "failed", time.Time{})
			return stepStop
		}
		if !pause(r.ctx, r.controller.timing.probe) {
			return stepStop
		}
	}
	return stepStop
}
