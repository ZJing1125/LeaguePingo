package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/diagnostics"
	"leaguepingo/internal/engine"
	"leaguepingo/internal/guides"
	"leaguepingo/internal/hold"
	"leaguepingo/internal/settings"
	"leaguepingo/internal/source"
	"leaguepingo/internal/updatecheck"
)

// One owner makes startup/shutdown dependencies explicit. Window mutation stays
// on Wails' dispatcher; background services use the shared lifetime context.
type desktopApplication struct {
	ctx                      context.Context
	cancel                   context.CancelFunc
	app                      *application.App
	backend                  *AppService
	offlineSmoke             bool
	mainWindow               *application.WebviewWindow
	windowReady              chan struct{}
	geometryReady            bool
	controller               *overlayController
	held                     *hold.State
	holdWake, companionWake  chan struct{}
	monitor                  *hold.Monitor
	clientMonitor            *clientWindowMonitor
	tray                     *application.SystemTray
	updates                  *updatecheck.Checker
	updateItem               *application.MenuItem
	holdConfigMu, shortcutMu sync.Mutex
	registeredShortcut       string
}

func newDesktopApplication(ctx context.Context, cancel context.CancelFunc, content fs.FS) (*desktopApplication, error) {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return nil, errors.New("无法找到用户设置目录")
	}
	offline := os.Getenv("LEAGUEPINGO_OFFLINE_SMOKE") == "1"
	configDir, err := desktopConfigDir(configRoot, offline, os.Getenv("LEAGUEPINGO_SMOKE_DIR"))
	if err != nil {
		return nil, err
	}
	store, settingsErr := settings.Open(filepath.Join(configDir, "settings.json"))
	r := &desktopApplication{ctx: ctx, cancel: cancel, offlineSmoke: offline, windowReady: make(chan struct{}), holdWake: make(chan struct{}, 1), companionWake: make(chan struct{}, 1)}
	r.held = hold.New(func() { wakeDesktop(r.holdWake) })
	r.backend = &AppService{settings: store, configDir: configDir, configureShortcut: func(string) error { return apperrors.New(apperrors.AppNotReady) }}
	if settingsErr != nil {
		r.backend.settingsWarning = apperrors.UserMessage(settingsErr, apperrors.SettingsReadFailed)
	}
	r.createApplication(content)
	// application.New exits duplicate instances before workers or tray exist.
	r.connectServices()
	r.createMainWindowAndTray()
	r.connectOverlay()
	r.app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) { r.started() })
	go r.backend.engine.Run(ctx)
	return r, nil
}

func desktopConfigDir(root string, offline bool, isolated string) (string, error) {
	if offline {
		if isolated == "" {
			return "", errors.New("离线测试必须指定隔离设置目录")
		}
		return isolated, nil
	}
	return filepath.Join(root, "LeaguePingo"), nil
}

func (r *desktopApplication) createApplication(content fs.FS) {
	instance := singleInstanceOptions(func() { <-r.windowReady; r.showMain() })
	if r.offlineSmoke {
		instance.UniqueID = fmt.Sprintf("com.leaguepingo.offline.p%d", os.Getpid())
	}
	r.app = application.New(application.Options{
		Name: "LeaguePingo", Icon: appIcon, Description: "低干扰的英雄联盟战绩助手",
		Services:       []application.Service{application.NewServiceWithOptions(r.backend, application.ServiceOptions{MarshalError: apperrors.Marshal})},
		Assets:         application.AssetOptions{Handler: application.AssetFileServerFS(content)},
		Mac:            application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		SingleInstance: instance, OnShutdown: r.shutdown,
	})
}

func (r *desktopApplication) connectServices() {
	r.backend.resources = diagnostics.NewResources()
	go r.backend.resources.Run(r.ctx)
	discover := source.DiscoverTencent
	if r.offlineSmoke {
		discover = func() (source.Source, error) { return nil, source.ErrNotConnected }
	}
	store := r.backend.settings
	r.backend.guides = guides.New(r.ctx, filepath.Join(r.backend.configDir, "guide-cache"), nil)
	r.backend.guides.SetEnabled(store.Snapshot().GuidesEnabled)
	go r.forwardGuides()
	var err error
	r.backend.engine, err = engine.NewWithHistoryCache(discover, r.observeSnapshot, filepath.Join(r.backend.configDir, "history-cache.json"))
	if err != nil {
		log.Print("本地战绩缓存不可用或已重建，将按需重新获取战绩")
	}
	cfg := store.Snapshot()
	_ = r.backend.engine.ConfigurePostGame(cfg.AutoReturnEnabled, cfg.AutoQueueEnabled, cfg.AutoQueueDelay, cfg.AutoSkipHonor)
	_ = r.backend.engine.ConfigureAutoAccept(cfg.AutoAcceptEnabled, cfg.AutoAcceptDelay)
	_ = r.backend.engine.ConfigureAnalysisBrief(cfg.AutoAnalysisBrief, cfg.AnalysisAudience, cfg.AnalysisIncludeSelf)
	go r.forwardAutomation()
	go r.forwardPlayers()
}

func (r *desktopApplication) createMainWindowAndTray() {
	r.mainWindow = r.app.Window.NewWithOptions(mainWindowOptions(r.offlineSmoke))
	r.mainWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) { r.mainWindow.Hide(); event.Cancel() })
	r.tray = r.app.SystemTray.New()
	r.tray.SetIcon(appIcon)
	r.tray.SetTooltip("LeaguePingo")
	menu := r.app.NewMenu()
	menu.Add("打开 LeaguePingo").OnClick(func(*application.Context) { r.showMain() })
	r.updates = updatecheck.New(Version, UpdateManifestURL, nil)
	r.updateItem = menu.Add("发现新版本").SetHidden(true)
	r.updateItem.OnClick(func(*application.Context) {
		if state := r.updates.Snapshot(); state.Status == "available" {
			_ = r.app.Browser.OpenURL(state.URL)
		}
	})
	r.tray.SetMenu(menu)
	r.tray.OnClick(r.showMain)
	// The callbacks run after connectOverlay, when the controller is ready.
	menu.Add("显示／收起游戏助手").OnClick(func(*application.Context) { r.controller.toggle() })
	menu.Add("退出").OnClick(func(*application.Context) { r.app.Quit() })
}

func (r *desktopApplication) showMain() {
	r.mainWindow.Show()
	r.mainWindow.UnMinimise()
	r.mainWindow.Focus()
}

func (r *desktopApplication) shutdown() {
	r.cancel()
	if r.monitor != nil {
		r.monitor.Stop()
	}
	if r.clientMonitor != nil {
		r.clientMonitor.Stop()
	}
	if err := r.backend.settings.Save(); err != nil {
		log.Print("无法保存窗口设置")
	}
}

func wakeDesktop(ch chan<- struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
