package source

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

func (s *TencentLCU) SupportsHistoryQueue(queue int) bool {
	s.sgpMu.Lock()
	defer s.sgpMu.Unlock()
	return s.sgp != nil && s.verifiedQueues[s.sgp.platform][queue]
}
func (s *TencentLCU) HistoryPageForQueue(ctx context.Context, id string, queue, offset int) (core.HistoryPage, error) {
	if !s.SupportsHistoryQueue(queue) {
		return core.HistoryPage{}, apperrors.New(apperrors.Unsupported)
	}
	if !core.IsKnownPUUID(id) || offset < 0 || offset > 980 {
		return core.HistoryPage{}, apperrors.New(apperrors.InvalidPage)
	}
	s.sgpMu.Lock()
	epoch := s.sgpEpoch
	s.sgpMu.Unlock()
	key := fmt.Sprintf("%d:%s:q_%d:%d", epoch, id, queue, offset)
	return s.pageReads.read(ctx, key, func(request context.Context) (core.HistoryPage, error) {
		s.sgpMu.Lock()
		current := s.sgpEpoch == epoch
		s.sgpMu.Unlock()
		if !current {
			return core.HistoryPage{}, apperrors.New(apperrors.StaleData)
		}
		if !s.SupportsHistoryQueue(queue) {
			return core.HistoryPage{}, apperrors.New(apperrors.Unsupported)
		}
		return s.loadSGPQueuePage(request, id, offset, queue, true)
	})
}

// Runs under sgpMu. The tag, offset and cached head belong to this queue only.
func validQueuePage(r *sgpHistory, path string, page core.HistoryPage, queue int) bool {
	if page.Meta.Excluded > 0 || page.Meta.Returned != len(page.Matches) || page.Meta.Returned < 0 || page.Meta.Returned > 20 || page.NextOffset != page.Offset+page.Meta.Returned {
		return false
	}
	seen := map[int64]bool{}
	for _, m := range page.Matches {
		if m.GameID <= 0 || m.QueueID != queue || seen[m.GameID] {
			return false
		}
		seen[m.GameID] = true
	}
	if page.Offset == 0 {
		return true
	}
	prefix := strings.SplitN(path, "?", 2)[0] + "?"
	tag := "&tag=" + url.QueryEscape(fmt.Sprintf("q_%d", queue))
	for key, element := range r.pages.items {
		if key == path || !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, tag) {
			continue
		}
		previous := element.Value.(cachedSGPPage).page
		if previous.Offset >= page.Offset {
			continue
		}
		for _, m := range previous.Matches {
			if seen[m.GameID] {
				return false
			}
		}
	}
	return true
}

