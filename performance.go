package main

import (
	"leaguepingo/internal/diagnostics"
	"math"
	"time"
)

type performanceDiagnostics struct {
	Ratings   diagnostics.TeamReadinessMetric `json:"champSelectRatings"`
	Readiness diagnostics.ReadinessMetric     `json:"champSelectFirstData"`
	Keyboard  diagnostics.CallbackMetric      `json:"keyboardCallback"`
	Resources diagnostics.ResourceMetric      `json:"inGameResources"`
	Scope     string                          `json:"scope"`
}

func (s *AppService) performanceDiagnostics() performanceDiagnostics {
	p := performanceDiagnostics{Keyboard: diagnostics.KeyboardCallback.Snapshot(), Scope: "首批数据为后端快照发布；ui分类为可见窗口两帧后本机DOM调度时长，rating/guide从资料变化起计；compute为解析/聚合；P50/P95为各分类最近256次（含失败）；选人评级为首位及最近退出覆盖；回调仅配置键非注入事件的自身耗时；资源为局内助手及可读后代的整机归一CPU、私有字节和工作集，工作集可重复计共享页、采样会遗漏短命进程；不含游戏FPS或网络RTT。"}
	if s.engine != nil {
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
	case "player-history-ready", "history-feedback", "history-cache-ready", "history-first-ready", "rating-render", "guide-render":
	default:
		return
	}
	if math.IsNaN(elapsedMS) || math.IsInf(elapsedMS, 0) || elapsedMS < 0 || elapsedMS > 180000 {
		return
	}
	diagnostics.Default.Record("/ui/"+category, time.Duration(elapsedMS*float64(time.Millisecond)), 200, false)
}
