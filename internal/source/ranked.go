package source

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"leaguepingo/internal/core"
)

const rankedAttemptBudget = 1200 * time.Millisecond

type rankedQueue struct {
	Queue                   string  `json:"queueType"`
	Tier                    *string `json:"tier"`
	Division                string  `json:"division"`
	Rank                    string  `json:"rank"`
	LP                      *int    `json:"leaguePoints"`
	Wins                    *int    `json:"wins"`
	Losses                  *int    `json:"losses"`
	HighestTier             string  `json:"highestTier"`
	HighestRank             string  `json:"highestRank"`
	HighestDivision         string  `json:"highestDivision"`
	PreviousHighestTier     string  `json:"previousSeasonHighestTier"`
	PreviousHighestRank     string  `json:"previousSeasonHighestRank"`
	PreviousHighestDivision string  `json:"previousSeasonHighestDivision"`
	PreviousTier            string  `json:"previousSeasonEndTier"`
	PreviousRank            string  `json:"previousSeasonEndRank"`
	PreviousDivision        string  `json:"previousSeasonEndDivision"`
}

func (s *TencentLCU) loadRanks(ctx context.Context, puuid string, out *core.Profile) {
	// This is part of the existing scheduled profile operation, not a second
	// preparation round. Reserve time for the local fallback within its budget.
	attempt := rankedAttemptBudget
	if deadline, ok := ctx.Deadline(); ok {
		attempt = min(attempt, max(0, time.Until(deadline)/2))
	}
	remoteCtx, cancel := context.WithTimeout(ctx, attempt)
	ranks, remoteErr := s.sgpRanks(remoteCtx, puuid)
	cancel()
	ranks = s.normalizeRankedCounters(puuid, ranks)
	if ranks == nil {
		ranks = []core.Rank{}
	}
	if len(ranks) == 2 {
		out.Ranks, out.RankState, out.RankMessage = ranks, "ready", ""
		return
	}
	local, err := s.lcuRanks(ctx, puuid)
	local = s.normalizeRankedCounters(puuid, local)
	// Choose a whole queue from one source. Never combine SGP wins with LCU
	// losses, rank positions or seasons, even if one field is missing.
	for _, rank := range local {
		found := false
		for _, remote := range ranks {
			found = found || rank.Queue == remote.Queue
		}
		if !found {
			ranks = append(ranks, rank)
		}
	}
	out.Ranks = ranks
	if len(ranks) > 0 {
		out.RankState, out.RankMessage = "ready", ""
		return
	}
	out.RankState, out.RankMessage = "unavailable", "客户端未返回可核验段位，不能据此判断未定级"
	if err != nil {
		out.RankState, out.RankMessage = "failed", "段位查询失败，可重试"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			out.RankMessage = "段位查询超时，可重试"
		}
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			out.RankMessage = "段位查询已取消，可重试"
		}
	} else if remoteErr != nil && ctx.Err() != nil {
		out.RankState, out.RankMessage = "failed", "段位查询超时，可重试"
	}
}

// Tencent's ranked endpoints can expose a non-self player's tier, LP and wins
// while returning losses as a literal zero. That value is not a verified
// undefeated record: using it produces the repeated 100% rates seen across a
// whole lobby. Preserve the rank fields and wins, but leave the restricted
// loss count unknown so the UI can use the independently scanned season
// history. A cached, verified current account keeps its explicit zero.
func (s *TencentLCU) normalizeRankedCounters(puuid string, ranks []core.Rank) []core.Rank {
	if puuid != "" && s.CachedAccountIdentity() == puuid {
		return ranks
	}
	for i := range ranks {
		if ranks[i].Wins != nil && *ranks[i].Wins > 0 && ranks[i].Losses != nil && *ranks[i].Losses == 0 {
			ranks[i].Losses = nil
		}
	}
	return ranks
}

func (s *TencentLCU) lcuRanks(ctx context.Context, puuid string) ([]core.Rank, error) {
	raw, err := s.client.Get(ctx, "/lol-ranked/v1/ranked-stats/"+url.PathEscape(puuid))
	if err != nil {
		return nil, err
	}
	return decodeRankedStats(raw, puuid, "lcu")
}

func (s *TencentLCU) sgpRanks(ctx context.Context, puuid string) ([]core.Rank, error) {
	gate := &s.channels.rank
	if err := gate.blocked(time.Now()); err != nil {
		return nil, err
	}
	// The ranked service needs league-session, not the entitlements token used
	// by SUMMARY history. Refreshing either session performs local I/O without
	// holding sgpMu, so a cold channel cannot block another one.
	session, err := s.sgpSession(ctx, true, gate)
	if err != nil {
		return nil, err
	}
	// Rank reads its own gate and transport slot: a stalled history page no
	// longer holds this request behind the shared lock.
	raw, err := s.getSGPAuthorized(ctx, session, "/leagues-ledge/v2/rankedStats/puuid/"+url.PathEscape(puuid), "/sgp-rank", gate)
	if err != nil {
		return nil, err
	}
	return decodeRankedStats(raw, puuid, "sgp")
}

