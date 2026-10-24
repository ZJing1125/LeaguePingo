package source

// Explicit live checks live in the test binary, never in the application or its
// preparation loops. Reports contain allow-listed counters/coverage only. Raw
// responses, identities, game IDs, destinations and credentials stay in memory.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/lcu"
)

type auditRank struct {
	Source string `json:"source"`
	Queue  string `json:"queue"`
	Tier   string `json:"tier"`
	LP     *int   `json:"lp"`
	Wins   *int   `json:"wins"`
	Losses *int   `json:"losses"`
	Season *int   `json:"season"`
}
type auditStep struct {
	Section   string         `json:"section"`
	State     string         `json:"state"`
	ElapsedMS int64          `json:"elapsedMs"`
	Counts    map[string]int `json:"counts,omitempty"`
	Ranks     []auditRank    `json:"ranks,omitempty"`
	Different []string       `json:"different,omitempty"`
}
type playerAudit struct {
	Schema        int         `json:"schema"`
	Section       string      `json:"section"`
	BudgetSeconds int         `json:"budgetSeconds"`
	Steps         []auditStep `json:"steps"`
}

func TestLivePlayerDataAudit(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_LIVE_DATA_AUDIT") != "1" {
		t.Skip("explicit Windows client audit only; no real data verified")
	}
	section := os.Getenv("LEAGUEPINGO_AUDIT_SECTION")
	if section == "" {
		section = "all"
	}
	if section != "all" && section != "rank" && section != "history" && section != "match" {
		t.Fatal("audit section must be all/rank/history/match")
	}
	c, err := lcu.Discover()
	if err != nil {
		t.Fatal("local client unavailable; audit not executed (redacted)")
	}
	s := &TencentLCU{client: c}
	defer func() {
		c.HTTP.CloseIdleConnections()
		if s.sgp != nil && s.sgp.http != nil {
			s.sgp.http.CloseIdleConnections()
		}
		s.ResetIdentity()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	self, err := s.currentPUUID(ctx)
	if err != nil || !core.IsKnownPUUID(self) {
		t.Fatal("current account unavailable (redacted)")
	}
	// Check account and phase before AND after each operation. Do not allow an
	// account switch or entering selection/gameplay to yield a passing report.
	guard := func() error {
		phase, err := s.Phase(ctx)
		if err != nil {
			return errors.New("phase unavailable")
		}
		switch phase {
		case "None", "Lobby", "EndOfGame", "PreEndOfGame", "WaitingForStats":
		default:
			return errors.New("audit requires idle/lobby/postgame")
		}
		current, err := s.AccountIdentity(ctx)
		if err != nil || current != self {
			return errors.New("account changed or unavailable")
		}
		return nil
	}
	if guard() != nil {
		t.Fatal("idle/lobby/postgame with stable account required; not verified")
	}
	target := self
	if input := os.Getenv("LEAGUEPINGO_AUDIT_TARGET"); input != "" {
		p, lookupErr := s.FindByRiotID(ctx, input)
		if lookupErr != nil || !core.IsKnownPUUID(p.PUUID) || guard() != nil {
			t.Fatal("target lookup incomplete (redacted)")
		}
		target = p.PUUID
	}
	report := runPlayerAudit(ctx, s, target, section, guard)
	raw, _ := json.Marshal(report)
	t.Log("PLAYER_DATA_AUDIT " + string(raw))
	for _, step := range report.Steps {
		if step.State != "verified" {
			t.Fatal("audit incomplete; inspect section states; source equality is not WeGame/UI acceptance")
		}
	}
	t.Log("read checks completed; independently compare same-queue/same-season client or WeGame values and product UI")
}

