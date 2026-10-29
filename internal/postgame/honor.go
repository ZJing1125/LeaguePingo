package postgame

import (
	"context"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/source"
)

const maxHonorReads = 24

type honorProgress struct {
	opened, due, sent time.Time
	reads             int
	manual, done      bool
	submittedPhase    string
}

func (c *Controller) readHonor(ctx context.Context, client source.HonorSource, p *honorProgress) (int64, bool, error) {
	if p.reads >= maxHonorReads {
		return 0, false, apperrors.New(apperrors.RequestTimeout)
	}
	p.reads++
	req, cancel := context.WithTimeout(ctx, c.timing.request)
	defer cancel()
	id, selected, err := client.HonorBallot(req)
	if req.Err() != nil {
		return 0, false, req.Err()
	}
	return id, selected, err
}

// Progress is private to the existing per-game task. Waiting never takes the
// player scheduler; limits and cancellation share the original 90s deadline.
func (c *Controller) advanceHonor(ctx context.Context, job *task, phase string, p *honorProgress, sequence *sequenceProgress) (ready, stop bool) {
	if phase == "Lobby" {
		p.done = true
		return true, false
	}
	client, ok := job.client.(source.HonorSource)
	if !ok {
		c.status(job, "honor-unavailable", time.Time{})
		return false, true
	}
	if p.opened.IsZero() {
		p.opened = time.Now()
	}
	if !p.sent.IsZero() {
		return c.verifyHonor(ctx, job, phase, p, sequence)
	}
	id, selected, err := c.readHonor(ctx, client, p)
	if ctx.Err() != nil {
		return false, true
	}
	if err != nil {
		c.status(job, "honor-unavailable", time.Time{})
		return false, true
	}
	if id != 0 && id != job.id {
		c.status(job, "honor-mismatch", time.Time{})
		return false, true
	}
	if c.honorFinished(ctx, job, phase, p, sequence) {
		p.done = true
		return true, false
	}
	if selected {
		p.manual = true
	}
	if id == 0 {
		p.due = time.Time{}
		// Allow a not-yet-published ceremony the existing PreEndOfGame grace.
		if phase == "PreEndOfGame" && time.Since(p.opened) < c.timing.honor {
			c.status(job, "waiting-honor", time.Time{})
			return false, false
		}
		p.done = true
		return true, false
	}
	if p.manual {
		c.status(job, "waiting-honor", time.Time{})
		return false, false
	}
	if p.due.IsZero() {
		p.due = time.Now().Add(c.timing.returnDelay)
	}
	if !c.status(job, "honor-countdown", p.due) {
		return false, true
	}
	if time.Now().Before(p.due) {
		return false, false
	}
	return c.submitEmptyHonor(ctx, job, client, p)
}

func (c *Controller) submitEmptyHonor(ctx context.Context, job *task, client source.HonorSource, p *honorProgress) (bool, bool) {
	if !c.sameAccount(ctx, job) {
		c.status(job, "cancelled", time.Time{})
		return false, true
	}
	phase, err := c.phase(ctx, job)
	if err != nil || (phase != "PreEndOfGame" && phase != "EndOfGame") {
		if err == nil && phase == "Lobby" {
			p.done = true
			return true, false
		}
		c.status(job, "cancelled", time.Time{})
		return false, true
	}
	// The user can select a vote during the countdown or account/phase reads.
	id, selected, err := c.readHonor(ctx, client, p)
	if ctx.Err() != nil {
		return false, true
	}
	if err != nil || (id != 0 && id != job.id) {
		c.status(job, "honor-unavailable", time.Time{})
		return false, true
	}
	if selected {
		p.manual = true
		c.status(job, "waiting-honor", time.Time{})
		return false, false
	}
	if id == 0 {
		p.due = time.Time{}
		return false, false
	}
	if !c.status(job, "skipping-honor", time.Time{}) || ctx.Err() != nil {
		return false, true
	}
	// A timeout may mean success: never repeat the submission, verify the phase
	// or explicit ceremony-completion evidence instead. Never submit a recipient.
	p.sent = time.Now()
	p.submittedPhase = phase
	req, cancel := context.WithTimeout(ctx, c.timing.request)
	_ = client.CompleteHonorBallot(req)
	cancel()
	return false, false
}

func (c *Controller) readHonorCompletion(ctx context.Context, job *task, p *honorProgress) (int64, error) {
	client, ok := job.client.(source.HonorCompletionSource)
	if !ok {
		return 0, nil
	}
	if p.reads >= maxHonorReads {
		return 0, apperrors.New(apperrors.RequestTimeout)
	}
	p.reads++
	req, cancel := context.WithTimeout(ctx, c.timing.request)
	defer cancel()
	id, err := client.HonorVoteCompletion(req)
	if req.Err() != nil {
		return 0, req.Err()
	}
	return id, err
}

func (c *Controller) honorFinished(ctx context.Context, job *task, phase string, p *honorProgress, sequence *sequenceProgress) bool {
	if phase == "EndOfGame" && p.submittedPhase == "PreEndOfGame" {
		return true
	}
	if phase == "EndOfGame" || (phase == "PreEndOfGame" && (!p.sent.IsZero() || p.manual)) {
		id, err := c.readHonorCompletion(ctx, job, p)
		if ctx.Err() == nil && err == nil && id == job.id {
			return true
		}
	}
	if phase == "PreEndOfGame" || phase == "EndOfGame" {
		name, available, err := c.readSequence(ctx, job, sequence)
		if ctx.Err() == nil && err == nil && available && name == "missions-celebration" {
			return true
		}
	}
	return false
}

func (c *Controller) verifyHonor(ctx context.Context, job *task, phase string, p *honorProgress, sequence *sequenceProgress) (bool, bool) {
	if time.Since(p.sent) >= c.timing.verify {
		c.status(job, "honor-unconfirmed", time.Time{})
		return false, true
	}
	verify, cancel := context.WithDeadline(ctx, p.sent.Add(c.timing.verify))
	defer cancel()
	if c.honorFinished(verify, job, phase, p, sequence) {
		p.done = true
		return true, false
	}
	id, _, err := c.readHonor(verify, job.client.(source.HonorSource), p)
	if ctx.Err() != nil {
		return false, true
	}
	if err == nil && id != 0 && id != job.id {
		c.status(job, "honor-mismatch", time.Time{})
		return false, true
	}
	if err == nil && id == 0 && phase == "EndOfGame" {
		p.done = true
		return true, false
	}
	// Cached/removed ballots are not definitive proof of an open/closed UI.
	// Keep verifying within the original deadline, never repeat the submission.
	c.status(job, "skipping-honor", time.Time{})
	return false, false
}
