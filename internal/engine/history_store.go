package engine

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
)

const historyStoreSchema = 3
const historyStorePlayers = 64
const historyStoreMaxBytes = 8 << 20
const historyStoreSecretBytes = 32

// historyStore persists only de-identified match summaries. The key is a
// keyed HMAC of the verified PUUID. Anonymous team/champion composition is
// retained, while roster names are removed before writing, so the cache never
// contains player IDs, Riot IDs, access tokens or names.
type historyStore struct {
	mu         sync.Mutex
	path       string
	secret     []byte
	entries    map[string]historyStoreEntry
	generation uint64
	maxBytes   int
	rename     func(string, string) error
	retryPause time.Duration
	wake       chan struct{}
	stop       chan struct{}
	done       chan struct{}
	once       sync.Once
}

type historyStoreFile struct {
	Schema  int                          `json:"schema"`
	Entries map[string]historyStoreEntry `json:"entries"`
}

type historyStoreEntry struct {
	UpdatedAt time.Time                   `json:"updatedAt"`
	Pages     map[string]core.HistoryPage `json:"pages"`
}

func openHistoryStore(path string) (*historyStore, error) {
	secret, resetSecret, err := loadHistoryStoreSecret(path + ".key")
	if err != nil {
		return nil, errors.New("无法准备本地战绩缓存密钥")
	}
	s := &historyStore{path: path, secret: secret, entries: map[string]historyStoreEntry{}, maxBytes: historyStoreMaxBytes, rename: os.Rename, retryPause: 25 * time.Millisecond, wake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{})}
	migrate := false
	reset := false
	b, err := readHistoryStoreFile(path, s.maxBytes)
	if err == nil {
		var file historyStoreFile
		if json.Unmarshal(b, &file) != nil {
			reset = true
		}
		if file.Schema == 1 || file.Schema == 2 {
			// Schema 1 removed whole rosters and caused the first page to lose
			// composition. Schema 2 used an unkeyed identity hash. Neither can
			// be served after the keyed schema upgrade.
			migrate = true
		} else if file.Schema != historyStoreSchema {
			reset = true
		}
		if file.Schema == historyStoreSchema && resetSecret {
			reset = true
		}
		for key, entry := range file.Entries {
			if migrate || reset {
				break
			}
			if !validHistoryStoreKey(key) || entry.UpdatedAt.IsZero() || len(entry.Pages) > 50 {
				continue
			}
			pages := map[string]core.HistoryPage{}
			for offset, page := range entry.Pages {
				value, parseErr := strconv.Atoi(offset)
				if parseErr != nil || !validStoredPage(page, value) {
					continue
				}
				pages[offset] = sanitizeHistoryPage(page)
			}
			if len(pages) > 0 {
				entry.Pages = pages
				s.entries[key] = entry
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		if errors.Is(err, errHistoryStoreInvalid) {
			reset = true
		} else {
			return nil, errors.New("无法读取本地战绩缓存")
		}
	}
	if reset || migrate {
		s.entries = map[string]historyStoreEntry{}
	}
	go s.writer()
	if reset || migrate {
		s.wake <- struct{}{}
	}
	if reset {
		return s, errors.New("本地战绩缓存已损坏并重建")
	}
	return s, nil
}

var errHistoryStoreInvalid = errors.New("invalid history store")

func readHistoryStoreFile(path string, maxBytes int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, int64(maxBytes)+1))
	if err != nil {
		return nil, errors.New("无法读取本地战绩缓存")
	}
	if len(b) > maxBytes {
		return nil, errHistoryStoreInvalid
	}
	return b, nil
}

func loadHistoryStoreSecret(path string) ([]byte, bool, error) {
	b, err := os.ReadFile(path)
	if err == nil && len(b) == historyStoreSecretBytes {
		return b, false, nil
	}
	reset := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}
	if reset && os.Remove(path) != nil {
		return nil, false, errHistoryStoreInvalid
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, false, err
	}
	secret := make([]byte, historyStoreSecretBytes)
	if _, err := rand.Read(secret); err != nil {
		return nil, false, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		if current, readErr := os.ReadFile(path); readErr == nil && len(current) == historyStoreSecretBytes {
			return current, false, nil
		}
	}
	if err != nil {
		return nil, false, err
	}
	var n int
	if n, err = f.Write(secret); err == nil && n != len(secret) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(path)
		if err != nil {
			return nil, false, err
		}
		return nil, false, closeErr
	}
	return secret, true, nil
}