func (s *TencentLCU) VerifyHistoryQueue(parent context.Context, queue int) (core.HistoryQueueVerification, error) {
	result := core.HistoryQueueVerification{QueueID: queue, State: "insufficient"}
	if queue != 2400 {
		return result, apperrors.New(apperrors.Unsupported)
	}
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()
	self, err := s.AccountIdentity(ctx)
	if err != nil {
		return result, err
	}
	s.sgpMu.Lock()
	epoch := s.sgpEpoch
	if s.sgp != nil {
		delete(s.verifiedQueues[s.sgp.platform], queue)
		s.sgp.pages.removePrefix("/match-history-query/v1/products/lol/player/" + url.PathEscape(self) + "/SUMMARY?")
	}
	s.sgpMu.Unlock()
	platform := ""
	read := func(offset, filter int) (core.HistoryPage, error) {
		if err := s.queueAuditGuard(ctx, self, epoch); err != nil {
			return core.HistoryPage{}, err
		}
		page, err := s.loadSGPQueuePage(ctx, self, offset, filter, false)
		if err != nil {
			return page, err
		}
		if page.Meta.Excluded > 0 || page.Meta.Returned != len(page.Matches) {
			return core.HistoryPage{}, apperrors.New(apperrors.InvalidResponse)
		}
		s.sgpMu.Lock()
		currentPlatform := s.sgp.platform
		s.sgpMu.Unlock()
		if platform != "" && currentPlatform != platform {
			return core.HistoryPage{}, apperrors.New(apperrors.StaleData)
		}
		platform = currentPlatform
		return page, s.queueAuditGuard(ctx, self, epoch)
	}
	var mixed, tagged []core.Match
	for offset := 0; offset < 60; offset += 20 {
		page, e := read(offset, 0)
		if e != nil {
			return result, e
		}
		mixed = append(mixed, page.Matches...)
		if !page.HasMore {
			break
		}
	}
	for offset := 0; offset < 40; offset += 20 {
		page, e := read(offset, queue)
		if e != nil {
			return result, e
		}
		tagged = append(tagged, page.Matches...)
		if !page.HasMore {
			break
		}
	}
	head, err := read(0, 0)
	if err != nil {
		return result, err
	}
	result.Mixed, result.Filtered = len(mixed), len(tagged)
	result.State, result.Compared = compareQueueHistory(mixed, tagged, head.Matches, queue)
	if result.State != "verified" {
		return result, nil
	}
	s.sgpMu.Lock()
	defer s.sgpMu.Unlock()
	if s.sgpEpoch != epoch || ctx.Err() != nil || s.sgp == nil || s.sgp.platform != platform {
		result.State = "changed"
		return result, apperrors.New(apperrors.StaleData)
	}
	if s.verifiedQueues == nil {
		s.verifiedQueues = map[string]map[int]bool{}
	}
	if s.verifiedQueues[s.sgp.platform] == nil {
		s.verifiedQueues[s.sgp.platform] = map[int]bool{}
	}
	s.verifiedQueues[s.sgp.platform][queue] = true
	return result, nil
}

func (s *TencentLCU) queueAuditGuard(ctx context.Context, self string, epoch uint64) error {
	phase, err := s.Phase(ctx)
	if err != nil {
		return err
	}
	switch phase {
	case "None", "Lobby", "EndOfGame", "PreEndOfGame", "WaitingForStats":
	default:
		return apperrors.New(apperrors.QueryBusy)
	}
	current, err := s.AccountIdentity(ctx)
	if err != nil {
		return err
	}
	s.sgpMu.Lock()
	same := s.sgpEpoch == epoch
	s.sgpMu.Unlock()
	if current != self || !same {
		return apperrors.New(apperrors.StaleData)
	}
	return nil
}

func compareQueueHistory(mixed, tagged, head []core.Match, queue int) (string, int) {
	if len(mixed) == 0 || len(head) == 0 || mixed[0].GameID != head[0].GameID {
		return "changed", 0
	}
	var expected []int64
	mixedSeen := map[int64]bool{}
	for _, m := range mixed {
		if m.GameID <= 0 || mixedSeen[m.GameID] {
			return "mismatch", 0
		}
		mixedSeen[m.GameID] = true
		if m.QueueID == queue {
			expected = append(expected, m.GameID)
		}
	}
	if len(expected) < 21 {
		return "insufficient", 0
	}
	n := min(len(expected), 40)
	if len(tagged) < n {
		return "mismatch", 0
	}
	seen := map[int64]bool{}
	for i, m := range tagged {
		if m.QueueID != queue || seen[m.GameID] || i < n && m.GameID != expected[i] {
			return "mismatch", 0
		}
		seen[m.GameID] = true
	}
	return "verified", n
}

func (s *TencentLCU) queueContractFailure(session sgpSession, queue int, original error) error {
	if queue == 0 {
		return original
	}
	s.sgpMu.Lock()
	defer s.sgpMu.Unlock()
	if !s.sessionValidLocked(session) {
		return apperrors.New(apperrors.StaleData)
	}
	delete(s.verifiedQueues[session.platform], queue)
	return apperrors.New(apperrors.Unsupported)
}
