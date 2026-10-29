package core

import (
	"fmt"
	"testing"
)

func TestGroupComparisonBothTeamsUnknownAndMistakes(t *testing.T) {
	truth := []int{1, 1, 2, 2, 2, 1, 1, 2, 3, 4}
	players := make([]Participant, 10)
	for i := range players {
		side := "ally"
		if i >= 5 {
			side = "enemy"
		}
		players[i] = Participant{PUUID: fmt.Sprint(i + 1), Team: side, GroupToken: fmt.Sprint(truth[i])}
	}
	r, err := CompareGroupSlots(players, truth)
	if err != nil || r.KnownPlayers != 10 || r.ComparedPairs != 20 || r.WrongMerge != 0 || r.WrongSplit != 0 {
		t.Fatal(r, err)
	}
	players[0].GroupToken = "2"
	r, _ = CompareGroupSlots(players, truth)
	if r.WrongMerge != 3 || r.WrongSplit != 1 {
		t.Fatal("incorrect merge/split counts", r)
	}
	players[0].GroupToken = ""
	r, _ = CompareGroupSlots(players, truth)
	if r.KnownPlayers != 9 || r.UnknownPairs != 4 {
		t.Fatal("unknown treated as solo", r)
	}
	if _, err := CompareGroupSlots(players, truth[:5]); err == nil {
		t.Fatal("partial ground truth accepted")
	}
}
func TestGroupMetadataMergeRejectsConflictAndPreservesRoster(t *testing.T) {
	local := []Participant{{PUUID: "self", Team: "ally"}, {PUUID: "ally", Team: "ally", GroupToken: "2"}}
	extra := []Participant{{PUUID: "self", Team: "ally", GroupToken: "2"}, {PUUID: "ally", Team: "ally", GroupToken: "2"}, {PUUID: "not-in-roster", Team: "enemy", GroupToken: "4"}}
	ps, err := MergeGroupFields(local, extra)
	if err != nil || len(ps) != 2 || ps[0].PremadeSize != 2 || ps[0].GroupSource != "sgp" {
		t.Fatal(ps, err)
	}
	extra[0].Team = "enemy"
	if _, err := MergeGroupFields(local, extra); err == nil {
		t.Fatal("wrong side accepted")
	}
	extra[0] = extra[1]
	if _, err := MergeGroupFields(local, extra); err == nil {
		t.Fatal("duplicate identities accepted")
	}
}

func TestGroupComparisonCountsExplicitAnonymousSlots(t *testing.T) {
	truth := []int{1, 1, 2, 2, 2, 1, 1, 2, 3, 4}
	players := make([]Participant, 10)
	for i := range players {
		side := "ally"
		if i >= 5 {
			side = "enemy"
		}
		players[i] = Participant{Key: fmt.Sprintf("slot-%d", i), Team: side, GroupToken: fmt.Sprint(truth[i])}
	}
	r, err := CompareGroupSlots(players, truth)
	if err != nil || r.KnownPlayers != 10 || r.ComparedPairs != 20 || r.UnknownPairs != 0 || r.WrongMerge != 0 || r.WrongSplit != 0 {
		t.Fatal("explicit anonymous group coverage lost", r, err)
	}
	players[0].GroupToken = ""
	r, _ = CompareGroupSlots(players, truth)
	if r.KnownPlayers != 9 || r.UnknownPairs != 4 {
		t.Fatal("anonymous slot without source field counted as known", r)
	}
}
