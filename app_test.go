package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"web-automation/internal/browser"
	"web-automation/internal/crypto"
	"web-automation/internal/database"
	"web-automation/internal/models"
	"web-automation/internal/queue"
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

	task, err := app.CreateTask("Test Task", "https://example.com", validSteps(), models.ProxyConfig{}, 5, false, nil)
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

	_, err := app.CreateTask("", "https://example.com", validSteps(), models.ProxyConfig{}, 5, false, nil)
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
	if !strings.Contains(err.Error(), "name must not be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAppCreateTaskInvalidURL(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.CreateTask("Task", "not-a-url", validSteps(), models.ProxyConfig{}, 5, false, nil)
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
	_, err := app.CreateTask("Task", "https://example.com", badSteps, models.ProxyConfig{}, 5, false, nil)
	if err == nil {
		t.Fatal("expected error for invalid step action, got nil")
	}
	if !strings.Contains(err.Error(), "invalid step action") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAppCreateTaskInvalidPriority(t *testing.T) {
	app := setupTestApp(t)

	_, err := app.CreateTask("Task", "https://example.com", validSteps(), models.ProxyConfig{}, 99, false, nil)
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

	task, err := app.CreateTask("Delete Me", "https://example.com", validSteps(), models.ProxyConfig{}, 5, false, nil)
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

	created, err := app.CreateTask("Get Me", "https://example.com", validSteps(), models.ProxyConfig{}, 5, false, nil)
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
		_, err := app.CreateTask("Task", "https://example.com", validSteps(), models.ProxyConfig{}, 5, false, nil)
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
	_, err := app.CreateTask("Eval Task", "https://example.com", evalSteps, models.ProxyConfig{}, 5, false, nil)
	if err == nil {
		t.Fatal("expected error for eval step, got nil")
	}
	if !strings.Contains(err.Error(), "eval") {
		t.Errorf("unexpected error message: %v", err)
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

	_, err := app.CreateTask("Pending 1", "https://example.com", validSteps(), models.ProxyConfig{}, 5, false, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	_, err = app.CreateTask("Pending 2", "https://example.com", validSteps(), models.ProxyConfig{}, 5, false, nil)
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
		_, err := app.CreateTask("Task", "https://example.com", validSteps(), models.ProxyConfig{}, 5, false, nil)
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

	task, err := app.CreateTask("Export Test", "https://example.com", validSteps(), models.ProxyConfig{}, 5, false, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := app.db.UpdateTaskStatus(task.ID, models.TaskStatusCompleted, ""); err != nil {
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

	task, err := app.CreateTask("CSV Test", "https://example.com", validSteps(), models.ProxyConfig{}, 5, false, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := app.db.UpdateTaskStatus(task.ID, models.TaskStatusCompleted, ""); err != nil {
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

	task, err := app.CreateTask("Queue Test", "https://example.com", validSteps(), models.ProxyConfig{}, 5, false, nil)
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

	task, err := app.CreateTask("Cancel Test", "https://example.com", validSteps(), models.ProxyConfig{}, 5, false, nil)
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

	task, err := app.CreateTask("Original", "https://example.com", validSteps(), models.ProxyConfig{}, 5, false, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	newSteps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://updated.com"},
		{Action: models.ActionClick, Selector: "#new"},
	}
	err = app.UpdateTask(task.ID, "Updated", "https://updated.com", newSteps, models.ProxyConfig{}, 10)
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

	inputs := []struct {
		Name     string             `json:"name"`
		URL      string             `json:"url"`
		Steps    []models.TaskStep  `json:"steps"`
		Proxy    models.ProxyConfig `json:"proxy"`
		Priority int                `json:"priority"`
	}{
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

	inputs := []struct {
		Name     string             `json:"name"`
		URL      string             `json:"url"`
		Steps    []models.TaskStep  `json:"steps"`
		Proxy    models.ProxyConfig `json:"proxy"`
		Priority int                `json:"priority"`
	}{
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

func TestAppUpdateTaskValidation(t *testing.T) {
	app := setupTestApp(t)

	task, err := app.CreateTask("Task", "https://example.com", validSteps(), models.ProxyConfig{}, 5, false, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	err = app.UpdateTask(task.ID, "", "https://example.com", validSteps(), models.ProxyConfig{}, 5)
	if err == nil {
		t.Fatal("expected validation error for empty name")
	}
}

func TestAppStartAllPending(t *testing.T) {
	app := setupTestAppWithQueue(t)

	for i := 0; i < 3; i++ {
		_, err := app.CreateTask(
			fmt.Sprintf("Pending %d", i),
			"https://example.com",
			validSteps(),
			models.ProxyConfig{},
			5,
			false,
			nil,
		)
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

	task, err := app.CreateTask("Cancel Delete", "https://example.com", validSteps(), models.ProxyConfig{}, 5, false, nil)
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

	task, err := app.CreateTask("Auto Start", "https://example.com", validSteps(), models.ProxyConfig{}, 5, true, []string{"test"})
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
