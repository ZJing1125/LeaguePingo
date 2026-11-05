package engine

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func resetDiscoveryForTest(e *Engine) {
	e.discovery.mu.Lock()
	e.discovery.lastLookup = time.Time{}
	e.discovery.mu.Unlock()
}

func TestClientDiscoveryExactCooldownAndConcurrentClaims(t *testing.T) {
	d := newClientDiscovery(nil)
	now := time.Now()
	var claimed atomic.Int32
	claim := func(at time.Time) {
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if d.claim(at) {
					claimed.Add(1)
				}
			}()
		}
		wg.Wait()
	}
	claim(now)
	if claimed.Load() != 1 || d.claim(now.Add(10*time.Second-time.Nanosecond)) {
		t.Fatal("discovery bypassed its ten-second cooldown")
	}
	claim(now.Add(10 * time.Second))
	if claimed.Load() != 2 {
		t.Fatal("cooldown boundary did not admit exactly one new discovery")
	}
}

func TestClientDiscoveryCallbackHoldsNeitherStateNorEngineLock(t *testing.T) {
	var e *Engine
	f := &fakeClient{}
	e = New(func() (Client, error) {
		if !e.mu.TryLock() {
			t.Fatal("discovery callback held Engine lock")
		}
		e.mu.Unlock()
		if !e.discovery.mu.TryLock() {
			t.Fatal("discovery callback held component lock")
		}
		e.discovery.mu.Unlock()
		return f, nil
	}, nil)
	e.poll(t.Context())
	if e.client != f {
		t.Fatal("discovered source was not installed")
	}
}
