package core

// Preserve the old range on a partial refresh, but merge each mode independently.
// A newly discovered mayhem sample must not be discarded because an old report
// contained more solo games. Published slices are never mutated.
func MergeScoutReports(old *ScoutReport, next ScoutReport) ScoutReport {
	if old == nil || next.State == "ready" || old.Read <= next.Read || !old.Complete30 && next.Read != 0 {
		return next
	}
	sameSeason := old.Season == nil && next.Season == nil || old.Season != nil && next.Season != nil && old.Season.Start.Equal(next.Season.Start) || old.Season != nil && next.Season == nil && !next.FetchedAt.Before(old.Season.Start) && next.FetchedAt.Before(old.Season.End)
	if !sameSeason {
		return next
	}
	result := *old
	result.State, result.Retained, result.Message = next.State, true, "保留上次统计"
	result.Queues = append([]ScoutQueue(nil), old.Queues...)
	result.Champions = append([]ScoutChampion(nil), old.Champions...)
	for _, q := range next.Queues {
		replace := true
		for i, previous := range result.Queues {
			if previous.QueueID != q.QueueID {
				continue
			}
			replace = scoutQueueImproved(previous, q)
			if replace {
				result.Queues[i] = q
			}
			break
		}
		if !replace {
			continue
		}
		present := false
		for _, previous := range result.Queues {
			present = present || previous.QueueID == q.QueueID
		}
		if !present {
			result.Queues = append(result.Queues, q)
		}
		mergeScoutHeroes(&result, next.Champions, q.QueueID)
	}
	return result
}

func scoutQueueImproved(old, next ScoutQueue) bool {
	if old.Coverage != nil && next.Coverage != nil {
		if next.Coverage.Complete30 != old.Coverage.Complete30 {
			return next.Coverage.Complete30
		}
	}
	return next.Month.OutcomeGames > old.Month.OutcomeGames || next.Month.KDAGames > old.Month.KDAGames || next.Month.Games > old.Month.Games
}

func mergeScoutHeroes(result *ScoutReport, heroes []ScoutChampion, queue int) {
	for _, incoming := range heroes {
		index := -1
		for i, h := range result.Champions {
			if h.ChampionID == incoming.ChampionID {
				index = i
				break
			}
		}
		if index < 0 {
			result.Champions = append(result.Champions, incoming)
			continue
		}
		target := &result.Champions[index]
		target.Queues = append([]ScoutHeroQueue(nil), target.Queues...)
		if incoming.Mastery != nil {
			target.Mastery = incoming.Mastery
		}
		for _, q := range incoming.Queues {
			if q.QueueID != queue {
				continue
			}
			found := false
			for i, previous := range target.Queues {
				if previous.QueueID == queue {
					target.Queues[i], found = q, true
					break
				}
			}
			if !found {
				target.Queues = append(target.Queues, q)
			}
		}
	}
}
