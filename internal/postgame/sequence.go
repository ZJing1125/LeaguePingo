package postgame

import (
	"context"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/source"
)

const maxSequenceReads = 12

type sequenceProgress struct {
	reads             int
	unavailable, done bool
	sent              time.Time
	submittedPhase    string
}

func (c *Controller) readSequence(ctx context.Context, job *task, p *sequenceProgress) (string, bool, error) {
	client, ok := job.client.(source.PostGameSequenceSource)
	if !ok || p.unavailable {
		return "", false, nil
	}
	if p.reads >= maxSequenceReads {
		return "", true, apperrors.New(apperrors.RequestTimeout)
	}
	p.reads++
	req, cancel := context.WithTimeout(ctx, c.timing.request)
	defer cancel()
	name, err := client.PostGameSequence(req)
	if req.Err() != nil {
		return "", true, req.Err()
	}
	if apperrors.Status(err) == 404 || apperrors.Status(err) == 405 {
		p.unavailable = true
		return "", false, nil
	}
	return name, true, err
}

// Only the explicit missions-celebration presentation is completed. This does
// not select a reward, cast a vote or dismiss arbitrary sequence events.
func (c *Controller) advanceContinue(ctx context.Context, job *task, phase string, p *sequenceProgress) (ready, stop bool) {
	if p.done || phase == "Lobby" || (!p.sent.IsZero() && p.submittedPhase == "PreEndOfGame" && phase == "EndOfGame") {
		p.done = true
		return true, false
	}
	if !p.sent.IsZero() {
		if time.Since(p.sent) >= c.timing.verify {
			c.status(job, "continue-unconfirmed", time.Time{})
			return false, true
		}
		verify, cancel := context.WithDeadline(ctx, p.sent.Add(c.timing.verify))
		defer cancel()
		name, available, err := c.readSequence(verify, job, p)
		if ctx.Err() != nil {
			return false, true
		}
		if err == nil && available && (name == "" || (phase == "EndOfGame" && name != "missions-celebration")) {
			p.done = true
			return true, false
		}
		c.status(job, "continuing-postgame", time.Time{})
		return false, false
	}
	name, available, err := c.readSequence(ctx, job, p)
	if ctx.Err() != nil {
		return false, true
	}
	if !available {
		return true, false
	} // Preserve the previous legacy return path.
	if err != nil {
		c.status(job, "continue-unavailable", time.Time{})
		return false, true
	}
	if name == "" {
		return true, false
	}
	if name != "missions-celebration" {
		if phase == "EndOfGame" {
			return true, false
		}
		c.status(job, "waiting-postgame", time.Time{})
		return false, false
	}
	return c.submitContinue(ctx, job, p)
}

func (c *Controller) submitContinue(ctx context.Context, job *task, p *sequenceProgress) (bool, bool) {
	if !c.sameAccount(ctx, job) {
		c.status(job, "cancelled", time.Time{})
		return false, true
	}
	phase, err := c.phase(ctx, job)
	if err != nil || (phase != "PreEndOfGame" && phase != "EndOfGame") {
		if err == nil && phase == "Lobby" {
			p.done = true
			return false, false
		}
		c.status(job, "cancelled", time.Time{})
		return false, true
	}
	name, available, err := c.readSequence(ctx, job, p)
	if ctx.Err() != nil {
		return false, true
	}
	if err != nil || !available {
		c.status(job, "continue-unavailable", time.Time{})
		return false, true
	}
	if name != "missions-celebration" {
		return false, false
	}
	if !c.status(job, "continuing-postgame", time.Time{}) || ctx.Err() != nil {
		return false, true
	}
	client := job.client.(source.PostGameSequenceSource)
	p.sent = time.Now()
	p.submittedPhase = phase
	req, cancel := context.WithTimeout(ctx, c.timing.request)
	_ = client.ContinuePostGameProgress(req)
	cancel()
	return false, false // Verify actual phase/event; never resubmit an uncertain write.
}
