package guides

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"leaguepingo/internal/apperrors"
)

// Public, per-champion OP.GG pages. No player identifiers, cookies, scripts,
// remote assets or arbitrary destinations are used. The MCP augment tool omits
// tiers 0–2, so it cannot supply the complete recommendation list.
func guidePageURL(q Query, tab string) string {
	return "https://op.gg/zh-cn/lol/modes/" + q.Mode + "/" + strings.ToLower(strings.ReplaceAll(champions[q.Champion], "_", "")) + "/" + tab
}
func (o *OPgg) publicPage(ctx context.Context, q Query, tab string) (result *flightPage, resultErr error) {
	defer func() { apperrors.Failures.Record(apperrors.GuideService, resultErr) }()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, guidePageURL(q, tab), nil)
	if err != nil {
		return nil, errors.New("模式攻略请求无效")
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "LeaguePingo/1 (+https://github.com/ZJing1125/LeaguePingo)")
	resp, err := o.http.Do(req)
	if err != nil {
		return nil, apperrors.Request(ctx, err, apperrors.GuideService)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.HTTP(apperrors.GuideService, resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (3<<20)+1))
	if err != nil || len(raw) > 3<<20 {
		return nil, errors.New("模式攻略响应超出限制")
	}
	return parseFlight(raw, guidePageURL(q, tab))
}
func (o *OPgg) fetchAugmentMode(ctx context.Context, q Query) (*Guide, error) {
	g := &Guide{Query: q, FetchedAt: time.Now().UTC(), SourceURL: guidePageURL(q, "build")}
	aug, aerr := o.publicPage(ctx, q, "augments")
	if aerr == nil {
		if q.Mode == "arena" {
			g.Augments, aerr = aug.arenaAugments()
		} else {
			g.Augments, aerr = aug.augments()
		}
		g.AugmentsOmitted = aug.omitted
	}
	build, berr := o.publicPage(ctx, q, "build")
	if berr == nil {
		g.Builds, g.Skills, g.Version = build.builds()
		if q.Mode == "arena" {
			g.Tips = build.arenaSynergies()
		}
		if len(g.Builds) == 0 || len(g.Skills) == 0 {
			berr = errors.New("出装或加点页面结构已变化")
		}
	}
	if aerr != nil && berr != nil {
		return nil, apperrors.Normalize(aerr, apperrors.GuideUnavailable)
	}
	if aerr != nil {
		g.Notice = "强化符文暂未读取成功，可重试或查看原文。"
	}
	if berr != nil {
		g.Notice = "出装或加点暂未完整读取，可重试或查看原文。"
	}
	encoded, _ := json.Marshal(g)
	if len(encoded) > 256<<10 {
		return nil, errors.New("模式攻略内容超出限制")
	}
	return g, nil
}

// Read only JSON literals inside Next Flight pushes. Never interpret JS, load
// modules, resolve URLs, or execute source text. References are traversed under
// depth/node limits and a cycle guard. HTML/transport changes fail closed.
var flightScript = regexp.MustCompile(`(?s)<script(?:\s[^>]*)?>\s*self\.__next_f\.push\((\[.*?)\)\s*</script>`)
var flightRef = regexp.MustCompile(`^\$L?([0-9a-f]+)$`)
var pageVersion = regexp.MustCompile(`<title>[^<]*模式([0-9]+\.[0-9]+)版本[^<]*</title>`)
var tags = regexp.MustCompile(`<[^>]*>`)
var breaks = regexp.MustCompile(`(?i)<br\s*/?>`)

type flightPage struct {
	records map[string]any
	order   []string
	version string
	invalid bool
	omitted int
}