func runPlayerAudit(ctx context.Context, s *TencentLCU, target, section string, guard func() error) playerAudit {
	r := playerAudit{Schema: 1, Section: section, BudgetSeconds: 45, Steps: []auditStep{}}
	stopped := false
	run := func(name string, budget time.Duration, fn func(context.Context, *auditStep) error) {
		step := auditStep{Section: name, State: "not_run", Counts: map[string]int{}}
		if stopped || ctx.Err() != nil {
			r.Steps = append(r.Steps, step)
			return
		}
		if guard() != nil {
			stopped = true
			step.State = "interrupted"
			r.Steps = append(r.Steps, step)
			return
		}
		started := time.Now()
		request, cancel := context.WithTimeout(ctx, budget)
		err := fn(request, &step)
		if errors.Is(request.Err(), context.DeadlineExceeded) {
			step.Counts["deadline_exceeded"] = 1
		}
		cancel()
		step.ElapsedMS = time.Since(started).Milliseconds()
		if err != nil {
			step.State = "failed"
		} else if step.State == "not_run" {
			step.State = "verified"
		}
		if guard() != nil {
			step.State = "interrupted"
			stopped = true
		}
		r.Steps = append(r.Steps, step)
	}
	if section == "all" || section == "rank" {
		run("rank_lcu", 3*time.Second, func(ctx context.Context, step *auditStep) error {
			raw, err := s.client.Get(ctx, "/lol-ranked/v1/ranked-stats/"+url.PathEscape(target))
			if err != nil {
				return err
			}
			step.Counts["response_received"] = 1
			return auditLocalRanks(raw, target, step)
		})
		run("rank_sgp", rankedAttemptBudget, func(ctx context.Context, step *auditStep) error {
			ranks, err := s.sgpRanks(ctx, target)
			if err != nil {
				return err
			}
			appendAuditRanks(step, "sgp", ranks)
			if len(ranks) != 2 {
				step.State = "partial"
			}
			return nil
		})
		run("rank_compare", time.Second, func(_ context.Context, step *auditStep) error {
			step.Counts["compared_queues"], step.Counts["same_known_season"] = 0, 0
			var local, remote []auditRank
			for _, prior := range r.Steps {
				for _, rank := range prior.Ranks {
					if rank.Source == "lcu/combined" {
						local = append(local, rank)
					}
					if rank.Source == "sgp" {
						remote = append(remote, rank)
					}
				}
			}
			for _, a := range local {
				for _, b := range remote {
					if a.Queue != b.Queue {
						continue
					}
					step.Counts["compared_queues"]++
					for _, field := range []struct {
						name string
						a, b *int
					}{{"lp", a.LP, b.LP}, {"wins", a.Wins, b.Wins}, {"losses", a.Losses, b.Losses}} {
						if !reflect.DeepEqual(field.a, field.b) {
							step.Different = append(step.Different, a.Queue+"."+field.name)
						}
					}
					if a.Tier != b.Tier {
						step.Different = append(step.Different, a.Queue+".tier")
					}
					if a.Season != nil && b.Season != nil && *a.Season == *b.Season {
						step.Counts["same_known_season"]++
					}
				}
			}
			if step.Counts["compared_queues"] != 2 {
				step.State = "partial"
			} else if len(step.Different) > 0 {
				step.State = "different"
			}
			return nil
		})
	}
	if section == "rank" {
		return r
	}
	var season *core.ScoutSeason
	run("season", 3*time.Second, func(ctx context.Context, step *auditStep) error {
		var err error
		season, err = s.ScoutSeason(ctx)
		if err == nil && season == nil {
			step.State = "partial"
		}
		return err
	})
	run("mastery", 2*time.Second, func(ctx context.Context, step *auditStep) error {
		items, err := s.ChampionMasteries(ctx, target)
		step.Counts["champions"] = len(items)
		return err
	})
	cutoff := time.Now().UTC()
	var matches []core.Match
	seen := map[int64]bool{}
	offset := 0
	more := true
	invalid := false
	var oldest time.Time
	pages := 3
	if section == "match" {
		pages = 1
	}
	for page := 0; page < pages && more; page++ {
		run(fmt.Sprintf("history_page_%d", page+1), 4*time.Second, func(ctx context.Context, step *auditStep) error {
			p, err := s.ScoutPage(ctx, target, offset)
			if err != nil {
				more = false
				invalid = true
				return err
			}
			step.Counts["returned"], step.Counts["parsed"], step.Counts["excluded"] = p.Meta.Returned, len(p.Matches), p.Meta.Excluded
			if p.Offset != offset || p.NextOffset != offset+p.Meta.Returned || p.Meta.Returned < 0 || p.Meta.Returned > 20 || p.LimitReason != "" {
				invalid = true
			}
			for _, m := range p.Matches {
				if seen[m.GameID] || m.GameID <= 0 {
					step.Counts["duplicates"]++
					invalid = true
					continue
				}
				seen[m.GameID] = true
				if m.PlayedAt == nil || m.PlayedAt.After(cutoff) || !oldest.IsZero() && m.PlayedAt.After(oldest) {
					invalid = true
				} else {
					oldest = *m.PlayedAt
				}
				matches = append(matches, m)
			}
			if p.Meta.Excluded > 0 {
				invalid = true
			}
			if invalid {
				step.State = "partial"
			}
			offset = p.NextOffset
			more = p.HasMore && !invalid
			return nil
		})
	}
	run("sample_analysis", time.Second, func(_ context.Context, step *auditStep) error {
		complete := !invalid && !oldest.IsZero()
		report := core.BuildScoutReport(matches, nil, season, cutoff, complete && season != nil && !oldest.After(season.Start), complete && !oldest.After(cutoff.Add(-12*time.Hour)), complete && !oldest.After(cutoff.Add(-30*24*time.Hour)))
		step.Counts["unique_games"] = len(seen)
		for _, flag := range []struct {
			name  string
			value bool
		}{{"season_boundary_reached", report.Complete}, {"hours12_boundary_reached", report.Complete12}, {"days30_boundary_reached", report.Complete30}} {
			if flag.value {
				step.Counts[flag.name] = 1
			} else {
				step.Counts[flag.name] = 0
			}
		}
		for _, q := range report.Queues {
			prefix := fmt.Sprintf("queue%d_", q.QueueID)
			step.Counts[prefix+"season_games"] = q.Season.Games
			step.Counts[prefix+"season_known_outcomes"] = q.Season.OutcomeGames
			step.Counts[prefix+"season_wins"] = q.Season.Wins
			step.Counts[prefix+"season_losses"] = q.Season.Losses
			step.Counts[prefix+"hours12_games"] = q.Twelve.Games
			step.Counts[prefix+"hours12_wins"] = q.Twelve.Wins
			step.Counts[prefix+"hours12_losses"] = q.Twelve.Losses
			step.Counts[prefix+"month_games"] = q.Month.Games
			step.Counts[prefix+"month_known_outcomes"] = q.Month.OutcomeGames
			step.Counts[prefix+"month_wins"] = q.Month.Wins
			step.Counts[prefix+"month_losses"] = q.Month.Losses
			for _, p := range q.Positions {
				step.Counts[prefix+"month_position_games"] += p.Games
			}
		}
		if invalid || len(matches) == 0 {
			step.State = "partial"
		}
		return nil
	})
	if section == "history" {
		return r
	}
	// Prefer ranked games; do not scan beyond the explicitly bounded pages.
	var chosen *core.Match
	for i := range matches {
		if matches[i].QueueID == 420 || matches[i].QueueID == 440 {
			chosen = &matches[i]
			break
		}
	}
	if chosen == nil && len(matches) > 0 {
		chosen = &matches[0]
	}
	var local, remote *core.MatchDetail
	for _, source := range []string{"lcu", "sgp"} {
		run("detail_"+source, 5*time.Second, func(ctx context.Context, step *auditStep) error {
			if chosen == nil {
				step.State = "not_available"
				return nil
			}
			var d core.MatchDetail
			var err error
			if source == "lcu" {
				var raw []byte
				raw, err = s.client.Get(ctx, fmt.Sprintf("/lol-match-history/v1/games/%d", chosen.GameID))
				if err == nil {
					d, err = core.ParseMatchDetail(raw, chosen.GameID, target)
				}
			} else {
				var raw []byte
				var platform string
				raw, platform, err = s.sgpGameData(ctx, chosen.GameID, "SUMMARY", "/sgp-detail")
				if err == nil {
					d, err = core.ParseSGPMatchDetail(raw, chosen.GameID, target, platform)
				}
			}
			if err != nil {
				return err
			}
			step.Counts = auditDetailCoverage(d)
			if source == "lcu" {
				local = &d
			} else {
				remote = &d
			}
			return nil
		})
	}
	run("detail_compare", time.Second, func(_ context.Context, step *auditStep) error {
		if local == nil || remote == nil {
			step.State = "not_available"
			return nil
		}
		step.Different = auditDetailDifferences(*local, *remote)
		if len(step.Different) > 0 {
			step.State = "different"
		}
		return nil
	})
	detail := remote
	if detail == nil {
		detail = local
	}
	for _, source := range []string{"lcu", "sgp"} {
		run("timeline_"+source, 5*time.Second, func(ctx context.Context, step *auditStep) error {
			if detail == nil {
				step.State = "not_available"
				return nil
			}
			var data core.MatchTimeline
			var err error
			var raw []byte
			if source == "lcu" {
				raw, err = s.client.Get(ctx, fmt.Sprintf("/lol-match-history/v1/game-timelines/%d", detail.GameID))
				if err == nil {
					data, err = core.ParseMatchTimeline(raw, *detail)
				}
			} else {
				var platform string
				raw, platform, err = s.sgpGameData(ctx, detail.GameID, "DETAILS", "/sgp-timeline")
				if err == nil {
					data, err = core.ParseSGPTimeline(raw, *detail, target, platform)
				}
			}
			if err != nil {
				return err
			}
			step.Counts = auditTimelineCoverage(data, *detail)
			return nil
		})
	}
	return r
}

