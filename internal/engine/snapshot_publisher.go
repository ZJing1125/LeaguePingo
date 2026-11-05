package engine

import (
	"sync"

	"leaguepingo/internal/core"
)

// snapshotPublisher owns the ordered callback worker and bounded update queue.
// Callers provide a private snapshot cloned under Engine's lock. The callback
// runs on its worker, outside both Engine and publisher critical sections.
type snapshotPublisher struct {
	mu      sync.RWMutex
	updates chan core.Snapshot
}

func newSnapshotPublisher(onChange func(core.Snapshot)) *snapshotPublisher {
	p := &snapshotPublisher{}
	if onChange != nil {
		p.updates = make(chan core.Snapshot, 32)
		go func() {
			for snapshot := range p.updates {
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
	select {
	case p.updates <- snapshot:
	default:
		// Retain the original drop-one policy and single-producer ordering.
		// A slow UI must not stall client polling or preparation.
		select {
		case <-p.updates:
		default:
		}
		p.updates <- snapshot
	}
}
