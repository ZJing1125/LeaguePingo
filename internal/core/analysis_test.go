package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func timelineFixture(t *testing.T) (MatchDetail, []byte) {
	t.Helper()
	detail, err := ParseMatchDetail(detailFixture(t, false), 42, "target")
	if err != nil {
		t.Fatal(err)
	}
	frames := []any{}
	for n := 0; n < 3; n++ {
		players := map[string]any{}
		for _, p := range detail.Players {
			players[jsonNumber(p.ParticipantID)] = map[string]any{"participantId": p.ParticipantID, "totalGold": 500 + n*100, "minionsKilled": n, "jungleMinionsKilled": 0, "level": 1 + n, "position": map[string]any{"x": 1000 + n*500, "y": 2000}}
		}
		events := []any{}
		if n == 1 {
			events = append(events, map[string]any{"timestamp": 30000, "type": "CHAMPION_KILL", "killerId": 1, "victimId": 6, "position": map[string]any{"x": 3000, "y": 4000}}, map[string]any{"timestamp": 20000, "type": "ITEM_PURCHASED", "participantId": 1, "itemId": 1054})
		}
		frames = append(frames, map[string]any{"timestamp": n * 60000, "participantFrames": players, "events": events})
	}
	raw, _ := json.Marshal(map[string]any{"gameId": 42, "frames": frames})
	return detail, raw
}
func jsonNumber(n int) string { b, _ := json.Marshal(n); return string(b) }
func TestTimelineValidatesGameFramesAndPreservesActualEvents(t *testing.T) {
	d, raw := timelineFixture(t)
	got, err := ParseMatchTimeline(raw, d)
	if err != nil || got.Partial || len(got.Frames) != 3 || len(got.Events) != 2 || got.Events[0].Timestamp != 20000 || *got.Frames[1].Players[0].CS != 1 {
		t.Fatalf("timeline not preserved: %+v %v", got, err)
	}
	for _, tc := range []struct{ old, new string }{{`"gameId":42`, `"gameId":43`}, {`"timestamp":60000`, `"timestamp":0`}, {`"participantId":1,`, `"participantId":99,`}, {`"victimId":6`, `"victimId":99`}} {
		broken := strings.Replace(string(raw), tc.old, tc.new, 1)
		if broken == string(raw) {
			t.Fatal("fixture edit missed", tc)
		}
		if _, err := ParseMatchTimeline([]byte(broken), d); err == nil {
			t.Fatal("unverified timeline accepted", tc)
		}
	}
	// A missing field is unknown, including a real zero elsewhere in the frame.
	missing := strings.Replace(string(raw), `"totalGold":500`, `"totalGold":null`, 1)
	got, err = ParseMatchTimeline([]byte(missing), d)
	if err != nil || !got.Partial || got.Frames[0].Players[0].Gold != nil {
		t.Fatal("missing gold was fabricated", err)
	}
}
func TestRichSummaryRosterAndSGPDetailShareTheSameIdentity(t *testing.T) {
	raw := `{"gameId":42,"platformId":"HN1","queueId":420,"mapId":11,"participants":[{"participantId":1,"puuid":"target","teamId":100,"championId":23,"riotIdGameName":"Synthetic","riotIdTagline":"TEST","kills":0,"deaths":0,"assists":1,"champLevel":18,"totalDamageTaken":0,"visionScore":8,"wardsPlaced":4,"wardsKilled":1,"visionWardsBoughtInGame":2,"tripleKills":1,"perks":{"statPerks":{"offense":5008,"flex":5008,"defense":5001},"styles":[{"style":8000,"selections":[{"perk":8005}]},{"style":8100,"selections":[{"perk":8139}]}]}}],"teams":[{"teamId":100,"objectives":{"tower":{"kills":0},"dragon":{"kills":2}}}]}`
	envelope := `{"metadata":{"product":"lol","info_type":"summary","match_id":"HN1_42"},"json":` + raw + `}`
	got, err := ParseSGPMatchDetail([]byte(envelope), 42, "target", "HN1")
	if err != nil {
		t.Fatal(err)
	}
	p := got.Players[0]
	m := p.Summary
	if !p.Target || p.DisplayName != "Synthetic#TEST" || m.ChampionLevel == nil || *m.ChampionLevel != 18 || m.DamageTaken == nil || *m.DamageTaken != 0 || m.ControlWards == nil || *m.ControlWards != 2 || len(m.RuneShards) != 3 || len(m.RuneIDs) != 2 || m.Participation != nil || got.MapID == nil || *got.MapID != 11 {
		t.Fatalf("summary fields not preserved: %+v", got)
	}
	if len(got.Teams) != 1 || got.Teams[0].Towers == nil || *got.Teams[0].Towers != 0 || got.Teams[0].Barons != nil {
		t.Fatal("objectives confused missing and zero", got.Teams)
	}
	for _, wrong := range []string{strings.Replace(envelope, "HN1_42", "HN2_42", 1), strings.Replace(envelope, `"gameId":42`, `"gameId":99`, 1)} {
		if _, err := ParseSGPMatchDetail([]byte(wrong), 42, "target", "HN1"); err == nil {
			t.Fatal("wrong-region or game detail accepted")
		}
	}
	d, timeline := timelineFixture(t)
	body := strings.TrimSuffix(string(timeline), "}") + `,"participants":[{"participantId":1,"puuid":"target"}]}`
	env := `{"metadata":{"product":"lol","info_type":"details","match_id":"HN1_42"},"json":` + body + `}`
	if _, err := ParseSGPTimeline([]byte(env), d, "target", "HN1"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseSGPTimeline([]byte(env), d, "other", "HN1"); err == nil {
		t.Fatal("timeline target mismatch accepted")
	}
}

func TestHistoryPeerSummaryUsesOnlyTheAlreadyReturnedVerifiedGame(t *testing.T) {
	raw := detailFixture(t, false)
	page := append([]byte(`{"games":{"games":[`), raw...)
	page = append(page, []byte(`]}}`)...)
	got, err := ParseHistory(page, "target")
	if err != nil || len(got.Matches) != 1 || len(got.Matches[0].Roster) != 10 || got.Matches[0].Participation == nil || *got.Matches[0].Participation != 40 {
		t.Fatal("existing summary did not supply verified roster/KP", err)
	}
	targets := 0
	for _, p := range got.Matches[0].Roster {
		if p.Target {
			targets++
		}
		if p.DisplayName != "Fixture#TEST" {
			t.Fatal("name not matched by participant identity")
		}
	}
	if targets != 1 {
		t.Fatal("target marker not unique")
	}
	fallback := strings.Replace(string(page), `"championId":23`, `"championId":0`, 1)
	fallback = strings.Replace(fallback, `"stats":{`, `"stats":{"championId":23,`, 1)
	fromStats, err := ParseHistory([]byte(fallback), "target")
	if err != nil || fromStats.Matches[0].ChampionID != 23 || fromStats.Matches[0].Roster[0].ChampionID != 23 {
		t.Fatal("roster and player chose different champion fields", err)
	}
	broken := strings.Replace(string(page), `"kills":1`, `"kills":null`, 1)
	got, err = ParseHistory([]byte(broken), "target")
	if err != nil || len(got.Matches[0].Roster) != 10 || got.Matches[0].Participation != nil {
		t.Fatal("missing team kills produced KP", err)
	}
}