func decodeRankedStats(raw []byte, puuid, source string) ([]core.Rank, error) {
	var response struct {
		PUUID    string                 `json:"puuid"`
		Queues   json.RawMessage        `json:"queues"`
		QueueMap map[string]rankedQueue `json:"queueMap"`
		Seasons  map[string]struct {
			ID *int `json:"currentSeasonId"`
		} `json:"seasons"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return nil, errors.New("段位响应格式暂不兼容")
	}
	if response.PUUID != "" && response.PUUID != puuid {
		return nil, errors.New("段位返回身份不匹配")
	}
	var queues []rankedQueue
	addMap := func(mapped map[string]rankedQueue) {
		for key, q := range mapped {
			if q.Queue == "" {
				q.Queue = key
			}
			if q.Queue == key {
				queues = append(queues, q)
			}
		}
	}
	if len(response.Queues) > 0 && string(response.Queues) != "null" {
		if json.Unmarshal(response.Queues, &queues) != nil {
			var mapped map[string]rankedQueue
			if json.Unmarshal(response.Queues, &mapped) != nil {
				return nil, errors.New("段位响应格式暂不兼容")
			}
			addMap(mapped)
		}
	}
	addMap(response.QueueMap)
	byQueue, conflicts := map[string]core.Rank{}, map[string]bool{}
	uncertainCounts := map[string]bool{}
	for _, q := range queues {
		if (q.Queue != "RANKED_SOLO_5x5" && q.Queue != "RANKED_FLEX_SR") || q.Tier == nil || !validTier(*q.Tier) {
			continue
		}
		division := q.Division
		if source == "sgp" {
			division = q.Rank
		}
		rank := core.Rank{Queue: q.Queue, Tier: *q.Tier, Division: division, LP: validCount(q.LP), Wins: validCount(q.Wins), Losses: validCount(q.Losses), Source: source}
		if validTier(q.HighestTier) {
			rank.HighestTier = q.HighestTier
			rank.HighestDivision = q.HighestDivision
			if source == "sgp" {
				rank.HighestDivision = q.HighestRank
			}
		}
		if validTier(q.PreviousHighestTier) {
			rank.PreviousHighestTier = q.PreviousHighestTier
			rank.PreviousHighestDivision = q.PreviousHighestDivision
			if source == "sgp" {
				rank.PreviousHighestDivision = q.PreviousHighestRank
			}
		}
		if validTier(q.PreviousTier) {
			rank.PreviousTier = q.PreviousTier
			rank.PreviousDivision = q.PreviousDivision
			if source == "sgp" {
				rank.PreviousDivision = q.PreviousRank
			}
		}
		if source == "sgp" {
			if id := response.Seasons[q.Queue].ID; id != nil && *id > 0 {
				rank.SeasonID = id
			}
		}
		if old, ok := byQueue[q.Queue]; ok {
			if (old.Wins != nil && rank.Wins != nil && *old.Wins != *rank.Wins) || (old.Losses != nil && rank.Losses != nil && *old.Losses != *rank.Losses) {
				uncertainCounts[q.Queue] = true
			}
			var conflict bool
			rank, conflict = mergeRankedQueue(rank, old)
			if conflict {
				conflicts[q.Queue] = true
			}
		}
		byQueue[q.Queue] = rank
	}
	out := []core.Rank{}
	for _, queue := range []string{"RANKED_SOLO_5x5", "RANKED_FLEX_SR"} {
		if rank, ok := byQueue[queue]; ok && !conflicts[queue] {
			if uncertainCounts[queue] {
				rank.Wins, rank.Losses = nil, nil
			}
			out = append(out, rank)
		}
	}
	return out, nil
}

func mergeRankedQueue(rank, old core.Rank) (core.Rank, bool) {
	conflict := false
	// Some client versions include both queues and queueMap. Fill truly
	// missing fields, but do not let container order choose conflicting
	// counters (in particular 0 losses versus a nonzero second value).
	counterConflict := false
	merge := func(next **int, previous *int) {
		if *next == nil {
			*next = previous
		} else if previous != nil && **next != *previous {
			*next = nil
			counterConflict = true
		}
	}
	merge(&rank.LP, old.LP)
	counterConflict = false
	merge(&rank.Wins, old.Wins)
	merge(&rank.Losses, old.Losses)
	if counterConflict {
		rank.Wins, rank.Losses = nil, nil
	}
	if rank.Division == "" {
		rank.Division = old.Division
	}
	// queueMap may only supplement current counters. Keep history from
	// the same response instead of erasing it with an absent field.
	if rank.HighestTier == "" {
		rank.HighestTier, rank.HighestDivision = old.HighestTier, old.HighestDivision
	} else if rank.HighestTier == old.HighestTier && rank.HighestDivision == "" {
		rank.HighestDivision = old.HighestDivision
	}
	if rank.PreviousHighestTier == "" {
		rank.PreviousHighestTier, rank.PreviousHighestDivision = old.PreviousHighestTier, old.PreviousHighestDivision
	} else if rank.PreviousHighestTier == old.PreviousHighestTier && rank.PreviousHighestDivision == "" {
		rank.PreviousHighestDivision = old.PreviousHighestDivision
	}
	if rank.PreviousTier == "" {
		rank.PreviousTier, rank.PreviousDivision = old.PreviousTier, old.PreviousDivision
	} else if rank.PreviousTier == old.PreviousTier && rank.PreviousDivision == "" {
		rank.PreviousDivision = old.PreviousDivision
	}
	if rank.Tier != old.Tier || (old.Division != "" && rank.Division != old.Division) {
		conflict = true
	}
	return rank, conflict
}
