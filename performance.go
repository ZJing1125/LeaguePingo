package main

import (
	"context"
	"math"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
	"leaguepingo/internal/engine"
)

type performanceDiagnostics struct {
	Publications engine.PublicationMetrics       `json:"snapshotPublications"`
	Responses    diagnostics.ResponseCounts      `json:"responses"`
	Ratings      diagnostics.TeamReadinessMetric `json:"champSelectRatings"`
	Readiness    diagnostics.ReadinessMetric     `json:"champSelectFirstData"`
	Keyboard     diagnostics.CallbackMetric      `json:"keyboardCallback"`
	Resources    diagnostics.ResourceMetric      `json:"inGameResources"`
	Scope        string                          `json:"scope"`
}

func (s *AppService) performanceDiagnostics() performanceDiagnostics {
	p := performanceDiagnostics{Responses: diagnostics.Responses.Counts(), Keyboard: diagnostics.KeyboardCallback.Snapshot(), Scope: "首批数据为后端快照发布；response分类通过本机临时编号关联后端至两帧后回执，回执含桥接返程；source含排队/解析不可与子段相加；未观测包含取消/淘汰；快照计数为本进程派发量；ui分类为可见窗口两帧后本机DOM调度时长，rating/guide从资料变化起计；compute为解析/聚合；P50/P95为各分类最近256次（含失败）；选人评级为首位及最近退出覆盖；回调仅配置键非注入事件的自身耗时；资源为局内助手及可读后代的整机归一CPU、私有字节和工作集，工作集可重复计共享页、采样会遗漏短命进程；不含游戏FPS或网络RTT。"}
	if s.engine != nil {
		p.Publications = s.engine.PublicationMetrics()
		p.Readiness = s.engine.ReadinessMetrics()
		p.Ratings = s.engine.TeamReadinessMetrics()
	}
	if s.resources != nil {
		p.Resources = s.resources.Snapshot()
	}
	return p
}

// RecordUIResponse accepts only fixed, anonymous measurements from local views.
func (s *AppService) RecordUIResponse(category string, elapsedMS float64) {
	switch category {
	case "room-render", "sidebar-render", "player-history-ready", "history-feedback", "history-cache-ready", "history-first-ready", "rating-render", "guide-render":
	default:
		return
	}
	if math.IsNaN(elapsedMS) || math.IsInf(elapsedMS, 0) || elapsedMS < 0 || elapsedMS > 180000 {
		return
	}
	diagnostics.Default.Record("/ui/"+category, time.Duration(elapsedMS*float64(time.Millisecond)), 200, false)
}

func (s *AppService) AcknowledgeResponse(id uint64, uiMS float64) {
	if math.IsNaN(uiMS) || math.IsInf(uiMS, 0) || uiMS < 0 || uiMS > 180000 {
		return
	}
	diagnostics.Responses.Acknowledge(id, time.Duration(uiMS*float64(time.Millisecond)))
}

// Trace IDs are attached after cache writes; they never enter persisted pages.
func traceHistory(ctx context.Context, read func(context.Context) (core.HistoryPage, error)) (core.HistoryPage, error) {
	id := diagnostics.Responses.Begin("history")
	ctx = diagnostics.WithResponse(ctx, id)
	started := time.Now()
	page, err := read(ctx)
	if err != nil || ctx.Err() != nil {
		diagnostics.Responses.Discard(id)
		if err == nil {
			err = ctx.Err()
		}
		return core.HistoryPage{}, err
	}
	diagnostics.Responses.Stage(id, "source", time.Since(started))
	page.ResponseID = id
	diagnostics.Responses.Ready(id)
	diagnostics.Responses.Published(id)
	return page, nil
}

func tracePlayer(ctx context.Context, read func(context.Context) (core.Participant, error)) (core.Participant, error) {
	id := diagnostics.Responses.Begin("history")
	ctx = diagnostics.WithResponse(ctx, id)
	started := time.Now()
	player, err := read(ctx)
	if err != nil || ctx.Err() != nil {
		diagnostics.Responses.Discard(id)
		if err == nil {
			err = ctx.Err()
		}
		return core.Participant{}, err
	}
	diagnostics.Responses.Stage(id, "source", time.Since(started))
	player.ResponseID = id
	diagnostics.Responses.Ready(id)
	diagnostics.Responses.Published(id)
	return player, nil
}
