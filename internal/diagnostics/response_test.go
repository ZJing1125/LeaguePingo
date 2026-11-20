package diagnostics

import (
	"sync"
	"testing"
	"time"
)

func TestResponseAckRequiresReadyPublicationAndIsOnce(t *testing.T) {
	r := &ResponseRegistry{}
	id := r.Begin("history")
	r.Acknowledge(id, time.Millisecond)
	r.Ready(id)
	r.Acknowledge(id, time.Millisecond)
	if r.Counts().Acknowledged != 0 {
		t.Fatal("unpublished result counted as visible")
	}
	r.Stage(id, "parse", time.Millisecond)
	r.Published(id)
	r.Published(id)
	r.Acknowledge(id, time.Millisecond)
	r.Acknowledge(id, time.Millisecond)
	if r.Counts().Acknowledged != 1 {
		t.Fatal("duplicate or absent acknowledgement")
	}
	if r.Begin("private-identity") != 0 {
		t.Fatal("unbounded category accepted")
	}
}
func TestResponseForkCancelExpiryAndCapacity(t *testing.T) {
	r := &ResponseRegistry{}
	id := r.Begin("guide")
	r.Stage(id, "source", time.Millisecond)
	fork := r.Fork(id)
	r.Ready(fork)
	r.Published(fork)
	r.Acknowledge(fork, 0)
	r.Discard(id)
	for range 300 {
		r.Begin("scout")
	}
	if len(r.spans) > 128 || r.Counts().Unobserved == 0 {
		t.Fatal("invisible data retained without bound")
	}
	old := r.Begin("history")
	r.spans[old].started = time.Now().Add(-4 * time.Minute)
	r.Ready(old)
	r.Published(old)
	r.Acknowledge(old, 0)
	if r.Counts().Acknowledged != 1 {
		t.Fatal("expired result accepted")
	}
}
func TestResponseConcurrentStageForkAndAck(t *testing.T) {
	r := &ResponseRegistry{}
	id := r.Begin("scout")
	r.Ready(id)
	r.Published(id)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				r.Stage(id, "compute", time.Microsecond)
				fork := r.Fork(id)
				r.Ready(fork)
				r.Published(fork)
				r.Acknowledge(fork, 0)
			}
		})
	}
	wg.Go(func() { r.Acknowledge(id, 0) })
	wg.Wait()
	if r.Counts().Acknowledged == 0 {
		t.Fatal("no observed result")
	}
}
