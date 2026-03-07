package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	"flowpilot/internal/validation"

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
	batchEngine  *batch.Engine
	logExporter  *logs.Exporter

	// Recorder state (nil when not recording)
	recorderMu     sync.Mutex
	activeRecorder *recorder.Recorder
	recorderCancel context.CancelFunc
	recordedSteps  []models.RecordedStep
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
	a.dataDir = filepath.Join(home, ".flowpilot")
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

	// Initialize batch engine
	a.batchEngine = batch.New(db)

	// Initialize log exporter
	logsDir := filepath.Join(a.dataDir, "logs")
	logExporter, err := logs.NewExporter(db, logsDir)
	if err != nil {
		wailsRuntime.LogFatalf(ctx, "Failed to init log exporter: %v", err)
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
	n, err := a.db.PurgeOldRecords(90)
	if err != nil {
		wailsRuntime.LogWarningf(a.ctx, "retention cleanup error: %v", err)
		return
	}
	if n > 0 {
		wailsRuntime.LogInfof(a.ctx, "retention cleanup purged %d old records", n)
	}
}

// cleanup releases all application resources. Safe to call with nil fields.
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

// shutdown is called when the app is closing.
func (a *App) shutdown(ctx context.Context) {
	a.cleanup()
}

// shutdownFromSignal is called from OS signal handler (no Wails context available).
func (a *App) shutdownFromSignal() {
	a.cleanup()
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

// ListTasksPaginated returns a page of tasks with optional filtering.
func (a *App) ListTasksPaginated(page, pageSize int, status, tag string) (models.PaginatedTasks, error) {
	return a.db.ListTasksPaginated(page, pageSize, status, tag)
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

	created := make([]models.Task, 0, len(inputs))
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

// --- Recorded Flow API ---

// CreateRecordedFlow saves a recorded flow for reuse.
func (a *App) CreateRecordedFlow(name, description, originURL string, steps []models.RecordedStep) (*models.RecordedFlow, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("create flow: name is required")
	}
	flow := models.RecordedFlow{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Steps:       steps,
		OriginURL:   originURL,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := a.db.CreateRecordedFlow(flow); err != nil {
		return nil, fmt.Errorf("create flow: %w", err)
	}
	return &flow, nil
}

// ListRecordedFlows returns all flows.
func (a *App) ListRecordedFlows() ([]models.RecordedFlow, error) {
	return a.db.ListRecordedFlows()
}

// GetRecordedFlow fetches a flow by ID.
func (a *App) GetRecordedFlow(id string) (*models.RecordedFlow, error) {
	if id == "" {
		return nil, fmt.Errorf("get flow: id is required")
	}
	return a.db.GetRecordedFlow(id)
}

// DeleteRecordedFlow removes a flow.
func (a *App) DeleteRecordedFlow(id string) error {
	if id == "" {
		return fmt.Errorf("delete flow: id is required")
	}
	return a.db.DeleteRecordedFlow(id)
}

// SaveDOMSnapshot persists a DOM snapshot.
func (a *App) SaveDOMSnapshot(snapshot models.DOMSnapshot) error {
	return a.db.CreateDOMSnapshot(snapshot)
}

// ListDOMSnapshots returns snapshots for a flow.
func (a *App) ListDOMSnapshots(flowID string) ([]models.DOMSnapshot, error) {
	return a.db.ListDOMSnapshots(flowID)
}

// CreateTaskFromFlow creates a single task from a flow.
func (a *App) CreateTaskFromFlow(flowID, name, url string, proxyConfig models.ProxyConfig, priority int, autoStart bool, tags []string) (*models.Task, error) {
	flow, err := a.db.GetRecordedFlow(flowID)
	if err != nil {
		return nil, fmt.Errorf("create task from flow: %w", err)
	}
	steps := models.FlowToTaskSteps(*flow)
	if len(steps) > 0 && steps[0].Action == models.ActionNavigate && steps[0].Value == "" {
		steps[0].Value = url
	}
	return a.CreateTask(name, url, steps, proxyConfig, priority, autoStart, tags)
}

// CreateBatchFromFlow creates batch tasks from a flow and returns the batch group.
func (a *App) CreateBatchFromFlow(input models.AdvancedBatchInput) (models.BatchGroup, []models.Task, error) {
	if a.batchEngine == nil {
		return models.BatchGroup{}, nil, fmt.Errorf("batch engine unavailable")
	}
	flow, err := a.db.GetRecordedFlow(input.FlowID)
	if err != nil {
		return models.BatchGroup{}, nil, fmt.Errorf("get flow: %w", err)
	}
	group, tasks, err := a.batchEngine.CreateBatchFromFlow(a.ctx, *flow, input)
	if err != nil {
		return models.BatchGroup{}, nil, err
	}
	if input.AutoStart {
		if err := a.queue.SubmitBatch(a.ctx, tasks); err != nil {
			return group, tasks, fmt.Errorf("submit batch: %w", err)
		}
	}
	return group, tasks, nil
}

// GetBatchProgress returns summary status for a batch.
func (a *App) GetBatchProgress(batchID string) (models.BatchProgress, error) {
	return a.db.GetBatchProgress(batchID)
}

// ListTasksByBatch returns tasks in a batch.
func (a *App) ListTasksByBatch(batchID string) ([]models.Task, error) {
	return a.db.ListTasksByBatch(batchID)
}

// RetryFailedBatch re-queues all failed tasks in a batch.
func (a *App) RetryFailedBatch(batchID string) ([]models.Task, error) {
	if batchID == "" {
		return nil, fmt.Errorf("retry batch: batchID is required")
	}
	failed, err := a.db.ListTasksByBatchStatus(batchID, models.TaskStatusFailed)
	if err != nil {
		return nil, fmt.Errorf("retry batch: %w", err)
	}
	if len(failed) == 0 {
		return failed, nil
	}
	for _, task := range failed {
		if err := a.db.UpdateTaskStatus(task.ID, models.TaskStatusPending, "retry batch"); err != nil {
			return failed, fmt.Errorf("retry batch update: %w", err)
		}
	}
	if err := a.queue.SubmitBatch(a.ctx, failed); err != nil {
		return failed, fmt.Errorf("retry batch submit: %w", err)
	}
	return failed, nil
}

// ExportTaskLogs exports logs for a task and returns file paths.
func (a *App) ExportTaskLogs(taskID string) (string, string, error) {
	if a.logExporter == nil {
		return "", "", fmt.Errorf("log exporter unavailable")
	}
	return a.logExporter.ExportTaskLogs(taskID)
}

// ExportBatchLogs exports logs for a batch as a ZIP file.
func (a *App) ExportBatchLogs(batchID string) (string, error) {
	if a.logExporter == nil {
		return "", fmt.Errorf("log exporter unavailable")
	}
	return a.logExporter.ExportBatchLogs(batchID)
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

// GetQueueMetrics returns current queue metrics.
func (a *App) GetQueueMetrics() models.QueueMetrics {
	if a.queue == nil {
		return models.QueueMetrics{}
	}
	return a.queue.Metrics()
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

// ListProxies returns all proxies with credentials masked.
func (a *App) ListProxies() ([]models.Proxy, error) {
	proxies, err := a.db.ListProxies()
	if err != nil {
		return nil, err
	}
	for i := range proxies {
		proxies[i].Username = maskCredential(proxies[i].Username)
		proxies[i].Password = maskCredential(proxies[i].Password)
	}
	return proxies, nil
}

func maskCredential(s string) string {
	if len(s) <= 2 {
		return strings.Repeat("*", len(s))
	}
	return string(s[0]) + strings.Repeat("*", len(s)-2) + string(s[len(s)-1])
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

// --- Recorder API ---

// StartRecording opens a browser and starts capturing interactions.
func (a *App) StartRecording(url string) error {
	a.recorderMu.Lock()
	defer a.recorderMu.Unlock()

	if a.activeRecorder != nil {
		return fmt.Errorf("recording already in progress")
	}

	if strings.TrimSpace(url) == "" {
		return fmt.Errorf("start recording: url is required")
	}

	flowID := uuid.New().String()
	recCtx, recCancel := context.WithCancel(a.ctx)

	a.recordedSteps = nil
	a.activeRecorder = recorder.New(recCtx, flowID, func(step models.RecordedStep) {
		a.recorderMu.Lock()
		a.recordedSteps = append(a.recordedSteps, step)
		a.recorderMu.Unlock()
		wailsRuntime.EventsEmit(a.ctx, "recorder:step", step)
	})
	a.recorderCancel = recCancel

	snapshotDir := filepath.Join(a.dataDir, "snapshots", flowID)
	if snapshotter, err := recorder.NewSnapshotter(snapshotDir); err != nil {
		wailsRuntime.LogWarningf(a.ctx, "snapshot init failed: %v", err)
	} else {
		a.activeRecorder.SetSnapshotter(snapshotter)
	}

	if err := a.activeRecorder.Start(url); err != nil {
		a.activeRecorder = nil
		recCancel()
		return fmt.Errorf("start recording: %w", err)
	}

	a.activeRecorder.SetWSCallback(func(log models.WebSocketLog) {
		wailsRuntime.EventsEmit(a.ctx, "recorder:websocket", log)
	})

	wailsRuntime.LogInfof(a.ctx, "Recording started for flow %s at %s", flowID, url)
	return nil
}

// StopRecording stops the active recording session and returns captured steps.
func (a *App) StopRecording() ([]models.RecordedStep, error) {
	a.recorderMu.Lock()
	defer a.recorderMu.Unlock()

	if a.activeRecorder == nil {
		return nil, fmt.Errorf("no active recording session")
	}

	netLogs := a.activeRecorder.NetworkLogs()
	wsLogs := a.activeRecorder.WebSocketLogs()
	flowID := a.activeRecorder.FlowID()

	a.activeRecorder.Stop()

	if a.recorderCancel != nil {
		a.recorderCancel()
	}

	steps := make([]models.RecordedStep, len(a.recordedSteps))
	copy(steps, a.recordedSteps)

	a.activeRecorder = nil
	a.recorderCancel = nil
	a.recordedSteps = nil

	if len(netLogs) > 0 && a.db != nil {
		if err := a.db.InsertNetworkLogs(flowID, netLogs); err != nil {
			wailsRuntime.LogWarningf(a.ctx, "failed to persist network logs: %v", err)
		}
	}

	if len(wsLogs) > 0 && a.db != nil {
		if err := a.db.InsertWebSocketLogs(flowID, wsLogs); err != nil {
			wailsRuntime.LogWarningf(a.ctx, "failed to persist websocket logs: %v", err)
		}
	}

	wailsRuntime.LogInfof(a.ctx, "Recording stopped, captured %d steps, %d network requests, %d websocket events", len(steps), len(netLogs), len(wsLogs))
	return steps, nil
}

// PlayRecordedFlow creates and immediately runs a task from a recorded flow.
func (a *App) PlayRecordedFlow(flowID, url string, headless bool) (*models.Task, error) {
	flow, err := a.db.GetRecordedFlow(flowID)
	if err != nil {
		return nil, fmt.Errorf("play flow: %w", err)
	}
	steps := models.FlowToTaskSteps(*flow)
	if len(steps) > 0 && steps[0].Action == models.ActionNavigate && steps[0].Value == "" {
		steps[0].Value = url
	}
	if url == "" && flow.OriginURL != "" {
		url = flow.OriginURL
	}

	task := models.Task{
		ID:         uuid.New().String(),
		Name:       "Playback: " + flow.Name,
		URL:        url,
		Steps:      steps,
		Priority:   models.PriorityNormal,
		Status:     models.TaskStatusPending,
		MaxRetries: 0,
		Headless:   headless,
		FlowID:     flowID,
		CreatedAt:  time.Now(),
	}

	if err := a.db.CreateTask(task); err != nil {
		return nil, fmt.Errorf("play flow create task: %w", err)
	}
	if err := a.queue.Submit(a.ctx, task); err != nil {
		return nil, fmt.Errorf("play flow submit: %w", err)
	}
	return &task, nil
}

// GetAuditTrail returns lifecycle events for a task (or all if taskID is empty).
func (a *App) GetAuditTrail(taskID string, limit int) ([]models.TaskLifecycleEvent, error) {
	return a.db.ListAuditTrail(taskID, limit)
}

// PurgeOldData manually triggers retention cleanup and returns how many records were removed.
func (a *App) PurgeOldData(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 90
	}
	return a.db.PurgeOldRecords(retentionDays)
}

// ListWebSocketLogs returns persisted WebSocket logs for a recorded flow.
func (a *App) ListWebSocketLogs(flowID string) ([]models.WebSocketLog, error) {
	if flowID == "" {
		return nil, fmt.Errorf("list websocket logs: flowID is required")
	}
	return a.db.ListWebSocketLogs(flowID)
}

// IsRecording returns whether a recording session is active.
func (a *App) IsRecording() bool {
	a.recorderMu.Lock()
	defer a.recorderMu.Unlock()
	return a.activeRecorder != nil
}
