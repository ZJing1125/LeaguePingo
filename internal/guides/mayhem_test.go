package guides

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func flightFixture(q Query, tab string, records string) []byte {
	b, _ := json.Marshal([]any{1, records})
	return []byte(`<link rel="canonical" href="` + mayhemURL(q, tab) + `"><title>英雄模式16.21版本</title><script>self.__next_f.push(` + string(b) + `)</script>`)
}

const augmentRecords = `1:["$","$L99",null,{"data":[{"id":1,"name":"坦克引擎","desc":"获得<shield>护盾</shield><br>持续生效。","tier":0,"rarity":4,"performance":110,"popular":3},{"id":2,"name":"强化二","desc":"说明二","tier":1,"rarity":1,"performance":0,"popular":0},{"id":3,"name":"强化三","desc":"说明三","tier":2,"rarity":8},{"id":4,"name":"强化四","desc":"说明四","tier":3,"rarity":1}]}]
`
const buildRecords = `1:["$","tr","core_items_0",{"children":"$L2"}]
2:["$","div",null,{"children":[["$","$L9",null,{"metaType":"item","metaId":3084,"children":["$","img",null,{"alt":"装备一"}]}],["$","$L9",null,{"metaType":"item","metaId":3083,"children":["$","img",null,{"alt":"装备二"}]}]]}]
3:["$","span","0",{"children":["$","strong",null,{"children":"Q"}]}]
4:["$","span","1",{"children":["$","strong",null,{"children":"E"}]}]
5:["$","tr",null,{"children":[["$","$L9",null,{"metaType":"spell","metaId":4}],["$","$L9",null,{"metaType":"spell","metaId":32}]]}]
6:"$L6"
`

func TestMayhemPageNormalization(t *testing.T) {
	q := Query{Mode: "aram-mayhem", Champion: 3, Position: "mid"}
	p, err := parseFlight(flightFixture(q, "augments", augmentRecords), mayhemURL(q, "augments"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.augments()
	if err != nil || len(a) != 4 || a[0].Tier != 0 || a[0].Rarity != 4 || strings.Contains(a[0].Description, "<") || a[1].Performance == nil || *a[1].Performance != 0 {
		t.Fatalf("augment loss %v %+v", err, a)
	}
	p, err = parseFlight(flightFixture(q, "build", buildRecords), mayhemURL(q, "build"))
	if err != nil {
		t.Fatal(err)
	}
	b, s, v := p.builds()
	if len(b) != 2 || len(b[0].Options[0].IDs) != 2 || b[0].Options[0].Names[0] != "装备一" || strings.Join(s, "") != "QE" || v != "16.21" || b[0].Options[0].WinRate != nil {
		t.Fatalf("bad normalized page %+v %v %s", b, s, v)
	}
	p.walk("$L6", true, func(any) {}) // cyclic references terminate
	if _, err = parseFlight(flightFixture(q, "build", buildRecords), mayhemURL(q, "augments")); err == nil {
		t.Fatal("wrong hero/tab accepted")
	}
	// A schema containing only MCP's lower tiers must not become a top list.
	partial := strings.ReplaceAll(strings.ReplaceAll(augmentRecords, `"tier":0`, `"tier":3`), `"tier":1`, `"tier":4`)
	partial = strings.ReplaceAll(partial, `"tier":2`, `"tier":5`)
	p, _ = parseFlight(flightFixture(q, "augments", partial), mayhemURL(q, "augments"))
	if _, err = p.augments(); err == nil {
		t.Fatal("incomplete low-tier list accepted")
	}
	raw := []byte(`<link rel="canonical" href="` + mayhemURL(q, "build") + `"><script>self.__next_f.push([1,run()])</script>`)
	if _, err = parseFlight(raw, mayhemURL(q, "build")); err == nil {
		t.Fatal("executable source accepted")
	}
}
func TestMayhemProviderUsesOnlyFixedPublicPages(t *testing.T) {
	q := Query{Mode: "aram-mayhem", Champion: 3, Position: "mid"}
	calls := 0
	o := NewOPgg()
	o.http = &http.Client{Transport: guideTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "op.gg" || r.Method != "GET" || r.URL.RawQuery != "" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Fatal("private data or wrong request")
		}
		tab := "build"
		records := buildRecords
		if strings.HasSuffix(r.URL.Path, "/augments") {
			tab = "augments"
			records = augmentRecords
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(flightFixture(q, tab, records)))), Header: http.Header{}}, nil
	})}
	g, err := o.Fetch(context.Background(), q)
	if err != nil || calls != 2 || len(g.Augments) != 4 || len(g.Runes) != 0 || g.Notice != "" {
		t.Fatalf("%v %+v calls%d", err, g, calls)
	}
}

type guideTransport func(*http.Request) (*http.Response, error)

func (f guideTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLiveMayhem(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_TEST_MAYHEM") != "1" {
		t.Skip("public OP.GG pages opt-in")
	}
	for _, id := range []int{3, 99, 22, 122, 16} {
		ctx, c := context.WithTimeout(context.Background(), 8*time.Second)
		g, err := NewOPgg().Fetch(ctx, Query{Mode: "aram-mayhem", Champion: id, Position: "mid"})
		c()
		if err != nil {
			t.Fatal(id, err)
		}
		tiers := map[int]int{}
		for _, a := range g.Augments {
			tiers[a.Tier]++
		}
		t.Logf("champion=%d augments=%d tiers=%v builds=%d skills=%d pageVersion=%s notice=%s", id, len(g.Augments), tiers, len(g.Builds), len(g.Skills), g.Version, g.Notice)
		if len(g.Augments) < 20 || tiers[0]+tiers[1]+tiers[2] == 0 || len(g.Builds) < 3 || len(g.Skills) < 10 || g.Notice != "" {
			t.Fatal(fmt.Sprintf("incomplete guide for %d", id))
		}
	}
}

func TestMayhemPartialPagesRetainAvailableData(t *testing.T) {
	q := Query{Mode: "aram-mayhem", Champion: 3, Position: "mid"}
	for _, failed := range []string{"augments", "build", "both"} {
		t.Run(failed, func(t *testing.T) {
			o := NewOPgg()
			o.http = &http.Client{Transport: guideTransport(func(r *http.Request) (*http.Response, error) {
				tab := "build"
				records := buildRecords
				if strings.HasSuffix(r.URL.Path, "/augments") {
					tab = "augments"
					records = augmentRecords
				}
				status := 200
				body := string(flightFixture(q, tab, records))
				if tab == failed || failed == "both" {
					status = 503
					body = "unavailable"
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
			})}
			g, err := o.Fetch(context.Background(), q)
			if failed == "both" {
				if err == nil {
					t.Fatal("failed pages shown as ready")
				}
				return
			}
			if err != nil || g.Notice == "" {
				t.Fatalf("partial data erased: %v", err)
			}
			if failed == "augments" && len(g.Builds) == 0 || failed == "build" && len(g.Augments) == 0 {
				t.Fatal("available content lost")
			}
		})
	}
}
