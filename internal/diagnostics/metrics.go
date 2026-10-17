package diagnostics

import (
	"strings"
	"sync"
	"time"
)

type Metric struct {
	Calls      uint64 `json:"calls"`
	Failed     uint64 `json:"failed"`
	TotalMS    int64  `json:"totalMs"`
	MaxMS      int64  `json:"maxMs"`
	LastStatus int    `json:"lastStatus"`
}
type Collector struct {
	mu     sync.Mutex
	values map[string]Metric
}

var Default = &Collector{values: map[string]Metric{}}

// Fixed allow-list: never retain paths, player IDs, credentials or error text.
func Category(path string) string {
	switch {
	case path == "/phase-events":
		return "phase-events"
	case path == "/lol-gameflow/v1/gameflow-phase":
		return "phase"
	case path == "/lol-gameflow/v1/session":
		return "roster"
	case path == "/lol-champ-select/v1/session":
		return "champ-select"
	case strings.HasPrefix(path, "/lol-match-history/"):
		return "history"
	case strings.HasPrefix(path, "/lol-ranked/"):
		return "rank"
	case strings.HasPrefix(path, "/lol-summoner/"):
		return "profile"
	default:
		return "other"
	}
}
func (c *Collector) Record(path string, elapsed time.Duration, status int, failed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.values == nil {
		c.values = map[string]Metric{}
	}
	key := Category(path)
	v := c.values[key]
	v.Calls++
	if failed {
		v.Failed++
	}
	v.TotalMS += elapsed.Milliseconds()
	v.MaxMS = max(v.MaxMS, elapsed.Milliseconds())
	v.LastStatus = status
	c.values[key] = v
}
func (c *Collector) Snapshot() map[string]Metric {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]Metric{}
	for k, v := range c.values {
		out[k] = v
	}
	return out
}