func appendAuditRanks(step *auditStep, source string, ranks []core.Rank) {
	for _, r := range ranks {
		step.Ranks = append(step.Ranks, auditRank{source, r.Queue, r.Tier, r.LP, r.Wins, r.Losses, r.SeasonID})
	}
}

func auditLocalRanks(raw []byte, target string, step *auditStep) error {
	var body struct {
		PUUID    string          `json:"puuid"`
		Queues   json.RawMessage `json:"queues"`
		QueueMap json.RawMessage `json:"queueMap"`
	}
	if json.Unmarshal(raw, &body) != nil || body.PUUID != "" && body.PUUID != target {
		return errors.New("invalid ranked identity or format")
	}
	for _, item := range []struct {
		name string
		raw  json.RawMessage
	}{{"queues", body.Queues}, {"queueMap", body.QueueMap}} {
		if len(item.raw) == 0 || string(item.raw) == "null" {
			continue
		}
		payload, _ := json.Marshal(map[string]json.RawMessage{item.name: item.raw})
		ranks, err := decodeRankedStats(payload, target, "lcu")
		if err != nil {
			return err
		}
		appendAuditRanks(step, "lcu/"+item.name, ranks)
	}
	ranks, err := decodeRankedStats(raw, target, "lcu")
	if err != nil {
		return err
	}
	appendAuditRanks(step, "lcu/combined", ranks)
	if len(ranks) != 2 {
		step.State = "partial"
	}
	return nil
}

