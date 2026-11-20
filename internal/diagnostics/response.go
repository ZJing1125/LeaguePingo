package diagnostics

import (
	"context"
	"sync"
	"time"
)

type responseKey struct{}
type responseSpan struct {
	kind                      string
	started, ready, published time.Time
	stages                    map[string]time.Duration
}
type ResponseCounts struct {
	Started      uint64 `json:"started"`
	Acknowledged uint64 `json:"acknowledged"`
	Unobserved   uint64 `json:"unobserved"`
}
type ResponseRegistry struct {
	mu     sync.Mutex
	next   uint64
	spans  map[uint64]*responseSpan
	counts ResponseCounts
}

var Responses = &ResponseRegistry{}

func (r *ResponseRegistry) Begin(kind string) uint64 {
	if kind != "history" && kind != "scout" && kind != "guide" {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.spans == nil {
		r.spans = map[uint64]*responseSpan{}
	}
	r.expireLocked()
	if len(r.spans) >= 128 {
		var oldest uint64
		for id, span := range r.spans {
			if oldest == 0 || span.started.Before(r.spans[oldest].started) {
				oldest = id
			}
		}
		delete(r.spans, oldest)
		r.counts.Unobserved++
	}
	r.next++
	r.spans[r.next] = &responseSpan{kind: kind, started: time.Now(), stages: map[string]time.Duration{}}
	r.counts.Started++
	return r.next
}
func WithResponse(ctx context.Context, id uint64) context.Context {
	return context.WithValue(ctx, responseKey{}, id)
}
func ResponseStage(ctx context.Context, stage string, elapsed time.Duration) {
	id, _ := ctx.Value(responseKey{}).(uint64)
	Responses.Stage(id, stage, elapsed)
}
func (r *ResponseRegistry) Stage(id uint64, stage string, elapsed time.Duration) {
	switch stage {
	case "queue", "source", "parse", "compute":
	default:
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if span := r.spans[id]; span != nil {
		span.stages[stage] += elapsed
	}
}
func (r *ResponseRegistry) Ready(id uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if span := r.spans[id]; span != nil {
		span.ready = time.Now()
	}
}
func (r *ResponseRegistry) Published(id uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if span := r.spans[id]; span != nil && span.published.IsZero() {
		span.published = time.Now()
	}
}
func (r *ResponseRegistry) Fork(id uint64) uint64 {
	r.mu.Lock()
	span := r.spans[id]
	if span == nil {
		r.mu.Unlock()
		return 0
	}
	kind, started := span.kind, span.started
	stages := map[string]time.Duration{}
	for k, v := range span.stages {
		stages[k] = v
	}
	r.mu.Unlock()
	fork := r.Begin(kind)
	r.mu.Lock()
	if next := r.spans[fork]; next != nil {
		next.started, next.stages = started, stages
	}
	r.mu.Unlock()
	return fork
}
func (r *ResponseRegistry) Discard(id uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.spans[id] != nil {
		delete(r.spans, id)
		r.counts.Unobserved++
	}
}
func (r *ResponseRegistry) Acknowledge(id uint64, ui time.Duration) {
	r.mu.Lock()
	span := r.spans[id]
	if span == nil || span.ready.IsZero() || span.published.IsZero() || time.Since(span.started) > 3*time.Minute {
		r.mu.Unlock()
		return
	}
	copy := *span
	copy.stages = make(map[string]time.Duration, len(span.stages))
	for key, value := range span.stages {
		copy.stages[key] = value
	}
	span = &copy
	delete(r.spans, id)
	r.counts.Acknowledged++
	r.mu.Unlock()
	for stage, elapsed := range span.stages {
		Default.Record("/response/"+span.kind+"/"+stage, elapsed, 200, false)
	}
	Default.Record("/response/"+span.kind+"/ready-to-publish", span.published.Sub(span.ready), 200, false)
	Default.Record("/response/"+span.kind+"/publish-to-frame-ack", time.Since(span.published), 200, false)
	Default.Record("/response/"+span.kind+"/end-to-end-ack", time.Since(span.started), 200, false)
	Default.Record("/response/"+span.kind+"/ui", ui, 200, false)
}
func (r *ResponseRegistry) expireLocked() {
	for id, span := range r.spans {
		if time.Since(span.started) > 3*time.Minute {
			delete(r.spans, id)
			r.counts.Unobserved++
		}
	}
}
func (r *ResponseRegistry) Counts() ResponseCounts {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked()
	return r.counts
}
