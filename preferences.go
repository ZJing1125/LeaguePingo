package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"leaguepingo/internal/diagnostics"
	"leaguepingo/internal/postgame"
	"leaguepingo/internal/readycheck"
	"leaguepingo/internal/settings"
)

var Version = "0.2.0-dev"
var Commit = "source-build"
var BuildTime = "unrecorded"

type AppInfo struct {
	AutoReturnEnabled bool                          `json:"autoReturnEnabled"`
	AutoQueueEnabled  bool                          `json:"autoQueueEnabled"`
	AutoQueueDelay    int                           `json:"autoQueueDelay"`
	AutoAcceptEnabled bool                          `json:"autoAcceptEnabled"`
	AutoAcceptDelay   int                           `json:"autoAcceptDelay"`
	Version           string                        `json:"version"`
	Commit            string                        `json:"commit"`
	BuildTime         string                        `json:"buildTime"`
	Shortcut          string                        `json:"shortcut"`
	HoldEnabled       bool                          `json:"holdEnabled"`
	HoldKey           string                        `json:"holdKey"`
	SidebarEnabled    bool                          `json:"sidebarEnabled"`
	PreviewEnabled    bool                          `json:"previewEnabled"`
	SidebarWidth      int                           `json:"sidebarWidth"`
	SettingsWarning   string                        `json:"settingsWarning"`
	GoHeapBytes       uint64                        `json:"goHeapBytes"`
	Goroutines        int                           `json:"goroutines"`
	Requests          map[string]diagnostics.Metric `json:"requests"`
	PerformanceScope  string                        `json:"performanceScope"`
}

func (s *AppService) Info() AppInfo {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	cfg := s.settings.Snapshot()
	return AppInfo{AutoReturnEnabled: cfg.AutoReturnEnabled, AutoQueueEnabled: cfg.AutoQueueEnabled, AutoQueueDelay: cfg.AutoQueueDelay, AutoAcceptEnabled: cfg.AutoAcceptEnabled, AutoAcceptDelay: cfg.AutoAcceptDelay, Version: Version, Commit: Commit, BuildTime: BuildTime, Shortcut: cfg.Shortcut, HoldEnabled: cfg.HoldEnabled, HoldKey: cfg.HoldKey, SidebarEnabled: cfg.SidebarEnabled, PreviewEnabled: cfg.PreviewEnabled, SidebarWidth: cfg.SidebarWidth, SettingsWarning: s.settingsWarning,
		GoHeapBytes: mem.HeapAlloc, Goroutines: runtime.NumGoroutine(), Requests: diagnostics.Default.Snapshot(), PerformanceScope: "仅 Go 堆；不包含 WebView2，不代表总内存、游戏 FPS 或网络延迟"}
}
func (s *AppService) ConfigureCompanion(sidebar, preview bool, width int) (AppInfo, error) {
	if width < 280 || width > 420 {
		return AppInfo{}, errors.New("侧栏宽度应为 280–420")
	}
	old := s.settings.Snapshot()
	s.settings.SetCompanion(sidebar, preview, width)
	if err := s.settings.Save(); err != nil {
		s.settings.SetCompanion(old.SidebarEnabled, old.PreviewEnabled, old.SidebarWidth)
		return AppInfo{}, err
	}
	s.overlay.update(func() {
		s.overlay.state.SidebarEnabled = sidebar
		s.overlay.state.PreviewEnabled = preview
		s.overlay.state.SidebarWidth = width
	})
	return s.Info(), nil
}
func (s *AppService) ConfigureHold(enabled bool, key string) (AppInfo, error) {
	if !settings.ValidHoldKey(key) {
		return AppInfo{}, errors.New("按住键仅支持 ~、F8、F9、F10")
	}
	if s.configureHold == nil {
		return AppInfo{}, errors.New("应用尚未准备好配置按住快览")
	}
	if err := s.configureHold(enabled, key); err != nil {
		return AppInfo{}, err
	}
	return s.Info(), nil
}
func (s *AppService) ConfigureShortcut(key string) (AppInfo, error) {
	key, err := settings.NormalizeShortcut(key)
	if err != nil {
		return AppInfo{}, err
	}
	if err = s.configureShortcut(key); err != nil {
		return AppInfo{}, err
	}
	return s.Info(), nil
}
func (s *AppService) ExportDiagnostics() (string, error) {
	// Explicit local export: aggregate runtime state and static window warnings;
	// no player snapshot, raw source errors, process paths or credentials.
	report := struct {
		Created     time.Time              `json:"created"`
		Info        AppInfo                `json:"info"`
		Companion   *companionDiagnostics  `json:"companion,omitempty"`
		Performance performanceDiagnostics `json:"performance"`
	}{Created: time.Now().UTC(), Info: s.Info()}
	report.Performance = s.performanceDiagnostics()
	if s.overlay != nil {
		runtime := s.overlay.diagnostics()
		report.Companion = &runtime
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", errors.New("无法编码诊断摘要")
	}
	dir := filepath.Join(s.configDir, "diagnostics")
	if os.MkdirAll(dir, 0700) != nil {
		return "", errors.New("无法创建诊断目录")
	}
	f, err := os.CreateTemp(dir, "leaguepingo-*.json")
	if err != nil {
		return "", errors.New("无法创建诊断文件")
	}
	name := f.Name()
	_, err = f.Write(b)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(name)
		return "", errors.New("无法保存诊断摘要")
	}
	return name, nil
}

func (s *AppService) ConfigureAutoAccept(enabled bool, delay int) (AppInfo, error) {
	if !readycheck.ValidDelay(delay) {
		return AppInfo{}, errors.New("接受延迟仅支持 0、1、3、5 秒")
	}
	old := s.settings.Snapshot()
	s.settings.SetAutoAccept(enabled, delay)
	if err := s.settings.Save(); err != nil {
		s.settings.SetAutoAccept(old.AutoAcceptEnabled, old.AutoAcceptDelay)
		return AppInfo{}, err
	}
	_ = s.engine.ConfigureAutoAccept(enabled, delay)
	return s.Info(), nil
}
func (s *AppService) AutoAcceptStatus() readycheck.State        { return s.engine.AutoAcceptStatus() }
func (s *AppService) SkipAutoAccept() (readycheck.State, error) { return s.engine.SkipAutoAccept() }

func (s *AppService) ConfigurePostGame(ret, queue bool, delay int) (AppInfo, error) {
	if !postgame.ValidDelay(delay) {
		return AppInfo{}, errors.New("排队延迟仅支持 5、10、15、20 秒")
	}
	old := s.settings.Snapshot()
	s.settings.SetPostGame(ret, queue, delay)
	if err := s.settings.Save(); err != nil {
		s.settings.SetPostGame(old.AutoReturnEnabled, old.AutoQueueEnabled, old.AutoQueueDelay)
		return AppInfo{}, err
	}
	_ = s.engine.ConfigurePostGame(ret, queue, delay)
	return s.Info(), nil
}
func (s *AppService) PostGameStatus() postgame.State          { return s.engine.PostGameStatus() }
func (s *AppService) CancelPostGame() (postgame.State, error) { return s.engine.CancelPostGame() }
