package guides

import "context"

// A provider can publish usable public content while the rest is still loading.
type progressiveProvider interface {
	FetchProgressive(context.Context, Query, func(*Guide)) (*Guide, error)
}

func (s *Service) fetchGuide(ctx context.Context, token uint64, query Query) (*Guide, error) {
	provider, ok := s.provider.(progressiveProvider)
	if !ok {
		return s.provider.Fetch(ctx, query)
	}
	return provider.FetchProgressive(ctx, query, func(g *Guide) {
		s.mu.Lock()
		if token != s.generation || ctx.Err() != nil {
			s.mu.Unlock()
			return
		}
		s.acceptGuideLocked(query, cacheKey(query), g)
		s.updateFreshness()
		s.publish()
		s.mu.Unlock()
		s.persistGuide(ctx, token, cacheKey(query))
	})
}

func mergeGuideParts(previous, next *Guide) *Guide {
	if previous == nil || previous.Query != next.Query || previous.Version == "" || previous.Version != next.Version {
		return next
	}
	merged := *next
	if len(merged.Augments) == 0 && len(previous.Augments) > 0 {
		merged.Augments, merged.AugmentsOmitted = previous.Augments, previous.AugmentsOmitted
		merged.AugmentsFetchedAt = previous.AugmentsFetchedAt
	}
	if len(merged.Builds) == 0 && len(previous.Builds) > 0 {
		merged.Builds, merged.Skills, merged.Tips = previous.Builds, previous.Skills, previous.Tips
		merged.BuildsFetchedAt = previous.BuildsFetchedAt
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
