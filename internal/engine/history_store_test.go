package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
)

func TestHistoryStorePersistsDeidentifiedPages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history-cache.json")
	store, err := openHistoryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	store.put("real-player-puuid", core.HistoryPage{
		Matches: []core.Match{{GameID: 42, QueueID: 420, Roster: []core.MatchPeer{{ParticipantID: 1, TeamID: 100, ChampionID: 23, DisplayName: "PrivateName#TAG", Target: true}}}},
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
	var stored historyStoreFile
	if json.Unmarshal(raw, &stored) != nil {
		t.Fatal("persistent cache is not valid JSON")
	}
	legacy := sha256.Sum256([]byte("real-player-puuid"))
	if _, ok := stored.Entries[hex.EncodeToString(legacy[:])]; ok {
		t.Fatal("persistent cache still uses an unkeyed identity hash")
	}
	if _, ok := stored.Entries[store.keyFor("real-player-puuid")]; !ok {
		t.Fatal("persistent cache did not use the local keyed identity")
	}
	secret, err := os.ReadFile(path + ".key")
	if err != nil || len(secret) != historyStoreSecretBytes {
		t.Fatal("local cache key was not persisted safely")
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
	if len(page.Matches[0].Roster) != 1 || page.Matches[0].Roster[0].ChampionID != 23 || page.Matches[0].Roster[0].DisplayName != "" {
		t.Fatal("restored page lost its anonymous composition or retained a name")
	}
}

func TestHistoryStoreRejectsIncompatibleFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history-cache.json")
	if err := os.WriteFile(path, []byte(`{"schema":99,"entries":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	e, err := NewWithHistoryCache(func() (Client, error) { return nil, nil }, nil, path)
	if err == nil || e.historyStore == nil {
		t.Fatal("incompatible persistent cache was not reported and rebuilt")
	}
	e.historyStore.close()
	raw, readErr := os.ReadFile(path)
	var rebuilt historyStoreFile
	if readErr != nil || json.Unmarshal(raw, &rebuilt) != nil || rebuilt.Schema != historyStoreSchema || len(rebuilt.Entries) != 0 {
		t.Fatalf("incompatible persistent cache was not replaced safely: %s", raw)
	}
}

func TestHistoryPageRetainsOnlyDerivedArenaResultAndCopiesIt(t *testing.T) {
	n := 8
	result := &core.ArenaResult{Placement: 2, TeamCount: &n}
	page := sanitizeHistoryPage(core.HistoryPage{Matches: []core.Match{{GameID: 1, QueueID: 1700, Arena: result}}})
	result.Placement, n = 5, 4
	if page.Matches[0].Arena.Placement != 2 || *page.Matches[0].Arena.TeamCount != 8 {
		t.Fatal("sanitized cache aliases mutable Arena data")
	}
	raw, _ := json.Marshal(page)
	var restored core.HistoryPage
	if json.Unmarshal(raw, &restored) != nil || restored.Matches[0].Arena.Placement != 2 {
		t.Fatal("derived result lost on cache roundtrip")
	}
}

func TestHistoryStoreMigratesSchemaOneWithoutServingIncompleteRosters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history-cache.json")
	secret, _, err := loadHistoryStoreSecret(path + ".key")
	if err != nil {
		t.Fatal(err)
	}
	key := historyStoreKey(secret, "player")
	old := `{"schema":1,"entries":{"` + key + `":{"updatedAt":"2026-10-28T00:00:00Z","pages":{"0":{"matches":[{"gameId":42,"queueId":420}],"meta":{"returned":1,"parsed":1},"offset":0,"nextOffset":1}}}}}`
	if err := os.WriteFile(path, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := openHistoryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.get("player", 0, 0); ok {
		t.Fatal("schema-one page without compositions was served after upgrade")
	}
	store.close()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file historyStoreFile
	if json.Unmarshal(raw, &file) != nil || file.Schema != historyStoreSchema || len(file.Entries) != 0 {
		t.Fatalf("schema-one cache was not replaced safely: %s", raw)
	}
}

func TestHistoryStoreMigratesUnkeyedSchemaTwo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history-cache.json")
	legacy := sha256.Sum256([]byte("player"))
	old := `{"schema":2,"entries":{"` + hex.EncodeToString(legacy[:]) + `":{"updatedAt":"2026-10-28T00:00:00Z","pages":{"0":{"matches":[{"gameId":42,"queueId":420}],"meta":{"returned":1,"parsed":1},"offset":0,"nextOffset":1}}}}}`
	if err := os.WriteFile(path, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := openHistoryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.get("player", 0, 0); ok {
		t.Fatal("unkeyed schema-two identity was served after upgrade")
	}
	store.close()
	raw, _ := os.ReadFile(path)
	var file historyStoreFile
	if json.Unmarshal(raw, &file) != nil || file.Schema != historyStoreSchema || len(file.Entries) != 0 {
		t.Fatalf("schema-two cache was not replaced safely: %s", raw)
	}
}

func TestHistoryStoreMissingLocalKeyRebuildsExistingCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history-cache.json")
	store, err := openHistoryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	store.put("player", core.HistoryPage{Offset: 0, NextOffset: 1, Matches: []core.Match{{GameID: 42}}, Meta: core.HistoryMeta{FetchedAt: &now, Returned: 1, Parsed: 1}})
	store.close()
	if err := os.Remove(path + ".key"); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := openHistoryStore(path)
	if err == nil || rebuilt == nil {
		t.Fatal("cache without its local key was not reported and rebuilt")
	}
	if _, ok := rebuilt.get("player", 0, 0); ok {
		t.Fatal("cache was served under a newly generated unrelated key")
	}
	rebuilt.close()
}

func TestHistoryStoreHasByteCapAndEvictsOldest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history-cache.json")
	store, err := openHistoryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for i := 0; i < 6; i++ {
		player := "player-" + strconv.Itoa(i)
		store.put(player, core.HistoryPage{Offset: 0, NextOffset: 1, Matches: []core.Match{{GameID: int64(i + 1), QueueID: 420}}, Meta: core.HistoryMeta{FetchedAt: &now, Returned: 1, Parsed: 1}})
		store.mu.Lock()
		entry := store.entries[store.keyFor(player)]
		entry.UpdatedAt = time.Unix(int64(i+1), 0).UTC()
		store.entries[store.keyFor(player)] = entry
		store.mu.Unlock()
	}
	store.mu.Lock()
	one := map[string]historyStoreEntry{store.keyFor("player-5"): store.entries[store.keyFor("player-5")]}
	store.mu.Unlock()
	oneRaw, _, err := marshalHistoryStore(one, historyStoreMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	capBytes := len(oneRaw) + 16
	store.mu.Lock()
	store.maxBytes = capBytes
	store.mu.Unlock()
	store.close()
	info, err := os.Stat(path)
	if err != nil || info.Size() > int64(capBytes) {
		t.Fatalf("history cache exceeded byte cap: %d > %d", info.Size(), capBytes)
	}
	raw, _ := os.ReadFile(path)
	var file historyStoreFile
	if json.Unmarshal(raw, &file) != nil || len(file.Entries) != 1 {
		t.Fatalf("bounded cache retained wrong entry count: %s", raw)
	}
	if _, ok := file.Entries[store.keyFor("player-5")]; !ok {
		t.Fatal("byte eviction did not retain the newest entry")
	}
}

func TestHistoryStoreRetriesRenameAndReportsWriteFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history-cache.json")
	store, err := openHistoryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.entries[store.keyFor("player")] = historyStoreEntry{UpdatedAt: time.Now().UTC(), Pages: map[string]core.HistoryPage{}}
	store.generation++
	store.mu.Unlock()
	attempts := 0
	store.retryPause = 0
	store.rename = func(source, target string) error {
		attempts++
		if attempts < 3 {
			return errors.New("synthetic rename failure")
		}
		return os.Rename(source, target)
	}
	store.write()
	if attempts != 3 {
		t.Fatalf("rename attempts = %d", attempts)
	}
	before := diagnostics.Default.Snapshot()["history-cache-write"].Failed
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	store.path = filepath.Join(blocker, "history-cache.json")
	store.write()
	after := diagnostics.Default.Snapshot()["history-cache-write"]
	if after.Failed != before+1 || after.LastStatus != 500 {
		t.Fatalf("cache write failure missing from diagnostics: %+v", after)
	}
	store.path = path
	store.rename = os.Rename
	store.close()
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
