package guides

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const endpoint = "https://mcp-api.op.gg/mcp"

type Provider interface {
	Fetch(context.Context, Query) (*Guide, error)
}
type OPgg struct {
	http     *http.Client
	endpoint string
}

func NewOPgg() *OPgg {
	return &OPgg{http: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, endpoint: endpoint}
}
func (o *OPgg) rpc(ctx context.Context, headers http.Header, id int, method string, params any) (json.RawMessage, error) {
	body := map[string]any{"jsonrpc": "2.0", "method": method}
	if id > 0 {
		body["id"] = id
	}
	if params != nil {
		body["params"] = params
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, errors.New("攻略请求无效")
	}
	req.Header = headers.Clone()
	resp, err := o.http.Do(req)
	if err != nil {
		return nil, errors.New("OP.GG 连接失败或超时")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 202 && resp.StatusCode != 204 {
		return nil, fmt.Errorf("OP.GG 服务暂不可用（HTTP %d）", resp.StatusCode)
	}
	if v := resp.Header.Get("Mcp-Session-Id"); v != "" && len(v) < 4096 {
		headers.Set("Mcp-Session-Id", v)
	}
	if id == 0 {
		return nil, nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil || len(data) > 2<<20 {
		return nil, errors.New("攻略响应超出限制")
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if !json.Valid(data) {
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Buffer(make([]byte, 4096), 2<<20)
		data = nil
		for scanner.Scan() {
			l := scanner.Text()
			if strings.HasPrefix(l, "data:") {
				candidate := []byte(strings.TrimSpace(strings.TrimPrefix(l, "data:")))
				if json.Unmarshal(candidate, &envelope) == nil && len(envelope.Result) > 0 {
					data = candidate
					break
				}
			}
		}
		if data == nil {
			return nil, errors.New("攻略事件响应不可用")
		}
	}
	if json.Unmarshal(data, &envelope) != nil || len(envelope.Error) > 0 || len(envelope.Result) == 0 {
		return nil, errors.New("OP.GG 未返回可用攻略")
	}
	return envelope.Result, nil
}
func (o *OPgg) Fetch(ctx context.Context, q Query) (*Guide, error) {
	if !validQuery(q) {
		return nil, errors.New("当前模式或英雄暂无已核验攻略")
	}
	if q.Mode == "aram-mayhem" {
		return o.fetchMayhem(ctx, q)
	}
	headers := http.Header{"Content-Type": {"application/json"}, "Accept": {"application/json, text/event-stream"}}
	init, err := o.rpc(ctx, headers, 1, "initialize", map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "LeaguePingo", "version": "1"}})
	if err != nil {
		return nil, err
	}
	var server struct {
		Protocol string `json:"protocolVersion"`
	}
	if json.Unmarshal(init, &server) != nil || server.Protocol == "" {
		return nil, errors.New("攻略服务协议不可用")
	}
	headers.Set("MCP-Protocol-Version", server.Protocol)
	if _, err = o.rpc(ctx, headers, 0, "notifications/initialized", nil); err != nil {
		return nil, err
	}
	name := "lol_get_champion_analysis"
	args := map[string]any{"game_mode": q.Mode, "champion": champions[q.Champion], "position": q.Position, "lang": "zh_CN", "desired_output_fields": []string{"champion", "data.starter_items.{ids[],ids_names[],play,win}", "data.core_items.{ids[],ids_names[],play,win}", "data.boots.{ids[],ids_names[],play,win}", "data.summoner_spells.{ids[],ids_names[],play,win}", "data.runes.{primary_rune_ids[],primary_rune_names[],secondary_rune_ids[],secondary_rune_names[],stat_mod_ids[],stat_mod_names[]}", "data.skills.{order[]}", "data.trends.win.{version}"}}
	if q.Opponent > 0 {
		name = "lol_get_lane_matchup_guide"
		args = map[string]any{"position": q.Position, "my_champion": champions[q.Champion], "opponent_champion": champions[q.Opponent], "lang": "zh_CN"}
	}
	raw, err := o.rpc(ctx, headers, 2, "tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return nil, err
	}
	var result struct {
		IsError    bool            `json:"isError"`
		Structured json.RawMessage `json:"structuredContent"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(raw, &result) != nil || result.IsError {
		return nil, errors.New("OP.GG 暂无这组攻略")
	}
	var value any
	if len(result.Structured) > 0 && string(result.Structured) != "null" {
		_ = json.Unmarshal(result.Structured, &value)
	}
	if value == nil {
		for _, c := range result.Content {
			if c.Type != "text" {
				continue
			}
			if json.Unmarshal([]byte(c.Text), &value) != nil {
				value, err = decodeCompact(c.Text)
			}
			if err == nil && value != nil {
				break
			}
		}
	}
	root, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("攻略内容格式不可用")
	}
	return normalize(q, root)
}
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func list(v any) []any {
	a, ok := v.([]any)
	if ok {
		return a
	}
	if object(v) != nil {
		return []any{v}
	}
	return nil
}
func str(v any) string { s, _ := v.(string); return s }
func num(v any) int    { f, _ := v.(float64); return int(f) }
func ints(v any) []int {
	a := []int{}
	for _, x := range list(v) {
		if n := num(x); n > 0 {
			a = append(a, n)
		}
	}
	return a
}
func texts(v any) []string {
	a := []string{}
	for _, x := range list(v) {
		if s := str(x); s != "" {
			a = append(a, s)
		}
	}
	return a
}
func normalize(q Query, root map[string]any) (*Guide, error) {
	data := object(root["data"])
	if data == nil {
		return nil, errors.New("暂无攻略数据")
	}
	g := &Guide{Query: q, FetchedAt: time.Now().UTC(), SourceURL: "https://op.gg/zh-cn/lol/champions/" + strings.ToLower(strings.ReplaceAll(champions[q.Champion], "_", "")) + "/build", Builds: []BuildSection{}, Runes: []int{}, RuneNames: []string{}, Skills: []string{}, Tips: []Tip{}}
	slug := strings.ToLower(strings.ReplaceAll(champions[q.Champion], "_", ""))
	if q.Mode == "aram" {
		g.SourceURL = "https://op.gg/zh-cn/lol/modes/aram/" + slug + "/build"
	} else {
		g.SourceURL = "https://op.gg/zh-cn/lol/champions/" + slug + "/build/" + q.Position + "?type=" + q.Mode + "&tier=all&region=global"
	}
	for _, entry := range []struct{ key, label string }{{"starter_items", "出门装"}, {"core_items", "核心装备"}, {"boots", "鞋子"}, {"summoner_spells", "召唤师技能"}} {
		section := BuildSection{Label: entry.label, Options: []Build{}}
		for _, v := range list(data[entry.key]) {
			m := object(v)
			ids := ints(m["ids"])
			if len(ids) == 0 {
				continue
			}
			b := Build{IDs: ids, Names: texts(m["ids_names"]), Games: num(m["play"])}
			if b.Games > 0 {
				if wins, ok := m["win"].(float64); ok && wins >= 0 && wins <= float64(b.Games) {
					rate := 100 * wins / float64(b.Games)
					b.WinRate = &rate
				}
			}
			section.Options = append(section.Options, b)
			if len(section.Options) == 3 {
				break
			}
		}
		if len(section.Options) > 0 {
			g.Builds = append(g.Builds, section)
		}
	}
	if a := list(data["runes"]); len(a) > 0 {
		r := object(a[0])
		for _, k := range []string{"primary", "secondary", "stat_mod"} {
			key := k + "_rune_ids"
			names := k + "_rune_names"
			if k == "stat_mod" {
				key = "stat_mod_ids"
				names = "stat_mod_names"
			}
			g.Runes = append(g.Runes, ints(r[key])...)
			g.RuneNames = append(g.RuneNames, texts(r[names])...)
		}
	}
	if a := list(data["skills"]); len(a) > 0 {
		for _, slot := range list(object(a[0])["order"]) {
			if value := str(slot); value != "" {
				g.Skills = append(g.Skills, value)
			} else if n := num(slot); n >= 1 && n <= 4 {
				g.Skills = append(g.Skills, []string{"", "Q", "W", "E", "R"}[n])
			}
		}
	}
	if a := list(object(data["trends"])["win"]); len(a) > 0 {
		g.Version = str(object(a[0])["version"])
	}
	for _, v := range []struct{ key, label string }{{"opponent_champion_tip", "对手应对"}, {"lane_solo_kill_advantage_champion", "单杀优势"}, {"lane_advantage_champion", "对线优势"}, {"recommended_play_style", "推荐打法"}} {
		if tip := strings.TrimSpace(str(data[v.key])); tip != "" && len(tip) <= 12000 {
			g.Tips = append(g.Tips, Tip{v.label, tip})
		}
	}
	if len(g.Builds) == 0 && len(g.Runes) == 0 && len(g.Tips) == 0 {
		return nil, errors.New("该模式或对位暂无足够攻略数据")
	}
	return g, nil
}
