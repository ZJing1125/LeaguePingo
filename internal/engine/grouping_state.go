package engine

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"time"

	"leaguepingo/internal/core"
)

// groupingState owns ephemeral group metadata and its one-shot task lifetime.
// Engine -> grouping is the only nested lock order. No source I/O or engine
// publication runs while mu is held. A zero value is ready to use.
type groupingState struct {
	mu        sync.Mutex
	scope     string
	attempted bool
	cancel    context.CancelFunc
	members   map[string]string
	sides     map[string]string
	positions map[string]string
	labels    map[string]string
}

func (g *groupingState) stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopLocked()
}

func (g *groupingState) stopLocked() {
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
}

func (g *groupingState) apply(snapshot *core.Snapshot) (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	defer updateGroupCoverage(snapshot)
	key := fmt.Sprintf("%d:%s:%d", snapshot.ConnectionID, snapshot.SelfKey, snapshot.GameID)
	supported := groupingQueueSupported(snapshot.QueueID)
	if !supported {
		clearGroupPresentation(snapshot.Participants)
	}
	phase := snapshotPhase(snapshot.Phase)
	active := supported && snapshot.Connected && (phase == PhaseGameStart || phase == PhaseInProgress) && snapshot.GameID > 0 && snapshot.SelfKey != ""
	if !active || key != g.scope {
		g.stopLocked()
		if key != g.scope {
			g.scope, g.attempted = key, false
			g.members, g.sides, g.positions, g.labels = nil, nil, nil, nil
		}
	}
	if active {
		g.restoreLocked(snapshot.Participants)
		core.AssignPremades(snapshot.Participants, "lcu")
		g.labelLocked(key, snapshot.Participants)
	}
	return key, active
}

func (g *groupingState) restoreLocked(players []core.Participant) {
	for i := range players {
		p := &players[i]
		if g.sides[p.PUUID] != p.Team {
			continue
		}
		if p.Position == "" {
			p.Position = g.positions[p.PUUID]
		}
		if p.GroupToken == "" && g.members[p.PUUID] != "" {
			p.GroupToken = g.members[p.PUUID]
		}
	}
}

func (g *groupingState) labelLocked(key string, players []core.Participant) {
	if g.labels == nil {
		g.labels = map[string]string{}
	}
	for i := range players {
		p := &players[i]
		p.PremadeLabel = ""
		if p.PremadeGroup != "" {
			p.PremadeGroup = fmt.Sprintf("%s-%x", p.Team, sha256.Sum256([]byte(key+":"+p.GroupToken)))
			label := g.labels[p.PremadeGroup]
			if label == "" {
				n := 0
				for group := range g.labels {
					if strings.HasPrefix(group, p.Team+"-") {
						n++
					}
				}
				label = fmt.Sprintf("%c", 'A'+n)
				g.labels[p.PremadeGroup] = label
			}
			p.PremadeLabel = label
		}
		if g.members[p.PUUID] != "" && g.sides[p.PUUID] == p.Team && g.members[p.PUUID] == p.GroupToken {
			p.GroupSource = "sgp"
		}
	}
}

func (g *groupingState) begin(parent context.Context, key string) (context.Context, context.CancelFunc, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if parent == nil || g.scope != key || g.attempted {
		return nil, nil, false
	}
	g.attempted = true
	ctx, cancel := context.WithTimeout(parent, 6*time.Second)
	g.cancel = cancel
	return ctx, cancel, true
}

func (g *groupingState) current(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.scope == key
}

// merge is called only after the engine checks the live roster/identity. Keep
// prior partial metadata so a later roster identity can still use it.
func (g *groupingState) merge(key string, members []core.Participant) (bool, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.scope != key {
		return false, false
	}
	for _, p := range members {
		if side := g.sides[p.PUUID]; side != "" && side != p.Team {
			return false, false
		}
		if token := g.members[p.PUUID]; token != "" && p.GroupToken != "" && token != p.GroupToken {
			return false, false
		}
	}
	if g.members == nil {
		g.members, g.sides, g.positions = map[string]string{}, map[string]string{}, map[string]string{}
	}
	for _, p := range members {
		if !core.IsKnownPUUID(p.PUUID) {
			continue
		}
		g.sides[p.PUUID] = p.Team
		if p.GroupToken != "" {
			g.members[p.PUUID] = p.GroupToken
		}
		if p.Position != "" {
			g.positions[p.PUUID] = p.Position
		}
	}
	return true, len(g.members) == 10
}

func groupingQueueSupported(queue int) bool {
	return queue == 420 || queue == 440 || queue == 450 || queue == 400 || queue == 430 || queue == 490
}

func clearGroupPresentation(players []core.Participant) {
	for i := range players {
		p := &players[i]
		p.PremadeLabel, p.PremadeGroup, p.GroupSource, p.PremadeSize = "", "", "", 0
	}
}

func updateGroupCoverage(snapshot *core.Snapshot) {
	snapshot.GroupCoverage, snapshot.GroupStatus = 0, "unavailable"
	for _, p := range snapshot.Participants {
		if p.GroupSource != "" {
			snapshot.GroupCoverage++
		}
	}
	if snapshot.GroupCoverage > 0 {
		snapshot.GroupStatus = "partial"
		if snapshot.GroupCoverage == len(snapshot.Participants) {
			snapshot.GroupStatus = "observed"
		}
	}
}
