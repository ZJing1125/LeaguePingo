package engine

import (
	"context"
	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"testing"
)

type queuePagingClient struct {
	*pagingClient
	enabled bool
	queueFn func(context.Context, string, int, int) (core.HistoryPage, error)
}

func (s *queuePagingClient) SupportsHistoryQueue(int) bool { return s.enabled }
func (s *queuePagingClient) HistoryPageForQueue(ctx context.Context, id string, q, offset int) (core.HistoryPage, error) {
	return s.queueFn(ctx, id, q, offset)
}
func (*queuePagingClient) VerifyHistoryQueue(context.Context, int) (core.HistoryQueueVerification, error) {
	return core.HistoryQueueVerification{}, nil
}

func TestQueueBrowseFallbackBudgetAndIndependentCursors(t *testing.T) {
	for _, budget := range []int{1, 2} {
		t.Run(string(rune('0'+budget)), func(t *testing.T) {
			mixed, filtered := 0, 0
			s := &queuePagingClient{pagingClient: &pagingClient{fakeClient: &fakeClient{}}, enabled: true}
			e := New(nil, nil)
			e.client = s
			e.snapshot.Participants = fullTestRoster()[:1]
			s.queueFn = func(ctx context.Context, id string, q, offset int) (core.HistoryPage, error) {
				filtered++
				if q != 2400 || offset != 0 {
					t.Fatal("wrong filtered cursor")
				}
				s.enabled = false
				return core.HistoryPage{}, apperrors.New(apperrors.Unsupported)
			}
			s.pageFn = func(ctx context.Context, id string, offset int) (core.HistoryPage, error) {
				mixed++
				if offset != 60 {
					t.Fatal("fallback used filtered cursor")
				}
				return core.HistoryPage{Offset: 60, NextOffset: 61, Matches: []core.Match{{GameID: 42}}}, nil
			}
			page, err := e.HistoryPageForQueue(context.Background(), "player-0", 0, core.HistoryPageRequest{QueueID: 2400, MixedOffset: 60, RemainingPages: budget})
			if filtered != 1 || mixed != budget-1 {
				t.Fatal("exceeded fallback budget")
			}
			if budget == 1 && err == nil || budget == 2 && (err != nil || page.Requests != 2 || page.QueueID != 0) {
				t.Fatalf("wrong fallback: %+v %v", page, err)
			}
		})
	}
}
func TestQueueBrowseRejectsLateConnectionAndUsesVerifiedQueue(t *testing.T) {
	s := &queuePagingClient{pagingClient: &pagingClient{fakeClient: &fakeClient{}}, enabled: true}
	e := New(nil, nil)
	e.client = s
	e.snapshot.Participants = fullTestRoster()[:1]
	s.queueFn = func(context.Context, string, int, int) (core.HistoryPage, error) {
		e.mu.Lock()
		e.invalidateConnectionLocked()
		e.mu.Unlock()
		return core.HistoryPage{QueueID: 2400}, nil
	}
	if _, err := e.HistoryPageForQueue(context.Background(), "player-0", 0, core.HistoryPageRequest{QueueID: 2400, RemainingPages: 1}); apperrors.CodeOf(err) != apperrors.StaleData {
		t.Fatal("stale queue page accepted")
	}
}

func TestQueueBrowseCanStartAfterMixedLimitWithoutRefetchingExhaustedPage(t *testing.T) {
	s := &queuePagingClient{pagingClient: &pagingClient{fakeClient: &fakeClient{}}, enabled: true}
	e := New(nil, nil)
	e.client = s
	e.snapshot.Participants = fullTestRoster()[:1]
	s.queueFn = func(context.Context, string, int, int) (core.HistoryPage, error) {
		return core.HistoryPage{QueueID: 2400, Offset: 0, NextOffset: 1, Matches: []core.Match{{GameID: 99, QueueID: 2400}}}, nil
	}
	s.pageFn = func(context.Context, string, int) (core.HistoryPage, error) {
		t.Fatal("exhausted mixed history requested again")
		return core.HistoryPage{}, nil
	}
	request := core.HistoryPageRequest{QueueID: 2400, MixedOffset: 1000, RemainingPages: 1}
	page, err := e.HistoryPageForQueue(context.Background(), "player-0", 0, request)
	if err != nil || page.QueueID != 2400 || len(page.Matches) != 1 {
		t.Fatal("filtered cursor incorrectly shared mixed limit", err)
	}
	request.MixedOnly = true
	page, err = e.HistoryPageForQueue(context.Background(), "player-0", 0, request)
	if err != nil || page.HasMore || page.LimitReason == "" {
		t.Fatal("missing bounded mixed fallback")
	}
}
