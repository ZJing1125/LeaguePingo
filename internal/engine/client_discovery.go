package engine

import (
	"sync"
	"time"
)

// clientDiscovery owns discovery throttling; the discovery callback is immutable
// and runs without this mutex or Engine's mutex. Engine validates/installs the
// returned connection and retains the existing error/publication policy.
type clientDiscovery struct {
	mu         sync.Mutex
	discover   Discoverer
	lastLookup time.Time
}

func newClientDiscovery(discover Discoverer) *clientDiscovery {
	return &clientDiscovery{discover: discover}
}

func (d *clientDiscovery) claim(now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if now.Sub(d.lastLookup) < 10*time.Second {
		return false
	}
	d.lastLookup = now
	return true
}

func (d *clientDiscovery) find() (Client, error) {
	return d.discover()
}
