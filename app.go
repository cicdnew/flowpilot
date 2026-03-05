package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"web-automation/internal/browser"
	"web-automation/internal/crypto"
	"web-automation/internal/database"
	"web-automation/internal/models"
	"web-automation/internal/proxy"
	"web-automation/internal/queue"
	"web-automation/internal/validation"

	"github.com/google/uuid"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct holds the application state and dependencies.
type App struct {
	ctx          context.Context
	db           *database.DB
	runner       *browser.Runner
	queue        *queue.Queue
	proxyManager *proxy.Manager
	dataDir      string
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. Initializes all dependencies.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Setup data directory
	home, err := os.UserHomeDir()
	if err != nil {
		wailsRuntime.LogFatalf(ctx, "Failed to get home directory: %v", err)
		return
	}
	a.dataDir = filepath.Join(home, ".web-automation")
	if err := os.MkdirAll(a.dataDir, 0o700); err != nil {
		wailsRuntime.LogFatalf(ctx, "Failed to create data directory: %v", err)
		return
	}

	// Initialize encryption key
	if err := crypto.InitKey(a.dataDir); err != nil {
		wailsRuntime.LogFatalf(ctx, "Failed to init encryption: %v", err)
		return
	}

	// Initialize database
	dbPath := filepath.Join(a.dataDir, "tasks.db")
	db, err := database.New(dbPath)
	if err != nil {
		wailsRuntime.LogFatalf(ctx, "Failed to init database: %v", err)
		return
	}
	a.db = db

	// Initialize browser runner
	screenshotDir := filepath.Join(a.dataDir, "screenshots")
	runner, err := browser.NewRunner(screenshotDir)
	if err != nil {
		wailsRuntime.LogFatalf(ctx, "Failed to init browser runner: %v", err)
		return
	}
	a.runner = runner

	// Initialize proxy manager
	a.proxyManager = proxy.NewManager(db, models.ProxyPoolConfig{
		Strategy:            models.RotationRoundRobin,
		HealthCheckInterval: 300,
		MaxFailures:         3,
	})
	go a.proxyManager.StartHealthChecks(ctx)

	// Initialize task queue
	a.queue = queue.New(db, runner, 100, func(event models.TaskEvent) {
		wailsRuntime.EventsEmit(ctx, "task:event", event)
	})
	a.queue.SetProxyManager(a.proxyManager)

	wailsRuntime.LogInfo(ctx, "Application started successfully")
}