func parseFlight(raw []byte, expected string) (*flightPage, error) {
	if len(raw) > 3<<20 {
		return nil, errors.New("模式攻略响应超出限制")
	}
	// Canonical must match the requested hero, mode and tab, even on HTTP 200.
	if !strings.Contains(string(raw), `rel="canonical" href="`+expected+`"`) {
		return nil, errors.New("模式攻略页面身份不匹配")
	}
	var stream strings.Builder
	for _, match := range flightScript.FindAllSubmatch(raw, -1) {
		var parts []json.RawMessage
		if json.Unmarshal(match[1], &parts) != nil || len(parts) != 2 || string(parts[0]) != "1" {
			continue
		}
		var text string
		if json.Unmarshal(parts[1], &text) == nil {
			stream.WriteString(text)
		}
	}
	page := &flightPage{records: map[string]any{}}
	if m := pageVersion.FindSubmatch(raw); m != nil {
		page.version = string(m[1])
	}
	for _, line := range strings.Split(stream.String(), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || len(key) > 8 || key == "" {
			continue
		}
		if _, err := strconv.ParseUint(key, 16, 32); err != nil {
			continue
		}
		var v any
		if json.Unmarshal([]byte(value), &v) != nil {
			continue
		}
		if _, exists := page.records[key]; exists {
			return nil, errors.New("模式攻略记录重复")
		}
		page.records[key] = v
		page.order = append(page.order, key)
	}
	if len(page.records) == 0 {
		return nil, errors.New("模式攻略页面结构已变化")
	}
	return page, nil
}
func (p *flightPage) walk(v any, refs bool, visit func(any)) {
	nodes := 0
	active := map[string]bool{}
	var step func(any, int)
	step = func(v any, depth int) {
		nodes++
		if depth > 64 || nodes > 100000 {
			p.invalid = true
			return
		}
		visit(v)
		switch x := v.(type) {
		case []any:
			for _, a := range x {
				step(a, depth+1)
			}
		case map[string]any:
			// Only semantic children; records carry page dictionaries and unrelated
			// strings. Sorting keeps normalization deterministic across map iteration.
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				step(x[k], depth+1)
			}
		case string:
			if !refs {
				return
			}
			m := flightRef.FindStringSubmatch(x)
			if m != nil && !active[m[1]] {
				if target, ok := p.records[m[1]]; ok {
					active[m[1]] = true
					step(target, depth+1)
					delete(active, m[1])
				}
			}
		}
	}
	step(v, 0)
}
func element(v any) (string, string, map[string]any) {
	a, ok := v.([]any)
	if !ok || len(a) != 4 || str(a[0]) != "$" {
		return "", "", nil
	}
	return str(a[1]), str(a[2]), object(a[3])
}
func plain(s string) string {
	return strings.TrimSpace(html.UnescapeString(tags.ReplaceAllString(breaks.ReplaceAllString(s, "\n"), "")))
}
func score(v any) *float64 {
	f, ok := v.(float64)
	if !ok || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 {
		return nil
	}
	return &f
}
func (p *flightPage) augments() ([]Augment, error) {
	var result []Augment
	for _, key := range p.order {
		p.walk(p.records[key], false, func(v any) {
			m := object(v)
			a, ok := m["data"].([]any)
			if !ok || len(a) < 2 {
				return
			}
			candidate := []Augment{}
			seen := map[int]bool{}
			valid := true
			for _, v := range a {
				m := object(v)
				id := num(m["id"])
				tier, hasTier := m["tier"].(float64)
				rarity := num(m["rarity"])
				if id <= 0 || !hasTier || tier != math.Trunc(tier) || tier < 0 || tier > 5 || seen[id] {
					valid = false
					break
				}
				seen[id] = true
				name := strings.TrimSpace(str(m["name"]))
				desc := plain(str(m["desc"]))
				if name == "" || len(name) > 256 || desc == "" || len(desc) > 4000 {
					continue
				}
				if rarity != 1 && rarity != 4 && rarity != 8 && rarity != 0 {
					valid = false
					break
				}
				candidate = append(candidate, Augment{ID: id, Name: name, Description: desc, Tier: int(tier), Rarity: rarity, Performance: score(m["performance"]), Popularity: score(m["popular"])})
			}
			// Complete tier schema must include the high recommendations absent in MCP.
			high := false
			for _, a := range candidate {
				high = high || a.Tier <= 2
			}
			if valid && high && len(candidate) > len(result) {
				result = candidate
				p.omitted = len(a) - len(candidate)
			}
		})
	}
	if p.invalid || len(result) == 0 {
		return nil, errors.New("强化符文列表不完整或页面结构已变化")
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Tier != result[j].Tier {
			return result[i].Tier < result[j].Tier
		}
		a, b := result[i].Popularity, result[j].Popularity
		return a != nil && (b == nil || *a > *b)
	})
	return result, nil
}
func (p *flightPage) media(v any, kind string) Build {
	b := Build{}
	p.walk(v, true, func(v any) {
		m := object(v)
		if str(m["metaType"]) != kind {
			return
		}
		id := num(m["metaId"])
		if id <= 0 {
			return
		}
		name := ""
		p.walk(m["children"], true, func(n any) {
			if s := str(object(n)["alt"]); name == "" && s != "" {
				name = s
			}
		})
		b.IDs = append(b.IDs, id)
		b.Names = append(b.Names, name)
	})
	return b
}
func (p *flightPage) builds() ([]BuildSection, []string, string) {
	rows := map[string]map[int]Build{}
	spells := []Build{}
	spellSeen := map[string]bool{}
	skills := map[int]string{}

	for _, key := range p.order {
		p.walk(p.records[key], false, func(v any) {
			typ, k, props := element(v)
			if props == nil {
				return
			}
			if typ == "tr" {
				for _, prefix := range []string{"starter_items", "core_items", "boots", "prism_items"} {
					if !strings.HasPrefix(k, prefix+"_") {
						continue
					}
					i, err := strconv.Atoi(strings.TrimPrefix(k, prefix+"_"))
					if err != nil || i < 0 || i > 20 {
						continue
					}
					b := p.media(v, "item")
					if len(b.IDs) > 0 && len(b.IDs) <= 10 {
						if rows[prefix] == nil {
							rows[prefix] = map[int]Build{}
						}
						rows[prefix][i] = b
					}
				}
				b := p.media(v, "spell")
				if len(b.IDs) == 2 {
					sig := fmt.Sprint(b.IDs)
					if !spellSeen[sig] {
						spells = append(spells, b)
						spellSeen[sig] = true
					}
				}
			}
			if typ == "span" {
				i, err := strconv.Atoi(k)
				if err != nil || i < 0 || i > 17 {
					return
				}
				p.walk(props["children"], true, func(v any) {
					typ, _, m := element(v)
					if typ == "strong" {
						s := str(m["children"])
						if s == "Q" || s == "W" || s == "E" || s == "R" {
							skills[i] = s
						}
					}
				})
			}
		})
	}
	result := []BuildSection{}
	for _, sec := range []struct{ key, label string }{{"starter_items", "出门装"}, {"core_items", "核心装备"}, {"boots", "鞋子"}, {"prism_items", "棱彩装备"}} {
		options := []Build{}
		for i := 0; i < 3; i++ {
			if b, ok := rows[sec.key][i]; ok {
				options = append(options, b)
			}
		}
		if len(options) > 0 {
			result = append(result, BuildSection{sec.label, options})
		}
	}
	if len(spells) > 0 {
		if len(spells) > 3 {
			spells = spells[:3]
		}
		result = append(result, BuildSection{"召唤师技能", spells})
	}
	order := []string{}
	for i := 0; i < 18; i++ {
		if s := skills[i]; s != "" {
			order = append(order, s)
		} else {
			break
		}
	}
	if p.invalid {
		return nil, nil, ""
	}
	return result, order, p.version
}
