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
	"flowpilot/internal/crypto"
	"flowpilot/internal/database"
	"flowpilot/internal/logs"
	"flowpilot/internal/models"
	"flowpilot/internal/proxy"
	"flowpilot/internal/queue"
	"flowpilot/internal/recorder"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type AppConfig struct {
	QueueConcurrency    int
	RetentionDays       int
	HealthCheckInterval int
	MaxProxyFailures    int
}

func DefaultAppConfig() AppConfig {
	return AppConfig{
		QueueConcurrency:    100,
		RetentionDays:       90,
		HealthCheckInterval: 300,
		MaxProxyFailures:    3,
	}
}

type App struct {
	ctx          context.Context
	db           *database.DB
	runner       *browser.Runner
	queue        *queue.Queue
	proxyManager *proxy.Manager
	dataDir      string
	batchEngine  *batch.Engine
	logExporter  *logs.Exporter
	config       AppConfig
	initErr      error

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

	a.batchEngine = batch.New(db)

	logsDir := filepath.Join(a.dataDir, "logs")
	logExporter, err := logs.NewExporter(db, logsDir)
	if err != nil {
		a.initErr = fmt.Errorf("init log exporter: %w", err)
		wailsRuntime.LogErrorf(ctx, "startup failed: %v", a.initErr)
		return
	}
	a.logExporter = logExporter

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
	if a.queue != nil {
		a.queue.Stop()
	}
	if a.proxyManager != nil {
		a.proxyManager.Stop()
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