// Counts reveal coverage without exporting even anonymized per-player records.
func auditDetailCoverage(d core.MatchDetail) map[string]int {
	c := map[string]int{"players": len(d.Players)}
	for _, key := range []string{"full_5v5", "targets", "kills", "deaths", "assists", "damage", "taken", "gold", "cs", "vision", "wards_placed", "wards_killed", "control_wards", "spell1", "spell2", "level", "seven_item_slots", "runes", "rune_shards", "damage_share", "gold_share", "participation", "teams_with_towers", "teams_with_dragons", "teams_with_barons", "teams_with_inhibitors", "teams_with_heralds", "teams_with_hordes"} {
		c[key] = 0
	}
	if !d.Partial {
		c["full_5v5"] = 1
	}
	for _, team := range d.Teams {
		for key, value := range map[string]*int{"towers": team.Towers, "dragons": team.Dragons, "barons": team.Barons, "inhibitors": team.Inhibitors, "heralds": team.Heralds, "hordes": team.Hordes} {
			if value != nil {
				c["teams_with_"+key]++
			}
		}
	}
	for _, p := range d.Players {
		m := p.Summary
		for k, v := range map[string]*int{"kills": m.Kills, "deaths": m.Deaths, "assists": m.Assists, "damage": m.Damage, "taken": m.DamageTaken, "gold": m.Gold, "cs": m.CS, "vision": m.VisionScore, "wards_placed": m.WardsPlaced, "wards_killed": m.WardsKilled, "control_wards": m.ControlWards, "spell1": m.Spell1, "spell2": m.Spell2, "level": m.ChampionLevel} {
			if v != nil {
				c[k]++
			}
		}
		if len(m.Items) == 7 {
			known := true
			for _, id := range m.Items {
				known = known && id != nil
			}
			if known {
				c["seven_item_slots"]++
			}
		}
		if len(m.RuneIDs) > 0 {
			c["runes"]++
		}
		if len(m.RuneShards) == 3 {
			c["rune_shards"]++
		}
		if p.DamageShare != nil {
			c["damage_share"]++
		}
		if p.GoldShare != nil {
			c["gold_share"]++
		}
		if p.Participation != nil {
			c["participation"]++
		}
		if p.Target {
			c["targets"]++
		}
	}
	return c
}

