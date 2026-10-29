package diagnostics

import (
	"sync/atomic"
	"time"
)

type CallbackMetric struct {
	Calls   uint64   `json:"calls"`
	TotalUS float64  `json:"totalUs"`
	MaxUS   *float64 `json:"maxUs"`
	MeanUS  *float64 `json:"meanUs"`
}

// A callback records only a duration. No locks, allocation, input values or I/O.
type Callback struct{ calls, total, maximum atomic.Uint64 }

var KeyboardCallback Callback

func (c *Callback) Record(elapsed time.Duration) {
	ns := uint64(max(0, elapsed.Nanoseconds()))
	c.total.Add(ns)
	for old := c.maximum.Load(); ns > old; old = c.maximum.Load() {
		if c.maximum.CompareAndSwap(old, ns) {
			break
		}
	}
	c.calls.Add(1)
}
func (c *Callback) Snapshot() CallbackMetric {
	m := CallbackMetric{Calls: c.calls.Load(), TotalUS: float64(c.total.Load()) / 1000}
	if m.Calls > 0 {
		maximum := float64(c.maximum.Load()) / 1000
		m.MaxUS = &maximum
		mean := m.TotalUS / float64(m.Calls)
		m.MeanUS = &mean
	}
	return m
}