// shutdown is called when the app is closing.
func (a *App) shutdown(ctx context.Context) {
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

// shutdownFromSignal is called from OS signal handler (no Wails context available).
func (a *App) shutdownFromSignal() {
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

// --- Task API (bound to frontend) ---

// CreateTask creates a new task and optionally starts it.
func (a *App) CreateTask(name, url string, steps []models.TaskStep, proxyConfig models.ProxyConfig, priority int, autoStart bool, tags []string) (*models.Task, error) {
	if err := validation.ValidateTask(name, url, steps, models.TaskPriority(priority), false); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	if err := validation.ValidateTags(tags); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	task := models.Task{
		ID:         uuid.New().String(),
		Name:       name,
		URL:        url,
		Steps:      steps,
		Proxy:      proxyConfig,
		Priority:   models.TaskPriority(priority),
		Status:     models.TaskStatusPending,
		MaxRetries: 3,
		Tags:       tags,
		CreatedAt:  time.Now(),
	}

	if err := a.db.CreateTask(task); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	if autoStart {
		if err := a.queue.Submit(a.ctx, task); err != nil {
			return nil, fmt.Errorf("submit task: %w", err)
		}
	}

	return &task, nil
}

// GetTask returns a task by ID.
func (a *App) GetTask(id string) (*models.Task, error) {
	if id == "" {
		return nil, fmt.Errorf("get task: id is required")
	}
	return a.db.GetTask(id)
}

// ListTasks returns all tasks.
func (a *App) ListTasks() ([]models.Task, error) {
	return a.db.ListTasks()
}

// ListTasksByStatus returns tasks with a given status.
func (a *App) ListTasksByStatus(status string) ([]models.Task, error) {
	if err := validation.ValidateStatus(status); err != nil {
		return nil, fmt.Errorf("list tasks by status: %w", err)
	}
	return a.db.ListTasksByStatus(models.TaskStatus(status))
}

// StartTask submits a pending task to the queue.
func (a *App) StartTask(id string) error {
	if id == "" {
		return fmt.Errorf("start task: id is required")
	}
	task, err := a.db.GetTask(id)
	if err != nil {
		return fmt.Errorf("get task for start: %w", err)
	}
	return a.queue.Submit(a.ctx, *task)
}

// StartAllPending submits all pending tasks to the queue.
func (a *App) StartAllPending() error {
	tasks, err := a.db.ListTasksByStatus(models.TaskStatusPending)
	if err != nil {
		return fmt.Errorf("list pending tasks: %w", err)
	}
	return a.queue.SubmitBatch(a.ctx, tasks)
}

// CancelTask cancels a running task.
func (a *App) CancelTask(id string) error {
	if id == "" {
		return fmt.Errorf("cancel task: id is required")
	}
	return a.queue.Cancel(id)
}

// UpdateTask updates an existing pending/failed task.
func (a *App) UpdateTask(id, name, url string, steps []models.TaskStep, proxyConfig models.ProxyConfig, priority int, tags []string) error {
	if err := validation.ValidateTask(name, url, steps, models.TaskPriority(priority), false); err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	if err := validation.ValidateTags(tags); err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return a.db.UpdateTask(id, name, url, steps, proxyConfig, models.TaskPriority(priority), tags)
}

// CreateBatch creates multiple tasks at once. Validates all before creating any.
func (a *App) CreateBatch(inputs []models.BatchTaskInput, autoStart bool) ([]models.Task, error) {
	for i, input := range inputs {
		if err := validation.ValidateTask(input.Name, input.URL, input.Steps, models.TaskPriority(input.Priority), false); err != nil {
			return nil, fmt.Errorf("task %d: %w", i, err)
		}
	}

	var created []models.Task
	for _, input := range inputs {
		task := models.Task{
			ID:         uuid.New().String(),
			Name:       input.Name,
			URL:        input.URL,
			Steps:      input.Steps,
			Proxy:      input.Proxy,
			Priority:   models.TaskPriority(input.Priority),
			Status:     models.TaskStatusPending,
			MaxRetries: 3,
			CreatedAt:  time.Now(),
		}
		if err := a.db.CreateTask(task); err != nil {
			return created, fmt.Errorf("create task %d: %w", len(created), err)
		}
		created = append(created, task)
	}

	if autoStart && a.queue != nil {
		if err := a.queue.SubmitBatch(a.ctx, created); err != nil {
			return created, fmt.Errorf("submit batch: %w", err)
		}
	}

	return created, nil
}

// DeleteTask cancels a running task (if any) and deletes it.
func (a *App) DeleteTask(id string) error {
	if id == "" {
		return fmt.Errorf("delete task: id is required")
	}
	if a.queue != nil {
		if err := a.queue.Cancel(id); err != nil {
			// Log but don't fail — task may not be running/queued, which is fine.
			wailsRuntime.LogWarningf(a.ctx, "cancel before delete for task %s: %v", id, err)
		}
	}
	return a.db.DeleteTask(id)
}

// GetTaskStats returns task count per status.
func (a *App) GetTaskStats() (map[string]int, error) {
	return a.db.GetTaskStats()
}

// GetRunningCount returns how many tasks are currently running.
func (a *App) GetRunningCount() int {
	if a.queue == nil {
		return 0
	}
	return a.queue.RunningCount()
}

// --- Proxy API ---

// AddProxy adds a proxy to the pool.
func (a *App) AddProxy(server, protocol, username, password, geo string) (*models.Proxy, error) {
	if err := validation.ValidateProxy(server, models.ProxyProtocol(protocol)); err != nil {
		return nil, fmt.Errorf("add proxy: %w", err)
	}

	p := models.Proxy{
		ID:        uuid.New().String(),
		Server:    server,
		Protocol:  models.ProxyProtocol(protocol),
		Username:  username,
		Password:  password,
		Geo:       geo,
		Status:    models.ProxyStatusUnknown,
		CreatedAt: time.Now(),
	}

	if err := a.db.CreateProxy(p); err != nil {
		return nil, fmt.Errorf("add proxy: %w", err)
	}
	return &p, nil
}

// ListProxies returns all proxies.
func (a *App) ListProxies() ([]models.Proxy, error) {
	return a.db.ListProxies()
}

// DeleteProxy removes a proxy.
func (a *App) DeleteProxy(id string) error {
	if id == "" {
		return fmt.Errorf("delete proxy: id is required")
	}
	return a.db.DeleteProxy(id)
}

// --- Export API ---

// ExportResultsJSON exports all task results as JSON.
func (a *App) ExportResultsJSON() (string, error) {
	tasks, err := a.db.ListTasksByStatus(models.TaskStatusCompleted)
	if err != nil {
		return "", fmt.Errorf("list completed tasks: %w", err)
	}
	if tasks == nil {
		tasks = []models.Task{}
	}

	exportPath := filepath.Join(a.dataDir, fmt.Sprintf("export_%d.json", time.Now().Unix()))
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal tasks to JSON: %w", err)
	}

	if err := os.WriteFile(exportPath, data, 0o600); err != nil {
		return "", fmt.Errorf("write export file: %w", err)
	}
	return exportPath, nil
}

// ExportResultsCSV exports task results as CSV.
func (a *App) ExportResultsCSV() (string, error) {
	tasks, err := a.db.ListTasksByStatus(models.TaskStatusCompleted)
	if err != nil {
		return "", fmt.Errorf("list completed tasks: %w", err)
	}

	exportPath := filepath.Join(a.dataDir, fmt.Sprintf("export_%d.csv", time.Now().Unix()))
	file, err := os.OpenFile(exportPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("create export file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)

	if err := writer.Write([]string{"ID", "Name", "URL", "Status", "Error", "Duration", "CreatedAt", "CompletedAt"}); err != nil {
		return "", fmt.Errorf("write CSV header: %w", err)
	}

	for _, t := range tasks {
		duration := ""
		if t.Result != nil {
			duration = t.Result.Duration.String()
		}
		completedAt := ""
		if t.CompletedAt != nil {
			completedAt = t.CompletedAt.Format(time.RFC3339)
		}
		if err := writer.Write([]string{
			t.ID, t.Name, t.URL, string(t.Status), t.Error,
			duration, t.CreatedAt.Format(time.RFC3339), completedAt,
		}); err != nil {
			return "", fmt.Errorf("write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("flush CSV writer: %w", err)
	}

	return exportPath, nil
}
