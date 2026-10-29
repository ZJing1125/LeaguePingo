package main

import "leaguepingo/internal/diagnostics"

type performanceDiagnostics struct {
	Readiness diagnostics.ReadinessMetric `json:"champSelectFirstData"`
	Keyboard  diagnostics.CallbackMetric  `json:"keyboardCallback"`
	Resources diagnostics.ResourceMetric  `json:"inGameResources"`
	Scope     string                      `json:"scope"`
}

func (s *AppService) performanceDiagnostics() performanceDiagnostics {
	p := performanceDiagnostics{Keyboard: diagnostics.KeyboardCallback.Snapshot(), Scope: "首批数据为后端快照发布，不含前端绘制；请求P95为各分类最近256次（含失败）；回调仅配置键非注入事件的自身耗时；资源为局内助手及可读后代的整机归一CPU、私有字节和工作集，工作集可重复计共享页、采样会遗漏短命进程；不含游戏FPS或网络RTT。"}
	if s.engine != nil {
		p.Readiness = s.engine.ReadinessMetrics()
	}
	if s.resources != nil {
		p.Resources = s.resources.Snapshot()
	}
	return p
}
