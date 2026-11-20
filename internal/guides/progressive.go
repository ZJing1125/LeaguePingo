package guides

import (
	"context"
	"time"

	"leaguepingo/internal/diagnostics"
)

// A provider can publish usable public content while the rest is still loading.
type progressiveProvider interface {
	FetchProgressive(context.Context, Query, func(*Guide)) (*Guide, error)
}

func (s *Service) fetchGuide(ctx context.Context, token uint64, query Query) (*Guide, error) {
	id := diagnostics.Responses.Begin("guide")
	ctx = diagnostics.WithResponse(ctx, id)
	started := time.Now()
	lastSource := started
	publish := func(g *Guide) {
		copy := *g
		diagnostics.Responses.Stage(id, "source", time.Since(lastSource))
		lastSource = time.Now()
		copy.ResponseID = diagnostics.Responses.Fork(id)
		diagnostics.Responses.Ready(copy.ResponseID)
		s.mu.Lock()
		if token != s.generation || ctx.Err() != nil {
			s.mu.Unlock()
			diagnostics.Responses.Discard(copy.ResponseID)
			return
		}
		s.acceptGuideLocked(query, cacheKey(query), &copy)
		s.updateFreshness()
		s.publish()
		s.mu.Unlock()
		s.persistGuide(ctx, token, cacheKey(query))
	}
	var g *Guide
	var err error
	if provider, ok := s.provider.(progressiveProvider); ok {
		g, err = provider.FetchProgressive(ctx, query, publish)
	} else {
		g, err = s.provider.Fetch(ctx, query)
	}
	if err != nil {
		diagnostics.Responses.Discard(id)
		return g, err
	}
	diagnostics.Responses.Stage(id, "source", time.Since(lastSource))
	copy := *g
	copy.ResponseID = id
	diagnostics.Responses.Ready(id)
	return &copy, nil
}

func mergeGuideParts(previous, next *Guide) *Guide {
	if previous == nil || previous.Query != next.Query || previous.Version == "" || previous.Version != next.Version {
		return next
	}
	merged := *next
	normalizeGuideParts(&merged)
	if len(merged.Augments) == 0 && len(previous.Augments) > 0 {
		merged.Augments, merged.AugmentsOmitted = previous.Augments, previous.AugmentsOmitted
		merged.AugmentsFetchedAt = previous.AugmentsFetchedAt
		merged.AugmentsStatus = &GuidePart{State: "retained", Version: previous.Version, FetchedAt: previous.AugmentsFetchedAt}
	}
	if len(merged.Builds) == 0 && len(previous.Builds) > 0 {
		merged.Builds, merged.Skills, merged.Tips = previous.Builds, previous.Skills, previous.Tips
		merged.BuildsFetchedAt = previous.BuildsFetchedAt
		merged.BuildsStatus = &GuidePart{State: "retained", Version: previous.Version, FetchedAt: previous.BuildsFetchedAt}
	}
	return &merged
}

// Serialize optional writes after publishing, so disk latency cannot hold the UI
// state lock or overwrite a newer accepted version with an old callback.
func (s *Service) persistGuide(ctx context.Context, token uint64, key string) {
	s.cacheWriteMu.Lock()
	defer s.cacheWriteMu.Unlock()
	s.mu.Lock()
	g := s.cache[key]
	current := token == s.generation && ctx.Err() == nil
	s.mu.Unlock()
	if current && g != nil {
		s.save(key, g)
	}
}

// Old public caches keep their content, without inventing a per-part timestamp.
func normalizeGuideParts(g *Guide) {
	if g.Query.Mode != "aram-mayhem" && g.Query.Mode != "arena" {
		return
	}
	if g.AugmentsStatus == nil {
		state := "failed"
		if len(g.Augments) > 0 {
			state = "ready"
		}
		g.AugmentsStatus = &GuidePart{State: state, Version: g.Version, FetchedAt: g.AugmentsFetchedAt}
	}
	if g.BuildsStatus == nil {
		state := "failed"
		if len(g.Builds) > 0 {
			state = "ready"
		}
		g.BuildsStatus = &GuidePart{State: state, Version: g.Version, FetchedAt: g.BuildsFetchedAt}
	}
}
func (s *Service) retainFailedGuide(key string) {
	previous := s.cache[key]
	if previous == nil || previous.Query.Mode != "aram-mayhem" && previous.Query.Mode != "arena" {
		return
	}
	next := *previous
	part := func(has bool, at *time.Time) *GuidePart {
		state := "failed"
		if has {
			state = "retained"
		}
		return &GuidePart{State: state, Version: previous.Version, FetchedAt: at}
	}
	next.ResponseID = 0
	next.AugmentsStatus = part(len(previous.Augments) > 0, previous.AugmentsFetchedAt)
	next.BuildsStatus = part(len(previous.Builds) > 0, previous.BuildsFetchedAt)
	s.cache[key] = &next
	if next.Query.Opponent == 0 {
		s.state.Base = &next
	} else {
		s.state.Matchup = &next
	}
}
