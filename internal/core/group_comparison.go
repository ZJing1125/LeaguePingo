package core

import "errors"

// MergeGroupFields attaches explicit metadata to an existing roster. It never
// adds identities, changes sides, or interprets missing identifiers as solo.
func MergeGroupFields(roster, extra []Participant) ([]Participant, error) {
	if len(extra) > 10 {
		return nil, errors.New("分组名单人数异常")
	}
	seen := map[string]Participant{}
	for _, p := range extra {
		if !IsKnownPUUID(p.PUUID) {
			continue
		}
		if p.Team != "ally" && p.Team != "enemy" {
			return nil, errors.New("分组阵营异常")
		}
		if _, ok := seen[p.PUUID]; ok {
			return nil, errors.New("分组身份重复")
		}
		seen[p.PUUID] = p
	}
	result := append([]Participant{}, roster...)
	remote := map[string]bool{}
	for i := range result {
		p := &result[i]
		q, ok := seen[p.PUUID]
		if !ok {
			continue
		}
		if p.Team != q.Team || p.GroupToken != "" && q.GroupToken != "" && p.GroupToken != q.GroupToken {
			return nil, errors.New("分组数据与当前名单冲突")
		}
		if p.GroupToken == "" && q.GroupToken != "" {
			p.GroupToken = q.GroupToken
			remote[p.PUUID] = true
		}
		if p.Position == "" {
			p.Position = q.Position
		}
	}
	AssignPremades(result, "lcu")
	for i := range result {
		if remote[result[i].PUUID] {
			result[i].GroupSource = "sgp"
		}
	}
	return result, nil
}

type GroupComparison struct {
	Players         int
	KnownPlayers    int
	ComparedPairs   int
	CorrectTogether int
	WrongMerge      int
	WrongSplit      int
	UnknownPairs    int
}

// Truth is an explicit ten-slot partition supplied by an observer who knows
// both real parties. Labels are scoped to each side; singleton labels are valid.
func CompareGroupSlots(players []Participant, truth []int) (GroupComparison, error) {
	r := GroupComparison{Players: len(players)}
	if len(players) != 10 || len(truth) != 10 {
		return r, errors.New("必须提供当前双方十个槽位的真实组队基准")
	}
	sides := map[string]int{}
	for i, p := range players {
		if truth[i] <= 0 || p.Team != "ally" && p.Team != "enemy" {
			return r, errors.New("槽位基准格式无效")
		}
		sides[p.Team]++
		if IsKnownPUUID(p.PUUID) && p.GroupToken != "" {
			r.KnownPlayers++
		}
	}
	if sides["ally"] != 5 || sides["enemy"] != 5 {
		return r, errors.New("仅核验双方各五人的匹配队伍")
	}
	for i, a := range players {
		for j := i + 1; j < len(players); j++ {
			b := players[j]
			if a.Team != b.Team {
				continue
			}
			if !IsKnownPUUID(a.PUUID) || !IsKnownPUUID(b.PUUID) || a.GroupToken == "" || b.GroupToken == "" {
				r.UnknownPairs++
				continue
			}
			r.ComparedPairs++
			actual := a.GroupToken == b.GroupToken
			expected := truth[i] == truth[j]
			if actual && expected {
				r.CorrectTogether++
			}
			if actual && !expected {
				r.WrongMerge++
			}
			if !actual && expected {
				r.WrongSplit++
			}
		}
	}
	return r, nil
}
