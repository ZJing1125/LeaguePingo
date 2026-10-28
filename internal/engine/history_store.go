package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"leaguepingo/internal/core"
)

const historyStoreSchema = 2
const historyStorePlayers = 64

// historyStore persists only de-identified match summaries. The key is a
// one-way hash of the verified PUUID. Anonymous team/champion composition is
// retained, while roster names are removed before writing, so the cache never
// contains player IDs, Riot IDs, access tokens or names.
type historyStore struct {
	mu      sync.Mutex
	path    string
	entries map[string]historyStoreEntry
	wake    chan struct{}
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
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
	s := &historyStore{path: path, entries: map[string]historyStoreEntry{}, wake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{})}
	migrate := false
	b, err := os.ReadFile(path)
	if err == nil {
		var file historyStoreFile
		if json.Unmarshal(b, &file) != nil {
			return nil, errors.New("本地战绩缓存格式不兼容")
		}
		if file.Schema == 1 {
			// Schema 1 removed whole rosters and caused the first page to lose
			// composition after restart. Drop those incomplete summaries once.
			migrate = true
		} else if file.Schema != historyStoreSchema {
			return nil, errors.New("本地战绩缓存格式不兼容")
		}
		for key, entry := range file.Entries {
			if migrate {
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
		return nil, errors.New("无法读取本地战绩缓存")
	}
	go s.writer()
	if migrate {
		s.wake <- struct{}{}
	}
	return s, nil
}

func validHistoryStoreKey(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func historyStoreKey(puuid string) string {
	sum := sha256.Sum256([]byte(puuid))
	return hex.EncodeToString(sum[:])
}

func validStoredPage(page core.HistoryPage, offset int) bool {
	return offset >= 0 && offset <= 980 && offset%20 == 0 && page.Offset == offset && page.NextOffset >= offset && page.NextOffset <= 1000 && len(page.Matches) <= 20
}

func sanitizeHistoryPage(page core.HistoryPage) core.HistoryPage {
	page.Matches = append([]core.Match(nil), page.Matches...)
	for i := range page.Matches {
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
	entry, ok := s.entries[historyStoreKey(puuid)]
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
	if s == nil || puuid == "" || !validStoredPage(page, page.Offset) {
		return
	}
	page = sanitizeHistoryPage(page)
	s.mu.Lock()
	key := historyStoreKey(puuid)
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
	s.mu.Lock()
	file := historyStoreFile{Schema: historyStoreSchema, Entries: s.entries}
	b, err := json.Marshal(file)
	s.mu.Unlock()
	if err != nil || os.MkdirAll(filepath.Dir(s.path), 0700) != nil {
		return
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), "history-*.tmp")
	if err != nil {
		return
	}
	name := f.Name()
	defer os.Remove(name)
	_ = f.Chmod(0600)
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil && closeErr == nil {
		_ = os.Rename(name, s.path)
	}
}

func (s *historyStore) close() {
	if s == nil {
		return
	}
	s.once.Do(func() { close(s.stop); <-s.done })
}
