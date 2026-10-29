package diagnostics

import (
	"context"
	"sync"
	"time"
)

const resourceInterval = 5 * time.Second
const resourceProcessLimit = 256

type processIdentity struct {
	pid     uint32
	created uint64
}
type processCounter struct {
	cpu              float64
	private, working uint64
}
type resourceReading struct {
	root      processIdentity
	processes map[processIdentity]processCounter
	partial   bool
}
type ResourceMetric struct {
	Supported                bool       `json:"supported"`
	Active                   bool       `json:"active"`
	IntervalMS               int64      `json:"intervalMs"`
	LogicalProcessors        int        `json:"logicalProcessors"`
	MemorySamples            uint64     `json:"memorySamples"`
	CPUIntervals             uint64     `json:"cpuIntervals"`
	PartialSamples           uint64     `json:"partialSamples"`
	FailedSamples            uint64     `json:"failedSamples"`
	LastAt                   *time.Time `json:"lastAt"`
	LastProcesses            int        `json:"lastProcesses"`
	LastPartial              bool       `json:"lastPartial"`
	LastPrivateBytes         *uint64    `json:"lastPrivateBytes"`
	PeakPrivateBytes         *uint64    `json:"peakPrivateBytes"`
	LastWorkingSetBytes      *uint64    `json:"lastWorkingSetBytes"`
	PeakWorkingSetBytes      *uint64    `json:"peakWorkingSetBytes"`
	LastCPUPercentMachine    *float64   `json:"lastCpuPercentMachine"`
	AverageCPUPercentMachine *float64   `json:"averageCpuPercentMachine"`
	PeakCPUPercentMachine    *float64   `json:"peakCpuPercentMachine"`
	ObservedSeconds          float64    `json:"observedSeconds"`
	SamplingTotalMS          int64      `json:"samplingTotalMs"`
	SamplingMaxMS            int64      `json:"samplingMaxMs"`
}

// Resources samples only while the confirmed snapshot is InProgress. The raw
// process identities live only in the current CPU baseline, never in exports.
type Resources struct {
	mu         sync.Mutex
	wake       chan struct{}
	epoch      uint64
	metric     ResourceMetric
	read       func() (resourceReading, error)
	cores      int
	previous   resourceReading
	previousAt time.Time
	cpuSeconds float64
	stopped    bool
}

func NewResources() *Resources {
	return newResources(readResources, resourceSupported(), logicalProcessors())
}
func newResources(read func() (resourceReading, error), supported bool, cores int) *Resources {
	return &Resources{wake: make(chan struct{}, 1), read: read, cores: max(1, cores), metric: ResourceMetric{Supported: supported, IntervalMS: resourceInterval.Milliseconds(), LogicalProcessors: max(1, cores)}}
}
func (r *Resources) SetActive(active bool) {
	r.mu.Lock()
	if r.stopped || r.metric.Active == active {
		r.mu.Unlock()
		return
	}
	r.metric.Active = active
	r.epoch++
	r.previous = resourceReading{}
	r.previousAt = time.Time{}
	r.metric.LastCPUPercentMachine = nil
	r.mu.Unlock()
	select {
	case r.wake <- struct{}{}:
	default:
	}
}
func (r *Resources) Snapshot() ResourceMetric {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.metric
}
func (r *Resources) Run(ctx context.Context) {
	defer func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.stopped = true
		r.metric.Active = false
		r.epoch++
		r.previous = resourceReading{}
		r.previousAt = time.Time{}
		r.metric.LastCPUPercentMachine = nil
	}()
	for ctx.Err() == nil {
		// Consume a coalesced notification before observing the latest state.
		select {
		case <-r.wake:
		default:
		}
		r.mu.Lock()
		active := r.metric.Active && r.metric.Supported
		r.mu.Unlock()
		if !active {
			select {
			case <-ctx.Done():
				return
			case <-r.wake:
			}
			continue
		}
		r.sample(ctx)
		timer := time.NewTimer(resourceInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-r.wake:
		case <-timer.C:
		}
		timer.Stop()
	}
}
func (r *Resources) sample(ctx context.Context) {
	r.mu.Lock()
	epoch, active := r.epoch, r.metric.Active && r.metric.Supported
	r.mu.Unlock()
	if !active || ctx.Err() != nil {
		return
	}
	started := time.Now()
	reading, err := r.read() // Native OS counters are read outside all application locks.
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	if ctx.Err() != nil || epoch != r.epoch || !r.metric.Active {
		return
	}
	elapsed := now.Sub(started).Milliseconds()
	r.metric.SamplingTotalMS += elapsed
	r.metric.SamplingMaxMS = max(r.metric.SamplingMaxMS, elapsed)
	if err != nil || len(reading.processes) == 0 || len(reading.processes) > resourceProcessLimit {
		r.metric.FailedSamples++
		r.metric.LastCPUPercentMachine = nil
		r.previous = resourceReading{}
		r.previousAt = time.Time{}
		return
	}
	r.accept(reading, now)
}

// accept is called with r.mu held; split out so CPU arithmetic is deterministic in tests.
func (r *Resources) accept(current resourceReading, now time.Time) {
	var private, working uint64
	for _, p := range current.processes {
		private += p.private
		working += p.working
	}
	r.metric.MemorySamples++
	if current.partial {
		r.metric.PartialSamples++
	}
	r.metric.LastAt = &now
	r.metric.LastProcesses = len(current.processes)
	r.metric.LastPartial = current.partial
	r.metric.LastPrivateBytes = &private
	r.metric.LastWorkingSetBytes = &working
	if r.metric.PeakPrivateBytes == nil || private > *r.metric.PeakPrivateBytes {
		r.metric.PeakPrivateBytes = &private
	}
	if r.metric.PeakWorkingSetBytes == nil || working > *r.metric.PeakWorkingSetBytes {
		r.metric.PeakWorkingSetBytes = &working
	}
	r.metric.LastCPUPercentMachine = nil
	seconds := now.Sub(r.previousAt).Seconds()
	if !r.previousAt.IsZero() && seconds > 0 && current.root == r.previous.root {
		r.metric.LastPartial = r.metric.LastPartial || r.previous.partial
		var delta float64
		for id, p := range current.processes {
			if old, ok := r.previous.processes[id]; ok {
				delta += max(0, p.cpu-old.cpu)
			}
		}
		cpu := 100 * delta / seconds / float64(r.cores)
		r.cpuSeconds += delta
		r.metric.ObservedSeconds += seconds
		r.metric.CPUIntervals++
		r.metric.LastCPUPercentMachine = &cpu
		average := 100 * r.cpuSeconds / r.metric.ObservedSeconds / float64(r.cores)
		r.metric.AverageCPUPercentMachine = &average
		if r.metric.PeakCPUPercentMachine == nil || cpu > *r.metric.PeakCPUPercentMachine {
			r.metric.PeakCPUPercentMachine = &cpu
		}
	}
	r.previous = current
	r.previousAt = now
}
