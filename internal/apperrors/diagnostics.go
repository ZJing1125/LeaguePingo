package apperrors

import (
	"sort"
	"sync"
)

// Failure observations, not unique failed operations: a request may be seen at
// the transport and UI layers. No raw text, path, timing or identity is retained.
type FailureMetric struct {
	Service Service `json:"service"`
	Code    Code    `json:"code"`
	Status  int     `json:"status,omitempty"`
	Count   uint64  `json:"count"`
}
type failureKey struct {
	service Service
	code    Code
	status  int
}
type FailureCollector struct {
	mu     sync.Mutex
	values map[failureKey]uint64
}

var Failures FailureCollector

func (c *FailureCollector) Record(service Service, err error) {
	if err == nil {
		return
	}
	e := Normalize(err, Unexpected)
	switch service {
	case LocalClient, DataService, GuideService, Desktop:
	default:
		service = Desktop
	}
	// Source fields can only be populated by this package's fixed constructors.
	key := failureKey{service, e.code, e.status}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.values == nil {
		c.values = make(map[failureKey]uint64)
	}
	// Fixed bound even if many HTTP statuses occur during a long session.
	if _, ok := c.values[key]; !ok && len(c.values) >= 127 {
		key = failureKey{Desktop, Unexpected, 0}
	}
	c.values[key]++
}
func (c *FailureCollector) Snapshot() []FailureMetric {
	c.mu.Lock()
	out := make([]FailureMetric, 0, len(c.values))
	for k, n := range c.values {
		out = append(out, FailureMetric{k.service, k.code, k.status, n})
	}
	c.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Service != out[j].Service {
			return out[i].Service < out[j].Service
		}
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		return out[i].Status < out[j].Status
	})
	return out
}
