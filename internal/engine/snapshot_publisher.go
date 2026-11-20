package engine

import (
	"sync"
	"sync/atomic"

	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
)

// snapshotPublisher owns the ordered callback worker and bounded update queue.
// Callers provide a private snapshot cloned under Engine's lock. The callback
// runs on its worker, outside both Engine and publisher critical sections.
type snapshotPublisher struct {
	sent, dropped, delivered atomic.Uint64
	mu                       sync.RWMutex
	updates                  chan core.Snapshot
}

func newSnapshotPublisher(onChange func(core.Snapshot)) *snapshotPublisher {
	p := &snapshotPublisher{}
	if onChange != nil {
		p.updates = make(chan core.Snapshot, 32)
		go func() {
			for snapshot := range p.updates {
				p.delivered.Add(1)
				for _, player := range snapshot.Participants {
					if player.Scout != nil {
						diagnostics.Responses.Published(player.Scout.ResponseID)
					}
				}
				onChange(snapshot)
			}
		}()
	}
	return p
}

func (p *snapshotPublisher) enabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.updates != nil
}

func (p *snapshotPublisher) publish(snapshot core.Snapshot) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.updates == nil {
		return
	}
	p.sent.Add(1)
	select {
	case p.updates <- snapshot:
	default:
		// Retain the original drop-one policy and single-producer ordering.
		// A slow UI must not stall client polling or preparation.
		select {
		case <-p.updates:
			p.dropped.Add(1)
		default:
		}
		p.updates <- snapshot
	}
}

type PublicationMetrics struct {
	Sent      uint64 `json:"sent"`
	Dropped   uint64 `json:"dropped"`
	Delivered uint64 `json:"delivered"`
}

func (e *Engine) PublicationMetrics() PublicationMetrics {
	p := e.publisher
	return PublicationMetrics{Sent: p.sent.Load(), Dropped: p.dropped.Load(), Delivered: p.delivered.Load()}
}
