package main

import (
	"context"
	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
	"testing"
)

func TestHistoryResponseTraceNeverMutatesCachedValue(t *testing.T) {
	cached := core.HistoryPage{Matches: []core.Match{{GameID: 42}}}
	got, err := traceHistory(context.Background(), func(context.Context) (core.HistoryPage, error) { return cached, nil })
	if err != nil || got.ResponseID == 0 || cached.ResponseID != 0 || got.Matches[0].GameID != 42 {
		t.Fatal("trace lost data or modified cached page")
	}
	defer diagnostics.Responses.Discard(got.ResponseID)
	player := core.Participant{Key: "synthetic", History: cached.Matches}
	visible, err := tracePlayer(context.Background(), func(context.Context) (core.Participant, error) { return player, nil })
	if err != nil || visible.ResponseID == 0 || player.ResponseID != 0 {
		t.Fatal("trace entered player cache")
	}
	defer diagnostics.Responses.Discard(visible.ResponseID)
}
func TestHistoryTraceRefusesLateSuccessAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	got, err := traceHistory(ctx, func(context.Context) (core.HistoryPage, error) {
		cancel()
		return core.HistoryPage{Matches: []core.Match{{GameID: 42}}}, nil
	})
	if err == nil || got.ResponseID != 0 || len(got.Matches) != 0 {
		t.Fatal("canceled result became visible measurement")
	}
}
