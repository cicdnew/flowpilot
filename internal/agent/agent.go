package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"flowpilot/internal/browser"
	"flowpilot/internal/crypto"
	"flowpilot/internal/database"
	"flowpilot/internal/models"
	"flowpilot/internal/proxy"
	"flowpilot/internal/queue"
)

// Agent is a headless background service that polls for pending tasks and executes them.
type Agent struct {
	db           *database.DB
	runner       *browser.Runner
	queue        *queue.Queue
	proxyManager *proxy.Manager
	dataDir      string
	pollInterval time.Duration
	cancel       context.CancelFunc
}

// Config holds settings for creating a background Agent.
type Config struct {
	DataDir             string
	MaxConcurrency      int
	ProxyConcurrency    int
	PollInterval        time.Duration
	HealthCheckInterval int
	MaxProxyFailures    int
	CaptureStepLogs     bool
	CaptureNetworkLogs  bool
	CaptureScreenshots  bool
	MaxExecutionLogs    int
}

func New(cfg Config) (*Agent, error) {
	if cfg.DataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		cfg.DataDir = filepath.Join(home, ".flowpilot")
	}
	if cfg.MaxConcurrency <= 0 {
		cfg.MaxConcurrency = 10
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 30 * time.Second
	}
	if cfg.MaxExecutionLogs <= 0 {
		cfg.MaxExecutionLogs = 250
	}

	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	if err := crypto.InitKey(cfg.DataDir); err != nil {
		return nil, fmt.Errorf("init encryption: %w", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "tasks.db")
	db, err := database.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}

	screenshotDir := filepath.Join(cfg.DataDir, "screenshots")
	runner, err := browser.NewRunner(screenshotDir)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("init browser runner: %w", err)
	}
	runner.SetForceHeadless(true)
	stepLogs := cfg.CaptureStepLogs
	networkLogs := cfg.CaptureNetworkLogs
	screenshots := cfg.CaptureScreenshots
	runner.SetDefaultLoggingPolicy(models.TaskLoggingPolicy{
		CaptureStepLogs:    &stepLogs,
		CaptureNetworkLogs: &networkLogs,
		CaptureScreenshots: &screenshots,
		MaxExecutionLogs:   cfg.MaxExecutionLogs,
	})

	q := queue.New(db, runner, cfg.MaxConcurrency, func(event models.TaskEvent) {
		log.Printf("[agent] task %s -> %s", event.TaskID, event.Status)
	})
	if cfg.ProxyConcurrency <= 0 {
		cfg.ProxyConcurrency = max(1, cfg.MaxConcurrency/2)
	}
	q.SetProxyConcurrencyLimit(cfg.ProxyConcurrency)

	healthInterval := cfg.HealthCheckInterval
	if healthInterval <= 0 {
		healthInterval = 300
	}
	maxFailures := cfg.MaxProxyFailures
	if maxFailures <= 0 {
		maxFailures = 3
	}
	pm := proxy.NewManager(db, models.ProxyPoolConfig{
		Strategy:            models.RotationRoundRobin,
		HealthCheckInterval: healthInterval,
		MaxFailures:         maxFailures,
	})
	q.SetProxyManager(pm)

	return &Agent{
		db:           db,
		runner:       runner,
		queue:        q,
		proxyManager: pm,
		dataDir:      cfg.DataDir,
		pollInterval: cfg.PollInterval,
	}, nil
}

func (a *Agent) Run(ctx context.Context) error {
	ctx, a.cancel = context.WithCancel(ctx)

	go a.proxyManager.StartHealthChecks(ctx)

	log.Println("[agent] started, polling for pending tasks...")

	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	a.processPending(ctx)

	for {
		select {
		case <-ticker.C:
			a.processPending(ctx)
		case <-ctx.Done():
			log.Println("[agent] shutting down...")
			a.Stop()
			return ctx.Err()
		}
	}
}

func (a *Agent) processPending(ctx context.Context) {
	tasks, err := a.db.ListTasksByStatus(ctx, models.TaskStatusPending)
	if err != nil {
		log.Printf("[agent] list pending tasks: %v", err)
		return
	}
	for _, task := range tasks {
		if err := a.queue.Submit(ctx, task); err != nil {
			log.Printf("[agent] submit task %s: %v", task.ID, err)
		}
	}
}

func (a *Agent) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
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
