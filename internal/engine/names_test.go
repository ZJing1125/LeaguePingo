package engine

import (
	"context"
	"leaguepingo/internal/core"
	"testing"
	"time"
)

func TestCachedHistoryStillResolvesMissingNameAndSharesIt(t *testing.T) {
	e, c, p := profileEnsureFixture()
	p.DisplayName = "玩家名称暂不可用"
	seedPlayerForTest(e, p.Key, p)
	e.snapshot.Participants[0] = p
	got, err := e.LookupPlayer(context.Background(), p.Key)
	if err != nil || got.DisplayName != "Alice#TEST" || cachedPlayerForTest(e, p.Key).DisplayName != "Alice#TEST" || e.Snapshot().Participants[0].DisplayName != "Alice#TEST" {
		t.Fatal("cached history bypassed missing name resolution", err)
	}
	for _, call := range c.calls {
		if call == "history" || call == "find" {
			t.Fatal("name repair refetched history or identity")
		}
	}
}

func TestQueuedNameRepairRejectsChangedConnection(t *testing.T) {
	e, c, p := profileEnsureFixture()
	p.DisplayName = "玩家名称暂不可用"
	seedPlayerForTest(e, p.Key, p)
	e.snapshot.Participants[0] = p
	release, err := e.scheduler.acquire(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan core.Participant, 1)
	go func() { got, _ := e.LookupPlayer(context.Background(), p.Key); done <- got }()
	deadline := time.Now().Add(time.Second)
	waiting := false
	for time.Now().Before(deadline) {
		e.scheduler.mu.Lock()
		waiting = e.scheduler.manual > 0
		e.scheduler.mu.Unlock()
		if waiting {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !waiting {
		release()
		t.Fatal("name request did not reach scheduler")
	}
	e.mu.Lock()
	e.invalidateConnectionLocked()
	e.mu.Unlock()
	release()
	select {
	case got := <-done:
		if !got.Stale || len(c.calls) != 0 || cacheCountsForTest(e).players != 0 {
			t.Fatal("old queued name query crossed connection boundary")
		}
	case <-time.After(time.Second):
		t.Fatal("queued name query did not finish")
	}
}

func TestResolvedIDDoesNotLoseNameWhenHistoryCacheHasPlaceholder(t *testing.T) {
	e, c, p := profileEnsureFixture()
	p.DisplayName = "玩家名称暂不可用"
	seedPlayerForTest(e, p.Key, p)
	e.snapshot.Participants[0] = p
	e.players.mu.Lock()
	e.players.aliases = map[string]string{}
	e.players.mu.Unlock()
	got, err := e.Lookup(context.Background(), "Manual#TEST")
	if err != nil || got.DisplayName != "Manual#TEST" || cachedPlayerForTest(e, p.Key).DisplayName != "Manual#TEST" || e.Snapshot().Participants[0].DisplayName != "Manual#TEST" {
		t.Fatal("freshly verified ID was replaced by cached placeholder", err)
	}
	for _, call := range c.calls {
		if call == "history" || call == "name" {
			t.Fatal("verified ID triggered unnecessary history or name request")
		}
	}
}

func TestHistoryRefreshRepairsMissingNameWithoutOptionalProfileRequests(t *testing.T) {
	e, c, p := profileEnsureFixture()
	p.DisplayName = "玩家名称暂不可用"
	seedPlayerForTest(e, p.Key, p)
	e.snapshot.Participants[0] = p
	got, err := e.RefreshHistory(context.Background(), p.Key, p.ConnectionID)
	if err != nil || got.DisplayName != "Alice#TEST" {
		t.Fatal("history refresh left missing name", err)
	}
	for _, call := range c.calls {
		if call != "name" && call != "history" {
			t.Fatal("name repair requested unrelated data")
		}
	}
}