func auditDetailDifferences(a, b core.MatchDetail) []string {
	differences := []string{}
	add := func(name string, x, y any) {
		if !reflect.DeepEqual(x, y) {
			differences = append(differences, name)
		}
	}
	add("players", len(a.Players), len(b.Players))
	add("map", a.MapID, b.MapID)
	teams := func(d core.MatchDetail) map[int]core.MatchTeam {
		out := map[int]core.MatchTeam{}
		for _, team := range d.Teams {
			out[team.TeamID] = team
		}
		return out
	}
	add("team_objectives", teams(a), teams(b))
	byID := map[int]core.MatchPlayer{}
	for _, p := range b.Players {
		byID[p.ParticipantID] = p
	}
	for _, p := range a.Players {
		q, ok := byID[p.ParticipantID]
		if !ok {
			differences = append(differences, "participant_set")
			continue
		}
		// No names, ids or original values in the report. Only fixed field names.
		x, y := p.Summary, q.Summary
		for _, f := range []struct {
			name string
			x, y any
		}{{"team", p.TeamID, q.TeamID}, {"target", p.Target, q.Target}, {"champion", x.ChampionID, y.ChampionID}, {"queue", x.QueueID, y.QueueID}, {"position", x.Position, y.Position}, {"win", x.Win, y.Win}, {"duration", x.Duration, y.Duration}, {"kills", x.Kills, y.Kills}, {"deaths", x.Deaths, y.Deaths}, {"assists", x.Assists, y.Assists}, {"damage", x.Damage, y.Damage}, {"taken", x.DamageTaken, y.DamageTaken}, {"gold", x.Gold, y.Gold}, {"cs", x.CS, y.CS}, {"vision", x.VisionScore, y.VisionScore}, {"wards_placed", x.WardsPlaced, y.WardsPlaced}, {"wards_killed", x.WardsKilled, y.WardsKilled}, {"control_wards", x.ControlWards, y.ControlWards}, {"level", x.ChampionLevel, y.ChampionLevel}, {"spell1", x.Spell1, y.Spell1}, {"spell2", x.Spell2, y.Spell2}, {"items", x.Items, y.Items}, {"rune_primary", x.RunePrimary, y.RunePrimary}, {"rune_secondary", x.RuneSecondary, y.RuneSecondary}, {"runes", x.RuneIDs, y.RuneIDs}, {"rune_shards", x.RuneShards, y.RuneShards}} {
			if !reflect.DeepEqual(f.x, f.y) {
				found := false
				for _, name := range differences {
					found = found || name == f.name
				}
				if !found {
					differences = append(differences, f.name)
				}
			}
		}
	}
	return differences
}

func auditTimelineCoverage(t core.MatchTimeline, d core.MatchDetail) map[string]int {
	c := map[string]int{"frames": len(t.Frames), "events": len(t.Events)}
	for _, key := range []string{"full_team_gold_frames", "positions", "CHAMPION_KILL", "BUILDING_KILL", "ELITE_MONSTER_KILL", "ITEM_PURCHASED", "ITEM_SOLD", "ITEM_DESTROYED", "ITEM_UNDO", "SKILL_LEVEL_UP"} {
		c[key] = 0
	}
	for _, f := range t.Frames {
		gold := len(f.Players) == len(d.Players) && !d.Partial
		for _, p := range f.Players {
			gold = gold && p.Gold != nil
			if p.Position != nil {
				c["positions"]++
			}
		}
		if gold {
			c["full_team_gold_frames"]++
		}
	}
	for _, e := range t.Events {
		c[e.Type]++
	}
	return c
}
