package diagnostics

import "time"

type ReadinessMetric struct {
	Started            uint64 `json:"started"`
	Completed          uint64 `json:"completed"`
	Abandoned          uint64 `json:"abandoned"`
	PreexistingResults uint64 `json:"preexistingResults"`
	LastMS             *int64 `json:"lastMs"`
	MaxMS              *int64 `json:"maxMs"`
	Pending            bool   `json:"pending"`
}

// Readiness is owned by the engine lock. It keeps no account, match or player IDs.
// Time starts at the first observed ChampSelect, not at the first roster result.
type Readiness struct {
	phase  string
	start  time.Time
	metric ReadinessMetric
}

func (r *Readiness) Phase(phase string, now time.Time) {
	if phase == r.phase {
		return
	}
	r.Reset()
	r.phase = phase
	if phase == "ChampSelect" {
		r.start = now
		r.metric.Started++
		r.metric.Pending = true
	}
}
func (r *Readiness) Reset() {
	if r.metric.Pending {
		r.metric.Abandoned++
	}
	r.metric.Pending = false
	r.start = time.Time{}
	r.phase = ""
}
func (r *Readiness) First(now, preparedAt time.Time) {
	if !r.metric.Pending || now.Before(r.start) {
		return
	}
	ms := now.Sub(r.start).Milliseconds()
	r.metric.LastMS = &ms
	if r.metric.MaxMS == nil || ms > *r.metric.MaxMS {
		r.metric.MaxMS = &ms
	}
	r.metric.Completed++
	if preparedAt.Before(r.start) {
		r.metric.PreexistingResults++
	}
	r.metric.Pending = false
}
func (r *Readiness) Snapshot() ReadinessMetric { return r.metric }
