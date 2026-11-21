package source

import (
	"context"
	"leaguepingo/internal/core"
	"net/http"
	"strings"
	"testing"
)

func TestMissingTimelineCategoriesReplaceWholeTargetSequence(t *testing.T) {
	duration := 1200
	detail := core.MatchDetail{Players: []core.MatchPlayer{{ParticipantID: 6, Target: true, Summary: core.Match{Duration: &duration}}}}
	local := core.MatchTimeline{Events: []core.TimelineEvent{{ParticipantID: 6, Type: "ITEM_PURCHASED", ItemID: 1054, Timestamp: 1000}, {ParticipantID: 2, Type: "SKILL_LEVEL_UP", SkillSlot: 2}}}
	remote := core.MatchTimeline{Events: []core.TimelineEvent{{ParticipantID: 6, Type: "ITEM_PURCHASED", ItemID: 1054, Timestamp: 1000}, {ParticipantID: 6, Type: "ITEM_UNDO", BeforeID: 1054, Timestamp: 2000}, {ParticipantID: 6, Type: "SKILL_LEVEL_UP", SkillSlot: 1, Timestamp: 1000}}}
	if !timelineNeedsEvents(local, detail) {
		t.Fatal("missing target skills was accepted as complete")
	}
	merged := mergeTimelineEvents(local, remote, detail)
	if len(merged.Events) != 4 || timelineNeedsEvents(merged, detail) {
		t.Fatal("categories duplicated or other player lost")
	}
	duration = 90
	if timelineNeedsEvents(core.MatchTimeline{}, detail) {
		t.Fatal("short remake triggered missing event repair")
	}
}

func TestTimelineProductionRepairsMissingSkillsOnlyOnce(t *testing.T) {
	for _, complete := range []bool{false, true} {
		s, _, _ := rankedTestSource(t, syntheticRanked)
		transport := s.client.HTTP.Transport
		events := `[{"type":"ITEM_PURCHASED","timestamp":1000,"participantId":6,"itemId":1054}]`
		if complete {
			events = `[{"type":"ITEM_PURCHASED","timestamp":1000,"participantId":6,"itemId":1054},{"type":"SKILL_LEVEL_UP","timestamp":1000,"participantId":6,"skillSlot":1}]`
		}
		local := `{"gameId":42,"frames":[{"timestamp":0,"participantFrames":{"6":{"participantId":6,"totalGold":500}},"events":` + events + `}]}`
		s.client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/lol-match-history/v1/game-timelines/42" {
				return jsonResponse(local), nil
			}
			return transport.RoundTrip(r)
		})
		remoteCalls := 0
		s.sgp.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			remoteCalls++
			if !strings.HasSuffix(r.URL.Path, "NJ100_42/DETAILS") {
				t.Fatal("unexpected additional game source")
			}
			return jsonResponse(`{"metadata":{"product":"lol","info_type":"details","match_id":"NJ100_42"},"json":{"gameId":42,"participants":[{"participantId":6,"puuid":"target"}],"frames":[{"timestamp":0,"participantFrames":{"6":{"participantId":6,"totalGold":500}},"events":[{"type":"SKILL_LEVEL_UP","timestamp":1000,"participantId":6,"skillSlot":1}]}]}}`), nil
		})}
		duration := 1200
		detail := core.MatchDetail{GameID: 42, Players: []core.MatchPlayer{{ParticipantID: 6, Target: true, Summary: core.Match{Duration: &duration, RuneShards: []int{5008, 5008, 5001}}}}}
		got, err := s.MatchTimeline(context.Background(), detail, "target")
		expected := 1
		if complete {
			expected = 0
		}
		if err != nil || remoteCalls != expected || len(got.Events) != 2 {
			t.Fatal("bounded production coverage repair failed", complete, remoteCalls, err)
		}
	}
}
