package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"flowpilot/internal/batch"
	"flowpilot/internal/browser"
	"flowpilot/internal/captcha"
	"flowpilot/internal/crypto"
	"flowpilot/internal/database"
	"flowpilot/internal/localproxy"
	"flowpilot/internal/logs"
	"flowpilot/internal/models"
	"flowpilot/internal/proxy"
	"flowpilot/internal/queue"
	"flowpilot/internal/recorder"
	"flowpilot/internal/scheduler"

	"github.com/chromedp/chromedp"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type AppConfig struct {
	QueueConcurrency    int
	ProxyConcurrency    int
	BrowserPoolSize     int
	BrowserMaxTabs      int
	RetentionDays       int
	HealthCheckInterval int
	MaxProxyFailures    int
	CaptureStepLogs     bool
	CaptureNetworkLogs  bool
	CaptureScreenshots  bool
	MaxExecutionLogs    int
}

func DefaultAppConfig() AppConfig {
	return AppConfig{
		QueueConcurrency:    200,
		ProxyConcurrency:    80,
		BrowserPoolSize:     100,
		BrowserMaxTabs:      20,
		RetentionDays:       90,
		HealthCheckInterval: 30,
		MaxProxyFailures:    2,
		CaptureStepLogs:     true,
		CaptureNetworkLogs:  false,
		CaptureScreenshots:  false,
		MaxExecutionLogs:    250,
	}
}

type App struct {
	ctx               context.Context
	db                *database.DB
	runner            *browser.Runner
	pool              *browser.BrowserPool
	queue             *queue.Queue
	proxyManager      *proxy.Manager
	localProxyManager *localproxy.Manager
	scheduler         *scheduler.Scheduler
	dataDir           string
	batchEngine       *batch.Engine
	logExporter       *logs.Exporter
	config            AppConfig
	initErr           error

	recorderMu     sync.Mutex
	activeRecorder *recorder.Recorder
	recorderCancel context.CancelFunc
	recordedSteps  []models.RecordedStep
}

func NewApp() *App {
	return &App{config: DefaultAppConfig()}
}

func (a *App) ready() error {
	if a.initErr != nil {
		return fmt.Errorf("app not initialized: %w", a.initErr)
	}
	return nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	home, err := os.UserHomeDir()
	if err != nil {
		a.initErr = fmt.Errorf("get home directory: %w", err)
		wailsRuntime.LogErrorf(ctx, "startup failed: %v", a.initErr)
		return
	}
	a.dataDir = filepath.Join(home, ".flowpilot")
	if err := os.MkdirAll(a.dataDir, 0o700); err != nil {
		a.initErr = fmt.Errorf("create data directory: %w", err)
		wailsRuntime.LogErrorf(ctx, "startup failed: %v", a.initErr)
		return
	}

	if err := crypto.InitKey(a.dataDir); err != nil {
		a.initErr = fmt.Errorf("init encryption: %w", err)
		wailsRuntime.LogErrorf(ctx, "startup failed: %v", a.initErr)
		return
	}

	dbPath := filepath.Join(a.dataDir, "tasks.db")
	db, err := database.New(dbPath)
	if err != nil {
		a.initErr = fmt.Errorf("init database: %w", err)
		wailsRuntime.LogErrorf(ctx, "startup failed: %v", a.initErr)
		return
	}
	a.db = db

	screenshotDir := filepath.Join(a.dataDir, "screenshots")
	runner, err := browser.NewRunner(screenshotDir)
	if err != nil {
		a.initErr = fmt.Errorf("init browser runner: %w", err)
		wailsRuntime.LogErrorf(ctx, "startup failed: %v", a.initErr)
		return
	}
	a.runner = runner
	a.localProxyManager = localproxy.NewManager(5 * time.Minute)
	a.runner.SetLocalProxyManager(a.localProxyManager)
	stepLogs := a.config.CaptureStepLogs
	networkLogs := a.config.CaptureNetworkLogs
	screenshots := a.config.CaptureScreenshots
	a.runner.SetDefaultLoggingPolicy(models.TaskLoggingPolicy{
		CaptureStepLogs:    &stepLogs,
		CaptureNetworkLogs: &networkLogs,
		CaptureScreenshots: &screenshots,
		MaxExecutionLogs:   a.config.MaxExecutionLogs,
	})

	pool := browser.NewBrowserPool(browser.PoolConfig{
		Size:    a.config.BrowserPoolSize,
		MaxTabs: a.config.BrowserMaxTabs,
	}, chromedp.DefaultExecAllocatorOptions[:])
	a.pool = pool
	a.runner.SetPool(pool)

	captchaConfig, err := a.db.GetActiveCaptchaConfig(a.ctx)
	if err == nil && captchaConfig != nil {
		solver, solverErr := captcha.NewSolver(*captchaConfig)
		if solverErr == nil {
			a.runner.SetCaptchaSolver(solver)
		}
	}

	a.proxyManager = proxy.NewManager(db, models.ProxyPoolConfig{
		Strategy:            models.RotationRoundRobin,
		HealthCheckInterval: a.config.HealthCheckInterval,
		MaxFailures:         a.config.MaxProxyFailures,
	})
	go a.proxyManager.StartHealthChecks(ctx)

	a.queue = queue.New(db, runner, a.config.QueueConcurrency, func(event models.TaskEvent) {
		wailsRuntime.EventsEmit(ctx, "task:event", event)
	})
	a.queue.SetProxyManager(a.proxyManager)
	a.queue.SetProxyConcurrencyLimit(a.config.ProxyConcurrency)

	// Recover tasks stuck in running/queued from a previous crash.
	if err := a.queue.RecoverStaleTasks(ctx); err != nil {
		wailsRuntime.LogWarningf(ctx, "recover stale tasks: %v", err)
	}

	a.batchEngine = batch.New(db)

	logsDir := filepath.Join(a.dataDir, "logs")
	logExporter, err := logs.NewExporter(db, logsDir)
	if err != nil {
		a.initErr = fmt.Errorf("init log exporter: %w", err)
		wailsRuntime.LogErrorf(ctx, "startup failed: %v", a.initErr)
		return
	}
	a.logExporter = logExporter

	a.scheduler = scheduler.New(a.db, a, 30*time.Second)
	a.scheduler.Start(ctx)

	go a.runRetentionCleanup(ctx)

	wailsRuntime.LogInfo(ctx, "Application started successfully")
}

func (a *App) runRetentionCleanup(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	a.purgeOnce()

	for {
		select {
		case <-ticker.C:
			a.purgeOnce()
		case <-ctx.Done():
			return
		}
	}
}

func (a *App) purgeOnce() {
	if a.db == nil {
		return
	}
	n, err := a.db.PurgeOldRecords(a.ctx, a.config.RetentionDays)
	if err != nil {
		wailsRuntime.LogWarningf(a.ctx, "retention cleanup error: %v", err)
		return
	}
	if n > 0 {
		wailsRuntime.LogInfof(a.ctx, "retention cleanup purged %d old records", n)
	}
}

func (a *App) cleanup() {
	if a.scheduler != nil {
		a.scheduler.Stop()
	}
	if a.queue != nil {
		a.queue.Stop()
	}
	if a.pool != nil {
		a.pool.Stop()
	}
	if a.proxyManager != nil {
		a.proxyManager.Stop()
	}
	if a.localProxyManager != nil {
		a.localProxyManager.Stop()
	}
	if a.db != nil {
		a.db.Close()
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.cleanup()
}

func (a *App) shutdownFromSignal() {
	a.cleanup()
}
