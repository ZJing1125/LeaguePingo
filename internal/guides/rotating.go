package guides

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (o *OPgg) fetchRotatingMode(ctx context.Context, q Query) (*Guide, error) {
	page, err := o.publicPage(ctx, q, "build")
	if err != nil {
		return nil, err
	}
	g := &Guide{Query: q, FetchedAt: time.Now().UTC(), SourceURL: guidePageURL(q, "build")}
	g.Builds, g.Skills, g.Version = page.builds()
	g.Runes, g.RuneNames = page.runePage()
	if len(g.Builds) == 0 || len(g.Skills) == 0 {
		return nil, errors.New("当前模式页面结构已变化或没有可用攻略")
	}
	if g.Version != "" {
		g.ReferenceNote = fmt.Sprintf("来源页面版本 %s；请核对当前轮换模式规则后参考。", g.Version)
	} else {
		g.ReferenceNote = "来源页面未提供版本，请核对当前轮换模式规则后参考。"
	}
	raw, _ := json.Marshal(g)
	if len(raw) > 256<<10 {
		return nil, errors.New("模式攻略内容超出限制")
	}
	return g, nil
}
func rate(v any) *float64 {
	f := score(v)
	if f != nil && *f <= 100 {
		return f
	}
	return nil
}
func games(v any) int {
	if s, ok := v.(string); ok {
		n, err := strconv.Atoi(strings.ReplaceAll(s, ",", ""))
		if err == nil && n >= 0 {
			return n
		}
		return 0
	}
	return num(v)
}
func (p *flightPage) arenaAugments() ([]Augment, error) {
	result := []Augment{}
	seen := map[int]bool{}
	for _, key := range p.order {
		p.walk(p.records[key], false, func(v any) {
			data := object(object(v)["data"])
			if len(data) == 0 {
				return
			}
			// Arena publishes independent per-quality rows, without Mayhem's tiers or scores.
			if _, ok := data["1"]; !ok {
				return
			}
			if _, ok := data["4"]; !ok {
				return
			}
			if _, ok := data["8"]; !ok {
				return
			}
			for _, quality := range []int{1, 4, 8} {
				for _, raw := range list(data[strconv.Itoa(quality)]) {
					m := object(raw)
					id := num(m["id"])
					if id <= 0 || seen[id] {
						continue
					}
					seen[id] = true
					name, desc := strings.TrimSpace(str(m["name"])), plain(str(m["desc"]))
					if name == "" || desc == "" {
						p.omitted++
						continue
					}
					if len(name) > 256 || len(desc) > 4000 || num(m["rareity"]) != quality {
						p.invalid = true
						continue
					}
					result = append(result, Augment{ID: id, Name: name, Description: desc, Rarity: quality, WinRate: rate(m["win_rate"]), Games: games(m["play"])})
				}
			}
		})
	}
	if p.invalid || len(result) == 0 {
		return nil, errors.New("竞技场强化页面结构已变化")
	}
	return result, nil
}
func (p *flightPage) runePage() ([]int, []string) {
	var ids []int
	names := map[int]string{}
	for _, key := range p.order {
		p.walk(p.records[key], false, func(v any) {
			m := object(v)
			if id := num(m["id"]); id > 0 && str(m["name"]) != "" {
				names[id] = str(m["name"])
			}
			if len(ids) > 0 {
				return
			}
			pages := list(m["rune_pages"])
			if len(pages) == 0 {
				return
			}
			builds := list(object(pages[0])["builds"])
			if len(builds) == 0 {
				return
			}
			b := object(builds[0])
			for _, key := range []string{"primary_rune_ids", "secondary_rune_ids", "stat_mod_ids"} {
				ids = append(ids, ints(b[key])...)
			}
		})
	}
	if p.invalid || len(ids) > 12 {
		return nil, nil
	}
	labels := []string{}
	for _, id := range ids {
		label := names[id]
		if label == "" {
			label = fmt.Sprintf("#%d", id)
		}
		labels = append(labels, label)
	}
	return ids, labels
}
func (p *flightPage) arenaSynergies() []Tip {
	tips := []Tip{}
	seen := map[int]bool{}
	for _, key := range p.order {
		p.walk(p.records[key], false, func(v any) {
			for _, raw := range list(object(v)["synergies"]) {
				m := object(raw)
				id := num(m["champion_id"])
				name := str(m["champion_name"])
				r := rate(m["win_rate"])
				n := games(m["play"])
				if id <= 0 || name == "" || r == nil || n <= 0 || seen[id] || len(tips) >= 5 {
					continue
				}
				seen[id] = true
				tips = append(tips, Tip{Label: name, Text: fmt.Sprintf("搭档组合胜率 %.2f%% · %d 场", *r, n)})
			}
		})
	}
	if p.invalid {
		return nil
	}
	return tips
}
