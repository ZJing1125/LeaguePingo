package core

import "encoding/json"

// Only derived results are retained; source subteam IDs never leave the parser.
type ArenaResult struct {
	Placement int  `json:"placement"`
	TeamCount *int `json:"teamCount,omitempty"`
}

type ArenaStats struct {
	PlacementGames int      `json:"placementGames"`
	FirstPlaces    int      `json:"firstPlaces"`
	AveragePlace   *float64 `json:"averagePlace"`
	FirstPlaceRate *float64 `json:"firstPlaceRate"`
	TopHalfGames   int      `json:"topHalfGames"`
	TopHalf        int      `json:"topHalf"`
	TopHalfRate    *float64 `json:"topHalfRate"`
}

func IsArenaQueue(queue int) bool {
	mode, ok := ScoutModeFor(queue)
	return ok && mode.Arena
}

// LCU stats.subteamPlacement and SGP's flat subteamPlacement share the same
// adapter. Neither the legacy win flag nor generic placement is an alias.
func arenaResult(game historyGame, target historyParticipant) *ArenaResult {
	mode, ok := ScoutModeFor(game.QueueID)
	if !ok || !mode.Arena || !mode.Matches(valueOrZero(game.MapID), "") || !validArenaPlace(target.Stats.Placement) {
		return nil
	}
	for _, raw := range game.Participants {
		var peer historyParticipant
		if json.Unmarshal(raw, &peer) != nil || target.Stats.SubteamID == nil || *target.Stats.SubteamID <= 0 {
			continue
		}
		if peer.Stats.SubteamID != nil && *peer.Stats.SubteamID == *target.Stats.SubteamID && validArenaPlace(peer.Stats.Placement) && *peer.Stats.Placement != *target.Stats.Placement {
			return nil // Two members of the same subteam cannot finish differently.
		}
	}
	result := &ArenaResult{Placement: *target.Stats.Placement}
	if teams := arenaTeamCount(game); teams > 0 {
		result.TeamCount = &teams
	}
	return result
}

func validArenaPlace(value *int) bool { return value != nil && *value > 0 && *value <= 32 }

// A partial summary cannot establish the denominator. Require distinct
// participants, exactly two members per subteam, consistent placement within
// each pair and a complete permutation of final places. No fixed four/eight.
func arenaTeamCount(game historyGame) int {
	if len(game.Participants) < 4 || len(game.Participants) > 64 || len(game.Participants)%2 != 0 || !arenaRosterVerified(game) {
		return 0
	}
	counts, places, participants := map[int]int{}, map[int]int{}, map[int]bool{}
	for _, raw := range game.Participants {
		var p historyParticipant
		if json.Unmarshal(raw, &p) != nil {
			return 0
		}
		normalizeHistoryParticipant(&p)
		if p.ParticipantID <= 0 || participants[p.ParticipantID] || p.Stats.SubteamID == nil || *p.Stats.SubteamID <= 0 || !validArenaPlace(p.Stats.Placement) {
			return 0
		}
		participants[p.ParticipantID] = true
		id, place := *p.Stats.SubteamID, *p.Stats.Placement
		if counts[id] > 0 && places[id] != place {
			return 0
		}
		counts[id]++
		places[id] = place
	}
	seen := map[int]bool{}
	for id, count := range counts {
		if count != 2 || places[id] > len(counts) || seen[places[id]] {
			return 0
		}
		seen[places[id]] = true
	}
	return len(counts)
}

func arenaRosterVerified(game historyGame) bool {
	if len(game.ParticipantIdentities) != len(game.Participants) {
		return false
	}
	ids, identities := map[int]string{}, map[string]bool{}
	for _, p := range game.ParticipantIdentities {
		if p.ParticipantID <= 0 || ids[p.ParticipantID] != "" || !IsKnownPUUID(p.Player.PUUID) || identities[p.Player.PUUID] {
			return false
		}
		ids[p.ParticipantID], identities[p.Player.PUUID] = p.Player.PUUID, true
	}
	for _, raw := range game.Participants {
		var p historyParticipant
		if json.Unmarshal(raw, &p) != nil {
			return false
		}
		normalizeHistoryParticipant(&p)
		if ids[p.ParticipantID] == "" || IsKnownPUUID(p.PUUID) && p.PUUID != ids[p.ParticipantID] {
			return false
		}
	}
	return true
}

// Metadata.participants is the SGP roster contract. Cross-check its unique
// identities before adapting it to LCU identities; a truncated participant
// array must not accidentally look like a complete smaller Arena format.
func sgpArenaIdentities(raw []json.RawMessage, expected []string) []historyIdentity {
	if len(expected) == 0 || len(expected) != len(raw) || len(raw) > 64 {
		return nil
	}
	ids, seen := map[string]bool{}, map[int]bool{}
	for _, id := range expected {
		if !IsKnownPUUID(id) || ids[id] {
			return nil
		}
		ids[id] = true
	}
	result := make([]historyIdentity, 0, len(raw))
	for _, item := range raw {
		var p historyParticipant
		if json.Unmarshal(item, &p) != nil || p.ParticipantID <= 0 || seen[p.ParticipantID] || !ids[p.PUUID] {
			return nil
		}
		delete(ids, p.PUUID)
		seen[p.ParticipantID] = true
		identity := historyIdentity{ParticipantID: p.ParticipantID}
		identity.Player.PUUID = p.PUUID
		result = append(result, identity)
	}
	return result
}

func CalculateArenaStats(matches []Match) *ArenaStats {
	var out *ArenaStats
	total := 0
	for _, m := range matches {
		if !IsArenaQueue(m.QueueID) {
			continue
		}
		if out == nil {
			out = &ArenaStats{}
		}
		if m.Arena == nil || !validArenaPlace(&m.Arena.Placement) || m.MapID != nil && *m.MapID != 30 {
			continue
		}
		p, n := m.Arena.Placement, m.Arena.TeamCount
		if n != nil && (*n < 2 || *n > 32 || p > *n) {
			continue
		}
		out.PlacementGames++
		total += p
		if p == 1 {
			out.FirstPlaces++
		}
		if n != nil {
			out.TopHalfGames++
			if p <= *n/2 {
				out.TopHalf++
			}
		}
	}
	if out != nil && out.PlacementGames > 0 {
		avg, rate := float64(total)/float64(out.PlacementGames), float64(out.FirstPlaces)*100/float64(out.PlacementGames)
		out.AveragePlace, out.FirstPlaceRate = &avg, &rate
	}
	if out != nil && out.TopHalfGames > 0 {
		rate := float64(out.TopHalf) * 100 / float64(out.TopHalfGames)
		out.TopHalfRate = &rate
	}
	return out
}
