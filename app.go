package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
)

type AppConfig struct {
	QueueConcurrency    int
	ProxyConcurrency    int
	BrowserPoolSize     int
	BrowserMaxTabs      int
	RetentionDays       int
	HealthCheckInterval int
	MaxProxyFailures    int
	HealthCheckURL      string
	CaptureStepLogs     bool
	CaptureNetworkLogs  bool
	CaptureScreenshots  bool
	MaxExecutionLogs    int
	RetryBackoffBaseMs  int
	LogSlogLevel        string
	MetricsAddr         string
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
		HealthCheckURL:      "https://httpbin.org/ip",
		CaptureStepLogs:     true,
		CaptureNetworkLogs:  false,
		CaptureScreenshots:  false,
		MaxExecutionLogs:    250,
		RetryBackoffBaseMs:  5000,
		LogSlogLevel:        "info",
		MetricsAddr:         defaultMetricsAddr(),
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
	metricsMu         sync.Mutex
	metricsServer     *http.Server
	metricsListener   net.Listener

	recorderMu     sync.Mutex
	activeRecorder *recorder.Recorder
	recorderCancel context.CancelFunc
	recordedSteps  []models.RecordedStep

	configMu      sync.Mutex
	configPath    string
	configModTime time.Time
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

func (a *App) loadConfigFromDisk() error {
	a.configMu.Lock()
	configPath := a.configPath
	cfg := a.config
	a.configMu.Unlock()
	if configPath == "" {
		return nil
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg.MetricsAddr = normalizeMetricsAddr(cfg.MetricsAddr)
			a.configMu.Lock()
			a.config = cfg
			a.configMu.Unlock()
			return nil
		}
		return fmt.Errorf("read config file: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}
	cfg.MetricsAddr = normalizeMetricsAddr(cfg.MetricsAddr)
	if cfg.LogSlogLevel == "" {
		cfg.LogSlogLevel = DefaultAppConfig().LogSlogLevel
	}
	a.configMu.Lock()
	a.config = cfg
	a.configMu.Unlock()
	return nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	home, err := os.UserHomeDir()
	if err != nil {
		a.initErr = fmt.Errorf("get home directory: %w", err)
		logErrorf(ctx, "startup failed: %v", a.initErr)
		return
	}
	a.dataDir = filepath.Join(home, ".flowpilot")
	if err := os.MkdirAll(a.dataDir, 0o700); err != nil {
		a.initErr = fmt.Errorf("create data directory: %w", err)
		logErrorf(ctx, "startup failed: %v", a.initErr)
		return
	}

	a.configMu.Lock()
	a.configPath = filepath.Join(a.dataDir, "config.json")
	a.configMu.Unlock()
	if err := a.loadConfigFromDisk(); err != nil {
		a.initErr = fmt.Errorf("load config: %w", err)
		logErrorf(ctx, "startup failed: %v", a.initErr)
		return
	}
	wailsRuntimeLoggingEnabled.Store(true)
	a.configMu.Lock()
	logLevel := a.config.LogSlogLevel
	a.configMu.Unlock()
	logs.Init(logLevel)

	if err := crypto.InitKey(a.dataDir); err != nil {
		a.initErr = fmt.Errorf("init encryption: %w", err)
		logErrorf(ctx, "startup failed: %v", a.initErr)
		return
	}

	dbConfig := database.DatabaseConfig{URL: filepath.Join(a.dataDir, "tasks.db")}
	if databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL")); databaseURL != "" {
		dbConfig.URL = databaseURL
		dbConfig.AuthToken = strings.TrimSpace(os.Getenv("TURSO_AUTH_TOKEN"))
		dbConfig.LocalPath = strings.TrimSpace(os.Getenv("DATABASE_PATH"))
	} else if databasePath := strings.TrimSpace(os.Getenv("DATABASE_PATH")); databasePath != "" {
		dbConfig.URL = databasePath
	}
	db, err := database.NewWithConfig(dbConfig)
	if err != nil {
		a.initErr = fmt.Errorf("init database: %w", err)
		logErrorf(ctx, "startup failed: %v", a.initErr)
		return
	}
	a.db = db

	screenshotDir := filepath.Join(a.dataDir, "screenshots")
	runner, err := browser.NewRunner(screenshotDir)
	if err != nil {
		a.initErr = fmt.Errorf("init browser runner: %w", err)
		logErrorf(ctx, "startup failed: %v", a.initErr)
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
		HealthCheckURL:      a.config.HealthCheckURL,
	})
	go a.proxyManager.StartHealthChecks(ctx)

	a.queue = queue.New(db, runner, a.config.QueueConcurrency, func(event models.TaskEvent) {
		safeWailsEmit(ctx, "task:event", event)
	})
	a.queue.SetProxyManager(a.proxyManager)
	a.queue.SetProxyConcurrencyLimit(a.config.ProxyConcurrency)
	a.queue.SetRetryBackoffBaseMs(a.config.RetryBackoffBaseMs)

	// Recover tasks stuck in running/queued from a previous crash.
	if err := a.queue.RecoverStaleTasks(ctx); err != nil {
		logWarningf(ctx, "recover stale tasks: %v", err)
	}

	a.batchEngine = batch.New(db)

	logsDir := filepath.Join(a.dataDir, "logs")
	logExporter, err := logs.NewExporter(db, logsDir)
	if err != nil {
		a.initErr = fmt.Errorf("init log exporter: %w", err)
		logErrorf(ctx, "startup failed: %v", a.initErr)
		return
	}
	a.logExporter = logExporter

	a.scheduler = scheduler.New(a.db, a, 30*time.Second)
	a.scheduler.Start(ctx)
	a.startMetricsServer(ctx)

	go a.runRetentionCleanup(ctx)

	a.configMu.Lock()
	if info, err := os.Stat(a.configPath); err == nil {
		a.configModTime = info.ModTime()
	} else {
		a.configModTime = time.Time{}
	}
	a.configMu.Unlock()
	go a.watchConfig(ctx)

	logInfof(ctx, "Application started successfully")
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
		logWarningf(a.ctx, "retention cleanup error: %v", err)
		return
	}
	if n > 0 {
		logInfof(a.ctx, "retention cleanup purged %d old records", n)
	}
}

