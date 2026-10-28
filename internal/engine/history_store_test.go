package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"leaguepingo/internal/core"
)

func TestHistoryStorePersistsDeidentifiedPages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history-cache.json")
	store, err := openHistoryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	store.put("real-player-puuid", core.HistoryPage{
		Matches: []core.Match{{GameID: 42, QueueID: 420, Roster: []core.MatchPeer{{DisplayName: "PrivateName#TAG"}}}},
		Meta:    core.HistoryMeta{FetchedAt: &now, Returned: 1, Parsed: 1},
		Offset:  0, NextOffset: 1,
	})
	store.close()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "real-player-puuid") || strings.Contains(string(raw), "PrivateName") {
		t.Fatal("persistent cache retained a player identifier or roster name")
	}
	reopened, err := openHistoryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.close()
	page, ok := reopened.get("real-player-puuid", 0, 0)
	if !ok || len(page.Matches) != 1 || page.Matches[0].GameID != 42 {
		t.Fatalf("stored page was not restored: %+v", page)
	}
	if len(page.Matches[0].Roster) != 0 {
		t.Fatal("restored page contained a roster")
	}
}

func TestHistoryStoreRejectsIncompatibleFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history-cache.json")
	if err := os.WriteFile(path, []byte(`{"schema":99,"entries":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := openHistoryStore(path); err == nil {
		t.Fatal("incompatible persistent cache was accepted")
	}
}

func TestHistoryStoreFreshnessIsPerPage(t *testing.T) {
	store, err := openHistoryStore(filepath.Join(t.TempDir(), "history-cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	old := time.Now().UTC().Add(-10 * time.Minute)
	store.put("player", core.HistoryPage{Offset: 0, NextOffset: 20, Meta: core.HistoryMeta{FetchedAt: &old}})
	fresh := time.Now().UTC()
	store.put("player", core.HistoryPage{Offset: 20, NextOffset: 40, Meta: core.HistoryMeta{FetchedAt: &fresh}})
	if _, ok := store.get("player", 0, 5*time.Minute); ok {
		t.Fatal("writing a fresh later page made the stale head page appear fresh")
	}
	if _, ok := store.get("player", 20, 5*time.Minute); !ok {
		t.Fatal("fresh page was not returned")
	}
}

func TestHistoryStoreNewHeadInvalidatesOffsetPages(t *testing.T) {
	store, err := openHistoryStore(filepath.Join(t.TempDir(), "history-cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	now := time.Now().UTC()
	store.put("player", core.HistoryPage{Offset: 0, NextOffset: 20, Matches: []core.Match{{GameID: 10}}, Meta: core.HistoryMeta{FetchedAt: &now}})
	store.put("player", core.HistoryPage{Offset: 20, NextOffset: 40, Matches: []core.Match{{GameID: 9}}, Meta: core.HistoryMeta{FetchedAt: &now}})
	store.put("player", core.HistoryPage{Offset: 0, NextOffset: 20, Matches: []core.Match{{GameID: 11}}, Meta: core.HistoryMeta{FetchedAt: &now}})
	if _, ok := store.get("player", 20, 0); ok {
		t.Fatal("new history head retained offset pages from the old snapshot")
	}
}

func TestEngineHydratesOnlyAfterVerifiedIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history-cache.json")
	seed, err := openHistoryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	seed.put("verified-puuid", core.HistoryPage{Offset: 0, NextOffset: 1, Matches: []core.Match{{GameID: 77, QueueID: 420}}, Meta: core.HistoryMeta{FetchedAt: &now, Returned: 1, Parsed: 1}})
	seed.close()
	e, err := NewWithHistoryCache(func() (Client, error) { return nil, nil }, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	defer e.historyStore.close()
	unknown := core.Participant{PUUID: "unavailable", IdentityState: core.IdentityUnavailable}
	if e.hydrateHistory(&unknown) {
		t.Fatal("unverified identity hydrated a persisted player snapshot")
	}
	verified := core.Participant{Key: "current-key", PUUID: "verified-puuid", IdentityState: core.IdentityKnown}
	if !e.hydrateHistory(&verified) || len(verified.History) != 1 || verified.History[0].GameID != 77 || !verified.Cached {
		t.Fatalf("verified identity did not hydrate its snapshot: %+v", verified)
	}
}
