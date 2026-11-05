package guides

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
)

type Service struct {
	mu               sync.Mutex
	parent           context.Context
	provider         Provider
	dir              string
	state            State
	snapshot         core.Snapshot
	scope            string
	current          string
	overridePosition string
	overrideOpponent int
	cancel           context.CancelFunc
	generation       uint64
	changes          chan State
	cache            map[string]*Guide
	failures         map[string]time.Time
	slots            chan struct{}
	enabled          bool
	fetchTimeout     time.Duration
}

func New(ctx context.Context, dir string, provider Provider) *Service {
	if provider == nil {
		provider = NewOPgg()
	}
	return &Service{parent: ctx, provider: provider, dir: dir, changes: make(chan State, 1), cache: map[string]*Guide{}, failures: map[string]time.Time{}, slots: make(chan struct{}, 2), enabled: true, fetchTimeout: 8 * time.Second, state: State{Status: "idle", Message: "选择英雄后准备攻略"}}
}
func (s *Service) Changes() <-chan State { return s.changes }
func (s *Service) Snapshot() State       { s.mu.Lock(); defer s.mu.Unlock(); return s.state }
func (s *Service) publish() {
	s.state.Revision++
	select {
	case s.changes <- s.state:
	default:
		select {
		case <-s.changes:
		default:
		}
		s.changes <- s.state
	}
}
func (s *Service) Observe(snapshot core.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = snapshot
	owner := scope(snapshot)
	if owner != s.scope {
		s.scope = owner
		s.overridePosition = ""
		s.overrideOpponent = 0
	}
	s.update(false)
}
func (s *Service) SetEnabled(enabled bool) State {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.enabled == enabled {
		return s.state
	}
	s.enabled = enabled
	s.current = ""
	s.update(false)
	return s.state
}
func (s *Service) Select(position string, opponent int, retry bool) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if position != "" && !validPosition(position) {
		return s.state, apperrors.New(apperrors.InvalidPosition)
	}
	if opponent != 0 {
		valid := false
		for _, p := range s.snapshot.Participants {
			if p.Team == "enemy" && p.ChampionID == opponent {
				valid = true
			}
		}
		if !valid {
			return s.state, apperrors.New(apperrors.StaleData)
		}
	}
	s.overridePosition = position
	s.overrideOpponent = opponent
	s.update(retry)
	return s.state, nil
}

// Candidates only changes shared in-memory selection; it never starts a fetch.
func (s *Service) Candidates(scope string, champion int, ids []int) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Scope != scope || s.state.Query.Champion != champion || (s.state.Query.Mode != "aram-mayhem" && s.state.Query.Mode != "arena") || s.state.Base == nil || len(ids) > 3 {
		return s.state, apperrors.New(apperrors.InvalidCandidates)
	}
	seen := map[int]bool{}
	for _, id := range ids {
		valid := false
		for _, a := range s.state.Base.Augments {
			valid = valid || a.ID == id
		}
		if !valid || seen[id] {
			return s.state, apperrors.New(apperrors.InvalidCandidates)
		}
		seen[id] = true
	}
	s.state.Candidates = append([]int{}, ids...)
	s.publish()
	return s.state, nil
}

type guideSelection struct {
	query                        Query
	self                         core.Participant
	mode, label                  string
	reference, enabled, explicit bool
}

func (s *Service) selectionLocked() guideSelection {
	snapshot := s.snapshot
	modeName, label, reference := mode(snapshot)
	var self core.Participant
	for _, p := range snapshot.Participants {
		if p.Key == snapshot.SelfKey && snapshot.SelfKey != "" {
			self = p
			break
		}
	}
	enabled := snapshot.Connected && (snapshot.Phase == "ChampSelect" || snapshot.Phase == "GameStart" || snapshot.Phase == "InProgress" || snapshot.Phase == "Reconnect")
	q := Query{Mode: modeName, Champion: self.ChampionID, Patch: patch(snapshot.GameVersion)}
	pos := s.overridePosition
	if pos == "" {
		pos = self.Position
	}
	explicit := validPosition(pos)
	if pos == "" {
		pos = "mid"
	} // required by provider for ARAM; never presented as an inferred lane
	q.Position = pos
	if noLane(modeName) {
		q.Position = "mid"
		explicit = false
	}
	if modeName == "ranked" && !reference {
		q.Opponent = s.overrideOpponent
		if q.Opponent > 0 {
			found := false
			for _, p := range snapshot.Participants {
				if p.Team == "enemy" && p.ChampionID == q.Opponent {
					found = true
				}
			}
			if !found {
				q.Opponent = 0
				s.overrideOpponent = 0
			}
		}
		if q.Opponent == 0 && explicit {
			for _, p := range snapshot.Participants {
				if p.Team == "enemy" && p.Position == pos && p.ChampionID > 0 {
					if q.Opponent != 0 {
						q.Opponent = 0
						break
					}
					q.Opponent = p.ChampionID
				}
			}
		}
	}
	if !noLane(modeName) && !explicit {
		q.Position = ""
	}
	return guideSelection{query: q, self: self, mode: modeName, label: label, reference: reference, enabled: enabled, explicit: explicit}
}