func validHistoryStoreKey(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func historyStoreKey(secret []byte, puuid string) string {
	hash := hmac.New(sha256.New, secret)
	_, _ = hash.Write([]byte(puuid))
	return hex.EncodeToString(hash.Sum(nil))
}

func (s *historyStore) keyFor(puuid string) string { return historyStoreKey(s.secret, puuid) }

func validStoredPage(page core.HistoryPage, offset int) bool {
	return offset >= 0 && offset <= 980 && offset%20 == 0 && page.Offset == offset && page.NextOffset >= offset && page.NextOffset <= 1000 && len(page.Matches) <= 20
}

func sanitizeHistoryPage(page core.HistoryPage) core.HistoryPage {
	page.Meta.Service = nil
	page.Meta.CheckedAt = nil
	if page.Meta.Reasons != nil {
		reasons := make(map[string]int, len(page.Meta.Reasons))
		for reason, count := range page.Meta.Reasons {
			reasons[reason] = count
		}
		page.Meta.Reasons = reasons
	}
	page.Matches = append([]core.Match(nil), page.Matches...)
	for i := range page.Matches {
		if performance := page.Matches[i].Performance; performance != nil {
			value := *performance
			value.Metrics = append([]core.PerformanceMetric(nil), performance.Metrics...)
			page.Matches[i].Performance = &value
		}
		if arena := page.Matches[i].Arena; arena != nil {
			value := *arena
			if arena.TeamCount != nil {
				n := *arena.TeamCount
				value.TeamCount = &n
			}
			page.Matches[i].Arena = &value
		}
		page.Matches[i].Items = append([]*int(nil), page.Matches[i].Items...)
		page.Matches[i].RuneIDs = append([]int(nil), page.Matches[i].RuneIDs...)
		page.Matches[i].RuneShards = append([]int(nil), page.Matches[i].RuneShards...)
		page.Matches[i].Roster = append([]core.MatchPeer(nil), page.Matches[i].Roster...)
		for j := range page.Matches[i].Roster {
			page.Matches[i].Roster[j].DisplayName = ""
		}
	}
	return page
}

func historyHeadChanged(previous, next core.HistoryPage) bool {
	if previous.NextOffset != next.NextOffset {
		return true
	}
	if len(previous.Matches) == 0 || len(next.Matches) == 0 {
		return len(previous.Matches) != len(next.Matches)
	}
	return previous.Matches[0].GameID != next.Matches[0].GameID
}

func (s *historyStore) get(puuid string, offset int, maxAge time.Duration) (core.HistoryPage, bool) {
	if s == nil || puuid == "" {
		return core.HistoryPage{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[s.keyFor(puuid)]
	if !ok {
		return core.HistoryPage{}, false
	}
	page, ok := entry.Pages[strconv.Itoa(offset)]
	if !ok {
		return core.HistoryPage{}, false
	}
	// Pages are refreshed independently. Updating a later page must not make an
	// older head page appear fresh, so prefer the source timestamp for the page.
	fetchedAt := entry.UpdatedAt
	if page.Meta.FetchedAt != nil && !page.Meta.FetchedAt.IsZero() {
		fetchedAt = *page.Meta.FetchedAt
	}
	if maxAge > 0 && time.Since(fetchedAt) > maxAge {
		return core.HistoryPage{}, false
	}
	return sanitizeHistoryPage(page), ok
}

func (s *historyStore) put(puuid string, page core.HistoryPage) {
	if page.Meta.Service != nil {
		return
	} // Do not persist degraded/session state as a fresh remote page.
	if s == nil || puuid == "" || !validStoredPage(page, page.Offset) {
		return
	}
	page = sanitizeHistoryPage(page)
	s.mu.Lock()
	key := s.keyFor(puuid)
	entry := s.entries[key]
	if entry.Pages == nil {
		entry.Pages = map[string]core.HistoryPage{}
	} else if page.Offset == 0 {
		if previous, ok := entry.Pages["0"]; ok && historyHeadChanged(previous, page) {
			// Offset pages are relative to the current head. Once a new match
			// shifts the head, deeper pages from the previous snapshot have gaps.
			entry.Pages = map[string]core.HistoryPage{}
		}
	}
	entry.UpdatedAt = time.Now().UTC()
	entry.Pages[strconv.Itoa(page.Offset)] = page
	s.entries[key] = entry
	if len(s.entries) > historyStorePlayers {
		var oldestKey string
		var oldest time.Time
		for candidate, item := range s.entries {
			if candidate != key && (oldestKey == "" || item.UpdatedAt.Before(oldest)) {
				oldestKey, oldest = candidate, item.UpdatedAt
			}
		}
		delete(s.entries, oldestKey)
	}
	s.generation++
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *historyStore) writer() {
	defer close(s.done)
	for {
		select {
		case <-s.wake:
			timer := time.NewTimer(500 * time.Millisecond)
			select {
			case <-timer.C:
			case <-s.stop:
				timer.Stop()
				s.write()
				return
			}
			s.write()
		case <-s.stop:
			s.write()
			return
		}
	}
}

func (s *historyStore) write() {
	started := time.Now()
	failed := true
	defer func() {
		status := 200
		if failed {
			status = 500
		}
		diagnostics.Default.Record("/history-cache/write", time.Since(started), status, failed)
	}()
	s.mu.Lock()
	generation := s.generation
	entries := cloneHistoryStoreEntries(s.entries)
	maxBytes := s.maxBytes
	s.mu.Unlock()
	b, retained, err := marshalHistoryStore(entries, maxBytes)
	if err != nil {
		return
	}
	if len(retained) != len(entries) {
		s.mu.Lock()
		if s.generation == generation {
			s.entries = retained
			s.generation++
		}
		s.mu.Unlock()
	}
	if os.MkdirAll(filepath.Dir(s.path), 0700) != nil {
		return
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), "history-*.tmp")
	if err != nil {
		return
	}
	name := f.Name()
	defer os.Remove(name)
	if f.Chmod(0600) != nil {
		_ = f.Close()
		return
	}
	var n int
	if n, err = f.Write(b); err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil && closeErr == nil {
		if s.renameWithRetry(name, s.path) == nil {
			failed = false
		}
	}
}

func cloneHistoryStoreEntries(entries map[string]historyStoreEntry) map[string]historyStoreEntry {
	copy := make(map[string]historyStoreEntry, len(entries))
	for key, entry := range entries {
		pages := make(map[string]core.HistoryPage, len(entry.Pages))
		for offset, page := range entry.Pages {
			pages[offset] = sanitizeHistoryPage(page)
		}
		entry.Pages = pages
		copy[key] = entry
	}
	return copy
}

func marshalHistoryStore(entries map[string]historyStoreEntry, maxBytes int) ([]byte, map[string]historyStoreEntry, error) {
	prefix := `{"schema":` + strconv.Itoa(historyStoreSchema) + `,"entries":{`
	const suffix = `}}`
	if maxBytes < len(prefix)+len(suffix) {
		return nil, nil, errHistoryStoreInvalid
	}
	type encodedEntry struct {
		key   string
		value historyStoreEntry
		raw   []byte
	}
	encoded := make([]encodedEntry, 0, len(entries))
	for key, entry := range entries {
		raw, err := json.Marshal(entry)
		if err != nil {
			return nil, nil, err
		}
		encoded = append(encoded, encodedEntry{key: key, value: entry, raw: raw})
	}
	sort.Slice(encoded, func(i, j int) bool {
		if encoded[i].value.UpdatedAt.Equal(encoded[j].value.UpdatedAt) {
			return encoded[i].key < encoded[j].key
		}
		return encoded[i].value.UpdatedAt.After(encoded[j].value.UpdatedAt)
	})
	retained := make(map[string]historyStoreEntry, len(encoded))
	selected := make([]encodedEntry, 0, len(encoded))
	used := len(prefix) + len(suffix)
	for _, entry := range encoded {
		key, _ := json.Marshal(entry.key)
		addition := len(key) + 1 + len(entry.raw)
		if len(selected) > 0 {
			addition++
		}
		if used+addition > maxBytes {
			continue
		}
		used += addition
		selected = append(selected, entry)
		retained[entry.key] = entry.value
	}
	var out bytes.Buffer
	out.Grow(used)
	out.WriteString(prefix)
	for i, entry := range selected {
		if i > 0 {
			out.WriteByte(',')
		}
		key, _ := json.Marshal(entry.key)
		out.Write(key)
		out.WriteByte(':')
		out.Write(entry.raw)
	}
	out.WriteString(suffix)
	return out.Bytes(), retained, nil
}

func (s *historyStore) renameWithRetry(source, target string) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = s.rename(source, target); err == nil {
			return nil
		}
		if attempt < 2 && s.retryPause > 0 {
			time.Sleep(time.Duration(attempt+1) * s.retryPause)
		}
	}
	return err
}

func (s *historyStore) close() {
	if s == nil {
		return
	}
	s.once.Do(func() { close(s.stop); <-s.done })
}
