package engine

import (
	"context"
	"testing"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

func TestExpiredLookupReturnsBeforeBackgroundReadCompletes(t *testing.T) {
	for _, kind := range []string{"id", "player", "home"} {
		t.Run(kind, func(t *testing.T) {
			e, f, p := recoveryEngine(t)
			e.accountIdentity = p.PUUID
			p.DisplayName = "Synthetic#TEST"
			e.cachePlayerLocked(p)
			entered, release := make(chan struct{}), make(chan struct{})
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			e.initialMatch.setParent(parent)
			f.historyFn = func(ctx context.Context, _ string) (core.HistoryResult, error) {
				close(entered)
				select {
				case <-release:
					return core.HistoryResult{Matches: []core.Match{{GameID: 11}}}, nil
				case <-ctx.Done():
					return core.HistoryResult{}, ctx.Err()
				}
			}
			result := make(chan core.Participant, 1)
			go func() {
				var got core.Participant
				var err error
				switch kind {
				case "id":
					got, err = e.Lookup(context.Background(), p.DisplayName)
				case "player":
					got, err = e.LookupPlayer(context.Background(), p.Key)
				default:
					got, err = e.CurrentPlayer(context.Background(), p.ConnectionID)
				}
				if err != nil {
					t.Errorf("cache lookup failed: %v", err)
				}
				result <- got
			}()
			select {
			case got := <-result:
				if !got.Cached || !got.Stale || len(got.History) != 1 || got.History[0].GameID != 10 || !got.HistoryMeta.FetchedAt.Equal(*p.HistoryMeta.FetchedAt) {
					t.Fatal("expired snapshot was not returned with its original timestamp")
				}
			case <-time.After(time.Second):
				t.Fatal("cached lookup waited for the blocked service")
			}
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("background read did not start")
			}
			close(release)
			select {
			case updated := <-e.PlayerUpdates():
				if updated.History[0].GameID != 11 || updated.Stale || updated.Profile == nil || !updated.HistoryMeta.FetchedAt.After(*p.HistoryMeta.FetchedAt) {
					t.Fatal("background result was not published with independent profile retained")
				}
			case <-time.After(time.Second):
				t.Fatal("background update was not published")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.calls) != 1 || f.calls[0] != "history" {
				t.Fatal("cached identity was queried again")
			}
		})
	}
}

func TestExpiredLookupFailurePreservesSnapshotAndAge(t *testing.T) {
	e, f, p := recoveryEngine(t)
	f.historyFn = func(context.Context, string) (core.HistoryResult, error) {
		return core.HistoryResult{Meta: core.HistoryMeta{Service: p.HistoryMeta.Service}}, apperrors.HTTP(apperrors.DataService, 503)
	}
	got, err := e.LookupPlayer(context.Background(), p.Key)
	if err != nil || got.History[0].GameID != 10 {
		t.Fatal("failed service blocked retained snapshot")
	}
	select {
	case updated := <-e.PlayerUpdates():
		if updated.History[0].GameID != 10 || !updated.HistoryMeta.FetchedAt.Equal(*p.HistoryMeta.FetchedAt) || updated.HistoryMeta.CheckedAt == nil || updated.HistoryMeta.Service == nil {
			t.Fatal("failed refresh changed the data or its original age")
		}
	case <-time.After(time.Second):
		t.Fatal("failed refresh did not publish retained data")
	}
}