// watchConfig polls the config file every 30 seconds for changes (by mtime).
// If the file has been modified, it reloads hot-reloadable settings:
// ProxyConcurrency, HealthCheckInterval, and HealthCheckURL are applied live.
// QueueConcurrency and BrowserPoolSize changes are logged as requiring a restart.
func (a *App) watchConfig(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.checkAndReloadConfig(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (a *App) checkAndReloadConfig(ctx context.Context) {
	a.configMu.Lock()
	configPath := a.configPath
	lastMod := a.configModTime
	a.configMu.Unlock()

	if configPath == "" {
		return
	}

	info, err := os.Stat(configPath)
	if err != nil {
		return
	}
	if !info.ModTime().After(lastMod) {
		return
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		logWarningf(ctx, "config hot-reload: read file: %v", err)
		return
	}

	var newCfg AppConfig
	if err := json.Unmarshal(data, &newCfg); err != nil {
		logWarningf(ctx, "config hot-reload: parse JSON: %v", err)
		return
	}

	a.configMu.Lock()
	a.configModTime = info.ModTime()
	old := a.config
	a.configMu.Unlock()

	if newCfg.QueueConcurrency != 0 && newCfg.QueueConcurrency != old.QueueConcurrency {
		logInfof(ctx, "config hot-reload: QueueConcurrency changed (%d → %d): restart required", old.QueueConcurrency, newCfg.QueueConcurrency)
	}
	if newCfg.BrowserPoolSize != 0 && newCfg.BrowserPoolSize != old.BrowserPoolSize {
		logInfof(ctx, "config hot-reload: BrowserPoolSize changed (%d → %d): restart required", old.BrowserPoolSize, newCfg.BrowserPoolSize)
	}

	if newCfg.ProxyConcurrency != 0 && newCfg.ProxyConcurrency != old.ProxyConcurrency {
		if a.queue != nil {
			a.queue.SetProxyConcurrencyLimit(newCfg.ProxyConcurrency)
		}
		logInfof(ctx, "config hot-reload: ProxyConcurrency updated (%d → %d)", old.ProxyConcurrency, newCfg.ProxyConcurrency)
	}

	if (newCfg.HealthCheckInterval != 0 && newCfg.HealthCheckInterval != old.HealthCheckInterval) ||
		(newCfg.HealthCheckURL != "" && newCfg.HealthCheckURL != old.HealthCheckURL) {
		if a.proxyManager != nil {
			interval := old.HealthCheckInterval
			if newCfg.HealthCheckInterval != 0 {
				interval = newCfg.HealthCheckInterval
			}
			url := old.HealthCheckURL
			if newCfg.HealthCheckURL != "" {
				url = newCfg.HealthCheckURL
			}
			a.proxyManager.UpdateHealthCheckConfig(interval, url)
			logInfof(ctx, "config hot-reload: HealthCheck updated (interval=%ds, url=%s)", interval, url)
		}
	}

	a.configMu.Lock()
	if newCfg.ProxyConcurrency != 0 {
		a.config.ProxyConcurrency = newCfg.ProxyConcurrency
	}
	if newCfg.HealthCheckInterval != 0 {
		a.config.HealthCheckInterval = newCfg.HealthCheckInterval
	}
	if newCfg.HealthCheckURL != "" {
		a.config.HealthCheckURL = newCfg.HealthCheckURL
	}
	if newCfg.MetricsAddr != "" {
		a.config.MetricsAddr = normalizeMetricsAddr(newCfg.MetricsAddr)
	}
	a.configMu.Unlock()
}

func (a *App) cleanup() {
	a.recorderMu.Lock()
	if a.activeRecorder != nil {
		a.activeRecorder.Stop()
	}
	if a.recorderCancel != nil {
		a.recorderCancel()
	}
	a.recorderMu.Unlock()
	if a.scheduler != nil {
		a.scheduler.Stop()
	}
	if a.queue != nil {
		a.queue.SetDrainTimeout(30 * time.Second)
		a.queue.Stop()
	}
	a.stopMetricsServer(a.ctx)
	if a.runner != nil {
		a.runner.StopProxyPools()
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

// getTaskMetrics returns the latest in-memory task metrics snapshot.
func (a *App) getTaskMetrics() models.TaskMetrics {
	if a.queue == nil {
		return models.TaskMetrics{}
	}
	return a.queue.TaskMetrics()
}

func (a *App) shutdownFromSignal() {
	a.cleanup()
}