func (s *Service) update(force bool) {
	selection := s.selectionLocked()
	q, self := selection.query, selection.self
	modeName, label, reference := selection.mode, selection.label, selection.reference
	enabled, explicit := selection.enabled, selection.explicit
	status, message := "loading", "正在准备 OP.GG 攻略"
	if !s.enabled {
		status = "disabled"
		message = "英雄攻略已关闭"
	} else if !enabled || self.ChampionID == 0 || self.ChampionState == "intent" {
		status = "idle"
		message = "确认英雄后准备攻略"
	} else if modeName == "" {
		status = "unsupported"
		message = label
	} else if !validPosition(q.Position) {
		status = "position"
		message = "请选择本局位置，准备对应攻略"
	} else if champions[self.ChampionID] == "" {
		status = "unsupported"
		message = "这个英雄的攻略映射尚未更新"
	}
	key := s.scope + ":" + cacheKey(q) + ":" + status
	if key == s.current && !force {
		return
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.generation++
	token := s.generation
	s.current = key
	var candidates []int
	if s.state.Query == q && s.state.Scope == s.scope {
		candidates = append([]int{}, s.state.Candidates...)
	}
	s.state = State{Revision: s.state.Revision, Scope: s.scope, Status: status, Message: message, Query: q, ModeLabel: label, Reference: reference, PositionExplicit: explicit}
	s.state.Candidates = candidates
	if modeName != "" && champions[q.Champion] != "" {
		s.state.SourceURL = sourceURL(q)
	}
	if status != "loading" {
		s.publish()
		return
	}
	baseQ := q
	baseQ.Opponent = 0
	s.state.Base = s.cached(baseQ)
	if q.Opponent > 0 {
		s.state.Matchup = s.cached(q)
	}
	s.updateFreshness()
	s.publish()
	ctx, cancel := context.WithCancel(s.parent)
	s.cancel = cancel
	go s.load(ctx, token, baseQ, q, force)
}
func (s *Service) updateFreshness() {
	s.state.Stale = false
	s.state.VersionMismatch = false
	for _, g := range []*Guide{s.state.Base, s.state.Matchup} {
		if g == nil {
			continue
		}
		s.state.Stale = s.state.Stale || time.Since(g.FetchedAt) > 6*time.Hour
		s.state.VersionMismatch = s.state.VersionMismatch || (s.state.Query.Patch != "" && patch(g.Version) != "" && patch(g.Version) != s.state.Query.Patch)
	}
}
func (s *Service) load(ctx context.Context, token uint64, base, q Query, force bool) {
	// Debounce actual selection changes; repeated snapshots never restart this timer.
	timer := time.NewTimer(800 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	queries := []Query{base}
	if q.Opponent > 0 {
		queries = append(queries, q)
	}
	lastError := ""
	for _, query := range queries {
		key := cacheKey(query)
		s.mu.Lock()
		if token != s.generation || ctx.Err() != nil {
			s.mu.Unlock()
			return
		}
		cached := s.cached(query)
		failedAt := s.failures[key]
		s.mu.Unlock()
		if cached != nil && time.Since(cached.FetchedAt) < 6*time.Hour && !force {
			continue
		}
		if !failedAt.IsZero() && time.Since(failedAt) < 30*time.Second {
			lastError = "攻略服务暂不可用，稍后重试"
			continue
		}
		select {
		case <-ctx.Done():
			return
		case s.slots <- struct{}{}:
		}
		req, cancel := context.WithTimeout(ctx, s.fetchTimeout)
		g, err := s.provider.Fetch(req, query)
		cancel()
		<-s.slots
		if ctx.Err() != nil {
			return
		}
		s.mu.Lock()
		if token != s.generation || ctx.Err() != nil {
			s.mu.Unlock()
			return
		}
		if err != nil {
			lastError = s.recordFailureLocked(key, err)
		} else {
			s.acceptGuideLocked(query, key, g)
		}
		s.updateFreshness()
		s.publish()
		s.mu.Unlock()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if token != s.generation || ctx.Err() != nil {
		return
	}
	for _, g := range []*Guide{s.state.Base, s.state.Matchup} {
		if g != nil && g.Notice != "" {
			lastError = g.Notice
		}
	}
	s.state.Status = "ready"
	s.state.Message = ""
	if lastError != "" {
		s.state.Status = "partial"
		s.state.Message = lastError
		if s.state.Base == nil && s.state.Matchup == nil {
			s.state.Status = "failed"
		}
	}
	s.updateFreshness()
	s.publish()
}
func (s *Service) recordFailureLocked(key string, err error) string {
	if len(s.failures) >= 128 {
		oldKey := ""
		var oldest time.Time
		for k, at := range s.failures {
			if oldest.IsZero() || at.Before(oldest) {
				oldKey = k
				oldest = at
			}
		}
		delete(s.failures, oldKey)
	}
	s.failures[key] = time.Now()
	return apperrors.UserMessage(err, apperrors.GuideUnavailable)
}

func (s *Service) acceptGuideLocked(query Query, key string, g *Guide) {
	if g.Notice != "" {
		s.failures[key] = time.Now()
	}
	s.cache[key] = g
	s.save(key, g)
	if query.Opponent == 0 {
		s.state.Base = g
		ids := s.state.Candidates
		s.state.Candidates = nil
		for _, id := range ids {
			for _, a := range g.Augments {
				if a.ID == id {
					s.state.Candidates = append(s.state.Candidates, id)
					break
				}
			}
		}
	} else {
		s.state.Matchup = g
	}
}

func cacheKey(q Query) string {
	raw, _ := json.Marshal(q)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}
func (s *Service) cached(q Query) *Guide {
	key := cacheKey(q)
	if g := s.cache[key]; g != nil {
		if !g.FetchedAt.After(time.Now().Add(time.Minute)) && time.Since(g.FetchedAt) <= 7*24*time.Hour {
			return g
		}
		delete(s.cache, key)
	}
	if s.dir == "" {
		return nil
	}
	file, err := os.Open(filepath.Join(s.dir, key+".json"))
	if err != nil {
		return nil
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (256<<10)+1))
	if err != nil || len(raw) > 256<<10 {
		return nil
	}
	var g Guide
	if json.Unmarshal(raw, &g) != nil || g.Query != q || g.FetchedAt.After(time.Now().Add(time.Minute)) || time.Since(g.FetchedAt) > 7*24*time.Hour || !strings.HasPrefix(g.SourceURL, "https://op.gg/") {
		return nil
	}
	s.cache[key] = &g
	s.trimMemory()
	return &g
}
func (s *Service) trimMemory() {
	if len(s.cache) > 128 {
		keys := make([]string, 0, len(s.cache))
		for k := range s.cache {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return s.cache[keys[i]].FetchedAt.Before(s.cache[keys[j]].FetchedAt) })
		for _, k := range keys[:len(keys)-128] {
			delete(s.cache, k)
		}
	}
}
func (s *Service) save(key string, g *Guide) {
	s.trimMemory()
	if s.dir == "" {
		return
	}
	started, failed := time.Now(), true
	defer func() {
		status := 200
		if failed {
			status = 500
		}
		diagnostics.Default.Record("/guide-cache/write", time.Since(started), status, failed)
	}()
	if os.MkdirAll(s.dir, 0700) != nil {
		return
	}
	raw, err := json.Marshal(g)
	if err != nil || len(raw) > 256<<10 {
		return
	}
	path := filepath.Join(s.dir, key+".json")
	temporary := path + ".tmp"
	defer os.Remove(temporary) // Only the temporary file created by this write.
	if os.WriteFile(temporary, raw, 0600) != nil || os.Rename(temporary, path) != nil {
		return // The already-normalized in-memory guide remains usable.
	}
	failed = false
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		failed = true
		return
	}
	type file struct {
		name string
		t    time.Time
	}
	files := []file{}
	for _, e := range entries {
		n := e.Name()
		if len(n) != 69 || !strings.HasSuffix(n, ".json") {
			continue
		}
		if info, err := e.Info(); err == nil {
			files = append(files, file{n, info.ModTime()})
		} else {
			failed = true
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].t.Before(files[j].t) })
	for len(files) > 128 {
		if err := os.Remove(filepath.Join(s.dir, files[0].name)); err != nil && !os.IsNotExist(err) {
			failed = true
		}
		files = files[1:]
	}
}
