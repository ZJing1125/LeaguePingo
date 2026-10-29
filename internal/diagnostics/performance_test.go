package diagnostics

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadinessCountsOneResultAndSeparatesAbandonment(t *testing.T) {
	var r Readiness
	start := time.Now()
	r.Phase("ChampSelect", start)
	r.Phase("ChampSelect", start.Add(time.Second))
	r.First(start.Add(2*time.Second), start.Add(-time.Second))
	r.First(start.Add(4*time.Second), start)
	m := r.Snapshot()
	if m.Started != 1 || m.Completed != 1 || m.Pending || m.LastMS == nil || *m.LastMS != 2000 || m.PreexistingResults != 1 {
		t.Fatal(m)
	}
	r.Phase("Lobby", start.Add(5*time.Second))
	r.Phase("ChampSelect", start.Add(6*time.Second))
	r.Reset()
	m = r.Snapshot()
	if m.Abandoned != 1 || m.Completed != 1 || *m.LastMS != 2000 {
		t.Fatal("cancellation became a zero result", m)
	}
	r.Phase("ChampSelect", start.Add(8*time.Second))
	r.First(start.Add(8*time.Second), start.Add(8*time.Second))
	if *r.Snapshot().LastMS != 0 || r.Snapshot().PreexistingResults != 1 {
		t.Fatal("known zero lost")
	}
}
func TestCallbackCountersAreConcurrentAndContainDurationsOnly(t *testing.T) {
	var c Callback
	if c.Snapshot().MaxUS != nil || c.Snapshot().MeanUS != nil {
		t.Fatal("unobserved callback represented as measured zero")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				c.Record(250 * time.Microsecond)
			}
		}()
	}
	wg.Wait()
	m := c.Snapshot()
	if m.Calls != 8000 || m.TotalUS != 2000000 || m.MaxUS == nil || *m.MaxUS != 250 {
		t.Fatal(m)
	}
}
func rawResource(rootCPU, childCPU float64, childCreated uint64) resourceReading {
	root := processIdentity{pid: 1, created: 10}
	return resourceReading{root: root, processes: map[processIdentity]processCounter{root: {cpu: rootCPU, private: 100, working: 200}, {pid: 2, created: childCreated}: {cpu: childCPU, private: 50, working: 75}}}
}
func TestResourceCPUUsesDeltasAndCreationIdentity(t *testing.T) {
	r := newResources(nil, true, 4)
	now := time.Now()
	r.accept(rawResource(100, 200, 20), now)
	if r.Snapshot().LastCPUPercentMachine != nil || r.Snapshot().CPUIntervals != 0 {
		t.Fatal("first sample fabricated CPU")
	}
	r.accept(rawResource(101, 202, 20), now.Add(5*time.Second))
	m := r.Snapshot()
	if m.CPUIntervals != 1 || *m.LastCPUPercentMachine != 15 || *m.LastPrivateBytes != 150 || *m.LastWorkingSetBytes != 275 {
		t.Fatal(m)
	}
	r.accept(rawResource(102, 9000, 21), now.Add(10*time.Second))
	if *r.Snapshot().LastCPUPercentMachine != 5 {
		t.Fatal("reused child PID produced CPU spike")
	}
	r.accept(rawResource(102, 9000, 21), now.Add(15*time.Second))
	if r.Snapshot().LastCPUPercentMachine == nil || *r.Snapshot().LastCPUPercentMachine != 0 {
		t.Fatal("known idle interval lost")
	}
	partial := rawResource(102, 9000, 21)
	partial.partial = true
	r.accept(partial, now.Add(20*time.Second))
	if r.Snapshot().PartialSamples != 1 {
		t.Fatal("incomplete readable tree hidden")
	}
	r.SetActive(true)
	r.SetActive(false)
	r.SetActive(true)
	r.accept(rawResource(103, 9001, 21), now.Add(30*time.Second))
	if r.Snapshot().LastCPUPercentMachine != nil {
		t.Fatal("CPU interval crossed inactive phase")
	}
}
func TestResourceSamplerIdleCancellationAndLateResult(t *testing.T) {
	var calls atomic.Int32
	r := newResources(func() (resourceReading, error) { calls.Add(1); return rawResource(1, 2, 20), nil }, true, 4)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.Run(ctx); close(done) }()
	time.Sleep(20 * time.Millisecond)
	if calls.Load() != 0 {
		t.Fatal("idle sampler read OS counters")
	}
	r.SetActive(true)
	end := time.Now().Add(time.Second)
	for r.Snapshot().MemorySamples == 0 && time.Now().Before(end) {
		time.Sleep(time.Millisecond)
	}
	if calls.Load() != 1 {
		t.Fatal("activation did not take exactly one baseline", calls.Load())
	}
	time.Sleep(20 * time.Millisecond)
	if calls.Load() != 1 {
		t.Fatal("queued wake caused a second immediate sample")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sampler failed to exit")
	}
	if r.Snapshot().Active {
		t.Fatal("shutdown kept sampler active")
	}
	r.SetActive(true)
	if r.Snapshot().Active {
		t.Fatal("late snapshot reactivated stopped sampler")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	r = newResources(func() (resourceReading, error) { close(entered); <-release; return rawResource(1, 2, 20), nil }, true, 4)
	r.SetActive(true)
	done = make(chan struct{})
	go func() { r.sample(context.Background()); close(done) }()
	<-entered
	r.SetActive(false)
	r.SetActive(true)
	close(release)
	<-done
	if r.Snapshot().MemorySamples != 0 {
		t.Fatal("late sample crossed active epoch")
	}
	r.read = func() (resourceReading, error) { return resourceReading{}, errors.New("not retained in metrics") }
	r.sample(context.Background())
	if r.Snapshot().FailedSamples != 1 || r.Snapshot().LastPrivateBytes != nil {
		t.Fatal("failed sample fabricated memory")
	}
}
