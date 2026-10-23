package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"leaguepingo/internal/diagnostics"
	"leaguepingo/internal/settings"
)

var Version = "0.2.0-dev"
var Commit = "source-build"
var BuildTime = "unrecorded"

type AppInfo struct {
	Version          string                        `json:"version"`
	Commit           string                        `json:"commit"`
	BuildTime        string                        `json:"buildTime"`
	Shortcut         string                        `json:"shortcut"`
	HoldEnabled      bool                          `json:"holdEnabled"`
	HoldKey          string                        `json:"holdKey"`
	SidebarEnabled   bool                          `json:"sidebarEnabled"`
	PreviewEnabled   bool                          `json:"previewEnabled"`
	SidebarWidth     int                           `json:"sidebarWidth"`
	SettingsWarning  string                        `json:"settingsWarning"`
	GoHeapBytes      uint64                        `json:"goHeapBytes"`
	Goroutines       int                           `json:"goroutines"`
	Requests         map[string]diagnostics.Metric `json:"requests"`
	PerformanceScope string                        `json:"performanceScope"`
}

func (s *AppService) Info() AppInfo {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	cfg := s.settings.Snapshot()
	return AppInfo{Version: Version, Commit: Commit, BuildTime: BuildTime, Shortcut: cfg.Shortcut, HoldEnabled: cfg.HoldEnabled, HoldKey: cfg.HoldKey, SidebarEnabled: cfg.SidebarEnabled, PreviewEnabled: cfg.PreviewEnabled, SidebarWidth: cfg.SidebarWidth, SettingsWarning: s.settingsWarning,
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
	// Explicit local export; no player snapshot, error text, paths or credentials.
	report := struct {
		Created time.Time `json:"created"`
		Info    AppInfo   `json:"info"`
	}{time.Now().UTC(), s.Info()}
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
