package core

import "testing"

func TestRelativeRatingsKeepTeamsAndQueuesSeparate(t *testing.T) {
	row := func(team string, score float64) Participant {
		return Participant{Team: team, IdentityState: IdentityKnown, Scout: &ScoutReport{
			Queues: []ScoutQueue{{QueueID: 2400, Analysis: &PlayerAnalysis{Score: &score}}},
		}}
	}
	players := []Participant{row("ally", 4), row("ally", 7), row("ally", 7), row("enemy", 2), {Team: "enemy", IdentityState: IdentityUnavailable}}
	ApplyTeamRatings(players, 2400)
	for i, want := range []int{3, 1, 1, 1, 0} {
		got := players[i].TeamRating
		if (got == nil) != (want == 0) || got != nil && got.Place != want {
			t.Fatalf("row %d: place %d expected, got %+v", i, want, got)
		}
	}
	if players[0].TeamRating.Rated != 3 || players[3].TeamRating.Rated != 1 ||
		players[0].TeamRating.Label != "中坚" || players[3].TeamRating.Label != "大腿" {
		t.Fatal("team sizes or relative labels differ")
	}
	lines, ready, rated := AnalysisBriefLines(Snapshot{QueueID: 2400, Participants: players}, true)
	if !ready || rated != 3 || len(lines) != 4 || lines[1][:4] != "2楼" {
		t.Fatalf("three-person chat lost standings or order: %v, %t, %d", lines, ready, rated)
	}
	ApplyTeamRatings(players, 450)
	for _, p := range players {
		if p.TeamRating != nil {
			t.Fatal("rating crossed queues")
		}
	}
}
