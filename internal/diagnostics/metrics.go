package diagnostics

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type Metric struct {
	Calls         uint64 `json:"calls"`
	Failed        uint64 `json:"failed"`
	TotalMS       int64  `json:"totalMs"`
	MaxMS         int64  `json:"maxMs"`
	LastStatus    int    `json:"lastStatus"`
	RecentSamples int    `json:"recentSamples"`
	P95MS         *int64 `json:"p95Ms"`
}

const latencyWindowSize = 256

type latencyWindow struct {
	values      [latencyWindowSize]int64
	count, next int
}
type Collector struct {
	mu     sync.Mutex
	values map[string]Metric
	recent map[string]*latencyWindow
}

var Default = &Collector{values: map[string]Metric{}}

// Fixed allow-list: never retain paths, player IDs, credentials or error text.
func Category(path string) string {
	switch {
	case path == "/lol-honor-v2/v1/vote-completion":
		return "honor-completion"
	case path == "/lol-pre-end-of-game/v1/currentSequenceEvent":
		return "post-game-sequence"
	case path == "/lol-pre-end-of-game/v1/complete/missions-celebration":
		return "post-game-continue"
	case path == "/lol-honor-v2/v1/ballot":
		return "honor-ballot"
	case path == "/lol-honor/v1/ballot":
		return "honor-submit"
	case path == "/lol-lobby/v2/lobby":
		return "lobby"
	case path == "/lol-lobby/v2/play-again":
		return "play-again"
	case path == "/lol-lobby/v2/lobby/matchmaking/search":
		return "start-search"
	case path == "/ready-check-events":
		return "ready-check-events"
	case path == "/lol-matchmaking/v1/ready-check":
		return "ready-check"
	case path == "/lol-matchmaking/v1/ready-check/accept":
		return "ready-check-accept"
	case path == "/sgp-history":
		return "sgp-history"
	case path == "/sgp-rank":
		return "sgp-rank"
	case path == "/sgp-detail":
		return "sgp-detail"
	case path == "/sgp-timeline":
		return "sgp-timeline"
	case path == "/sgp-group":
		return "sgp-group"
	case path == "/history-cache/write":
		return "history-cache-write"
	case path == "/lol-league-session/v1/league-session-token":
		return "rank-session"
	case path == "/phase-events":
		return "phase-events"
	case path == "/champ-select-events":
		return "champ-select-events"
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
	case strings.HasPrefix(path, "/lol-seasons/"):
		return "season"
	case strings.HasPrefix(path, "/lol-champion-mastery/"):
		return "mastery"
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
	if c.recent == nil {
		c.recent = map[string]*latencyWindow{}
	}
	w := c.recent[key]
	if w == nil {
		w = &latencyWindow{}
		c.recent[key] = w
	}
	w.values[w.next] = max(0, elapsed.Milliseconds())
	w.next = (w.next + 1) % latencyWindowSize
	w.count = min(w.count+1, latencyWindowSize)
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

// Percentile returns the p-th percentile (1..100) of the recent latency window
// for one fixed category. It copies only that window, so a hot path can ask
// without materialising every category.
func (c *Collector) Percentile(category string, p int) (int64, bool) {
	if p <= 0 || p > 100 {
		return 0, false
	}
	c.mu.Lock()
	samples := []int64{}
	if w := c.recent[category]; w != nil && w.count > 0 {
		samples = append(samples, w.values[:w.count]...)
	}
	c.mu.Unlock()
	if len(samples) == 0 {
		return 0, false
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return samples[(p*len(samples)+99)/100-1], true
}

func (c *Collector) Snapshot() map[string]Metric {
	c.mu.Lock()
	out := map[string]Metric{}
	windows := map[string][]int64{}
	for k, v := range c.values {
		out[k] = v
		if w := c.recent[k]; w != nil {
			windows[k] = append([]int64(nil), w.values[:w.count]...)
		}
	}
	c.mu.Unlock()
	// Sort only copied, bounded windows; request recording never sorts or waits on I/O.
	for k, samples := range windows {
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		v := out[k]
		v.RecentSamples = len(samples)
		if len(samples) > 0 {
			p95 := samples[(95*len(samples)+99)/100-1]
			v.P95MS = &p95
		}
		out[k] = v
	}
	return out
}
