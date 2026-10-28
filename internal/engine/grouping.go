package engine

import (
	"context"
	"crypto/sha256"
	"fmt"

	"strings"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/source"
)

func (e *Engine) syncGroupingLocked() {
	s := e.snapshot
	e.snapshot.GroupCoverage = 0
	e.snapshot.GroupStatus = "unavailable"
	defer func() {
		for _, p := range e.snapshot.Participants {
			if p.GroupSource != "" {
				e.snapshot.GroupCoverage++
			}
		}
		if e.snapshot.GroupCoverage > 0 {
			e.snapshot.GroupStatus = "partial"
			if e.snapshot.GroupCoverage == len(e.snapshot.Participants) {
				e.snapshot.GroupStatus = "observed"
			}
		}
	}()
	key := fmt.Sprintf("%d:%s:%d", s.ConnectionID, s.SelfKey, s.GameID)
	supported := s.QueueID == 420 || s.QueueID == 440 || s.QueueID == 450 || s.QueueID == 400 || s.QueueID == 430 || s.QueueID == 490
	if !supported {
		for i := range e.snapshot.Participants {
			p := &e.snapshot.Participants[i]
			p.PremadeLabel = ""
			p.PremadeGroup = ""
			p.PremadeSize = 0
			p.GroupSource = ""
		}
	}
	active := supported && s.Connected && (s.Phase == "GameStart" || s.Phase == "InProgress") && s.GameID > 0 && s.SelfKey != ""
	if !active || key != e.groupScope {
		if e.groupCancel != nil {
			e.groupCancel()
			e.groupCancel = nil
		}
		if key != e.groupScope {
			e.groupScope = key
			e.groupAttempted = false
			e.groupMembers = nil
			e.groupSides = nil
			e.groupPositions = nil
			e.groupLabels = nil
		}
	}
	if !active {
		return
	}
	// Reapply to replacement rosters only within the same game and identity.
	for i := range e.snapshot.Participants {
		p := &e.snapshot.Participants[i]
		if p.Position == "" && e.groupSides[p.PUUID] == p.Team {
			p.Position = e.groupPositions[p.PUUID]
		}
		if token := e.groupMembers[p.PUUID]; p.GroupToken == "" && token != "" && e.groupSides[p.PUUID] == p.Team {
			p.GroupToken = token
		}
	}
	core.AssignPremades(e.snapshot.Participants, "lcu")
	if e.groupLabels == nil {
		e.groupLabels = map[string]string{}
	}
	for i := range e.snapshot.Participants {
		p := &e.snapshot.Participants[i]
		p.PremadeLabel = ""
		if p.PremadeGroup != "" {
			p.PremadeGroup = fmt.Sprintf("%s-%x", p.Team, sha256.Sum256([]byte(key+":"+p.GroupToken)))
			label := e.groupLabels[p.PremadeGroup]
			if label == "" {
				n := 0
				for k := range e.groupLabels {
					if strings.HasPrefix(k, p.Team+"-") {
						n++
					}
				}
				label = fmt.Sprintf("%c", 'A'+n)
				e.groupLabels[p.PremadeGroup] = label
			}
			p.PremadeLabel = label
		}
	}

	for i := range e.snapshot.Participants {
		p := &e.snapshot.Participants[i]
		if e.groupMembers[p.PUUID] != "" && p.PremadeGroup != "" {
			p.GroupSource = "sgp"
		}
	}
	complete := len(s.Participants) == 10
	for _, p := range s.Participants {
		complete = complete && p.GroupToken != ""
	}
	src, ok := e.client.(source.GroupingSource)
	if complete || !ok || e.groupAttempted || e.initialMatch.parent == nil {
		return
	}
	e.groupAttempted = true
	ctx, cancel := context.WithTimeout(e.initialMatch.parent, 6*time.Second)
	e.groupCancel = cancel
	go func() {
		defer cancel()
		for attempt := 0; attempt < 2; attempt++ {
			req, stop := context.WithTimeout(ctx, 2*time.Second)
			members, err := src.Grouping(req, s.GameID)
			stop()
			if ctx.Err() != nil {
				return
			}
			e.mu.Lock()
			valid := ctx.Err() == nil && e.groupScope == key && e.clientVersion == s.ConnectionID && e.snapshot.SelfKey == s.SelfKey && e.snapshot.GameID == s.GameID && activeMatch(e.snapshot.Phase)
			if !valid {
				e.mu.Unlock()
				return
			}
			if err == nil {
				// Cross-check every member and side, without replacing the current roster.
				tokens := map[string]string{}
				positions := map[string]string{}
				sides := map[string]string{}
				conflict := false
				for _, p := range members {
					if core.IsKnownPUUID(p.PUUID) {
						positions[p.PUUID] = p.Position
						sides[p.PUUID] = p.Team
						if p.GroupToken != "" {
							tokens[p.PUUID] = p.GroupToken
						}
					}
					for _, q := range e.snapshot.Participants {
						if p.PUUID == q.PUUID && p.Team != q.Team {
							conflict = true
						}
						if p.PUUID == q.PUUID && p.Team == q.Team {
							if q.GroupToken != "" && p.GroupToken != "" && q.GroupToken != p.GroupToken {
								conflict = true
							}
							positions[p.PUUID] = p.Position
							if p.GroupToken != "" {
								tokens[p.PUUID] = p.GroupToken
							}
						}
					}
				}
				if conflict {
					e.mu.Unlock()
					return
				}
				e.groupMembers = tokens
				e.groupSides = sides
				e.groupPositions = positions
				e.publishLocked()
				all := len(tokens) == 10 && len(members) == 10
				e.mu.Unlock()
				if all {
					return
				}
			} else {
				e.mu.Unlock()
			}
			if attempt == 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
				}
			}
		}
	}()
}
