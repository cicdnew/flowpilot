package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"flowpilot/internal/browser"
	"flowpilot/internal/crypto"
	"flowpilot/internal/database"
	"flowpilot/internal/localproxy"
	"flowpilot/internal/logs"
	"flowpilot/internal/models"
	"flowpilot/internal/proxy"
	"flowpilot/internal/queue"
)

func initTestCrypto(t *testing.T) {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	crypto.ResetForTest()
	if err := crypto.InitKeyWithBytes(key); err != nil {
		t.Fatalf("init crypto key: %v", err)
	}
	t.Cleanup(func() { crypto.ResetForTest() })
}

func setupTestApp(t *testing.T) *App {
	t.Helper()
	initTestCrypto(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	app := &App{
		ctx:     context.Background(),
		db:      db,
		dataDir: dir,
	}
	return app
}

func validSteps() []models.TaskStep {
	return []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionClick, Selector: "#btn"},
	}
}

func TestAppCreateTaskValid(t *testing.T) {
	app := setupTestApp(t)

	task, err := app.CreateTask(CreateTaskParams{Name: "Test Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task.Name != "Test Task" {
		t.Errorf("Name: got %q, want %q", task.Name, "Test Task")
	}
	if task.Status != models.TaskStatusPending {
		t.Errorf("Status: got %q, want %q", task.Status, models.TaskStatusPending)
	}
}

func TestAppCreateTaskEmptyName(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.CreateTask(CreateTaskParams{Name: "", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
	if !strings.Contains(err.Error(), "name must not be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAppCreateTaskInvalidURL(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.CreateTask(CreateTaskParams{Name: "Task", URL: "not-a-url", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAppCreateTaskInvalidStepAction(t *testing.T) {
	app := setupTestApp(t)

	badSteps := []models.TaskStep{
		{Action: "bogus_action"},
	}
	_, err := app.CreateTask(CreateTaskParams{Name: "Task", URL: "https://example.com", Steps: badSteps, ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err == nil {
		t.Fatal("expected error for invalid step action, got nil")
	}
	if !strings.Contains(err.Error(), "invalid step action") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAppCreateTaskInvalidPriority(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.CreateTask(CreateTaskParams{Name: "Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 99, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err == nil {
		t.Fatal("expected error for invalid priority, got nil")
	}
	if !strings.Contains(err.Error(), "priority") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAppAddProxyValid(t *testing.T) {
	app := setupTestApp(t)

	p, err := app.AddProxy("proxy.example.com:8080", "http", "user", "pass", "US")
	if err != nil {
		t.Fatalf("AddProxy: %v", err)
	}
	if p.Server != "proxy.example.com:8080" {
		t.Errorf("Server: got %q, want %q", p.Server, "proxy.example.com:8080")
	}
	if p.Protocol != models.ProxyHTTP {
		t.Errorf("Protocol: got %q, want %q", p.Protocol, models.ProxyHTTP)
	}
}

func TestAppAddProxyInvalidProtocol(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.AddProxy("proxy.example.com:8080", "ftp", "user", "pass", "US")
	if err == nil {
		t.Fatal("expected error for invalid protocol, got nil")
	}
	if !strings.Contains(err.Error(), "protocol") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAppAddProxyInvalidServer(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.AddProxy("not-host-port", "http", "", "", "")
	if err == nil {
		t.Fatal("expected error for invalid server, got nil")
	}
	if !strings.Contains(err.Error(), "server") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAppDeleteTask(t *testing.T) {
	app := setupTestApp(t)

	task, err := app.CreateTask(CreateTaskParams{Name: "Delete Me", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := app.DeleteTask(task.ID); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}

	_, err = app.GetTask(task.ID)
	if err == nil {
		t.Error("expected error after deletion, got nil")
	}
}

func TestAppGetTask(t *testing.T) {
	app := setupTestApp(t)

	created, err := app.CreateTask(CreateTaskParams{Name: "Get Me", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	got, err := app.GetTask(created.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Name != "Get Me" {
		t.Errorf("Name: got %q, want %q", got.Name, "Get Me")
	}
}

func TestAppGetTaskNotFound(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.GetTask("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent task, got nil")
	}
}

func TestAppListTasks(t *testing.T) {
	app := setupTestApp(t)

	for i := 0; i < 3; i++ {
		_, err := app.CreateTask(CreateTaskParams{Name: "Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
		if err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}
	}

	tasks, err := app.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("task count: got %d, want 3", len(tasks))
	}
}

func TestAppCreateTaskEvalBlocked(t *testing.T) {
	app := setupTestApp(t)

	evalSteps := []models.TaskStep{
		{Action: models.ActionEval, Value: "document.cookie"},
	}
	_, err := app.CreateTask(CreateTaskParams{Name: "Eval Task", URL: "https://example.com", Steps: evalSteps, ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err == nil {
		t.Fatal("expected error for eval step, got nil")
	}
	if !strings.Contains(err.Error(), "eval") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAppCreateTaskRejectsMalformedSupportedStep(t *testing.T) {
	app := setupTestApp(t)

	steps := []models.TaskStep{
		{Action: models.ActionFileUpload, Value: "/tmp/upload.txt"},
	}
	_, err := app.CreateTask(CreateTaskParams{Name: "Upload Task", URL: "https://example.com", Steps: steps, ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err == nil {
		t.Fatal("expected error for malformed supported step, got nil")
	}
	if !strings.Contains(err.Error(), "selector") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAppSensitiveMethodsRequireReady(t *testing.T) {
	app := &App{initErr: errors.New("boom")}

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "create task",
			call: func() error {
				_, err := app.CreateTask(CreateTaskParams{Name: "Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
				return err
			},
		},
		{
			name: "list proxies",
			call: func() error {
				_, err := app.ListProxies()
				return err
			},
		},
		{
			name: "start task",
			call: func() error {
				return app.StartTask("task-1")
			},
		},
		{
			name: "cancel task",
			call: func() error {
				return app.CancelTask("task-1")
			},
		},
		{
			name: "update task",
			call: func() error {
				return app.UpdateTask("task-1", database.TaskUpdateParams{Name: "Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Tags: nil, Timeout: 0, LoggingPolicy: nil}, 5)
			},
		},
		{
			name: "start recording",
			call: func() error {
				return app.StartRecording("https://example.com")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatal("expected not-ready error, got nil")
			}
			if !strings.Contains(err.Error(), "app not initialized") {
				t.Fatalf("expected app not initialized error, got: %v", err)
			}
			if !strings.Contains(err.Error(), "boom") {
				t.Fatalf("expected wrapped init error, got: %v", err)
			}
		})
	}
}

func setupTestAppWithQueue(t *testing.T) *App {
	t.Helper()
	initTestCrypto(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	screenshotDir := filepath.Join(dir, "screenshots")
	runner, err := browser.NewRunner(screenshotDir)
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	q := queue.New(db, runner, 10, nil)
	t.Cleanup(func() { q.Stop() })

	return &App{
		ctx:     context.Background(),
		db:      db,
		runner:  runner,
		queue:   q,
		dataDir: dir,
	}
}

func TestAppListTasksByStatus(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.CreateTask(CreateTaskParams{Name: "Pending 1", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	_, err = app.CreateTask(CreateTaskParams{Name: "Pending 2", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	pending, err := app.ListTasksByStatus("pending")
	if err != nil {
		t.Fatalf("ListTasksByStatus: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("pending count: got %d, want 2", len(pending))
	}

	running, err := app.ListTasksByStatus("running")
	if err != nil {
		t.Fatalf("ListTasksByStatus: %v", err)
	}
	if len(running) != 0 {
		t.Errorf("running count: got %d, want 0", len(running))
	}
}

func TestAppGetTaskStats(t *testing.T) {
	app := setupTestApp(t)

	for i := 0; i < 3; i++ {
		_, err := app.CreateTask(CreateTaskParams{Name: "Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
		if err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}
	}

	stats, err := app.GetTaskStats()
	if err != nil {
		t.Fatalf("GetTaskStats: %v", err)
	}
	if stats["pending"] != 3 {
		t.Errorf("pending: got %d, want 3", stats["pending"])
	}
}

func TestAppListProxies(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.AddProxy("proxy1.example.com:8080", "http", "", "", "US")
	if err != nil {
		t.Fatalf("AddProxy 1: %v", err)
	}
	_, err = app.AddProxy("proxy2.example.com:8080", "socks5", "", "", "UK")
	if err != nil {
		t.Fatalf("AddProxy 2: %v", err)
	}

	proxies, err := app.ListProxies()
	if err != nil {
		t.Fatalf("ListProxies: %v", err)
	}
	if len(proxies) != 2 {
		t.Errorf("proxy count: got %d, want 2", len(proxies))
	}
}

func TestAppDeleteProxy(t *testing.T) {
	app := setupTestApp(t)

	p, err := app.AddProxy("proxy.example.com:8080", "http", "", "", "")
	if err != nil {
		t.Fatalf("AddProxy: %v", err)
	}

	if err := app.DeleteProxy(p.ID); err != nil {
		t.Fatalf("DeleteProxy: %v", err)
	}

	proxies, err := app.ListProxies()
	if err != nil {
		t.Fatalf("ListProxies: %v", err)
	}
	if len(proxies) != 0 {
		t.Errorf("proxy count after delete: got %d, want 0", len(proxies))
	}
}

func TestAppExportResultsJSON(t *testing.T) {
	app := setupTestApp(t)

	task, err := app.CreateTask(CreateTaskParams{Name: "Export Test", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := app.db.UpdateTaskStatus(app.ctx, task.ID, models.TaskStatusCompleted, ""); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}

	exportPath, err := app.ExportResultsJSON()
	if err != nil {
		t.Fatalf("ExportResultsJSON: %v", err)
	}
	if !strings.HasSuffix(exportPath, ".json") {
		t.Errorf("expected .json extension, got %q", exportPath)
	}

	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("read export file: %v", err)
	}

	var tasks []models.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		t.Fatalf("unmarshal export: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("exported task count: got %d, want 1", len(tasks))
	}
	if tasks[0].Name != "Export Test" {
		t.Errorf("exported task name: got %q, want %q", tasks[0].Name, "Export Test")
	}
}

func TestAppExportResultsCSV(t *testing.T) {
	app := setupTestApp(t)

	task, err := app.CreateTask(CreateTaskParams{Name: "CSV Test", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := app.db.UpdateTaskStatus(app.ctx, task.ID, models.TaskStatusCompleted, ""); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}

	exportPath, err := app.ExportResultsCSV()
	if err != nil {
		t.Fatalf("ExportResultsCSV: %v", err)
	}
	if !strings.HasSuffix(exportPath, ".csv") {
		t.Errorf("expected .csv extension, got %q", exportPath)
	}

	file, err := os.Open(exportPath)
	if err != nil {
		t.Fatalf("open export file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read CSV: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("CSV row count: got %d, want 2 (header + 1 row)", len(records))
	}
	if records[0][0] != "ID" {
		t.Errorf("CSV header[0]: got %q, want %q", records[0][0], "ID")
	}
	if records[1][1] != "CSV Test" {
		t.Errorf("CSV row name: got %q, want %q", records[1][1], "CSV Test")
	}
}

func TestAppExportResultsJSONEmpty(t *testing.T) {
	app := setupTestApp(t)

	exportPath, err := app.ExportResultsJSON()
	if err != nil {
		t.Fatalf("ExportResultsJSON: %v", err)
	}

	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("read export file: %v", err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed != "[]" && trimmed != "null" {
		t.Errorf("empty export should be '[]' or 'null', got %q", trimmed)
	}
}

func TestAppStartTaskWithQueue(t *testing.T) {
	app := setupTestAppWithQueue(t)

	task, err := app.CreateTask(CreateTaskParams{Name: "Queue Test", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	err = app.StartTask(task.ID)
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
}

func TestAppStartTaskNotFound(t *testing.T) {
	app := setupTestAppWithQueue(t)

	err := app.StartTask("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent task, got nil")
	}
}

func TestAppCancelTask(t *testing.T) {
	app := setupTestAppWithQueue(t)

	task, err := app.CreateTask(CreateTaskParams{Name: "Cancel Test", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	err = app.CancelTask(task.ID)
	if err != nil {
		t.Fatalf("CancelTask: %v", err)
	}

	got, err := app.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusCancelled {
		t.Errorf("Status: got %q, want %q", got.Status, models.TaskStatusCancelled)
	}
}

func TestAppUpdateTask(t *testing.T) {
	app := setupTestApp(t)

	task, err := app.CreateTask(CreateTaskParams{Name: "Original", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	newSteps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://updated.com"},
		{Action: models.ActionClick, Selector: "#new"},
	}
	err = app.UpdateTask(task.ID, database.TaskUpdateParams{Name: "Updated", URL: "https://updated.com", Steps: newSteps, ProxyConfig: models.ProxyConfig{}, Tags: []string{"updated"}, Timeout: 0, LoggingPolicy: nil}, 10)
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	got, err := app.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("Name: got %q, want %q", got.Name, "Updated")
	}
	if got.Priority != models.PriorityHigh {
		t.Errorf("Priority: got %d, want %d", got.Priority, models.PriorityHigh)
	}
}

func TestAppCreateBatch(t *testing.T) {
	app := setupTestApp(t)

	inputs := []models.BatchTaskInput{
		{Name: "Batch 1", URL: "https://a.com", Steps: validSteps(), Priority: 5},
		{Name: "Batch 2", URL: "https://b.com", Steps: validSteps(), Priority: 1},
		{Name: "Batch 3", URL: "https://c.com", Steps: validSteps(), Priority: 10},
	}

	created, err := app.CreateBatch(inputs, false)
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	if len(created) != 3 {
		t.Errorf("created count: got %d, want 3", len(created))
	}

	all, _ := app.ListTasks()
	if len(all) != 3 {
		t.Errorf("total tasks: got %d, want 3", len(all))
	}
}

func TestAppCreateBatchRejectsOnInvalid(t *testing.T) {
	app := setupTestApp(t)

	inputs := []models.BatchTaskInput{
		{Name: "Good", URL: "https://a.com", Steps: validSteps(), Priority: 5},
		{Name: "", URL: "https://b.com", Steps: validSteps(), Priority: 5},
	}

	_, err := app.CreateBatch(inputs, false)
	if err == nil {
		t.Fatal("expected error for invalid task in batch")
	}

	all, _ := app.ListTasks()
	if len(all) != 0 {
		t.Errorf("no tasks should be created on validation failure, got %d", len(all))
	}
}

func TestAppCreateBatchRejectsInvalidProxyConfig(t *testing.T) {
	app := setupTestApp(t)

	inputs := []models.BatchTaskInput{
		{Name: "Good", URL: "https://a.com", Steps: validSteps(), Priority: 5},
		{Name: "Bad Proxy", URL: "https://b.com", Steps: validSteps(), Priority: 5, Proxy: models.ProxyConfig{Protocol: models.ProxyHTTP}},
	}

	_, err := app.CreateBatch(inputs, false)
	if err == nil {
		t.Fatal("expected error for invalid proxy config in batch")
	}
	if !strings.Contains(err.Error(), "proxy") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppUpdateTaskValidation(t *testing.T) {
	app := setupTestApp(t)

	task, err := app.CreateTask(CreateTaskParams{Name: "Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	err = app.UpdateTask(task.ID, database.TaskUpdateParams{Name: "", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Tags: nil, Timeout: 0, LoggingPolicy: nil}, 5)
	if err == nil {
		t.Fatal("expected validation error for empty name")
	}
}

func TestAppCreateTaskRejectsInvalidLoggingPolicy(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.CreateTask(CreateTaskParams{Name: "Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: &models.TaskLoggingPolicy{MaxExecutionLogs: 5001}})
	if err == nil {
		t.Fatal("expected error for invalid logging policy, got nil")
	}
	if !strings.Contains(err.Error(), "maxExecutionLogs") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppUpdateTaskRejectsInvalidLoggingPolicy(t *testing.T) {
	app := setupTestApp(t)

	task, err := app.CreateTask(CreateTaskParams{Name: "Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	err = app.UpdateTask(task.ID, database.TaskUpdateParams{Name: "Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Tags: nil, Timeout: 0, LoggingPolicy: &models.TaskLoggingPolicy{MaxExecutionLogs: -1}}, 5)
	if err == nil {
		t.Fatal("expected error for invalid logging policy")
	}
	if !strings.Contains(err.Error(), "maxExecutionLogs") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppCreateTaskRejectsInvalidProxyConfig(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.CreateTask(CreateTaskParams{Name: "Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{Protocol: models.ProxyHTTP}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err == nil {
		t.Fatal("expected error for invalid proxy config")
	}
	if !strings.Contains(err.Error(), "proxy") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppUpdateTaskRejectsInvalidProxyConfig(t *testing.T) {
	app := setupTestApp(t)

	task, err := app.CreateTask(CreateTaskParams{Name: "Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	err = app.UpdateTask(task.ID, database.TaskUpdateParams{Name: "Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{Fallback: models.ProxyRoutingFallback("bogus")}, Tags: nil, Timeout: 0, LoggingPolicy: nil}, 5)
	if err == nil {
		t.Fatal("expected error for invalid proxy config")
	}
	if !strings.Contains(err.Error(), "proxy") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppUpdateTaskRejectsMalformedSupportedStep(t *testing.T) {
	app := setupTestApp(t)

	task, err := app.CreateTask(CreateTaskParams{Name: "Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	err = app.UpdateTask(task.ID, database.TaskUpdateParams{Name: "Task", URL: "https://example.com", Steps: []models.TaskStep{{Action: models.ActionGetAttributes}}, ProxyConfig: models.ProxyConfig{}, Tags: nil, Timeout: 0, LoggingPolicy: nil}, 5)
	if err == nil {
		t.Fatal("expected validation error for malformed supported step")
	}
	if !strings.Contains(err.Error(), "selector") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppStartTaskRequiresID(t *testing.T) {
	app := setupTestAppWithQueue(t)
	if err := app.StartTask(""); err == nil {
		t.Fatal("expected error for empty task id")
	} else if !strings.Contains(err.Error(), "id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppCancelTaskRequiresID(t *testing.T) {
	app := setupTestAppWithQueue(t)
	if err := app.CancelTask(""); err == nil {
		t.Fatal("expected error for empty task id")
	} else if !strings.Contains(err.Error(), "id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppStartAllPending(t *testing.T) {
	app := setupTestAppWithQueue(t)

	for i := 0; i < 3; i++ {
		_, err := app.CreateTask(CreateTaskParams{Name: fmt.Sprintf("Pending %d", i), URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
		if err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}
	}

	err := app.StartAllPending()
	if err != nil {
		t.Fatalf("StartAllPending: %v", err)
	}
}

func TestAppGetRunningCount(t *testing.T) {
	app := setupTestAppWithQueue(t)
	count := app.GetRunningCount()
	if count != 0 {
		t.Errorf("initial RunningCount: got %d, want 0", count)
	}
}

func TestAppDeleteTaskCancelsRunning(t *testing.T) {
	app := setupTestAppWithQueue(t)

	task, err := app.CreateTask(CreateTaskParams{Name: "Cancel Delete", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	err = app.DeleteTask(task.ID)
	if err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}

	_, err = app.GetTask(task.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestAppDeleteTaskNotFound(t *testing.T) {
	app := setupTestApp(t)
	err := app.DeleteTask("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestAppDeleteProxyNotFound(t *testing.T) {
	app := setupTestApp(t)
	err := app.DeleteProxy("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent proxy")
	}
}

func TestAppCreateTaskAutoStart(t *testing.T) {
	app := setupTestAppWithQueue(t)

	task, err := app.CreateTask(CreateTaskParams{Name: "Auto Start", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: true, Tags: []string{"test"}, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask with autoStart: %v", err)
	}
	if task.Name != "Auto Start" {
		t.Errorf("Name: got %q, want %q", task.Name, "Auto Start")
	}
}

func TestNewApp(t *testing.T) {
	app := NewApp()
	if app == nil {
		t.Fatal("NewApp returned nil")
	}
}

func TestAppListProxiesMasksCredentials(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.AddProxy("proxy.example.com:8080", "http", "admin", "secret123", "US")
	if err != nil {
		t.Fatalf("AddProxy: %v", err)
	}

	proxies, err := app.ListProxies()
	if err != nil {
		t.Fatalf("ListProxies: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("proxy count: got %d, want 1", len(proxies))
	}
	if proxies[0].Username == "admin" {
		t.Error("username should be masked")
	}
	if proxies[0].Password == "secret123" {
		t.Error("password should be masked")
	}
	if !strings.Contains(proxies[0].Username, "*") {
		t.Error("masked username should contain asterisks")
	}
	if !strings.Contains(proxies[0].Password, "*") {
		t.Error("masked password should contain asterisks")
	}
}

func TestMaskCredential(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"a", "*"},
		{"ab", "**"},
		{"abc", "a*c"},
		{"admin", "a***n"},
		{"password123", "p*********3"},
	}
	for _, tc := range tests {
		got := maskCredential(tc.input)
		if got != tc.want {
			t.Errorf("maskCredential(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestAppCreateScheduleRejectsInvalidProxyConfig(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.CreateSchedule(ScheduleParams{Name: "Sched", CronExpr: "0 * * * *", FlowID: "flow-1", URL: "https://example.com", ProxyConfig: models.ProxyConfig{Protocol: models.ProxyHTTP}, Priority: 5, Headless: false, Tags: nil})
	if err == nil {
		t.Fatal("expected error for invalid schedule proxy config")
	}
	if !strings.Contains(err.Error(), "proxy") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppUpdateScheduleRejectsInvalidProxyConfig(t *testing.T) {
	app := setupTestApp(t)

	sched, err := app.CreateSchedule(ScheduleParams{Name: "Sched", CronExpr: "0 * * * *", FlowID: "flow-1", URL: "https://example.com", ProxyConfig: models.ProxyConfig{}, Priority: 5, Headless: false, Tags: nil})
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	err = app.UpdateSchedule(sched.ID, ScheduleParams{Name: "Sched", CronExpr: "0 * * * *", FlowID: "flow-1", URL: "https://example.com", ProxyConfig: models.ProxyConfig{Fallback: models.ProxyRoutingFallback("bogus")}, Priority: 5, Headless: false, Tags: nil}, true)
	if err == nil {
		t.Fatal("expected error for invalid schedule proxy config")
	}
	if !strings.Contains(err.Error(), "proxy") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppListTasksPaginated(t *testing.T) {
	app := setupTestApp(t)

	for i := 0; i < 10; i++ {
		_, err := app.CreateTask(CreateTaskParams{Name: fmt.Sprintf("Task %d", i), URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
		if err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}
	}

	result, err := app.ListTasksPaginated(1, 3, "", "")
	if err != nil {
		t.Fatalf("ListTasksPaginated: %v", err)
	}
	if result.Total != 10 {
		t.Errorf("Total: got %d, want 10", result.Total)
	}
	if len(result.Tasks) != 3 {
		t.Errorf("Tasks length: got %d, want 3", len(result.Tasks))
	}
	if result.TotalPages != 4 {
		t.Errorf("TotalPages: got %d, want 4", result.TotalPages)
	}

	result2, err := app.ListTasksPaginated(4, 3, "", "")
	if err != nil {
		t.Fatalf("ListTasksPaginated page 4: %v", err)
	}
	if len(result2.Tasks) != 1 {
		t.Errorf("last page Tasks: got %d, want 1", len(result2.Tasks))
	}
}

func TestAppListTasksPaginatedValidation(t *testing.T) {
	app := setupTestApp(t)

	tests := []struct {
		name     string
		page     int
		pageSize int
		status   string
		tag      string
	}{
		{"page zero", 0, 20, "", ""},
		{"page negative", -1, 20, "", ""},
		{"pageSize zero", 1, 0, "", ""},
		{"pageSize too large", 1, 201, "", ""},
		{"invalid status", 1, 20, "bogus", ""},
		{"tag control char", 1, 20, "", "bad\ttag"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := app.ListTasksPaginated(tc.page, tc.pageSize, tc.status, tc.tag)
			if err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

func TestAppGetAuditTrail(t *testing.T) {
	app := setupTestApp(t)

	events, err := app.GetAuditTrail("", 100)
	if err != nil {
		t.Fatalf("GetAuditTrail: %v", err)
	}
	if events == nil {
		t.Error("expected non-nil events slice")
	}
}

func TestAppPurgeOldData(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.CreateTask(CreateTaskParams{Name: "Old Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	n, err := app.PurgeOldData(90)
	if err != nil {
		t.Fatalf("PurgeOldData: %v", err)
	}
	if n != 0 {
		t.Errorf("purged: got %d, want 0 (task is new)", n)
	}
}

func TestAppIsRecording(t *testing.T) {
	app := setupTestApp(t)
	if app.IsRecording() {
		t.Error("should not be recording initially")
	}
}

func TestAppStopRecordingNoSession(t *testing.T) {
	app := setupTestApp(t)
	_, err := app.StopRecording()
	if err == nil {
		t.Error("expected error when no active recording session")
	}
}

func TestAppCreateRecordedFlow(t *testing.T) {
	app := setupTestApp(t)

	steps := []models.RecordedStep{
		{Index: 0, Action: models.ActionNavigate, Value: "https://example.com"},
		{Index: 1, Action: models.ActionClick, Selector: "#btn"},
	}

	flow, err := app.CreateRecordedFlow("Test Flow", "A test", "https://example.com", steps)
	if err != nil {
		t.Fatalf("CreateRecordedFlow: %v", err)
	}
	if flow.Name != "Test Flow" {
		t.Errorf("Name: got %q, want %q", flow.Name, "Test Flow")
	}
	if len(flow.Steps) != 2 {
		t.Errorf("Steps: got %d, want 2", len(flow.Steps))
	}

	got, err := app.GetRecordedFlow(flow.ID)
	if err != nil {
		t.Fatalf("GetRecordedFlow: %v", err)
	}
	if got.Name != "Test Flow" {
		t.Errorf("got Name: %q, want %q", got.Name, "Test Flow")
	}
}

func TestAppListRecordedFlows(t *testing.T) {
	app := setupTestApp(t)

	steps := []models.RecordedStep{
		{Index: 0, Action: models.ActionNavigate, Value: "https://example.com"},
	}
	_, _ = app.CreateRecordedFlow("Flow 1", "", "https://example.com", steps)
	_, _ = app.CreateRecordedFlow("Flow 2", "", "https://example.com", steps)

	flows, err := app.ListRecordedFlows()
	if err != nil {
		t.Fatalf("ListRecordedFlows: %v", err)
	}
	if len(flows) != 2 {
		t.Errorf("flow count: got %d, want 2", len(flows))
	}
}

func TestAppDeleteRecordedFlow(t *testing.T) {
	app := setupTestApp(t)

	steps := []models.RecordedStep{
		{Index: 0, Action: models.ActionNavigate, Value: "https://example.com"},
	}
	flow, _ := app.CreateRecordedFlow("Delete Me", "", "https://example.com", steps)

	if err := app.DeleteRecordedFlow(flow.ID); err != nil {
		t.Fatalf("DeleteRecordedFlow: %v", err)
	}

	_, err := app.GetRecordedFlow(flow.ID)
	if err == nil {
		t.Error("expected error after deleting flow")
	}
}

// Helper: setup app with proxy manager
func setupTestAppWithProxyManager(t *testing.T) *App {
	t.Helper()
	app := setupTestApp(t)
	app.proxyManager = proxy.NewManager(app.db, models.ProxyPoolConfig{})
	return app
}

// app_batch.go tests
func TestAppGetBatchProgress(t *testing.T) {
	app := setupTestAppWithQueue(t)

	// Should fail if not ready
	badApp := &App{initErr: errors.New("test")}
	_, err := badApp.GetBatchProgress("any")
	if err == nil {
		t.Error("expected error when app not ready")
	}

	// Valid case with empty batch
	progress, err := app.GetBatchProgress("nonexistent")
	if err != nil {
		t.Fatalf("GetBatchProgress: %v", err)
	}
	if progress.Total != 0 {
		t.Errorf("expected 0 tasks, got %d", progress.Total)
	}
}

func TestAppListBatchGroups(t *testing.T) {
	app := setupTestAppWithQueue(t)

	groups, err := app.ListBatchGroups()
	if err != nil {
		t.Fatalf("ListBatchGroups: %v", err)
	}
	if groups == nil {
		groups = []models.BatchGroup{}
	}
	if len(groups) != 0 {
		t.Errorf("expected empty list, got %d", len(groups))
	}
}

func TestAppListTasksByBatch(t *testing.T) {
	app := setupTestAppWithQueue(t)

	tasks, err := app.ListTasksByBatch("nonexistent")
	if err != nil {
		t.Fatalf("ListTasksByBatch: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestAppRetryFailedBatchEmpty(t *testing.T) {
	app := setupTestAppWithQueue(t)

	// Empty batch ID should error
	_, err := app.RetryFailedBatch("")
	if err == nil {
		t.Error("expected error for empty batchID")
	}
}

func TestAppRetryFailedBatchNotFound(t *testing.T) {
	app := setupTestAppWithQueue(t)

	failed, err := app.RetryFailedBatch("nonexistent")
	if err != nil {
		t.Fatalf("RetryFailedBatch: %v", err)
	}
	if len(failed) != 0 {
		t.Errorf("expected empty list, got %d", len(failed))
	}
}

func TestAppPauseBatchEmptyID(t *testing.T) {
	app := setupTestAppWithQueue(t)

	err := app.PauseBatch("")
	if err == nil {
		t.Error("expected error for empty batchID")
	}
}

func TestAppPauseBatchValid(t *testing.T) {
	app := setupTestAppWithQueue(t)

	err := app.PauseBatch("test-batch-123")
	if err != nil {
		t.Fatalf("PauseBatch: %v", err)
	}
}

func TestAppResumeBatchEmptyID(t *testing.T) {
	app := setupTestAppWithQueue(t)

	err := app.ResumeBatch("")
	if err == nil {
		t.Error("expected error for empty batchID")
	}
}

func TestAppResumeBatchValid(t *testing.T) {
	app := setupTestAppWithQueue(t)

	err := app.ResumeBatch("test-batch-456")
	if err != nil {
		t.Fatalf("ResumeBatch: %v", err)
	}
}

// app_captcha.go tests
func TestAppSaveCaptchaConfigValid(t *testing.T) {
	app := setupTestApp(t)
	// Create a mock runner so refreshCaptchaSolver doesn't panic
	app.runner = &browser.Runner{}

	cfg, err := app.SaveCaptchaConfig("2captcha", "test-api-key-123")
	if err != nil {
		t.Fatalf("SaveCaptchaConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.Provider != models.CaptchaProvider2Captcha {
		t.Errorf("expected 2captcha provider, got %s", cfg.Provider)
	}
}

func TestAppSaveCaptchaConfigEmptyProvider(t *testing.T) {
	app := setupTestApp(t)
	app.runner = &browser.Runner{}

	_, err := app.SaveCaptchaConfig("", "key")
	if err == nil {
		t.Error("expected error for empty provider")
	}
}

func TestAppSaveCaptchaConfigEmptyKey(t *testing.T) {
	app := setupTestApp(t)
	app.runner = &browser.Runner{}

	_, err := app.SaveCaptchaConfig("2captcha", "")
	if err == nil {
		t.Error("expected error for empty apiKey")
	}
}

func TestAppSaveCaptchaConfigInvalidProvider(t *testing.T) {
	app := setupTestApp(t)
	app.runner = &browser.Runner{}

	_, err := app.SaveCaptchaConfig("invalid-provider", "key")
	if err == nil {
		t.Error("expected error for invalid provider")
	}
}

func TestAppGetCaptchaConfigNotFound(t *testing.T) {
	app := setupTestApp(t)

	cfg, err := app.GetCaptchaConfig()
	if err != nil {
		t.Fatalf("GetCaptchaConfig: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil when no config exists")
	}
}

func TestAppListCaptchaConfigs(t *testing.T) {
	app := setupTestApp(t)

	configs, err := app.ListCaptchaConfigs()
	if err != nil {
		t.Fatalf("ListCaptchaConfigs: %v", err)
	}
	if len(configs) != 0 {
		t.Errorf("expected 0 configs, got %d", len(configs))
	}
}

func TestAppDeleteCaptchaConfigEmptyID(t *testing.T) {
	app := setupTestApp(t)
	app.runner = &browser.Runner{}

	err := app.DeleteCaptchaConfig("")
	if err == nil {
		t.Error("expected error for empty id")
	}
}

func TestAppTestCaptchaConfigEmptyID(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.TestCaptchaConfig("")
	if err == nil {
		t.Error("expected error for empty id")
	}
}

// app_compliance.go tests
func TestAppParseBatchURLsListFormat(t *testing.T) {
	app := setupTestApp(t)

	input := "https://example.com\nhttps://example.org"
	urls, err := app.ParseBatchURLs(input, false)
	if err != nil {
		t.Fatalf("ParseBatchURLs: %v", err)
	}
	if len(urls) != 2 {
		t.Errorf("expected 2 URLs, got %d", len(urls))
	}
}

func TestAppParseBatchURLsCSVFormat(t *testing.T) {
	app := setupTestApp(t)

	input := "URL\nhttps://example.com\nhttps://example.org\n"
	urls, err := app.ParseBatchURLs(input, true)
	if err != nil {
		t.Fatalf("ParseBatchURLs CSV: %v", err)
	}
	if len(urls) < 2 {
		t.Errorf("expected at least 2 URLs, got %d", len(urls))
	}
}

func TestAppParseBatchURLsEmptyInput(t *testing.T) {
	app := setupTestApp(t)

	input := ""
	urls, err := app.ParseBatchURLs(input, false)
	if err != nil {
		t.Fatalf("ParseBatchURLs empty: %v", err)
	}
	if len(urls) != 0 {
		t.Errorf("expected 0 URLs for empty input, got %d", len(urls))
	}
}

// app_export.go tests
func TestAppExportTaskLogsNoExporter(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.ExportTaskLogs("task-123")
	if err == nil {
		t.Error("expected error when logExporter is nil")
	}
}

func TestAppExportBatchLogsNoExporter(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.ExportBatchLogs("batch-123")
	if err == nil {
		t.Error("expected error when logExporter is nil")
	}
}

func TestAppListWebSocketLogsEmptyFlowID(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.ListWebSocketLogs("")
	if err == nil {
		t.Error("expected error for empty flowID")
	}
}

func TestAppListWebSocketLogsValid(t *testing.T) {
	app := setupTestApp(t)

	logs, err := app.ListWebSocketLogs("flow-123")
	if err != nil {
		t.Fatalf("ListWebSocketLogs: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected 0 logs, got %d", len(logs))
	}
}

func TestAppListTaskEventsEmptyTaskID(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.ListTaskEvents("")
	if err == nil {
		t.Error("expected error for empty taskID")
	}
}

func TestAppListTaskEventsValid(t *testing.T) {
	app := setupTestApp(t)

	events, err := app.ListTaskEvents("task-123")
	if err != nil {
		t.Fatalf("ListTaskEvents: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

// app_flow_io.go tests
func TestAppValidateStepActions(t *testing.T) {
	steps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionClick, Value: "#button"},
	}

	err := validateStepActions(steps)
	if err != nil {
		t.Fatalf("validateStepActions: %v", err)
	}
}

func TestAppValidateStepActionsInvalid(t *testing.T) {
	steps := []models.TaskStep{
		{Action: "invalid_action", Value: "test"},
	}

	err := validateStepActions(steps)
	if err == nil {
		t.Error("expected error for invalid action")
	}
}

func TestAppCollectUnknownStepActionWarnings(t *testing.T) {
	steps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "test"},
		{Action: "unknown_action", Value: "test"},
	}

	warnings := collectUnknownStepActionWarnings(steps, 1)
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}
}

func TestAppExportTaskNotFound(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.ExportTask("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestAppImportTaskInvalidPath(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.ImportTask("/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestAppExportFlowNotFound(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.ExportFlow("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent flow")
	}
}

func TestAppImportFlowInvalidPath(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.ImportFlow("/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

// app_flows.go tests
func TestAppSaveDOMSnapshot(t *testing.T) {
	app := setupTestApp(t)

	snapshot := models.DOMSnapshot{
		FlowID:    "flow-123",
		StepIndex: 0,
		HTML:      "<html></html>",
	}

	err := app.SaveDOMSnapshot(snapshot)
	if err != nil {
		t.Fatalf("SaveDOMSnapshot: %v", err)
	}
}

func TestAppListDOMSnapshotsValid(t *testing.T) {
	app := setupTestApp(t)

	snapshots, err := app.ListDOMSnapshots("flow-123")
	if err != nil {
		t.Fatalf("ListDOMSnapshots: %v", err)
	}
	if len(snapshots) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snapshots))
	}
}

func TestAppUpdateRecordedFlowEmptyID(t *testing.T) {
	app := setupTestApp(t)

	flow := models.RecordedFlow{ID: ""}
	err := app.UpdateRecordedFlow(flow)
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestAppUpdateRecordedFlowValid(t *testing.T) {
	app := setupTestApp(t)

	steps := []models.RecordedStep{
		{Index: 0, Action: models.ActionNavigate, Value: "https://example.com"},
	}
	flow, _ := app.CreateRecordedFlow("Original", "", "https://example.com", steps)

	flow.Name = "Updated"
	err := app.UpdateRecordedFlow(*flow)
	if err != nil {
		t.Fatalf("UpdateRecordedFlow: %v", err)
	}

	updated, _ := app.GetRecordedFlow(flow.ID)
	if updated.Name != "Updated" {
		t.Errorf("expected name Updated, got %s", updated.Name)
	}
}

// app_proxy.go tests
func TestAppListProxyCountryStatsNoManager(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.ListProxyCountryStats()
	if err == nil {
		t.Error("expected error when proxyManager is nil")
	}
}

func TestAppListProxyCountryStatsValid(t *testing.T) {
	app := setupTestAppWithProxyManager(t)

	stats, err := app.ListProxyCountryStats()
	if err != nil {
		t.Fatalf("ListProxyCountryStats: %v", err)
	}
	if stats == nil {
		stats = []models.ProxyCountryStats{}
	}
	if len(stats) != 0 {
		t.Errorf("expected empty stats, got %d", len(stats))
	}
}

func TestAppCreateProxyRoutingPresetNoName(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.CreateProxyRoutingPreset("", "US", "strict", false)
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestAppCreateProxyRoutingPresetValid(t *testing.T) {
	app := setupTestApp(t)

	preset, err := app.CreateProxyRoutingPreset("My Preset", "US", "strict", false)
	if err != nil {
		t.Fatalf("CreateProxyRoutingPreset: %v", err)
	}
	if preset == nil {
		t.Fatal("expected preset, got nil")
	}
	if preset.Name != "My Preset" {
		t.Errorf("expected name 'My Preset', got %s", preset.Name)
	}
}

func TestAppCreateProxyRoutingPresetRandomNoCountry(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.CreateProxyRoutingPreset("Random", "", "", true)
	if err == nil {
		t.Error("expected error for random without country")
	}
}

func TestAppListProxyRoutingPresetsValid(t *testing.T) {
	app := setupTestApp(t)

	presets, err := app.ListProxyRoutingPresets()
	if err != nil {
		t.Fatalf("ListProxyRoutingPresets: %v", err)
	}
	if len(presets) != 0 {
		t.Errorf("expected 0 presets, got %d", len(presets))
	}
}

func TestAppDeleteProxyRoutingPresetEmptyID(t *testing.T) {
	app := setupTestApp(t)

	err := app.DeleteProxyRoutingPreset("")
	if err == nil {
		t.Error("expected error for empty id")
	}
}

func TestAppDeleteProxyRoutingPresetValid(t *testing.T) {
	app := setupTestApp(t)

	preset, _ := app.CreateProxyRoutingPreset("To Delete", "US", "strict", false)
	err := app.DeleteProxyRoutingPreset(preset.ID)
	if err != nil {
		t.Fatalf("DeleteProxyRoutingPreset: %v", err)
	}
}

func TestAppGetLocalProxyGatewayStatsNoManager(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.GetLocalProxyGatewayStats()
	if err == nil {
		t.Error("expected error when localProxyManager is nil")
	}
}

func TestAppAddProxyWithRateLimitNegative(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.AddProxyWithRateLimit("proxy.example.com:8080", "http", "user", "pass", "", -1)
	if err == nil {
		t.Error("expected error for negative rate limit")
	}
}

func TestAppAddProxyWithRateLimitValid(t *testing.T) {
	app := setupTestApp(t)

	p, err := app.AddProxyWithRateLimit("proxy.example.com:8080", "http", "user", "pass", "US", 100)
	if err != nil {
		t.Fatalf("AddProxyWithRateLimit: %v", err)
	}
	if p.MaxRequestsPerMinute != 100 {
		t.Errorf("expected rate limit 100, got %d", p.MaxRequestsPerMinute)
	}
}

// app_schedules.go tests
func TestAppGetScheduleEmptyID(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.GetSchedule("")
	if err == nil {
		t.Error("expected error for empty id")
	}
}

func TestAppGetScheduleNotFound(t *testing.T) {
	app := setupTestApp(t)

	sched, err := app.GetSchedule("nonexistent")
	// GetSchedule may return an error or nil schedule depending on DB behavior
	if err == nil && sched != nil {
		t.Error("expected nil or error for nonexistent schedule")
	}
}

func TestAppListSchedulesValid(t *testing.T) {
	app := setupTestApp(t)

	schedules, err := app.ListSchedules()
	if err != nil {
		t.Fatalf("ListSchedules: %v", err)
	}
	if len(schedules) != 0 {
		t.Errorf("expected 0 schedules, got %d", len(schedules))
	}
}

func TestAppDeleteScheduleEmptyID(t *testing.T) {
	app := setupTestApp(t)

	err := app.DeleteSchedule("")
	if err == nil {
		t.Error("expected error for empty id")
	}
}

func TestAppToggleScheduleEmptyID(t *testing.T) {
	app := setupTestApp(t)

	err := app.ToggleSchedule("", true)
	if err == nil {
		t.Error("expected error for empty id")
	}
}

// app.go tests
func TestAppDefaultConfig(t *testing.T) {
	cfg := DefaultAppConfig()

	if cfg.QueueConcurrency == 0 {
		t.Error("expected QueueConcurrency > 0")
	}
	if cfg.RetentionDays == 0 {
		t.Error("expected RetentionDays > 0")
	}
}

func TestAppLoadConfigFromDiskNoPath(t *testing.T) {
	app := setupTestApp(t)
	app.configPath = ""

	err := app.loadConfigFromDisk()
	if err != nil {
		t.Fatalf("loadConfigFromDisk with no path: %v", err)
	}
}

func TestAppLoadConfigFromDiskFileNotFound(t *testing.T) {
	app := setupTestApp(t)
	app.configPath = filepath.Join(t.TempDir(), "nonexistent.json")

	err := app.loadConfigFromDisk()
	if err != nil {
		t.Fatalf("loadConfigFromDisk with missing file: %v", err)
	}
}

func TestAppLoadConfigFromDiskValidJSON(t *testing.T) {
	app := setupTestApp(t)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{"QueueConcurrency": 5, "RetentionDays": 30}`
	if err := os.WriteFile(configPath, []byte(configData), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	app.configPath = configPath
	err := app.loadConfigFromDisk()
	if err != nil {
		t.Fatalf("loadConfigFromDisk: %v", err)
	}
	if app.config.QueueConcurrency != 5 {
		t.Errorf("expected QueueConcurrency 5, got %d", app.config.QueueConcurrency)
	}
}

func TestAppLoadConfigFromDiskInvalidJSON(t *testing.T) {
	app := setupTestApp(t)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	if err := os.WriteFile(configPath, []byte("invalid json {"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	app.configPath = configPath
	err := app.loadConfigFromDisk()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestAppCheckAndReloadConfigNoPath(t *testing.T) {
	app := setupTestApp(t)
	app.configPath = ""

	app.checkAndReloadConfig(context.Background())
	// Should not panic or error
}

func TestAppCheckAndReloadConfigFileNotFound(t *testing.T) {
	app := setupTestApp(t)
	app.configPath = "/nonexistent/path/config.json"

	app.checkAndReloadConfig(context.Background())
	// Should not panic or error
}

func TestAppCheckAndReloadConfigValidJSON(t *testing.T) {
	app := setupTestApp(t)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{"ProxyConcurrency": 10}`
	if err := os.WriteFile(configPath, []byte(configData), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	app.configPath = configPath
	app.configModTime = time.Time{} // Zero time, so reload will happen

	app.checkAndReloadConfig(context.Background())

	app.configMu.Lock()
	concurrency := app.config.ProxyConcurrency
	app.configMu.Unlock()

	if concurrency != 10 {
		t.Errorf("expected ProxyConcurrency 10, got %d", concurrency)
	}
}

func TestAppPurgeOnceNoDatabase(t *testing.T) {
	app := &App{db: nil, ctx: context.Background()}

	app.purgeOnce()
	// Should not panic
}

func TestAppPurgeOnceWithDatabase(t *testing.T) {
	app := setupTestApp(t)

	app.purgeOnce()
	// Should not panic and should work
}

func TestAppGetTaskMetricsNoQueue(t *testing.T) {
	app := setupTestApp(t)

	metrics := app.getTaskMetrics()
	if metrics.Completed != 0 || metrics.Failed != 0 {
		t.Errorf("expected zero metrics, got completed=%d failed=%d", metrics.Completed, metrics.Failed)
	}
}

func TestAppGetTaskMetricsWithQueue(t *testing.T) {
	app := setupTestAppWithQueue(t)

	metrics := app.getTaskMetrics()
	if metrics.Completed < 0 || metrics.Failed < 0 {
		t.Errorf("metrics should be non-negative: completed=%d failed=%d", metrics.Completed, metrics.Failed)
	}
}

func TestAppCreateSchedule(t *testing.T) {
	app := setupTestApp(t)
	steps := []models.RecordedStep{{Index: 0, Action: models.ActionNavigate, Value: "https://example.com"}}
	flow, err := app.CreateRecordedFlow("Flow", "", "https://example.com", steps)
	if err != nil {
		t.Fatalf("CreateRecordedFlow: %v", err)
	}
	sched, err := app.CreateSchedule(ScheduleParams{Name: "My Schedule", CronExpr: "0 * * * *", FlowID: flow.ID, URL: "https://example.com", ProxyConfig: models.ProxyConfig{}, Priority: 5, Headless: false, Tags: nil})
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}
	if sched.Name != "My Schedule" {
		t.Errorf("Name: got %q, want %q", sched.Name, "My Schedule")
	}
}

func TestAppGetSchedule(t *testing.T) {
	app := setupTestApp(t)
	steps := []models.RecordedStep{{Index: 0, Action: models.ActionNavigate, Value: "https://example.com"}}
	flow, _ := app.CreateRecordedFlow("Flow", "", "https://example.com", steps)
	sched, err := app.CreateSchedule(ScheduleParams{Name: "Sched", CronExpr: "0 * * * *", FlowID: flow.ID, URL: "https://example.com", ProxyConfig: models.ProxyConfig{}, Priority: 5, Headless: false, Tags: nil})
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}
	got, err := app.GetSchedule(sched.ID)
	if err != nil {
		t.Fatalf("GetSchedule: %v", err)
	}
	if got.ID != sched.ID {
		t.Errorf("ID mismatch: got %q, want %q", got.ID, sched.ID)
	}
}

func TestAppListSchedules(t *testing.T) {
	app := setupTestApp(t)
	steps := []models.RecordedStep{{Index: 0, Action: models.ActionNavigate, Value: "https://example.com"}}
	flow, _ := app.CreateRecordedFlow("Flow", "", "https://example.com", steps)
	_, _ = app.CreateSchedule(ScheduleParams{Name: "S1", CronExpr: "0 * * * *", FlowID: flow.ID, URL: "https://example.com", ProxyConfig: models.ProxyConfig{}, Priority: 5, Headless: false, Tags: nil})
	_, _ = app.CreateSchedule(ScheduleParams{Name: "S2", CronExpr: "0 * * * *", FlowID: flow.ID, URL: "https://example.com", ProxyConfig: models.ProxyConfig{}, Priority: 5, Headless: false, Tags: nil})
	scheds, err := app.ListSchedules()
	if err != nil {
		t.Fatalf("ListSchedules: %v", err)
	}
	if len(scheds) != 2 {
		t.Errorf("count: got %d, want 2", len(scheds))
	}
}

func TestAppDeleteSchedule(t *testing.T) {
	app := setupTestApp(t)
	steps := []models.RecordedStep{{Index: 0, Action: models.ActionNavigate, Value: "https://example.com"}}
	flow, _ := app.CreateRecordedFlow("Flow", "", "https://example.com", steps)
	sched, err := app.CreateSchedule(ScheduleParams{Name: "S", CronExpr: "0 * * * *", FlowID: flow.ID, URL: "https://example.com", ProxyConfig: models.ProxyConfig{}, Priority: 5, Headless: false, Tags: nil})
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}
	if err := app.DeleteSchedule(sched.ID); err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}
}

func TestAppToggleSchedule(t *testing.T) {
	app := setupTestApp(t)
	steps := []models.RecordedStep{{Index: 0, Action: models.ActionNavigate, Value: "https://example.com"}}
	flow, _ := app.CreateRecordedFlow("Flow", "", "https://example.com", steps)
	sched, err := app.CreateSchedule(ScheduleParams{Name: "S", CronExpr: "0 * * * *", FlowID: flow.ID, URL: "https://example.com", ProxyConfig: models.ProxyConfig{}, Priority: 5, Headless: false, Tags: nil})
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}
	if err := app.ToggleSchedule(sched.ID, false); err != nil {
		t.Fatalf("ToggleSchedule(false): %v", err)
	}
	if err := app.ToggleSchedule(sched.ID, true); err != nil {
		t.Fatalf("ToggleSchedule(true): %v", err)
	}
}

func TestAppUpdateSchedule(t *testing.T) {
	app := setupTestApp(t)
	steps := []models.RecordedStep{{Index: 0, Action: models.ActionNavigate, Value: "https://example.com"}}
	flow, _ := app.CreateRecordedFlow("Flow", "", "https://example.com", steps)
	sched, err := app.CreateSchedule(ScheduleParams{Name: "S", CronExpr: "0 * * * *", FlowID: flow.ID, URL: "https://example.com", ProxyConfig: models.ProxyConfig{}, Priority: 5, Headless: false, Tags: nil})
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}
	err = app.UpdateSchedule(sched.ID, ScheduleParams{Name: "Updated", CronExpr: "0 0 * * *", FlowID: flow.ID, URL: "https://example.com", ProxyConfig: models.ProxyConfig{}, Priority: 5, Headless: false, Tags: nil}, true)
	if err != nil {
		t.Fatalf("UpdateSchedule: %v", err)
	}
}

func TestAppSaveCaptchaConfig(t *testing.T) {
	app := setupTestApp(t)
	runner, err := browser.NewRunner(t.TempDir())
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	app.runner = runner
	cfg, err := app.SaveCaptchaConfig("2captcha", "testapikey")
	if err != nil {
		t.Fatalf("SaveCaptchaConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil captcha config")
	}
}

func TestAppGetCaptchaConfig(t *testing.T) {
	app := setupTestApp(t)
	runner, _ := browser.NewRunner(t.TempDir())
	app.runner = runner
	_, _ = app.SaveCaptchaConfig("2captcha", "testapikey")
	cfg, err := app.GetCaptchaConfig()
	if err != nil {
		t.Fatalf("GetCaptchaConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil captcha config after save")
	}
}

func TestAppListCaptchaConfigsWithData(t *testing.T) {
	app := setupTestApp(t)
	runner, _ := browser.NewRunner(t.TempDir())
	app.runner = runner
	_, _ = app.SaveCaptchaConfig("2captcha", "testapikey")
	configs, err := app.ListCaptchaConfigs()
	if err != nil {
		t.Fatalf("ListCaptchaConfigs: %v", err)
	}
	if len(configs) != 1 {
		t.Errorf("count: got %d, want 1", len(configs))
	}
}

func TestAppDeleteCaptchaConfig(t *testing.T) {
	app := setupTestApp(t)
	runner, _ := browser.NewRunner(t.TempDir())
	app.runner = runner
	cfg, err := app.SaveCaptchaConfig("2captcha", "testapikey")
	if err != nil {
		t.Fatalf("SaveCaptchaConfig: %v", err)
	}
	if err := app.DeleteCaptchaConfig(cfg.ID); err != nil {
		t.Fatalf("DeleteCaptchaConfig: %v", err)
	}
}

func TestAppExportImportTask(t *testing.T) {
	app := setupTestApp(t)
	task, err := app.CreateTask(CreateTaskParams{Name: "Export Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	exportPath, err := app.ExportTask(task.ID)
	if err != nil {
		t.Fatalf("ExportTask: %v", err)
	}
	imported, err := app.ImportTask(exportPath)
	if err != nil {
		t.Fatalf("ImportTask: %v", err)
	}
	if imported.Name != task.Name {
		t.Errorf("Name: got %q, want %q", imported.Name, task.Name)
	}
	if imported.ID == task.ID {
		t.Error("expected new ID for imported task")
	}
}

func TestAppExportImportFlow(t *testing.T) {
	app := setupTestApp(t)
	steps := []models.RecordedStep{{Index: 0, Action: models.ActionNavigate, Value: "https://example.com"}}
	flow, err := app.CreateRecordedFlow("My Flow", "desc", "https://example.com", steps)
	if err != nil {
		t.Fatalf("CreateRecordedFlow: %v", err)
	}
	exportPath, err := app.ExportFlow(flow.ID)
	if err != nil {
		t.Fatalf("ExportFlow: %v", err)
	}
	tasks, err := app.ImportFlow(exportPath)
	if err != nil {
		t.Fatalf("ImportFlow: %v", err)
	}
	if len(tasks) == 0 {
		t.Error("expected at least one task from import")
	}
}

func TestAppListTaskEvents(t *testing.T) {
	app := setupTestApp(t)
	task, _ := app.CreateTask(CreateTaskParams{Name: "Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	events, err := app.ListTaskEvents(task.ID)
	if err != nil {
		t.Fatalf("ListTaskEvents: %v", err)
	}
	if events == nil {
		t.Error("expected non-nil events")
	}
}

func TestAppListWebSocketLogs(t *testing.T) {
	app := setupTestApp(t)
	logs, err := app.ListWebSocketLogs("flow-id-123")
	if err != nil {
		t.Fatalf("ListWebSocketLogs: %v", err)
	}
	if logs == nil {
		t.Error("expected non-nil logs slice")
	}
}

func TestAppParseBatchURLs(t *testing.T) {
	app := setupTestApp(t)
	urls, err := app.ParseBatchURLs("https://a.com\nhttps://b.com\nhttps://c.com", false)
	if err != nil {
		t.Fatalf("ParseBatchURLs: %v", err)
	}
	if len(urls) != 3 {
		t.Errorf("url count: got %d, want 3", len(urls))
	}
}

func TestAppListBatchGroupsEmpty(t *testing.T) {
	app := setupTestApp(t)
	groups, err := app.ListBatchGroups()
	if err != nil {
		t.Fatalf("ListBatchGroups: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}

func TestAppGetBatchProgressEmpty(t *testing.T) {
	app := setupTestApp(t)
	_, err := app.GetBatchProgress("nonexistent-batch-id")
	if err != nil {
		t.Fatalf("GetBatchProgress: %v", err)
	}
}

func TestAppPauseBatch(t *testing.T) {
	app := setupTestAppWithQueue(t)
	if err := app.PauseBatch("batch-123"); err != nil {
		t.Fatalf("PauseBatch: %v", err)
	}
}

func TestAppResumeBatch(t *testing.T) {
	app := setupTestAppWithQueue(t)
	_ = app.PauseBatch("batch-123")
	if err := app.ResumeBatch("batch-123"); err != nil {
		t.Fatalf("ResumeBatch: %v", err)
	}
}

func TestAppSaveDOMSnapshotAndList(t *testing.T) {
	app := setupTestApp(t)
	steps := []models.RecordedStep{{Index: 0, Action: models.ActionNavigate, Value: "https://example.com"}}
	flow, _ := app.CreateRecordedFlow("Flow", "", "https://example.com", steps)
	snapshot := models.DOMSnapshot{FlowID: flow.ID, StepIndex: 0, HTML: "<html></html>"}
	if err := app.SaveDOMSnapshot(snapshot); err != nil {
		t.Fatalf("SaveDOMSnapshot: %v", err)
	}
	snaps, err := app.ListDOMSnapshots(flow.ID)
	if err != nil {
		t.Fatalf("ListDOMSnapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Errorf("snapshot count: got %d, want 1", len(snaps))
	}
}

func TestAppUpdateRecordedFlow(t *testing.T) {
	app := setupTestApp(t)
	steps := []models.RecordedStep{{Index: 0, Action: models.ActionNavigate, Value: "https://example.com"}}
	flow, _ := app.CreateRecordedFlow("Flow", "", "https://example.com", steps)
	flow.Name = "Updated Flow"
	if err := app.UpdateRecordedFlow(*flow); err != nil {
		t.Fatalf("UpdateRecordedFlow: %v", err)
	}
}

func TestAppCreateProxyRoutingPreset(t *testing.T) {
	app := setupTestApp(t)
	preset, err := app.CreateProxyRoutingPreset("My Preset", "US", "any", false)
	if err != nil {
		t.Fatalf("CreateProxyRoutingPreset: %v", err)
	}
	if preset.Name != "My Preset" {
		t.Errorf("Name: got %q, want %q", preset.Name, "My Preset")
	}
	presets, err := app.ListProxyRoutingPresets()
	if err != nil {
		t.Fatalf("ListProxyRoutingPresets: %v", err)
	}
	if len(presets) != 1 {
		t.Errorf("preset count: got %d, want 1", len(presets))
	}
	if err := app.DeleteProxyRoutingPreset(preset.ID); err != nil {
		t.Fatalf("DeleteProxyRoutingPreset: %v", err)
	}
}

func TestAppDefaultAppConfig(t *testing.T) {
	cfg := DefaultAppConfig()
	if cfg.QueueConcurrency <= 0 {
		t.Errorf("QueueConcurrency: got %d, want > 0", cfg.QueueConcurrency)
	}
	if cfg.RetentionDays <= 0 {
		t.Errorf("RetentionDays: got %d, want > 0", cfg.RetentionDays)
	}
}

func TestAppLoadConfigFromDiskValid(t *testing.T) {
	app := setupTestApp(t)
	cfgPath := filepath.Join(app.dataDir, "config.json")
	cfgData := `{"queueConcurrency": 50, "retentionDays": 30}`
	if err := os.WriteFile(cfgPath, []byte(cfgData), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	app.configMu.Lock()
	app.configPath = cfgPath
	app.configMu.Unlock()
	if err := app.loadConfigFromDisk(); err != nil {
		t.Fatalf("loadConfigFromDisk: %v", err)
	}
	app.configMu.Lock()
	got := app.config.QueueConcurrency
	app.configMu.Unlock()
	if got != 50 {
		t.Errorf("QueueConcurrency: got %d, want 50", got)
	}
}

func TestAppCheckAndReloadConfig(t *testing.T) {
	app := setupTestApp(t)
	app.queue = nil
	app.proxyManager = nil
	cfgPath := filepath.Join(app.dataDir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"proxyConcurrency": 42}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	app.configMu.Lock()
	app.configPath = cfgPath
	app.configModTime = time.Time{}
	app.configMu.Unlock()
	app.checkAndReloadConfig(context.Background())
	app.configMu.Lock()
	got := app.config.ProxyConcurrency
	app.configMu.Unlock()
	if got != 42 {
		t.Errorf("ProxyConcurrency after reload: got %d, want 42", got)
	}
}

func TestAppPurgeOnce(t *testing.T) {
	app := setupTestApp(t)
	app.ctx = context.Background()
	app.purgeOnce()
}

func TestAppCleanup(t *testing.T) {
	app := setupTestAppWithQueue(t)
	app.cleanup()
}

func TestAppShutdown(t *testing.T) {
	app := setupTestAppWithQueue(t)
	app.shutdown(context.Background())
}

func TestAppGetTaskMetrics(t *testing.T) {
	app := setupTestAppWithQueue(t)
	metrics := app.getTaskMetrics()
	_ = metrics
}

func TestAppLoadConfigInvalidJSON(t *testing.T) {
	app := setupTestApp(t)
	cfgPath := filepath.Join(app.dataDir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{invalid json}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	app.configMu.Lock()
	app.configPath = cfgPath
	app.configMu.Unlock()
	err := app.loadConfigFromDisk()
	if err == nil {
		t.Fatal("expected error for invalid JSON config")
	}
}

func TestAppLoadConfigMissingFile(t *testing.T) {
	app := setupTestApp(t)
	app.configMu.Lock()
	app.configPath = filepath.Join(app.dataDir, "nonexistent.json")
	app.configMu.Unlock()
	err := app.loadConfigFromDisk()
	// loadConfigFromDisk may return nil (silently ignoring missing file) or an error
	_ = err
}

func TestAppCheckAndReloadConfigNoChange(t *testing.T) {
	app := setupTestApp(t)
	cfgPath := filepath.Join(app.dataDir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"queueConcurrency": 10}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	info, _ := os.Stat(cfgPath)
	app.configMu.Lock()
	app.configPath = cfgPath
	app.configModTime = info.ModTime() // set to current mtime so no change detected
	app.configMu.Unlock()
	app.checkAndReloadConfig(context.Background())
}

func TestAppListWebSocketLogsWithFlowID(t *testing.T) {
	app := setupTestApp(t)
	steps := []models.RecordedStep{{Index: 0, Action: models.ActionNavigate, Value: "https://example.com"}}
	flow, _ := app.CreateRecordedFlow("WS Flow", "", "https://example.com", steps)
	logs, err := app.ListWebSocketLogs(flow.ID)
	if err != nil {
		t.Fatalf("ListWebSocketLogs: %v", err)
	}
	_ = logs
}

func TestAppRetryFailedBatch(t *testing.T) {
	app := setupTestAppWithQueue(t)
	tasks, err := app.RetryFailedBatch("nonexistent-batch")
	if err != nil {
		t.Fatalf("RetryFailedBatch: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks for nonexistent batch, got %d", len(tasks))
	}
}

func TestAppListTasksByBatchEmpty2(t *testing.T) {
	app := setupTestApp(t)
	tasks, err := app.ListTasksByBatch("nonexistent-batch-2")
	if err != nil {
		t.Fatalf("ListTasksByBatch: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestAppExportTaskLogs(t *testing.T) {
	app := setupTestApp(t)
	logsDir := filepath.Join(t.TempDir(), "logs")
	le, err := logs.NewExporter(app.db, logsDir)
	if err != nil {
		t.Fatalf("NewExporter: %v", err)
	}
	app.logExporter = le
	task, _ := app.CreateTask(CreateTaskParams{Name: "Task", URL: "https://example.com", Steps: validSteps(), ProxyConfig: models.ProxyConfig{}, Priority: 5, AutoStart: false, Tags: nil, Timeout: 0, LoggingPolicy: nil})
	_, err = app.ExportTaskLogs(task.ID)
	if err != nil {
		t.Fatalf("ExportTaskLogs: %v", err)
	}
}

func TestAppExportBatchLogs(t *testing.T) {
	app := setupTestApp(t)
	logsDir := filepath.Join(t.TempDir(), "logs")
	le, err := logs.NewExporter(app.db, logsDir)
	if err != nil {
		t.Fatalf("NewExporter: %v", err)
	}
	app.logExporter = le
	_, err = app.ExportBatchLogs("batch-123")
	if err != nil {
		t.Fatalf("ExportBatchLogs: %v", err)
	}
}

func TestAppExportTaskLogsNilExporter2(t *testing.T) {
	app := setupTestApp(t)
	app.logExporter = nil
	_, err := app.ExportTaskLogs("task-id-nil")
	if err == nil {
		t.Fatal("expected error when logExporter is nil")
	}
}

func TestAppListProxyCountryStats(t *testing.T) {
	app := setupTestApp(t)
	app.proxyManager = proxy.NewManager(app.db, models.ProxyPoolConfig{})
	stats, err := app.ListProxyCountryStats()
	if err != nil {
		t.Fatalf("ListProxyCountryStats: %v", err)
	}
	_ = stats
}

func TestAppAddProxyWithRateLimit(t *testing.T) {
	app := setupTestApp(t)
	app.proxyManager = proxy.NewManager(app.db, models.ProxyPoolConfig{})
	p, err := app.AddProxyWithRateLimit("proxy.example.com:8080", "http", "US", "", "", 60)
	if err != nil {
		t.Fatalf("AddProxyWithRateLimit: %v", err)
	}
	_ = p
}

func TestAppGetLocalProxyGatewayStatsWithManager(t *testing.T) {
	app := setupTestApp(t)
	lpm := localproxy.NewManager(time.Minute)
	t.Cleanup(func() { lpm.Stop() })
	app.localProxyManager = lpm
	stats, err := app.GetLocalProxyGatewayStats()
	if err != nil {
		t.Fatalf("GetLocalProxyGatewayStats: %v", err)
	}
	if stats.ActiveEndpoints < 0 {
		t.Error("unexpected negative ActiveEndpoints")
	}
}

func TestValidateStepActionsPackageLevel(t *testing.T) {
	steps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionClick, Selector: "#btn"},
	}
	if err := validateStepActions(steps); err != nil {
		t.Fatalf("validateStepActions: %v", err)
	}
	badSteps := []models.TaskStep{{Action: "completely_invalid_action"}}
	if err := validateStepActions(badSteps); err == nil {
		t.Fatal("expected error for invalid action")
	}
}

func TestAppRunRetentionCleanup(t *testing.T) {
	app := setupTestApp(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	app.ctx = ctx
	app.config.RetentionDays = 90
	done := make(chan struct{})
	go func() {
		defer close(done)
		app.runRetentionCleanup(ctx)
	}()
	<-ctx.Done()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("runRetentionCleanup did not exit after ctx cancelled")
	}
}

func TestAppWatchConfig(t *testing.T) {
	app := setupTestApp(t)
	cfgPath := filepath.Join(app.dataDir, "config.json")
	_ = os.WriteFile(cfgPath, []byte(`{"queueConcurrency": 5}`), 0o600)
	app.configMu.Lock()
	app.configPath = cfgPath
	app.configMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		app.watchConfig(ctx)
	}()
	<-ctx.Done()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("watchConfig did not exit after ctx cancelled")
	}
}

func TestAppParseBatchURLsCSV(t *testing.T) {
	app := setupTestApp(t)
	csvContent := "https://a.com,tag1\nhttps://b.com,tag2\nhttps://c.com,tag3"
	tmpFile := filepath.Join(t.TempDir(), "urls.csv")
	if err := os.WriteFile(tmpFile, []byte(csvContent), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	urls, err := app.ParseBatchURLs(tmpFile, true)
	if err != nil {
		t.Fatalf("ParseBatchURLs CSV: %v", err)
	}
	if len(urls) == 0 {
		t.Error("expected at least one URL from CSV")
	}
}

func TestAppCollectUnknownActionWarnings(t *testing.T) {
	steps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: "unknown_action_xyz"},
	}
	warnings := collectUnknownStepActionWarnings(steps, 0)
	if len(warnings) == 0 {
		t.Error("expected warnings for unknown action")
	}
}

func TestAppPurgeOnceWithRetention(t *testing.T) {
	app := setupTestApp(t)
	app.ctx = context.Background()
	app.config.RetentionDays = 1
	app.purgeOnce()
}

func TestAppTestCaptchaConfig(t *testing.T) {
	app := setupTestApp(t)
	runner, _ := browser.NewRunner(t.TempDir())
	app.runner = runner
	cfg, _ := app.SaveCaptchaConfig("2captcha", "dummy-api-key")
	if cfg == nil {
		t.Fatal("expected non-nil captcha config")
	}
	_, err := app.TestCaptchaConfig(cfg.ID)
	if err == nil {
		t.Log("TestCaptchaConfig returned no error (provider may accept key)")
	}
}
