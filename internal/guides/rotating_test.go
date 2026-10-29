package guides

import (
	"context"
	"leaguepingo/internal/core"
	"os"
	"strings"
	"testing"
	"time"
)

func TestExtendedModeIsolation(t *testing.T) {
	for _, tc := range []struct {
		q, mapID   int
		game, mode string
	}{{900, 11, "URF", "urf"}, {1900, 11, "URF", "urf"}, {1300, 21, "NEXUSBLITZ", "nexus-blitz"}, {1700, 30, "CHERRY", "arena"}, {1710, 30, "CHERRY", "arena"}, {900, 12, "ARAM", ""}, {1710, 11, "CLASSIC", ""}, {1010, 11, "URF", ""}} {
		m, _, _ := mode(core.Snapshot{QueueID: tc.q, MapID: tc.mapID, GameMode: tc.game})
		if m != tc.mode {
			t.Fatalf("wrong classification %+v ->%s", tc, m)
		}
	}
}
func TestArenaFieldsRemainIndependentFromMayhem(t *testing.T) {
	q := Query{Mode: "arena", Champion: 3, Position: "mid"}
	raw := `1:["$","$L9",null,{"data":{"1":[{"id":1,"name":"测试","desc":"<shield>护盾</shield>","rareity":1,"win_rate":0,"play":"1,234"}],"4":[],"8":[]}}]
`
	p, err := parseFlight(flightFixture(q, "augments", raw), guidePageURL(q, "augments"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.arenaAugments()
	if err != nil || len(a) != 1 || a[0].Games != 1234 || a[0].WinRate == nil || *a[0].WinRate != 0 || a[0].Performance != nil {
		t.Fatal(a, err)
	}
	if _, err := p.augments(); err == nil {
		t.Fatal("Arena interpreted as Mayhem grades")
	}
}
func TestPageRunesUseExplicitSelectedIDs(t *testing.T) {
	q := Query{Mode: "urf", Champion: 3, Position: "mid"}
	raw := `1:["$","$L9",null,{"rune_pages":[{"builds":[{"primary_rune_ids":[8112,8143],"secondary_rune_ids":[8237],"stat_mod_ids":[5008,5008,5001]}]}],"names":[{"id":8112,"name":"电刑"}]}]
`
	p, _ := parseFlight(flightFixture(q, "build", raw), guidePageURL(q, "build"))
	ids, names := p.runePage()
	if len(ids) != 6 || ids[3] != ids[4] || names[0] != "电刑" {
		t.Fatal(ids, names)
	}
}
func TestLiveExtendedModes(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_TEST_EXTENDED_MODES") != "1" {
		t.Skip("public network opt-in")
	}
	for _, q := range []Query{{Mode: "urf", Champion: 3, Position: "mid"}, {Mode: "nexus-blitz", Champion: 3, Position: "mid"}, {Mode: "arena", Champion: 3, Position: "mid"}, {Mode: "arena", Champion: 145, Position: "mid"}, {Mode: "arena", Champion: 122, Position: "mid"}} {
		ctx, c := context.WithTimeout(context.Background(), 8*time.Second)
		g, err := NewOPgg().Fetch(ctx, q)
		c()
		if err != nil {
			t.Fatal(q.Mode, q.Champion, err)
		}
		if len(g.Builds) < 2 || len(g.Skills) < 10 {
			t.Fatal("incomplete public build")
		}
		if q.Mode == "arena" {
			if len(g.Augments) == 0 || len(g.Runes) > 0 || strings.Contains(g.SourceURL, "mayhem") {
				t.Fatal("Arena source mixed")
			}
		} else {
			if len(g.Runes) == 0 || g.ReferenceNote == "" {
				t.Fatal("rotating mode runes/reference omitted")
			}
		}
		t.Logf("mode=%s champion=%d builds=%d runes=%d augments=%d tips=%d version=%s notice=%s", q.Mode, q.Champion, len(g.Builds), len(g.Runes), len(g.Augments), len(g.Tips), g.Version, g.Notice)
	}
}

func TestPageVersionPaddingDoesNotCreateFalseMismatch(t *testing.T) {
	if patch("16.04") != patch("16.4.1") {
		t.Fatal("same patch treated as mismatch")
	}
}
