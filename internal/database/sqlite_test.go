package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"flowpilot/internal/crypto"
	"flowpilot/internal/models"
)

func setupTestDB(t *testing.T) *DB {
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

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func makeTask(id, name string) models.Task {
	return models.Task{
		ID:   id,
		Name: name,
		URL:  "https://example.com",
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: "https://example.com"},
			{Action: models.ActionClick, Selector: "#btn"},
		},
		Proxy: models.ProxyConfig{
			Server:   "proxy.example.com:8080",
			Username: "user",
			Password: "pass",
			Geo:      "US",
		},
		Priority:   models.PriorityNormal,
		Status:     models.TaskStatusPending,
		MaxRetries: 3,
		Tags:       []string{"test", "demo"},
		CreatedAt:  time.Now().Truncate(time.Second),
	}
}

func makeProxy(id, server, geo string) models.Proxy {
	return models.Proxy{
		ID:        id,
		Server:    server,
		Protocol:  models.ProxyHTTP,
		Username:  "user",
		Password:  "pass",
		Geo:       geo,
		Status:    models.ProxyStatusUnknown,
		CreatedAt: time.Now().Truncate(time.Second),
	}
}

// --- Task CRUD Tests ---

func TestCreateAndGetTask(t *testing.T) {
	db := setupTestDB(t)
	task := makeTask("task-1", "Test Task")

	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	got, err := db.GetTask(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}

	if got.ID != task.ID {
		t.Errorf("ID: got %q, want %q", got.ID, task.ID)
	}
	if got.Name != task.Name {
		t.Errorf("Name: got %q, want %q", got.Name, task.Name)
	}
	if got.URL != task.URL {
		t.Errorf("URL: got %q, want %q", got.URL, task.URL)
	}
	if len(got.Steps) != len(task.Steps) {
		t.Errorf("Steps length: got %d, want %d", len(got.Steps), len(task.Steps))
	}
	if got.Proxy.Server != task.Proxy.Server {
		t.Errorf("Proxy.Server: got %q, want %q", got.Proxy.Server, task.Proxy.Server)
	}
	if got.Priority != task.Priority {
		t.Errorf("Priority: got %d, want %d", got.Priority, task.Priority)
	}
	if got.Status != task.Status {
		t.Errorf("Status: got %q, want %q", got.Status, task.Status)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "test" {
		t.Errorf("Tags: got %v, want %v", got.Tags, task.Tags)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetTask(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent task, got nil")
	}
}

func TestCreateTaskDuplicateID(t *testing.T) {
	db := setupTestDB(t)
	task := makeTask("dup-1", "First")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("first CreateTask: %v", err)
	}
	task2 := makeTask("dup-1", "Second")
	err := db.CreateTask(context.Background(), task2)
	if err == nil {
		t.Fatal("expected error for duplicate ID, got nil")
	}
}

func TestListTasks(t *testing.T) {
	db := setupTestDB(t)

	for i := 0; i < 5; i++ {
		task := makeTask(
			"list-"+string(rune('a'+i)),
			"Task "+string(rune('A'+i)),
		)
		task.Priority = models.TaskPriority(i + 1)
		if err := db.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}
	}

	tasks, err := db.ListTasks(context.Background())
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 5 {
		t.Fatalf("ListTasks count: got %d, want 5", len(tasks))
	}

	// Should be ordered by priority DESC
	for i := 1; i < len(tasks); i++ {
		if tasks[i].Priority > tasks[i-1].Priority {
			t.Errorf("tasks not sorted by priority DESC: %d > %d at index %d",
				tasks[i].Priority, tasks[i-1].Priority, i)
		}
	}
}

func TestListTasksByStatus(t *testing.T) {
	db := setupTestDB(t)

	task1 := makeTask("status-1", "Pending Task")
	task1.Status = models.TaskStatusPending
	task2 := makeTask("status-2", "Running Task")
	task2.Status = models.TaskStatusRunning
	task3 := makeTask("status-3", "Also Pending")
	task3.Status = models.TaskStatusPending

	for _, task := range []models.Task{task1, task2, task3} {
		if err := db.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("CreateTask %s: %v", task.ID, err)
		}
	}

	pending, err := db.ListTasksByStatus(context.Background(), models.TaskStatusPending)
	if err != nil {
		t.Fatalf("ListTasksByStatus(pending): %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("pending count: got %d, want 2", len(pending))
	}

	running, err := db.ListTasksByStatus(context.Background(), models.TaskStatusRunning)
	if err != nil {
		t.Fatalf("ListTasksByStatus(running): %v", err)
	}
	if len(running) != 1 {
		t.Errorf("running count: got %d, want 1", len(running))
	}
}

func TestUpdateTaskStatus(t *testing.T) {
	db := setupTestDB(t)
	task := makeTask("upd-status-1", "Update Status Test")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	tests := []struct {
		name   string
		status models.TaskStatus
		errMsg string
	}{
		{"to running", models.TaskStatusRunning, ""},
		{"to failed", models.TaskStatusFailed, "something broke"},
		{"to queued", models.TaskStatusQueued, ""},
		{"to completed", models.TaskStatusCompleted, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := db.UpdateTaskStatus(context.Background(), task.ID, tc.status, tc.errMsg); err != nil {
				t.Fatalf("UpdateTaskStatus: %v", err)
			}
			got, err := db.GetTask(context.Background(), task.ID)
			if err != nil {
				t.Fatalf("GetTask: %v", err)
			}
			if got.Status != tc.status {
				t.Errorf("status: got %q, want %q", got.Status, tc.status)
			}
		})
	}
}

func TestUpdateTaskStatusSetsTimestamps(t *testing.T) {
	db := setupTestDB(t)
	task := makeTask("ts-1", "Timestamp Test")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Running should set started_at
	if err := db.UpdateTaskStatus(context.Background(), task.ID, models.TaskStatusRunning, ""); err != nil {
		t.Fatalf("UpdateTaskStatus(running): %v", err)
	}
	got, _ := db.GetTask(context.Background(), task.ID)
	if got.StartedAt == nil {
		t.Error("StartedAt should be set after running")
	}

	// Completed should set completed_at
	if err := db.UpdateTaskStatus(context.Background(), task.ID, models.TaskStatusCompleted, ""); err != nil {
		t.Fatalf("UpdateTaskStatus(completed): %v", err)
	}
	got, _ = db.GetTask(context.Background(), task.ID)
	if got.CompletedAt == nil {
		t.Error("CompletedAt should be set after completed")
	}
}

func TestUpdateTaskResult(t *testing.T) {
	db := setupTestDB(t)
	task := makeTask("result-1", "Result Test")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	result := models.TaskResult{
		TaskID:  task.ID,
		Success: true,
		ExtractedData: map[string]string{
			"title": "Example",
		},
		Screenshots: []string{"/tmp/shot1.png"},
		StepLogs:    []models.StepLog{{TaskID: task.ID, StepIndex: 0, Action: models.ActionNavigate}},
		NetworkLogs: []models.NetworkLog{{TaskID: task.ID, StepIndex: 0, RequestURL: "https://example.com", Method: "GET"}},
		Duration:    5 * time.Second,
	}

	if err := db.UpdateTaskResult(context.Background(), task.ID, result); err != nil {
		t.Fatalf("UpdateTaskResult: %v", err)
	}

	got, err := db.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Result == nil {
		t.Fatal("Result is nil after update")
	}
	if !got.Result.Success {
		t.Error("Result.Success should be true")
	}
	if len(got.Result.StepLogs) != 0 {
		t.Errorf("StepLogs should not be stored in task result, got %d", len(got.Result.StepLogs))
	}
	if len(got.Result.NetworkLogs) != 0 {
		t.Errorf("NetworkLogs should not be stored in task result, got %d", len(got.Result.NetworkLogs))
	}
	if got.Result.ExtractedData["title"] != "Example" {
		t.Errorf("ExtractedData[title]: got %q, want %q", got.Result.ExtractedData["title"], "Example")
	}
}

func TestIncrementRetry(t *testing.T) {
	db := setupTestDB(t)
	task := makeTask("retry-1", "Retry Test")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := db.IncrementRetry(context.Background(), task.ID); err != nil {
			t.Fatalf("IncrementRetry %d: %v", i, err)
		}
	}

	got, err := db.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.RetryCount != 3 {
		t.Errorf("RetryCount: got %d, want 3", got.RetryCount)
	}
	if got.Status != models.TaskStatusRetrying {
		t.Errorf("Status: got %q, want %q", got.Status, models.TaskStatusRetrying)
	}
}

func TestDeleteTask(t *testing.T) {
	db := setupTestDB(t)
	task := makeTask("del-1", "Delete Test")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := db.DeleteTask(context.Background(), task.ID); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}

	_, err := db.GetTask(context.Background(), task.ID)
	if err == nil {
		t.Error("GetTask should return error after deletion")
	}
}

func TestGetTaskStats(t *testing.T) {
	db := setupTestDB(t)

	tasks := []models.Task{
		makeTask("stats-1", "S1"),
		makeTask("stats-2", "S2"),
		makeTask("stats-3", "S3"),
	}
	tasks[0].Status = models.TaskStatusPending
	tasks[1].Status = models.TaskStatusPending
	tasks[2].Status = models.TaskStatusCompleted

	for _, task := range tasks {
		if err := db.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
	}

	stats, err := db.GetTaskStats(context.Background())
	if err != nil {
		t.Fatalf("GetTaskStats: %v", err)
	}

	if stats["pending"] != 2 {
		t.Errorf("pending count: got %d, want 2", stats["pending"])
	}
	if stats["completed"] != 1 {
		t.Errorf("completed count: got %d, want 1", stats["completed"])
	}
}

// --- Proxy CRUD Tests ---

func TestCreateAndListProxies(t *testing.T) {
	db := setupTestDB(t)

	p1 := makeProxy("proxy-1", "proxy1.example.com:8080", "US")
	p2 := makeProxy("proxy-2", "proxy2.example.com:8080", "UK")

	if err := db.CreateProxy(context.Background(), p1); err != nil {
		t.Fatalf("CreateProxy 1: %v", err)
	}
	if err := db.CreateProxy(context.Background(), p2); err != nil {
		t.Fatalf("CreateProxy 2: %v", err)
	}

	proxies, err := db.ListProxies(context.Background())
	if err != nil {
		t.Fatalf("ListProxies: %v", err)
	}
	if len(proxies) != 2 {
		t.Errorf("proxy count: got %d, want 2", len(proxies))
	}
}

func TestListHealthyProxies(t *testing.T) {
	db := setupTestDB(t)

	p1 := makeProxy("hp-1", "h1.example.com:8080", "US")
	p2 := makeProxy("hp-2", "h2.example.com:8080", "UK")

	if err := db.CreateProxy(context.Background(), p1); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}
	if err := db.CreateProxy(context.Background(), p2); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}

	// Mark one as healthy
	if err := db.UpdateProxyHealth(context.Background(), "hp-1", models.ProxyStatusHealthy, 50); err != nil {
		t.Fatalf("UpdateProxyHealth: %v", err)
	}

	healthy, err := db.ListHealthyProxies(context.Background())
	if err != nil {
		t.Fatalf("ListHealthyProxies: %v", err)
	}
	if len(healthy) != 1 {
		t.Errorf("healthy count: got %d, want 1", len(healthy))
	}
	if healthy[0].ID != "hp-1" {
		t.Errorf("healthy proxy ID: got %q, want %q", healthy[0].ID, "hp-1")
	}
}

func TestUpdateProxyHealth(t *testing.T) {
	db := setupTestDB(t)
	p := makeProxy("health-1", "health.example.com:8080", "US")
	if err := db.CreateProxy(context.Background(), p); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}

	if err := db.UpdateProxyHealth(context.Background(), "health-1", models.ProxyStatusHealthy, 100); err != nil {
		t.Fatalf("UpdateProxyHealth: %v", err)
	}

	proxies, err := db.ListProxies(context.Background())
	if err != nil {
		t.Fatalf("ListProxies: %v", err)
	}

	var found bool
	for _, px := range proxies {
		if px.ID == "health-1" {
			found = true
			if px.Status != models.ProxyStatusHealthy {
				t.Errorf("Status: got %q, want %q", px.Status, models.ProxyStatusHealthy)
			}
			if px.Latency != 100 {
				t.Errorf("Latency: got %d, want 100", px.Latency)
			}
			if px.LastChecked == nil {
				t.Error("LastChecked should be set")
			}
		}
	}
	if !found {
		t.Error("proxy not found in list")
	}
}

func TestIncrementProxyUsage(t *testing.T) {
	db := setupTestDB(t)
	p := makeProxy("usage-1", "usage.example.com:8080", "US")
	if err := db.CreateProxy(context.Background(), p); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}

	// Record 3 successes and 1 failure
	for i := 0; i < 3; i++ {
		if err := db.IncrementProxyUsage(context.Background(), "usage-1", true); err != nil {
			t.Fatalf("IncrementProxyUsage(success) %d: %v", i, err)
		}
	}
	if err := db.IncrementProxyUsage(context.Background(), "usage-1", false); err != nil {
		t.Fatalf("IncrementProxyUsage(failure): %v", err)
	}

	proxies, err := db.ListProxies(context.Background())
	if err != nil {
		t.Fatalf("ListProxies: %v", err)
	}
	for _, px := range proxies {
		if px.ID == "usage-1" {
			if px.TotalUsed != 4 {
				t.Errorf("TotalUsed: got %d, want 4", px.TotalUsed)
			}
			// Success rate should be 0.75 (3/4)
			if px.SuccessRate < 0.74 || px.SuccessRate > 0.76 {
				t.Errorf("SuccessRate: got %f, want ~0.75", px.SuccessRate)
			}
		}
	}
}

func TestDeleteProxy(t *testing.T) {
	db := setupTestDB(t)
	p := makeProxy("del-p-1", "del.example.com:8080", "US")
	if err := db.CreateProxy(context.Background(), p); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}

	if err := db.DeleteProxy(context.Background(), "del-p-1"); err != nil {
		t.Fatalf("DeleteProxy: %v", err)
	}

	proxies, err := db.ListProxies(context.Background())
	if err != nil {
		t.Fatalf("ListProxies: %v", err)
	}
	if len(proxies) != 0 {
		t.Errorf("proxy count after delete: got %d, want 0", len(proxies))
	}
}

func TestNewDatabaseInvalidPath(t *testing.T) {
	_, err := New("/nonexistent/path/to/db.db")
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

func TestCreateTaskWithNilStepsAndTags(t *testing.T) {
	db := setupTestDB(t)
	task := models.Task{
		ID:        "nil-fields",
		Name:      "Nil Fields",
		URL:       "https://example.com",
		Status:    models.TaskStatusPending,
		CreatedAt: time.Now(),
		// Steps and Tags are nil
	}

	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask with nil slices: %v", err)
	}

	got, err := db.GetTask(context.Background(), "nil-fields")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	// nil steps serializes as "null", which Unmarshal treats as nil slice
	if got.Steps != nil && len(got.Steps) != 0 {
		t.Errorf("Steps: got %v, want nil or empty", got.Steps)
	}
}

func TestMigrationIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Create database twice - migration should be idempotent
	db1, err := New(dbPath)
	if err != nil {
		t.Fatalf("first New: %v", err)
	}
	db1.Close()

	db2, err := New(dbPath)
	if err != nil {
		t.Fatalf("second New: %v", err)
	}
	db2.Close()
}

func TestNewDatabasePermissions(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}
	dir := t.TempDir()
	// Make directory read-only
	if err := os.Chmod(dir, 0o444); err != nil {
		t.Skip("cannot change permissions")
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	dbPath := filepath.Join(dir, "test.db")
	_, err := New(dbPath)
	if err == nil {
		t.Error("expected error for read-only directory")
	}
}

func TestProxyCredentialsEncryptedAtRest(t *testing.T) {
	db := setupTestDB(t)
	p := makeProxy("enc-1", "proxy.example.com:8080", "US")
	p.Username = "cleartext_user"
	p.Password = "cleartext_pass"

	if err := db.CreateProxy(context.Background(), p); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}

	var rawUsername, rawPassword string
	err := db.conn.QueryRow(`SELECT username, password FROM proxies WHERE id = ?`, "enc-1").Scan(&rawUsername, &rawPassword)
	if err != nil {
		t.Fatalf("raw query: %v", err)
	}

	if rawUsername == "cleartext_user" {
		t.Error("username stored in plaintext — expected ciphertext")
	}
	if rawPassword == "cleartext_pass" {
		t.Error("password stored in plaintext — expected ciphertext")
	}

	got, err := db.ListProxies(context.Background())
	if err != nil {
		t.Fatalf("ListProxies: %v", err)
	}
	var found *models.Proxy
	for i := range got {
		if got[i].ID == "enc-1" {
			found = &got[i]
			break
		}
	}
	if found == nil {
		t.Fatal("proxy not found")
	}
	if found.Username != "cleartext_user" {
		t.Errorf("decrypted username: got %q, want %q", found.Username, "cleartext_user")
	}
	if found.Password != "cleartext_pass" {
		t.Errorf("decrypted password: got %q, want %q", found.Password, "cleartext_pass")
	}
}

func TestTaskProxyCredentialsEncryptedAtRest(t *testing.T) {
	db := setupTestDB(t)
	task := makeTask("enc-task-1", "Encrypted Task")
	task.Proxy.Username = "task_user"
	task.Proxy.Password = "task_pass"

	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	var rawUsername, rawPassword string
	err := db.conn.QueryRow(`SELECT proxy_username, proxy_password FROM tasks WHERE id = ?`, "enc-task-1").Scan(&rawUsername, &rawPassword)
	if err != nil {
		t.Fatalf("raw query: %v", err)
	}

	if rawUsername == "task_user" {
		t.Error("proxy_username stored in plaintext — expected ciphertext")
	}
	if rawPassword == "task_pass" {
		t.Error("proxy_password stored in plaintext — expected ciphertext")
	}

	got, err := db.GetTask(context.Background(), "enc-task-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Proxy.Username != "task_user" {
		t.Errorf("decrypted proxy username: got %q, want %q", got.Proxy.Username, "task_user")
	}
	if got.Proxy.Password != "task_pass" {
		t.Errorf("decrypted proxy password: got %q, want %q", got.Proxy.Password, "task_pass")
	}
}

func TestUpdateTask(t *testing.T) {
	db := setupTestDB(t)
	task := makeTask("upd-1", "Original")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	newSteps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://updated.com"},
	}
	newProxy := models.ProxyConfig{Server: "new.proxy:9090", Username: "u2", Password: "p2"}

	if err := db.UpdateTask(context.Background(), "upd-1", "Updated", "https://updated.com", newSteps, newProxy, models.PriorityHigh, []string{"updated-tag"}, 0, nil); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	got, err := db.GetTask(context.Background(), "upd-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("Name: got %q, want %q", got.Name, "Updated")
	}
	if got.URL != "https://updated.com" {
		t.Errorf("URL: got %q, want %q", got.URL, "https://updated.com")
	}
	if got.Priority != models.PriorityHigh {
		t.Errorf("Priority: got %d, want %d", got.Priority, models.PriorityHigh)
	}
	if len(got.Steps) != 1 || got.Steps[0].Value != "https://updated.com" {
		t.Errorf("Steps not updated correctly: %v", got.Steps)
	}
	if got.Proxy.Server != "new.proxy:9090" {
		t.Errorf("Proxy.Server: got %q, want %q", got.Proxy.Server, "new.proxy:9090")
	}
}

func TestDeleteTaskNotFound(t *testing.T) {
	db := setupTestDB(t)
	err := db.DeleteTask(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent task ID")
	}
}

func TestDeleteProxyNotFound(t *testing.T) {
	db := setupTestDB(t)
	err := db.DeleteProxy(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent proxy ID")
	}
}

func TestListTasksPaginated(t *testing.T) {
	db := setupTestDB(t)

	for i := 0; i < 15; i++ {
		task := makeTask(
			fmt.Sprintf("page-%d", i),
			fmt.Sprintf("Task %d", i),
		)
		if err := db.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}
	}

	result, err := db.ListTasksPaginated(context.Background(), 1, 5, "", "")
	if err != nil {
		t.Fatalf("ListTasksPaginated: %v", err)
	}
	if result.Total != 15 {
		t.Errorf("Total: got %d, want 15", result.Total)
	}
	if len(result.Tasks) != 5 {
		t.Errorf("Tasks: got %d, want 5", len(result.Tasks))
	}
	if result.TotalPages != 3 {
		t.Errorf("TotalPages: got %d, want 3", result.TotalPages)
	}
	for _, task := range result.Tasks {
		if len(task.Steps) != 0 {
			t.Errorf("paginated task %s should not include steps", task.ID)
		}
		if task.Result != nil {
			t.Errorf("paginated task %s should not include result", task.ID)
		}
	}

	result2, err := db.ListTasksPaginated(context.Background(), 3, 5, "", "")
	if err != nil {
		t.Fatalf("ListTasksPaginated page 3: %v", err)
	}
	if len(result2.Tasks) != 5 {
		t.Errorf("last page Tasks: got %d, want 5", len(result2.Tasks))
	}
}

func TestListTasksPaginatedWithStatusFilter(t *testing.T) {
	db := setupTestDB(t)

	for i := 0; i < 5; i++ {
		task := makeTask(fmt.Sprintf("filter-%d", i), fmt.Sprintf("Task %d", i))
		if i%2 == 0 {
			task.Status = models.TaskStatusCompleted
		}
		if err := db.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}
	}

	result, err := db.ListTasksPaginated(context.Background(), 1, 10, "completed", "")
	if err != nil {
		t.Fatalf("ListTasksPaginated: %v", err)
	}
	if result.Total != 3 {
		t.Errorf("Total completed: got %d, want 3", result.Total)
	}
}

func TestPurgeOldRecords(t *testing.T) {
	db := setupTestDB(t)

	task := makeTask("old-1", "Old Task")
	task.Status = models.TaskStatusCompleted
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := db.UpdateTaskStatus(context.Background(), "old-1", models.TaskStatusCompleted, ""); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}

	n, err := db.PurgeOldRecords(context.Background(), 90)
	if err != nil {
		t.Fatalf("PurgeOldRecords: %v", err)
	}
	if n != 0 {
		t.Errorf("purged: got %d, want 0 (task is new)", n)
	}
}

func TestListAuditTrail(t *testing.T) {
	db := setupTestDB(t)

	event := models.TaskLifecycleEvent{
		ID:        "evt-1",
		TaskID:    "task-1",
		FromState: models.TaskStatusPending,
		ToState:   models.TaskStatusRunning,
		Timestamp: time.Now(),
	}
	if err := db.InsertTaskEvent(context.Background(), event); err != nil {
		t.Fatalf("InsertTaskEvent: %v", err)
	}

	events, err := db.ListAuditTrail(context.Background(), "task-1", 10)
	if err != nil {
		t.Fatalf("ListAuditTrail: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("event count: got %d, want 1", len(events))
	}
	if events[0].ToState != models.TaskStatusRunning {
		t.Errorf("ToState: got %q, want %q", events[0].ToState, models.TaskStatusRunning)
	}

	all, err := db.ListAuditTrail(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("ListAuditTrail all: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("all events: got %d, want 1", len(all))
	}
}

func TestUpdateTaskRejectsRunning(t *testing.T) {
	db := setupTestDB(t)
	task := makeTask("upd-run-1", "Running Task")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := db.UpdateTaskStatus(context.Background(), "upd-run-1", models.TaskStatusRunning, ""); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}

	err := db.UpdateTask(context.Background(), "upd-run-1", "New Name", "https://x.com", nil, models.ProxyConfig{}, models.PriorityNormal, nil, 0, nil)
	if err == nil {
		t.Fatal("expected error when updating running task")
	}
}

// --- Recorded Flows CRUD Tests ---

func makeFlow(id, name string) models.RecordedFlow {
	now := time.Now().Truncate(time.Second)
	return models.RecordedFlow{
		ID:          id,
		Name:        name,
		Description: "Test flow description",
		Steps: []models.RecordedStep{
			{Index: 0, Action: models.ActionNavigate, Value: "https://example.com", Timestamp: now},
			{Index: 1, Action: models.ActionClick, Selector: "#btn", Timestamp: now},
		},
		OriginURL: "https://example.com",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestCreateAndGetRecordedFlow(t *testing.T) {
	db := setupTestDB(t)
	flow := makeFlow("flow-1", "Test Flow")

	if err := db.CreateRecordedFlow(context.Background(), flow); err != nil {
		t.Fatalf("CreateRecordedFlow: %v", err)
	}

	got, err := db.GetRecordedFlow(context.Background(), "flow-1")
	if err != nil {
		t.Fatalf("GetRecordedFlow: %v", err)
	}

	if got.ID != flow.ID {
		t.Errorf("ID: got %q, want %q", got.ID, flow.ID)
	}
	if got.Name != flow.Name {
		t.Errorf("Name: got %q, want %q", got.Name, flow.Name)
	}
	if got.Description != flow.Description {
		t.Errorf("Description: got %q, want %q", got.Description, flow.Description)
	}
	if len(got.Steps) != 2 {
		t.Errorf("Steps count: got %d, want 2", len(got.Steps))
	}
	if got.OriginURL != flow.OriginURL {
		t.Errorf("OriginURL: got %q, want %q", got.OriginURL, flow.OriginURL)
	}
}

func TestGetRecordedFlowNotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.GetRecordedFlow(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent flow")
	}
}

func TestListRecordedFlows(t *testing.T) {
	db := setupTestDB(t)

	for i := 0; i < 3; i++ {
		flow := makeFlow(fmt.Sprintf("list-flow-%d", i), fmt.Sprintf("Flow %d", i))
		flow.UpdatedAt = time.Now().Add(time.Duration(i) * time.Second)
		if err := db.CreateRecordedFlow(context.Background(), flow); err != nil {
			t.Fatalf("CreateRecordedFlow %d: %v", i, err)
		}
	}

	flows, err := db.ListRecordedFlows(context.Background())
	if err != nil {
		t.Fatalf("ListRecordedFlows: %v", err)
	}
	if len(flows) != 3 {
		t.Errorf("flow count: got %d, want 3", len(flows))
	}
}

func TestListRecordedFlowsEmpty(t *testing.T) {
	db := setupTestDB(t)
	flows, err := db.ListRecordedFlows(context.Background())
	if err != nil {
		t.Fatalf("ListRecordedFlows: %v", err)
	}
	if len(flows) != 0 {
		t.Errorf("expected 0 flows, got %d", len(flows))
	}
}

func TestUpdateRecordedFlow(t *testing.T) {
	db := setupTestDB(t)
	flow := makeFlow("upd-flow-1", "Original")

	if err := db.CreateRecordedFlow(context.Background(), flow); err != nil {
		t.Fatalf("CreateRecordedFlow: %v", err)
	}

	flow.Name = "Updated"
	flow.Description = "Updated description"
	flow.UpdatedAt = time.Now()

	if err := db.UpdateRecordedFlow(context.Background(), flow); err != nil {
		t.Fatalf("UpdateRecordedFlow: %v", err)
	}

	got, err := db.GetRecordedFlow(context.Background(), "upd-flow-1")
	if err != nil {
		t.Fatalf("GetRecordedFlow: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("Name: got %q, want %q", got.Name, "Updated")
	}
	if got.Description != "Updated description" {
		t.Errorf("Description: got %q, want %q", got.Description, "Updated description")
	}
}

func TestUpdateRecordedFlowNotFound(t *testing.T) {
	db := setupTestDB(t)
	flow := makeFlow("nonexistent-flow", "Ghost")
	err := db.UpdateRecordedFlow(context.Background(), flow)
	if err == nil {
		t.Fatal("expected error for nonexistent flow update")
	}
}

func TestDeleteRecordedFlow(t *testing.T) {
	db := setupTestDB(t)
	flow := makeFlow("del-flow-1", "Delete Me")

	if err := db.CreateRecordedFlow(context.Background(), flow); err != nil {
		t.Fatalf("CreateRecordedFlow: %v", err)
	}

	if err := db.DeleteRecordedFlow(context.Background(), "del-flow-1"); err != nil {
		t.Fatalf("DeleteRecordedFlow: %v", err)
	}

	_, err := db.GetRecordedFlow(context.Background(), "del-flow-1")
	if err == nil {
		t.Error("GetRecordedFlow should fail after deletion")
	}
}

func TestDeleteRecordedFlowNotFound(t *testing.T) {
	db := setupTestDB(t)
	err := db.DeleteRecordedFlow(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent flow deletion")
	}
}

func TestDeleteRecordedFlowCascade(t *testing.T) {
	db := setupTestDB(t)
	flow := makeFlow("cascade-flow", "Cascade Test")
	if err := db.CreateRecordedFlow(context.Background(), flow); err != nil {
		t.Fatalf("CreateRecordedFlow: %v", err)
	}

	snap := models.DOMSnapshot{
		ID: "cascade-snap-1", FlowID: "cascade-flow", StepIndex: 0,
		HTML: "<html></html>", ScreenshotPath: "/tmp/s.png",
		URL: "https://example.com", CapturedAt: time.Now(),
	}
	if err := db.CreateDOMSnapshot(context.Background(), snap); err != nil {
		t.Fatalf("CreateDOMSnapshot: %v", err)
	}

	wsLogs := []models.WebSocketLog{
		{FlowID: "cascade-flow", StepIndex: 0, RequestID: "ws-1",
			URL: "wss://example.com", EventType: models.WSEventCreated,
			Timestamp: time.Now()},
	}
	if err := db.InsertWebSocketLogs(context.Background(), "cascade-flow", wsLogs); err != nil {
		t.Fatalf("InsertWebSocketLogs: %v", err)
	}

	if err := db.DeleteRecordedFlow(context.Background(), "cascade-flow"); err != nil {
		t.Fatalf("DeleteRecordedFlow: %v", err)
	}

	snaps, err := db.ListDOMSnapshots(context.Background(), "cascade-flow")
	if err != nil {
		t.Fatalf("ListDOMSnapshots: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("expected 0 snapshots after cascade delete, got %d", len(snaps))
	}

	remaining, err := db.ListWebSocketLogs(context.Background(), "cascade-flow")
	if err != nil {
		t.Fatalf("ListWebSocketLogs: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 websocket logs after cascade delete, got %d", len(remaining))
	}
}

// --- DOM Snapshots Tests ---

func TestCreateAndListDOMSnapshots(t *testing.T) {
	db := setupTestDB(t)

	snapshots := []models.DOMSnapshot{
		{ID: "snap-1", FlowID: "flow-1", StepIndex: 0, HTML: "<html>step0</html>", ScreenshotPath: "/tmp/s0.png", URL: "https://example.com", CapturedAt: time.Now()},
		{ID: "snap-2", FlowID: "flow-1", StepIndex: 1, HTML: "<html>step1</html>", ScreenshotPath: "/tmp/s1.png", URL: "https://example.com/page", CapturedAt: time.Now()},
		{ID: "snap-3", FlowID: "flow-2", StepIndex: 0, HTML: "<html>other</html>", ScreenshotPath: "/tmp/s2.png", URL: "https://other.com", CapturedAt: time.Now()},
	}

	for _, s := range snapshots {
		if err := db.CreateDOMSnapshot(context.Background(), s); err != nil {
			t.Fatalf("CreateDOMSnapshot %s: %v", s.ID, err)
		}
	}

	got, err := db.ListDOMSnapshots(context.Background(), "flow-1")
	if err != nil {
		t.Fatalf("ListDOMSnapshots: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("snapshot count for flow-1: got %d, want 2", len(got))
	}

	if got[0].StepIndex != 0 {
		t.Errorf("first snapshot StepIndex: got %d, want 0", got[0].StepIndex)
	}
	if got[1].StepIndex != 1 {
		t.Errorf("second snapshot StepIndex: got %d, want 1", got[1].StepIndex)
	}
}

func TestListDOMSnapshotsEmpty(t *testing.T) {
	db := setupTestDB(t)
	got, err := db.ListDOMSnapshots(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("ListDOMSnapshots: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(got))
	}
}

// --- Batch Groups Tests ---

func TestCreateBatchGroupAndProgress(t *testing.T) {
	db := setupTestDB(t)

	group := models.BatchGroup{
		ID:     "batch-1",
		FlowID: "flow-1",
		Name:   "Test Batch",
		Total:  3,
	}
	if err := db.CreateBatchGroup(context.Background(), group); err != nil {
		t.Fatalf("CreateBatchGroup: %v", err)
	}

	for i := 0; i < 3; i++ {
		task := makeTask(fmt.Sprintf("bt-%d", i), fmt.Sprintf("Batch Task %d", i))
		task.BatchID = "batch-1"
		if i == 0 {
			task.Status = models.TaskStatusCompleted
		} else if i == 1 {
			task.Status = models.TaskStatusFailed
		}
		if err := db.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}
	}

	progress, err := db.GetBatchProgress(context.Background(), "batch-1")
	if err != nil {
		t.Fatalf("GetBatchProgress: %v", err)
	}
	if progress.BatchID != "batch-1" {
		t.Errorf("BatchID: got %q, want %q", progress.BatchID, "batch-1")
	}
	if progress.Total != 3 {
		t.Errorf("Total: got %d, want 3", progress.Total)
	}
	if progress.Completed != 1 {
		t.Errorf("Completed: got %d, want 1", progress.Completed)
	}
	if progress.Failed != 1 {
		t.Errorf("Failed: got %d, want 1", progress.Failed)
	}
	if progress.Pending != 1 {
		t.Errorf("Pending: got %d, want 1", progress.Pending)
	}
}

func TestGetBatchProgressEmpty(t *testing.T) {
	db := setupTestDB(t)
	progress, err := db.GetBatchProgress(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("GetBatchProgress: %v", err)
	}
	if progress.Total != 0 {
		t.Errorf("Total: got %d, want 0", progress.Total)
	}
}

// --- ListTasksByBatch Tests ---

func TestListTasksByBatch(t *testing.T) {
	db := setupTestDB(t)

	for i := 0; i < 3; i++ {
		task := makeTask(fmt.Sprintf("batch-task-%d", i), fmt.Sprintf("Task %d", i))
		task.BatchID = "batch-x"
		if err := db.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}
	}

	unbatched := makeTask("unbatched-1", "Unbatched")
	if err := db.CreateTask(context.Background(), unbatched); err != nil {
		t.Fatalf("CreateTask unbatched: %v", err)
	}

	tasks, err := db.ListTasksByBatch(context.Background(), "batch-x")
	if err != nil {
		t.Fatalf("ListTasksByBatch: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("batch task count: got %d, want 3", len(tasks))
	}
}

func TestListTasksByBatchEmpty(t *testing.T) {
	db := setupTestDB(t)
	tasks, err := db.ListTasksByBatch(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("ListTasksByBatch: %v", err)
	}
	if tasks != nil && len(tasks) != 0 {
		t.Errorf("expected no tasks, got %d", len(tasks))
	}
}

// --- ListTasksByBatchStatus Tests ---

func TestListTasksByBatchStatus(t *testing.T) {
	db := setupTestDB(t)

	for i := 0; i < 4; i++ {
		task := makeTask(fmt.Sprintf("bs-task-%d", i), fmt.Sprintf("Task %d", i))
		task.BatchID = "batch-status"
		if i < 2 {
			task.Status = models.TaskStatusCompleted
		} else {
			task.Status = models.TaskStatusFailed
		}
		if err := db.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}
	}

	completed, err := db.ListTasksByBatchStatus(context.Background(), "batch-status", models.TaskStatusCompleted)
	if err != nil {
		t.Fatalf("ListTasksByBatchStatus: %v", err)
	}
	if len(completed) != 2 {
		t.Errorf("completed count: got %d, want 2", len(completed))
	}

	failed, err := db.ListTasksByBatchStatus(context.Background(), "batch-status", models.TaskStatusFailed)
	if err != nil {
		t.Fatalf("ListTasksByBatchStatus: %v", err)
	}
	if len(failed) != 2 {
		t.Errorf("failed count: got %d, want 2", len(failed))
	}
}

// --- Step Logs Tests ---

func TestInsertAndListStepLogs(t *testing.T) {
	db := setupTestDB(t)

	task := makeTask("step-log-task", "Step Log Task")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	logs := []models.StepLog{
		{TaskID: "step-log-task", StepIndex: 0, Action: models.ActionNavigate, Value: "https://example.com", DurationMs: 100, StartedAt: time.Now()},
		{TaskID: "step-log-task", StepIndex: 1, Action: models.ActionClick, Selector: "#btn", DurationMs: 50, StartedAt: time.Now()},
		{TaskID: "step-log-task", StepIndex: 2, Action: models.ActionType, Selector: "#input", Value: "hello", ErrorCode: "TIMEOUT", ErrorMsg: "timed out", DurationMs: 5000, StartedAt: time.Now()},
	}

	if err := db.InsertStepLogs(context.Background(), "step-log-task", logs); err != nil {
		t.Fatalf("InsertStepLogs: %v", err)
	}

	got, err := db.ListStepLogs(context.Background(), "step-log-task")
	if err != nil {
		t.Fatalf("ListStepLogs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("step log count: got %d, want 3", len(got))
	}

	if got[0].Action != models.ActionNavigate {
		t.Errorf("log[0].Action: got %q, want %q", got[0].Action, models.ActionNavigate)
	}
	if got[2].ErrorCode != "TIMEOUT" {
		t.Errorf("log[2].ErrorCode: got %q, want TIMEOUT", got[2].ErrorCode)
	}
	if got[2].ErrorMsg != "timed out" {
		t.Errorf("log[2].ErrorMsg: got %q, want %q", got[2].ErrorMsg, "timed out")
	}
}

func TestInsertStepLogsEmpty(t *testing.T) {
	db := setupTestDB(t)
	err := db.InsertStepLogs(context.Background(), "task-1", nil)
	if err != nil {
		t.Errorf("InsertStepLogs(nil) should return nil, got: %v", err)
	}

	err = db.InsertStepLogs(context.Background(), "task-1", []models.StepLog{})
	if err != nil {
		t.Errorf("InsertStepLogs(empty) should return nil, got: %v", err)
	}
}

func TestListStepLogsEmpty(t *testing.T) {
	db := setupTestDB(t)
	got, err := db.ListStepLogs(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("ListStepLogs: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 logs, got %d", len(got))
	}
}

// --- Network Logs Tests ---

func TestInsertAndListNetworkLogs(t *testing.T) {
	db := setupTestDB(t)

	task := makeTask("net-log-task", "Network Log Task")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	logs := []models.NetworkLog{
		{TaskID: "net-log-task", StepIndex: 0, RequestURL: "https://example.com", Method: "GET", StatusCode: 200, MimeType: "text/html", DurationMs: 150, Timestamp: time.Now()},
		{TaskID: "net-log-task", StepIndex: 0, RequestURL: "https://cdn.example.com/style.css", Method: "GET", StatusCode: 200, MimeType: "text/css", ResponseSize: 1024, DurationMs: 50, Timestamp: time.Now()},
		{TaskID: "net-log-task", StepIndex: 1, RequestURL: "https://api.example.com/data", Method: "POST", StatusCode: 500, Error: "server error", DurationMs: 300, Timestamp: time.Now()},
	}

	if err := db.InsertNetworkLogs(context.Background(), "net-log-task", logs); err != nil {
		t.Fatalf("InsertNetworkLogs: %v", err)
	}

	got, err := db.ListNetworkLogs(context.Background(), "net-log-task")
	if err != nil {
		t.Fatalf("ListNetworkLogs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("network log count: got %d, want 3", len(got))
	}

	if got[0].Method != "GET" {
		t.Errorf("log[0].Method: got %q, want GET", got[0].Method)
	}
	if got[0].StatusCode != 200 {
		t.Errorf("log[0].StatusCode: got %d, want 200", got[0].StatusCode)
	}
	if got[2].Error != "server error" {
		t.Errorf("log[2].Error: got %q, want %q", got[2].Error, "server error")
	}
}

func TestInsertNetworkLogsEmpty(t *testing.T) {
	db := setupTestDB(t)
	err := db.InsertNetworkLogs(context.Background(), "task-1", nil)
	if err != nil {
		t.Errorf("InsertNetworkLogs(nil) should return nil, got: %v", err)
	}
}

func TestListNetworkLogsEmpty(t *testing.T) {
	db := setupTestDB(t)
	got, err := db.ListNetworkLogs(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("ListNetworkLogs: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 logs, got %d", len(got))
	}
}

// --- Task Events Tests ---

func TestInsertAndListTaskEvents(t *testing.T) {
	db := setupTestDB(t)

	events := []models.TaskLifecycleEvent{
		{ID: "ev-1", TaskID: "task-1", BatchID: "batch-1", FromState: models.TaskStatusPending, ToState: models.TaskStatusQueued, Timestamp: time.Now()},
		{ID: "ev-2", TaskID: "task-1", BatchID: "batch-1", FromState: models.TaskStatusQueued, ToState: models.TaskStatusRunning, Timestamp: time.Now()},
		{ID: "ev-3", TaskID: "task-1", BatchID: "batch-1", FromState: models.TaskStatusRunning, ToState: models.TaskStatusCompleted, Timestamp: time.Now()},
	}

	for _, ev := range events {
		if err := db.InsertTaskEvent(context.Background(), ev); err != nil {
			t.Fatalf("InsertTaskEvent %s: %v", ev.ID, err)
		}
	}

	got, err := db.ListTaskEvents(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("ListTaskEvents: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("event count: got %d, want 3", len(got))
	}

	if got[0].FromState != models.TaskStatusPending {
		t.Errorf("event[0].FromState: got %q, want %q", got[0].FromState, models.TaskStatusPending)
	}
	if got[2].ToState != models.TaskStatusCompleted {
		t.Errorf("event[2].ToState: got %q, want %q", got[2].ToState, models.TaskStatusCompleted)
	}
}

func TestListTaskEventsEmpty(t *testing.T) {
	db := setupTestDB(t)
	got, err := db.ListTaskEvents(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("ListTaskEvents: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 events, got %d", len(got))
	}
}

// --- ListAuditTrail with limit Tests ---

func TestListAuditTrailWithLimit(t *testing.T) {
	db := setupTestDB(t)

	for i := 0; i < 10; i++ {
		ev := models.TaskLifecycleEvent{
			ID:        fmt.Sprintf("audit-ev-%d", i),
			TaskID:    "task-1",
			FromState: models.TaskStatusPending,
			ToState:   models.TaskStatusRunning,
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := db.InsertTaskEvent(context.Background(), ev); err != nil {
			t.Fatalf("InsertTaskEvent %d: %v", i, err)
		}
	}

	got, err := db.ListAuditTrail(context.Background(), "", 5)
	if err != nil {
		t.Fatalf("ListAuditTrail: %v", err)
	}
	if len(got) != 5 {
		t.Errorf("limited audit trail: got %d, want 5", len(got))
	}
}

// --- ListTasksPaginated edge cases ---

func TestListTasksPaginatedInvalidPage(t *testing.T) {
	db := setupTestDB(t)

	task := makeTask("page-edge-1", "Task")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	result, err := db.ListTasksPaginated(context.Background(), 0, 10, "", "")
	if err != nil {
		t.Fatalf("ListTasksPaginated with page 0: %v", err)
	}
	if result.Page != 1 {
		t.Errorf("Page: got %d, want 1 (corrected from 0)", result.Page)
	}
}

func TestListTasksPaginatedInvalidPageSize(t *testing.T) {
	db := setupTestDB(t)

	task := makeTask("page-size-1", "Task")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	result, err := db.ListTasksPaginated(context.Background(), 1, 0, "", "")
	if err != nil {
		t.Fatalf("ListTasksPaginated with pageSize 0: %v", err)
	}
	if result.PageSize != 50 {
		t.Errorf("PageSize: got %d, want 50 (default)", result.PageSize)
	}

	result2, err := db.ListTasksPaginated(context.Background(), 1, 999, "", "")
	if err != nil {
		t.Fatalf("ListTasksPaginated with pageSize 999: %v", err)
	}
	if result2.PageSize != 50 {
		t.Errorf("PageSize: got %d, want 50 (clamped)", result2.PageSize)
	}
}

func TestListTasksPaginatedWithTagFilter(t *testing.T) {
	db := setupTestDB(t)

	task1 := makeTask("tag-filter-1", "With Tag")
	task1.Tags = []string{"web", "automation"}
	task2 := makeTask("tag-filter-2", "Without Tag")
	task2.Tags = []string{"other"}

	if err := db.CreateTask(context.Background(), task1); err != nil {
		t.Fatalf("CreateTask 1: %v", err)
	}
	if err := db.CreateTask(context.Background(), task2); err != nil {
		t.Fatalf("CreateTask 2: %v", err)
	}

	result, err := db.ListTasksPaginated(context.Background(), 1, 10, "", "web")
	if err != nil {
		t.Fatalf("ListTasksPaginated with tag: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Total with tag filter: got %d, want 1", result.Total)
	}
}

func TestListTasksPaginatedWithStatusAll(t *testing.T) {
	db := setupTestDB(t)

	task1 := makeTask("all-1", "Pending")
	task1.Status = models.TaskStatusPending
	task2 := makeTask("all-2", "Completed")
	task2.Status = models.TaskStatusCompleted

	if err := db.CreateTask(context.Background(), task1); err != nil {
		t.Fatalf("CreateTask 1: %v", err)
	}
	if err := db.CreateTask(context.Background(), task2); err != nil {
		t.Fatalf("CreateTask 2: %v", err)
	}

	result, err := db.ListTasksPaginated(context.Background(), 1, 10, "all", "")
	if err != nil {
		t.Fatalf("ListTasksPaginated with status=all: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("Total with status=all: got %d, want 2", result.Total)
	}
}

// --- UpdateTaskStatus Not Found ---

func TestUpdateTaskStatusNotFound(t *testing.T) {
	db := setupTestDB(t)
	err := db.UpdateTaskStatus(context.Background(), "nonexistent", models.TaskStatusRunning, "")
	if err == nil {
		t.Fatal("expected error for nonexistent task status update")
	}
}

// --- UpdateTaskResult Not Found ---

func TestUpdateTaskResultNotFound(t *testing.T) {
	db := setupTestDB(t)
	result := models.TaskResult{TaskID: "x", Success: true}
	err := db.UpdateTaskResult(context.Background(), "nonexistent", result)
	if err == nil {
		t.Fatal("expected error for nonexistent task result update")
	}
}

// --- IncrementRetry Not Found ---

func TestResetRetryCount(t *testing.T) {
	db := setupTestDB(t)
	task := makeTask("reset-retry-1", "Reset Retry")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := db.IncrementRetry(context.Background(), task.ID); err != nil {
			t.Fatalf("IncrementRetry: %v", err)
		}
	}

	if err := db.ResetRetryCount(context.Background(), task.ID); err != nil {
		t.Fatalf("ResetRetryCount: %v", err)
	}

	got, err := db.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.RetryCount != 0 {
		t.Errorf("RetryCount: got %d, want 0", got.RetryCount)
	}
}

func TestResetRetryCountNotFound(t *testing.T) {
	db := setupTestDB(t)
	err := db.ResetRetryCount(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestIncrementRetryNotFound(t *testing.T) {
	db := setupTestDB(t)
	err := db.IncrementRetry(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent task retry increment")
	}
}

// --- UpdateProxyHealth Not Found ---

func TestUpdateProxyHealthNotFound(t *testing.T) {
	db := setupTestDB(t)
	err := db.UpdateProxyHealth(context.Background(), "nonexistent", models.ProxyStatusHealthy, 100)
	if err == nil {
		t.Fatal("expected error for nonexistent proxy health update")
	}
}

// --- UpdateTask Not Found ---

func TestUpdateTaskNotFound(t *testing.T) {
	db := setupTestDB(t)
	err := db.UpdateTask(context.Background(), "nonexistent", "Name", "https://example.com", nil, models.ProxyConfig{}, models.PriorityNormal, nil, 0, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent task update")
	}
}

// --- Headless field Tests ---

func TestTaskHeadlessField(t *testing.T) {
	db := setupTestDB(t)

	task := makeTask("headless-1", "Headless Task")
	task.Headless = true
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask headless=true: %v", err)
	}

	got, err := db.GetTask(context.Background(), "headless-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if !got.Headless {
		t.Error("Headless should be true")
	}

	task2 := makeTask("headless-2", "Non-Headless Task")
	task2.Headless = false
	if err := db.CreateTask(context.Background(), task2); err != nil {
		t.Fatalf("CreateTask headless=false: %v", err)
	}

	got2, err := db.GetTask(context.Background(), "headless-2")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got2.Headless {
		t.Error("Headless should be false")
	}
}

// --- PurgeOldRecords comprehensive test ---

// --- UpdateTask on Failed Task (allowed) ---

func TestUpdateTaskOnFailedTask(t *testing.T) {
	db := setupTestDB(t)
	task := makeTask("upd-failed-1", "Failed Task")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := db.UpdateTaskStatus(context.Background(), "upd-failed-1", models.TaskStatusFailed, "some error"); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}

	err := db.UpdateTask(context.Background(), "upd-failed-1", "Retried", "https://example.com", []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://example.com"},
	}, models.ProxyConfig{}, models.PriorityNormal, nil, 0, nil)
	if err != nil {
		t.Fatalf("UpdateTask on failed task should succeed: %v", err)
	}

	got, err := db.GetTask(context.Background(), "upd-failed-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Name != "Retried" {
		t.Errorf("Name: got %q, want %q", got.Name, "Retried")
	}
}

// --- GetTaskStats empty ---

func TestGetTaskStatsEmpty(t *testing.T) {
	db := setupTestDB(t)
	stats, err := db.GetTaskStats(context.Background())
	if err != nil {
		t.Fatalf("GetTaskStats: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected empty stats, got %v", stats)
	}
}

// --- Create and Get RecordedFlow with empty steps ---

func TestCreateRecordedFlowEmptySteps(t *testing.T) {
	db := setupTestDB(t)
	flow := models.RecordedFlow{
		ID:        "flow-empty-steps",
		Name:      "Empty Steps Flow",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := db.CreateRecordedFlow(context.Background(), flow); err != nil {
		t.Fatalf("CreateRecordedFlow: %v", err)
	}

	got, err := db.GetRecordedFlow(context.Background(), "flow-empty-steps")
	if err != nil {
		t.Fatalf("GetRecordedFlow: %v", err)
	}
	if got.Steps != nil && len(got.Steps) != 0 {
		t.Errorf("Steps should be nil or empty, got %v", got.Steps)
	}
}

// --- Close idempotent ---

func TestCloseIdempotent(t *testing.T) {
	dir := t.TempDir()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	crypto.ResetForTest()
	if err := crypto.InitKeyWithBytes(key); err != nil {
		t.Fatalf("init crypto: %v", err)
	}

	db, err := New(filepath.Join(dir, "close-test.db"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	crypto.ResetForTest()
}

// --- CreateTask with empty proxy ---

func TestCreateTaskWithEmptyProxy(t *testing.T) {
	db := setupTestDB(t)
	task := models.Task{
		ID:        "empty-proxy",
		Name:      "Empty Proxy",
		URL:       "https://example.com",
		Status:    models.TaskStatusPending,
		CreatedAt: time.Now(),
	}

	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	got, err := db.GetTask(context.Background(), "empty-proxy")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Proxy.Server != "" {
		t.Errorf("Proxy.Server should be empty, got %q", got.Proxy.Server)
	}
	if got.Proxy.Username != "" {
		t.Errorf("Proxy.Username should be empty, got %q", got.Proxy.Username)
	}
}

// --- BatchProgress with all statuses ---

func TestBatchProgressAllStatuses(t *testing.T) {
	db := setupTestDB(t)

	statuses := []models.TaskStatus{
		models.TaskStatusPending,
		models.TaskStatusQueued,
		models.TaskStatusRunning,
		models.TaskStatusCompleted,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
		models.TaskStatusRetrying,
	}

	for i, status := range statuses {
		task := makeTask(fmt.Sprintf("bp-all-%d", i), fmt.Sprintf("Task %d", i))
		task.BatchID = "batch-all-statuses"
		task.Status = status
		if err := db.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}
	}

	progress, err := db.GetBatchProgress(context.Background(), "batch-all-statuses")
	if err != nil {
		t.Fatalf("GetBatchProgress: %v", err)
	}

	if progress.Total != 7 {
		t.Errorf("Total: got %d, want 7", progress.Total)
	}
	if progress.Pending != 1 {
		t.Errorf("Pending: got %d, want 1", progress.Pending)
	}
	if progress.Queued != 1 {
		t.Errorf("Queued: got %d, want 1", progress.Queued)
	}
	if progress.Running != 1 {
		t.Errorf("Running: got %d, want 1", progress.Running)
	}
	if progress.Completed != 1 {
		t.Errorf("Completed: got %d, want 1", progress.Completed)
	}
	if progress.Failed != 1 {
		t.Errorf("Failed: got %d, want 1", progress.Failed)
	}
	if progress.Cancelled != 1 {
		t.Errorf("Cancelled: got %d, want 1", progress.Cancelled)
	}
}

// --- CreateTask with result ---

func TestCreateAndGetTaskWithResult(t *testing.T) {
	db := setupTestDB(t)
	task := makeTask("result-round-trip", "Result Test")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	result := models.TaskResult{
		TaskID:  task.ID,
		Success: true,
		ExtractedData: map[string]string{
			"title": "Test Page",
			"h1":    "Welcome",
		},
		Screenshots: []string{"/tmp/s1.png", "/tmp/s2.png"},
		Duration:    3 * time.Second,
	}

	if err := db.UpdateTaskResult(context.Background(), task.ID, result); err != nil {
		t.Fatalf("UpdateTaskResult: %v", err)
	}

	got, err := db.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Result == nil {
		t.Fatal("Result should be set")
	}
	if len(got.Result.ExtractedData) != 2 {
		t.Errorf("ExtractedData count: got %d, want 2", len(got.Result.ExtractedData))
	}
	if len(got.Result.Screenshots) != 2 {
		t.Errorf("Screenshots count: got %d, want 2", len(got.Result.Screenshots))
	}
}

// --- ListTasks empty ---

func TestListTasksEmpty(t *testing.T) {
	db := setupTestDB(t)
	tasks, err := db.ListTasks(context.Background())
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if tasks != nil && len(tasks) != 0 {
		t.Errorf("expected no tasks, got %d", len(tasks))
	}
}

// --- CreateProxy with credentials ---

func TestCreateProxyWithCredentials(t *testing.T) {
	db := setupTestDB(t)
	p := models.Proxy{
		ID:        "cred-proxy",
		Server:    "cred.proxy.com:8080",
		Protocol:  models.ProxyHTTPS,
		Username:  "admin",
		Password:  "secret123",
		Geo:       "DE",
		Status:    models.ProxyStatusUnknown,
		CreatedAt: time.Now(),
	}

	if err := db.CreateProxy(context.Background(), p); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}

	proxies, err := db.ListProxies(context.Background())
	if err != nil {
		t.Fatalf("ListProxies: %v", err)
	}

	var found *models.Proxy
	for i := range proxies {
		if proxies[i].ID == "cred-proxy" {
			found = &proxies[i]
			break
		}
	}
	if found == nil {
		t.Fatal("proxy not found")
	}
	if found.Username != "admin" {
		t.Errorf("Username: got %q, want admin", found.Username)
	}
	if found.Password != "secret123" {
		t.Errorf("Password: got %q, want secret123", found.Password)
	}
	if found.Protocol != models.ProxyHTTPS {
		t.Errorf("Protocol: got %q, want %q", found.Protocol, models.ProxyHTTPS)
	}
	if found.Geo != "DE" {
		t.Errorf("Geo: got %q, want DE", found.Geo)
	}
}

// --- CreateProxy no credentials ---

func TestCreateProxyWithoutCredentials(t *testing.T) {
	db := setupTestDB(t)
	p := models.Proxy{
		ID:        "no-cred-proxy",
		Server:    "nocred.proxy.com:3128",
		Protocol:  models.ProxySOCKS5,
		Status:    models.ProxyStatusUnknown,
		CreatedAt: time.Now(),
	}

	if err := db.CreateProxy(context.Background(), p); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}

	proxies, err := db.ListProxies(context.Background())
	if err != nil {
		t.Fatalf("ListProxies: %v", err)
	}

	for _, px := range proxies {
		if px.ID == "no-cred-proxy" {
			if px.Username != "" {
				t.Errorf("Username should be empty, got %q", px.Username)
			}
			if px.Password != "" {
				t.Errorf("Password should be empty, got %q", px.Password)
			}
			return
		}
	}
	t.Error("proxy not found")
}

// --- Multiple step log inserts ---

func TestInsertStepLogsMultiple(t *testing.T) {
	db := setupTestDB(t)

	task := makeTask("sl-multi-1", "Step Log Multi Task")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	logs := make([]models.StepLog, 10)
	for i := range logs {
		logs[i] = models.StepLog{
			TaskID:     "sl-multi-1",
			StepIndex:  i,
			Action:     models.ActionClick,
			Selector:   "#btn",
			SnapshotID: fmt.Sprintf("snap-%d", i),
			DurationMs: int64(i * 10),
			StartedAt:  time.Now(),
		}
	}

	if err := db.InsertStepLogs(context.Background(), "sl-multi-1", logs); err != nil {
		t.Fatalf("InsertStepLogs: %v", err)
	}

	got, err := db.ListStepLogs(context.Background(), "sl-multi-1")
	if err != nil {
		t.Fatalf("ListStepLogs: %v", err)
	}
	if len(got) != 10 {
		t.Errorf("expected 10 step logs, got %d", len(got))
	}

	for i, log := range got {
		if log.StepIndex != i {
			t.Errorf("log[%d].StepIndex: got %d, want %d", i, log.StepIndex, i)
		}
	}
}

// --- Multiple network log inserts ---

func TestInsertNetworkLogsMultiple(t *testing.T) {
	db := setupTestDB(t)

	task := makeTask("nl-multi-1", "Network Log Multi Task")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	logs := make([]models.NetworkLog, 5)
	for i := range logs {
		logs[i] = models.NetworkLog{
			TaskID:          "nl-multi-1",
			StepIndex:       i,
			RequestURL:      fmt.Sprintf("https://example.com/page%d", i),
			Method:          "GET",
			StatusCode:      200,
			MimeType:        "text/html",
			RequestHeaders:  `{"Accept":"text/html"}`,
			ResponseHeaders: `{"Content-Type":"text/html"}`,
			RequestSize:     100,
			ResponseSize:    int64(i * 500),
			DurationMs:      int64(i * 50),
			Timestamp:       time.Now(),
		}
	}

	if err := db.InsertNetworkLogs(context.Background(), "nl-multi-1", logs); err != nil {
		t.Fatalf("InsertNetworkLogs: %v", err)
	}

	got, err := db.ListNetworkLogs(context.Background(), "nl-multi-1")
	if err != nil {
		t.Fatalf("ListNetworkLogs: %v", err)
	}
	if len(got) != 5 {
		t.Errorf("expected 5 network logs, got %d", len(got))
	}

	for i, log := range got {
		if log.RequestURL != fmt.Sprintf("https://example.com/page%d", i) {
			t.Errorf("log[%d].RequestURL: got %q", i, log.RequestURL)
		}
		if log.RequestHeaders != `{"Accept":"text/html"}` {
			t.Errorf("log[%d].RequestHeaders: got %q", i, log.RequestHeaders)
		}
	}
}

// --- Task with FlowID and BatchID ---

func TestTaskWithFlowAndBatchIDs(t *testing.T) {
	db := setupTestDB(t)
	task := makeTask("fb-1", "Flow Batch Task")
	task.FlowID = "flow-123"
	task.BatchID = "batch-456"

	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	got, err := db.GetTask(context.Background(), "fb-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.FlowID != "flow-123" {
		t.Errorf("FlowID: got %q, want %q", got.FlowID, "flow-123")
	}
	if got.BatchID != "batch-456" {
		t.Errorf("BatchID: got %q, want %q", got.BatchID, "batch-456")
	}
}

func TestPurgeOldRecordsWithOldData(t *testing.T) {
	db := setupTestDB(t)

	task := makeTask("purge-old-1", "Old Task")
	task.Status = models.TaskStatusCompleted
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	oldTime := time.Now().Add(-100 * 24 * time.Hour)
	_, err := db.conn.Exec(`UPDATE tasks SET completed_at = ?, status = 'completed' WHERE id = ?`, oldTime, "purge-old-1")
	if err != nil {
		t.Fatalf("set old completed_at: %v", err)
	}

	if err := db.InsertStepLogs(context.Background(), "purge-old-1", []models.StepLog{
		{TaskID: "purge-old-1", StepIndex: 0, Action: models.ActionClick, DurationMs: 100, StartedAt: time.Now()},
	}); err != nil {
		t.Fatalf("InsertStepLogs: %v", err)
	}

	if err := db.InsertNetworkLogs(context.Background(), "purge-old-1", []models.NetworkLog{
		{TaskID: "purge-old-1", StepIndex: 0, RequestURL: "https://example.com", Method: "GET", StatusCode: 200, Timestamp: time.Now()},
	}); err != nil {
		t.Fatalf("InsertNetworkLogs: %v", err)
	}

	if err := db.InsertTaskEvent(context.Background(), models.TaskLifecycleEvent{
		ID: "purge-evt-1", TaskID: "purge-old-1", FromState: models.TaskStatusPending, ToState: models.TaskStatusCompleted, Timestamp: time.Now(),
	}); err != nil {
		t.Fatalf("InsertTaskEvent: %v", err)
	}

	n, err := db.PurgeOldRecords(context.Background(), 90)
	if err != nil {
		t.Fatalf("PurgeOldRecords: %v", err)
	}
	if n == 0 {
		t.Error("expected some records to be purged")
	}

	_, err = db.GetTask(context.Background(), "purge-old-1")
	if err == nil {
		t.Error("task should be purged")
	}

	stepLogs, _ := db.ListStepLogs(context.Background(), "purge-old-1")
	if len(stepLogs) != 0 {
		t.Errorf("step logs should be purged, got %d", len(stepLogs))
	}

	netLogs, _ := db.ListNetworkLogs(context.Background(), "purge-old-1")
	if len(netLogs) != 0 {
		t.Errorf("network logs should be purged, got %d", len(netLogs))
	}

	events, _ := db.ListTaskEvents(context.Background(), "purge-old-1")
	if len(events) != 0 {
		t.Errorf("events should be purged, got %d", len(events))
	}
}

// --- WebSocket Logs CRUD Tests ---

func TestInsertAndListWebSocketLogs(t *testing.T) {
	db := setupTestDB(t)

	logs := []models.WebSocketLog{
		{
			FlowID:         "flow-ws-1",
			StepIndex:      0,
			RequestID:      "ws-req-1",
			URL:            "wss://example.com/ws",
			EventType:      models.WSEventCreated,
			Direction:      "",
			Opcode:         0,
			PayloadSize:    0,
			PayloadSnippet: "",
			CloseCode:      0,
			CloseReason:    "",
			ErrorMessage:   "",
			Timestamp:      time.Now().Truncate(time.Second),
		},
		{
			FlowID:         "flow-ws-1",
			StepIndex:      0,
			RequestID:      "ws-req-1",
			URL:            "wss://example.com/ws",
			EventType:      models.WSEventHandshake,
			Direction:      "",
			Opcode:         0,
			PayloadSize:    0,
			PayloadSnippet: "",
			Timestamp:      time.Now().Truncate(time.Second),
		},
		{
			FlowID:         "flow-ws-1",
			StepIndex:      1,
			RequestID:      "ws-req-1",
			URL:            "wss://example.com/ws",
			EventType:      models.WSEventFrameSent,
			Direction:      "send",
			Opcode:         1,
			PayloadSize:    12,
			PayloadSnippet: "hello server",
			Timestamp:      time.Now().Truncate(time.Second),
		},
		{
			FlowID:         "flow-ws-1",
			StepIndex:      1,
			RequestID:      "ws-req-1",
			URL:            "wss://example.com/ws",
			EventType:      models.WSEventFrameReceived,
			Direction:      "receive",
			Opcode:         1,
			PayloadSize:    11,
			PayloadSnippet: "hello back",
			Timestamp:      time.Now().Truncate(time.Second),
		},
		{
			FlowID:      "flow-ws-1",
			StepIndex:   2,
			RequestID:   "ws-req-1",
			URL:         "wss://example.com/ws",
			EventType:   models.WSEventClosed,
			CloseCode:   1000,
			CloseReason: "normal closure",
			Timestamp:   time.Now().Truncate(time.Second),
		},
	}

	if err := db.InsertWebSocketLogs(context.Background(), "flow-ws-1", logs); err != nil {
		t.Fatalf("InsertWebSocketLogs: %v", err)
	}

	got, err := db.ListWebSocketLogs(context.Background(), "flow-ws-1")
	if err != nil {
		t.Fatalf("ListWebSocketLogs: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("expected 5 websocket logs, got %d", len(got))
	}

	if got[0].FlowID != "flow-ws-1" {
		t.Errorf("log[0].FlowID: got %q", got[0].FlowID)
	}
	if got[0].EventType != models.WSEventCreated {
		t.Errorf("log[0].EventType: got %q, want %q", got[0].EventType, models.WSEventCreated)
	}
	if got[0].URL != "wss://example.com/ws" {
		t.Errorf("log[0].URL: got %q", got[0].URL)
	}
	if got[0].RequestID != "ws-req-1" {
		t.Errorf("log[0].RequestID: got %q", got[0].RequestID)
	}

	if got[2].Direction != "send" {
		t.Errorf("log[2].Direction: got %q, want send", got[2].Direction)
	}
	if got[2].Opcode != 1 {
		t.Errorf("log[2].Opcode: got %d, want 1", got[2].Opcode)
	}
	if got[2].PayloadSize != 12 {
		t.Errorf("log[2].PayloadSize: got %d, want 12", got[2].PayloadSize)
	}
	if got[2].PayloadSnippet != "hello server" {
		t.Errorf("log[2].PayloadSnippet: got %q", got[2].PayloadSnippet)
	}

	if got[3].Direction != "receive" {
		t.Errorf("log[3].Direction: got %q, want receive", got[3].Direction)
	}

	if got[4].CloseCode != 1000 {
		t.Errorf("log[4].CloseCode: got %d, want 1000", got[4].CloseCode)
	}
	if got[4].CloseReason != "normal closure" {
		t.Errorf("log[4].CloseReason: got %q", got[4].CloseReason)
	}
}

func TestInsertWebSocketLogsEmpty(t *testing.T) {
	db := setupTestDB(t)

	if err := db.InsertWebSocketLogs(context.Background(), "flow-ws-empty", nil); err != nil {
		t.Fatalf("InsertWebSocketLogs(nil): %v", err)
	}
	if err := db.InsertWebSocketLogs(context.Background(), "flow-ws-empty", []models.WebSocketLog{}); err != nil {
		t.Fatalf("InsertWebSocketLogs(empty): %v", err)
	}
}

func TestListWebSocketLogsNoResults(t *testing.T) {
	db := setupTestDB(t)

	got, err := db.ListWebSocketLogs(context.Background(), "nonexistent-flow")
	if err != nil {
		t.Fatalf("ListWebSocketLogs: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 results, got %d", len(got))
	}
}

func TestInsertWebSocketLogsErrorEvent(t *testing.T) {
	db := setupTestDB(t)

	logs := []models.WebSocketLog{
		{
			FlowID:       "flow-ws-err",
			StepIndex:    0,
			RequestID:    "ws-err-1",
			URL:          "wss://example.com/ws",
			EventType:    models.WSEventError,
			ErrorMessage: "connection reset by peer",
			Timestamp:    time.Now().Truncate(time.Second),
		},
	}

	if err := db.InsertWebSocketLogs(context.Background(), "flow-ws-err", logs); err != nil {
		t.Fatalf("InsertWebSocketLogs: %v", err)
	}

	got, err := db.ListWebSocketLogs(context.Background(), "flow-ws-err")
	if err != nil {
		t.Fatalf("ListWebSocketLogs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 log, got %d", len(got))
	}
	if got[0].ErrorMessage != "connection reset by peer" {
		t.Errorf("ErrorMessage: got %q", got[0].ErrorMessage)
	}
	if got[0].EventType != models.WSEventError {
		t.Errorf("EventType: got %q, want %q", got[0].EventType, models.WSEventError)
	}
}

func TestWebSocketLogsIsolatedByFlowID(t *testing.T) {
	db := setupTestDB(t)

	logs1 := []models.WebSocketLog{
		{FlowID: "flow-a", StepIndex: 0, RequestID: "r1", EventType: models.WSEventCreated, Timestamp: time.Now()},
	}
	logs2 := []models.WebSocketLog{
		{FlowID: "flow-b", StepIndex: 0, RequestID: "r2", EventType: models.WSEventCreated, Timestamp: time.Now()},
		{FlowID: "flow-b", StepIndex: 1, RequestID: "r2", EventType: models.WSEventClosed, Timestamp: time.Now()},
	}

	if err := db.InsertWebSocketLogs(context.Background(), "flow-a", logs1); err != nil {
		t.Fatalf("InsertWebSocketLogs flow-a: %v", err)
	}
	if err := db.InsertWebSocketLogs(context.Background(), "flow-b", logs2); err != nil {
		t.Fatalf("InsertWebSocketLogs flow-b: %v", err)
	}

	gotA, err := db.ListWebSocketLogs(context.Background(), "flow-a")
	if err != nil {
		t.Fatalf("ListWebSocketLogs flow-a: %v", err)
	}
	if len(gotA) != 1 {
		t.Errorf("flow-a: expected 1 log, got %d", len(gotA))
	}

	gotB, err := db.ListWebSocketLogs(context.Background(), "flow-b")
	if err != nil {
		t.Fatalf("ListWebSocketLogs flow-b: %v", err)
	}
	if len(gotB) != 2 {
		t.Errorf("flow-b: expected 2 logs, got %d", len(gotB))
	}
}

// --- Additional Insert/List edge case tests ---

func TestInsertStepLogsEmptySlice(t *testing.T) {
	db := setupTestDB(t)
	if err := db.InsertStepLogs(context.Background(), "task-empty-steps", nil); err != nil {
		t.Fatalf("InsertStepLogs(nil): %v", err)
	}
	if err := db.InsertStepLogs(context.Background(), "task-empty-steps", []models.StepLog{}); err != nil {
		t.Fatalf("InsertStepLogs(empty): %v", err)
	}
}

func TestInsertNetworkLogsEmptySlice(t *testing.T) {
	db := setupTestDB(t)
	if err := db.InsertNetworkLogs(context.Background(), "task-empty-net", nil); err != nil {
		t.Fatalf("InsertNetworkLogs(nil): %v", err)
	}
	if err := db.InsertNetworkLogs(context.Background(), "task-empty-net", []models.NetworkLog{}); err != nil {
		t.Fatalf("InsertNetworkLogs(empty): %v", err)
	}
}

func TestListStepLogsNoResults(t *testing.T) {
	db := setupTestDB(t)
	logs, err := db.ListStepLogs(context.Background(), "nonexistent-task")
	if err != nil {
		t.Fatalf("ListStepLogs: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected 0, got %d", len(logs))
	}
}

func TestListNetworkLogsNoResults(t *testing.T) {
	db := setupTestDB(t)
	logs, err := db.ListNetworkLogs(context.Background(), "nonexistent-task")
	if err != nil {
		t.Fatalf("ListNetworkLogs: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected 0, got %d", len(logs))
	}
}

func TestInsertAndListStepLogsRoundTrip(t *testing.T) {
	db := setupTestDB(t)

	task := makeTask("sl-round-1", "Step Log Round Trip")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	logs := []models.StepLog{
		{TaskID: "sl-round-1", StepIndex: 0, Action: models.ActionNavigate, Value: "https://example.com", DurationMs: 150, StartedAt: time.Now().Truncate(time.Second)},
		{TaskID: "sl-round-1", StepIndex: 1, Action: models.ActionClick, Selector: "#btn", SnapshotID: "snap-1", ErrorCode: "TIMEOUT", ErrorMsg: "timed out", DurationMs: 200, StartedAt: time.Now().Truncate(time.Second)},
	}

	if err := db.InsertStepLogs(context.Background(), "sl-round-1", logs); err != nil {
		t.Fatalf("InsertStepLogs: %v", err)
	}

	got, err := db.ListStepLogs(context.Background(), "sl-round-1")
	if err != nil {
		t.Fatalf("ListStepLogs: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(got))
	}
	if got[0].Action != models.ActionNavigate {
		t.Errorf("log[0].Action: got %q", got[0].Action)
	}
	if got[0].Value != "https://example.com" {
		t.Errorf("log[0].Value: got %q", got[0].Value)
	}
	if got[1].Selector != "#btn" {
		t.Errorf("log[1].Selector: got %q", got[1].Selector)
	}
	if got[1].SnapshotID != "snap-1" {
		t.Errorf("log[1].SnapshotID: got %q", got[1].SnapshotID)
	}
	if got[1].ErrorCode != "TIMEOUT" {
		t.Errorf("log[1].ErrorCode: got %q", got[1].ErrorCode)
	}
	if got[1].ErrorMsg != "timed out" {
		t.Errorf("log[1].ErrorMsg: got %q", got[1].ErrorMsg)
	}
}

func TestInsertAndListNetworkLogsRoundTrip(t *testing.T) {
	db := setupTestDB(t)

	task := makeTask("nl-round-1", "Net Log Round Trip")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	logs := []models.NetworkLog{
		{
			TaskID:          "nl-round-1",
			StepIndex:       0,
			RequestURL:      "https://example.com/api",
			Method:          "POST",
			StatusCode:      201,
			MimeType:        "application/json",
			RequestHeaders:  `{"Content-Type":"application/json"}`,
			ResponseHeaders: `{"X-Request-Id":"abc"}`,
			RequestSize:     256,
			ResponseSize:    1024,
			DurationMs:      350,
			Error:           "",
			Timestamp:       time.Now().Truncate(time.Second),
		},
	}

	if err := db.InsertNetworkLogs(context.Background(), "nl-round-1", logs); err != nil {
		t.Fatalf("InsertNetworkLogs: %v", err)
	}

	got, err := db.ListNetworkLogs(context.Background(), "nl-round-1")
	if err != nil {
		t.Fatalf("ListNetworkLogs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	if got[0].Method != "POST" {
		t.Errorf("Method: got %q", got[0].Method)
	}
	if got[0].StatusCode != 201 {
		t.Errorf("StatusCode: got %d", got[0].StatusCode)
	}
	if got[0].MimeType != "application/json" {
		t.Errorf("MimeType: got %q", got[0].MimeType)
	}
	if got[0].RequestSize != 256 {
		t.Errorf("RequestSize: got %d", got[0].RequestSize)
	}
	if got[0].ResponseSize != 1024 {
		t.Errorf("ResponseSize: got %d", got[0].ResponseSize)
	}
}

// --- Phase 2.5: JSON corruption error paths and additional coverage ---

func TestScanTaskCorruptedStepsJSON(t *testing.T) {
	db := setupTestDB(t)

	task := models.Task{
		ID:     "corrupt-steps-1",
		Name:   "test",
		URL:    "https://example.com",
		Status: "pending",
		Steps:  []models.TaskStep{{Action: models.ActionClick, Selector: "#btn"}},
		Tags:   []string{"ok"},
	}
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Corrupt the steps JSON directly via SQL
	if _, err := db.conn.Exec(`UPDATE tasks SET steps = '<<<invalid>>>' WHERE id = ?`, "corrupt-steps-1"); err != nil {
		t.Fatalf("corrupt steps: %v", err)
	}

	_, err := db.GetTask(context.Background(), "corrupt-steps-1")
	if err == nil {
		t.Fatal("expected error from GetTask with corrupted steps JSON")
	}
}

func TestScanTaskCorruptedResultJSON(t *testing.T) {
	db := setupTestDB(t)

	task := models.Task{
		ID:     "corrupt-result-1",
		Name:   "test",
		URL:    "https://example.com",
		Status: "completed",
		Steps:  []models.TaskStep{{Action: models.ActionClick, Selector: "#btn"}},
		Tags:   []string{},
	}
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Corrupt the result JSON directly (must be non-empty to trigger parse)
	if _, err := db.conn.Exec(`UPDATE tasks SET result = '<<<invalid>>>' WHERE id = ?`, "corrupt-result-1"); err != nil {
		t.Fatalf("corrupt result: %v", err)
	}

	_, err := db.GetTask(context.Background(), "corrupt-result-1")
	if err == nil {
		t.Fatal("expected error from GetTask with corrupted result JSON")
	}
}

func TestScanTaskCorruptedTagsJSON(t *testing.T) {
	db := setupTestDB(t)

	task := models.Task{
		ID:     "corrupt-tags-1",
		Name:   "test",
		URL:    "https://example.com",
		Status: "pending",
		Steps:  []models.TaskStep{{Action: models.ActionClick, Selector: "#btn"}},
		Tags:   []string{"valid"},
	}
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if _, err := db.conn.Exec(`UPDATE tasks SET tags = '<<<invalid>>>' WHERE id = ?`, "corrupt-tags-1"); err != nil {
		t.Fatalf("corrupt tags: %v", err)
	}

	_, err := db.GetTask(context.Background(), "corrupt-tags-1")
	if err == nil {
		t.Fatal("expected error from GetTask with corrupted tags JSON")
	}
}

func TestListTasksCorruptedJSON(t *testing.T) {
	db := setupTestDB(t)

	task := models.Task{
		ID:     "corrupt-list-1",
		Name:   "test",
		URL:    "https://example.com",
		Status: "pending",
		Steps:  []models.TaskStep{{Action: models.ActionClick, Selector: "#btn"}},
		Tags:   []string{},
	}
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if _, err := db.conn.Exec(`UPDATE tasks SET steps = '<<<invalid>>>' WHERE id = ?`, "corrupt-list-1"); err != nil {
		t.Fatalf("corrupt steps: %v", err)
	}

	_, err := db.ListTasks(context.Background())
	if err == nil {
		t.Fatal("expected error from ListTasks with corrupted steps JSON")
	}
}

func TestListTasksByStatusCorruptedJSON(t *testing.T) {
	db := setupTestDB(t)

	task := models.Task{
		ID:     "corrupt-status-1",
		Name:   "test",
		URL:    "https://example.com",
		Status: "pending",
		Steps:  []models.TaskStep{{Action: models.ActionClick, Selector: "#btn"}},
		Tags:   []string{},
	}
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if _, err := db.conn.Exec(`UPDATE tasks SET steps = '<<<invalid>>>' WHERE id = ?`, "corrupt-status-1"); err != nil {
		t.Fatalf("corrupt steps: %v", err)
	}

	_, err := db.ListTasksByStatus(context.Background(), "pending")
	if err == nil {
		t.Fatal("expected error from ListTasksByStatus with corrupted JSON")
	}
}

func TestGetRecordedFlowCorruptedSteps(t *testing.T) {
	db := setupTestDB(t)

	now := time.Now()
	flow := models.RecordedFlow{
		ID:          "flow-corrupt-1",
		Name:        "corrupt flow",
		Description: "testing corrupted steps",
		Steps:       []models.RecordedStep{{Index: 0, Action: models.ActionClick, Selector: "#btn", Timestamp: now}},
		OriginURL:   "https://example.com",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.CreateRecordedFlow(context.Background(), flow); err != nil {
		t.Fatalf("CreateRecordedFlow: %v", err)
	}

	if _, err := db.conn.Exec(`UPDATE recorded_flows SET steps = '<<<invalid>>>' WHERE id = ?`, "flow-corrupt-1"); err != nil {
		t.Fatalf("corrupt flow steps: %v", err)
	}

	_, err := db.GetRecordedFlow(context.Background(), "flow-corrupt-1")
	if err == nil {
		t.Fatal("expected error from GetRecordedFlow with corrupted steps")
	}
}

func TestListRecordedFlowsCorruptedSteps(t *testing.T) {
	db := setupTestDB(t)

	now := time.Now()
	flow := models.RecordedFlow{
		ID:          "flow-corrupt-2",
		Name:        "corrupt flow list",
		Description: "testing corrupted steps",
		Steps:       []models.RecordedStep{{Index: 0, Action: models.ActionClick, Selector: "#btn", Timestamp: now}},
		OriginURL:   "https://example.com",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.CreateRecordedFlow(context.Background(), flow); err != nil {
		t.Fatalf("CreateRecordedFlow: %v", err)
	}

	if _, err := db.conn.Exec(`UPDATE recorded_flows SET steps = '<<<invalid>>>' WHERE id = ?`, "flow-corrupt-2"); err != nil {
		t.Fatalf("corrupt flow steps: %v", err)
	}

	_, err := db.ListRecordedFlows(context.Background())
	if err == nil {
		t.Fatal("expected error from ListRecordedFlows with corrupted steps")
	}
}

func TestListTasksPaginatedIgnoresCorruptedHeavyJSON(t *testing.T) {
	db := setupTestDB(t)

	task := models.Task{
		ID:     "corrupt-pag-1",
		Name:   "test",
		URL:    "https://example.com",
		Status: "pending",
		Steps:  []models.TaskStep{{Action: models.ActionClick, Selector: "#btn"}},
		Tags:   []string{},
	}
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if _, err := db.conn.Exec(`UPDATE tasks SET steps = '<<<invalid>>>' WHERE id = ?`, "corrupt-pag-1"); err != nil {
		t.Fatalf("corrupt steps: %v", err)
	}
	if _, err := db.conn.Exec(`UPDATE tasks SET result = '<<<invalid>>>' WHERE id = ?`, "corrupt-pag-1"); err != nil {
		t.Fatalf("corrupt result: %v", err)
	}

	result, err := db.ListTasksPaginated(context.Background(), 1, 10, "all", "")
	if err != nil {
		t.Fatalf("ListTasksPaginated: %v", err)
	}
	if len(result.Tasks) != 1 {
		t.Fatalf("Tasks length: got %d, want 1", len(result.Tasks))
	}
	if len(result.Tasks[0].Steps) != 0 {
		t.Fatalf("paginated task should omit steps, got %d", len(result.Tasks[0].Steps))
	}
	if result.Tasks[0].Result != nil {
		t.Fatal("paginated task should omit result")
	}
}

func TestGetTaskStatsMultipleStatuses(t *testing.T) {
	db := setupTestDB(t)

	statuses := []models.TaskStatus{models.TaskStatusPending, models.TaskStatusRunning, models.TaskStatusCompleted, models.TaskStatusFailed}
	for i, status := range statuses {
		task := models.Task{
			ID:     fmt.Sprintf("stats-multi-%d", i),
			Name:   "stats test",
			URL:    "https://example.com",
			Status: status,
			Steps:  []models.TaskStep{{Action: models.ActionClick, Selector: "#btn"}},
			Tags:   []string{},
		}
		if err := db.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("CreateTask %s: %v", status, err)
		}
	}

	stats, err := db.GetTaskStats(context.Background())
	if err != nil {
		t.Fatalf("GetTaskStats: %v", err)
	}
	if len(stats) != 4 {
		t.Errorf("expected 4 statuses, got %d: %v", len(stats), stats)
	}
	for _, s := range []string{"pending", "running", "completed", "failed"} {
		if stats[s] != 1 {
			t.Errorf("expected 1 for %s, got %d", s, stats[s])
		}
	}
}

func TestListAuditTrailFilteredByTask(t *testing.T) {
	db := setupTestDB(t)

	ev := models.TaskLifecycleEvent{
		ID:        "ev-audit-filt-1",
		TaskID:    "task-audit-filt-1",
		BatchID:   "batch-audit-filt-1",
		FromState: "pending",
		ToState:   "running",
		Timestamp: time.Now(),
	}
	if err := db.InsertTaskEvent(context.Background(), ev); err != nil {
		t.Fatalf("InsertTaskEvent: %v", err)
	}

	// Query filtered by task
	events, err := db.ListAuditTrail(context.Background(), "task-audit-filt-1", 10)
	if err != nil {
		t.Fatalf("ListAuditTrail filtered: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].FromState != "pending" || events[0].ToState != "running" {
		t.Errorf("unexpected event states: %q -> %q", events[0].FromState, events[0].ToState)
	}

	// Query for non-existent task returns empty
	none, err := db.ListAuditTrail(context.Background(), "no-such-task-filter", 5)
	if err != nil {
		t.Fatalf("ListAuditTrail no results: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("expected 0 events, got %d", len(none))
	}
}

func TestListTasksByBatchCorruptedJSON(t *testing.T) {
	db := setupTestDB(t)

	task := models.Task{
		ID:      "corrupt-batch-1",
		Name:    "test",
		URL:     "https://example.com",
		Status:  "pending",
		BatchID: "batch-corrupt-1",
		Steps:   []models.TaskStep{{Action: models.ActionClick, Selector: "#btn"}},
		Tags:    []string{},
	}
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if _, err := db.conn.Exec(`UPDATE tasks SET steps = '<<<invalid>>>' WHERE id = ?`, "corrupt-batch-1"); err != nil {
		t.Fatalf("corrupt steps: %v", err)
	}

	_, err := db.ListTasksByBatch(context.Background(), "batch-corrupt-1")
	if err == nil {
		t.Fatal("expected error from ListTasksByBatch with corrupted JSON")
	}
}

func TestListTasksByBatchStatusCorruptedJSON(t *testing.T) {
	db := setupTestDB(t)

	task := models.Task{
		ID:      "corrupt-bstat-1",
		Name:    "test",
		URL:     "https://example.com",
		Status:  "pending",
		BatchID: "batch-bstat-corrupt-1",
		Steps:   []models.TaskStep{{Action: models.ActionClick, Selector: "#btn"}},
		Tags:    []string{},
	}
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if _, err := db.conn.Exec(`UPDATE tasks SET steps = '<<<invalid>>>' WHERE id = ?`, "corrupt-bstat-1"); err != nil {
		t.Fatalf("corrupt steps: %v", err)
	}

	_, err := db.ListTasksByBatchStatus(context.Background(), "batch-bstat-corrupt-1", "pending")
	if err == nil {
		t.Fatal("expected error from ListTasksByBatchStatus with corrupted JSON")
	}
}

// --- Schedule CRUD Tests ---

func makeSchedule(id, name string) models.Schedule {
	now := time.Now().Truncate(time.Second)
	next := now.Add(1 * time.Hour)
	return models.Schedule{
		ID:       id,
		Name:     name,
		CronExpr: "*/15 * * * *",
		FlowID:   "flow-1",
		URL:      "https://example.com",
		ProxyConfig: models.ProxyConfig{
			Server:   "proxy.example.com:8080",
			Username: "user",
			Password: "pass",
		},
		Priority:  models.PriorityNormal,
		Headless:  true,
		Tags:      []string{"sched-test"},
		Enabled:   true,
		NextRunAt: &next,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestCreateSchedule(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	s := makeSchedule("sched-1", "Daily Run")

	if err := db.CreateSchedule(ctx, s); err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	got, err := db.GetSchedule(ctx, "sched-1")
	if err != nil {
		t.Fatalf("GetSchedule: %v", err)
	}
	if got.ID != s.ID {
		t.Errorf("ID: got %q, want %q", got.ID, s.ID)
	}
	if got.Name != s.Name {
		t.Errorf("Name: got %q, want %q", got.Name, s.Name)
	}
	if got.CronExpr != s.CronExpr {
		t.Errorf("CronExpr: got %q, want %q", got.CronExpr, s.CronExpr)
	}
	if got.FlowID != s.FlowID {
		t.Errorf("FlowID: got %q, want %q", got.FlowID, s.FlowID)
	}
	if !got.Enabled {
		t.Error("expected Enabled=true")
	}
	if !got.Headless {
		t.Error("expected Headless=true")
	}
	if got.ProxyConfig.Username != "user" {
		t.Errorf("proxy username: got %q, want %q", got.ProxyConfig.Username, "user")
	}
	if got.ProxyConfig.Password != "pass" {
		t.Errorf("proxy password: got %q, want %q", got.ProxyConfig.Password, "pass")
	}
}

func TestListSchedules(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		s := makeSchedule(fmt.Sprintf("sched-list-%d", i), fmt.Sprintf("Schedule %d", i))
		if err := db.CreateSchedule(ctx, s); err != nil {
			t.Fatalf("CreateSchedule %d: %v", i, err)
		}
	}

	schedules, err := db.ListSchedules(ctx)
	if err != nil {
		t.Fatalf("ListSchedules: %v", err)
	}
	if len(schedules) != 3 {
		t.Errorf("expected 3 schedules, got %d", len(schedules))
	}
}

func TestUpdateSchedule(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	s := makeSchedule("sched-upd-1", "Original")

	if err := db.CreateSchedule(ctx, s); err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	s.Name = "Updated"
	s.CronExpr = "0 9 * * 1-5"
	s.Enabled = false
	s.UpdatedAt = time.Now().Truncate(time.Second)

	if err := db.UpdateSchedule(ctx, s); err != nil {
		t.Fatalf("UpdateSchedule: %v", err)
	}

	got, err := db.GetSchedule(ctx, "sched-upd-1")
	if err != nil {
		t.Fatalf("GetSchedule: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("Name: got %q, want %q", got.Name, "Updated")
	}
	if got.CronExpr != "0 9 * * 1-5" {
		t.Errorf("CronExpr: got %q, want %q", got.CronExpr, "0 9 * * 1-5")
	}
	if got.Enabled {
		t.Error("expected Enabled=false")
	}
}

func TestDeleteSchedule(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	s := makeSchedule("sched-del-1", "ToDelete")

	if err := db.CreateSchedule(ctx, s); err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	if err := db.DeleteSchedule(ctx, "sched-del-1"); err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}

	_, err := db.GetSchedule(ctx, "sched-del-1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestListDueSchedules(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	past := time.Now().Add(-10 * time.Minute).Truncate(time.Second)
	future := time.Now().Add(10 * time.Hour).Truncate(time.Second)

	s1 := makeSchedule("sched-due-1", "Past Due")
	s1.NextRunAt = &past
	s1.Enabled = true

	s2 := makeSchedule("sched-due-2", "Future")
	s2.NextRunAt = &future
	s2.Enabled = true

	s3 := makeSchedule("sched-due-3", "Disabled Past")
	s3.NextRunAt = &past
	s3.Enabled = false

	for _, s := range []models.Schedule{s1, s2, s3} {
		if err := db.CreateSchedule(ctx, s); err != nil {
			t.Fatalf("CreateSchedule %s: %v", s.ID, err)
		}
	}

	due, err := db.ListDueSchedules(ctx, time.Now())
	if err != nil {
		t.Fatalf("ListDueSchedules: %v", err)
	}
	if len(due) != 1 {
		t.Errorf("expected 1 due schedule, got %d", len(due))
	}
	if len(due) > 0 && due[0].ID != "sched-due-1" {
		t.Errorf("expected sched-due-1, got %s", due[0].ID)
	}
}

func TestUpdateScheduleRun(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	s := makeSchedule("sched-run-1", "RunTest")

	if err := db.CreateSchedule(ctx, s); err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	lastRun := time.Now().Truncate(time.Second)
	nextRun := lastRun.Add(15 * time.Minute)

	if err := db.UpdateScheduleRun(ctx, "sched-run-1", lastRun, nextRun); err != nil {
		t.Fatalf("UpdateScheduleRun: %v", err)
	}

	got, err := db.GetSchedule(ctx, "sched-run-1")
	if err != nil {
		t.Fatalf("GetSchedule: %v", err)
	}
	if got.LastRunAt == nil {
		t.Fatal("LastRunAt is nil after update")
	}
	if got.NextRunAt == nil {
		t.Fatal("NextRunAt is nil after update")
	}
}

// --- Captcha Config CRUD Tests ---

func TestCreateCaptchaConfig(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	c := models.CaptchaConfig{
		ID:        "cap-1",
		Provider:  models.CaptchaProvider2Captcha,
		APIKey:    "secret-api-key-123",
		Enabled:   true,
		CreatedAt: time.Now().Truncate(time.Second),
		UpdatedAt: time.Now().Truncate(time.Second),
	}

	if err := db.CreateCaptchaConfig(ctx, c); err != nil {
		t.Fatalf("CreateCaptchaConfig: %v", err)
	}

	got, err := db.GetCaptchaConfig(ctx, "cap-1")
	if err != nil {
		t.Fatalf("GetCaptchaConfig: %v", err)
	}
	if got.APIKey != "secret-api-key-123" {
		t.Errorf("APIKey decrypted: got %q, want %q", got.APIKey, "secret-api-key-123")
	}
	if got.Provider != models.CaptchaProvider2Captcha {
		t.Errorf("Provider: got %q, want %q", got.Provider, models.CaptchaProvider2Captcha)
	}
	if !got.Enabled {
		t.Error("expected Enabled=true")
	}
}

func TestGetActiveCaptchaConfig(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	c := models.CaptchaConfig{
		ID:        "cap-active-1",
		Provider:  models.CaptchaProviderAntiCaptcha,
		APIKey:    "active-key",
		Enabled:   true,
		CreatedAt: time.Now().Truncate(time.Second),
		UpdatedAt: time.Now().Truncate(time.Second),
	}
	if err := db.CreateCaptchaConfig(ctx, c); err != nil {
		t.Fatalf("CreateCaptchaConfig: %v", err)
	}

	got, err := db.GetActiveCaptchaConfig(ctx)
	if err != nil {
		t.Fatalf("GetActiveCaptchaConfig: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil active config")
	}
	if got.ID != "cap-active-1" {
		t.Errorf("ID: got %q, want %q", got.ID, "cap-active-1")
	}
	if got.APIKey != "active-key" {
		t.Errorf("APIKey: got %q, want %q", got.APIKey, "active-key")
	}
}

func TestListCaptchaConfigs(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		c := models.CaptchaConfig{
			ID:        fmt.Sprintf("cap-list-%d", i),
			Provider:  models.CaptchaProvider2Captcha,
			APIKey:    fmt.Sprintf("key-%d", i),
			Enabled:   i%2 == 0,
			CreatedAt: time.Now().Truncate(time.Second),
			UpdatedAt: time.Now().Truncate(time.Second),
		}
		if err := db.CreateCaptchaConfig(ctx, c); err != nil {
			t.Fatalf("CreateCaptchaConfig %d: %v", i, err)
		}
	}

	configs, err := db.ListCaptchaConfigs(ctx)
	if err != nil {
		t.Fatalf("ListCaptchaConfigs: %v", err)
	}
	if len(configs) != 3 {
		t.Errorf("expected 3 configs, got %d", len(configs))
	}
}

func TestDeleteCaptchaConfig(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	c := models.CaptchaConfig{
		ID:        "cap-del-1",
		Provider:  models.CaptchaProvider2Captcha,
		APIKey:    "delete-key",
		Enabled:   true,
		CreatedAt: time.Now().Truncate(time.Second),
		UpdatedAt: time.Now().Truncate(time.Second),
	}
	if err := db.CreateCaptchaConfig(ctx, c); err != nil {
		t.Fatalf("CreateCaptchaConfig: %v", err)
	}

	if err := db.DeleteCaptchaConfig(ctx, "cap-del-1"); err != nil {
		t.Fatalf("DeleteCaptchaConfig: %v", err)
	}

	_, err := db.GetCaptchaConfig(ctx, "cap-del-1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

// --- Visual Baseline & Diff CRUD Tests ---

func TestCreateVisualBaseline(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	b := models.VisualBaseline{
		ID:             "vb-1",
		Name:           "Homepage",
		TaskID:         "task-1",
		URL:            "https://example.com",
		ScreenshotPath: "/tmp/baseline.png",
		Width:          1920,
		Height:         1080,
		CreatedAt:      time.Now().Truncate(time.Second),
	}

	if err := db.CreateVisualBaseline(ctx, b); err != nil {
		t.Fatalf("CreateVisualBaseline: %v", err)
	}

	got, err := db.GetVisualBaseline(ctx, "vb-1")
	if err != nil {
		t.Fatalf("GetVisualBaseline: %v", err)
	}
	if got.Name != "Homepage" {
		t.Errorf("Name: got %q, want %q", got.Name, "Homepage")
	}
	if got.Width != 1920 {
		t.Errorf("Width: got %d, want %d", got.Width, 1920)
	}
	if got.Height != 1080 {
		t.Errorf("Height: got %d, want %d", got.Height, 1080)
	}
	if got.ScreenshotPath != "/tmp/baseline.png" {
		t.Errorf("ScreenshotPath: got %q, want %q", got.ScreenshotPath, "/tmp/baseline.png")
	}
}

func TestListVisualBaselines(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		b := models.VisualBaseline{
			ID:             fmt.Sprintf("vb-list-%d", i),
			Name:           fmt.Sprintf("Baseline %d", i),
			URL:            "https://example.com",
			ScreenshotPath: fmt.Sprintf("/tmp/baseline-%d.png", i),
			Width:          1920,
			Height:         1080,
			CreatedAt:      time.Now().Truncate(time.Second),
		}
		if err := db.CreateVisualBaseline(ctx, b); err != nil {
			t.Fatalf("CreateVisualBaseline %d: %v", i, err)
		}
	}

	baselines, err := db.ListVisualBaselines(ctx)
	if err != nil {
		t.Fatalf("ListVisualBaselines: %v", err)
	}
	if len(baselines) != 3 {
		t.Errorf("expected 3 baselines, got %d", len(baselines))
	}
}

func TestDeleteVisualBaseline(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	b := models.VisualBaseline{
		ID:             "vb-del-1",
		Name:           "ToDelete",
		URL:            "https://example.com",
		ScreenshotPath: "/tmp/del.png",
		Width:          800,
		Height:         600,
		CreatedAt:      time.Now().Truncate(time.Second),
	}
	if err := db.CreateVisualBaseline(ctx, b); err != nil {
		t.Fatalf("CreateVisualBaseline: %v", err)
	}

	if err := db.DeleteVisualBaseline(ctx, "vb-del-1"); err != nil {
		t.Fatalf("DeleteVisualBaseline: %v", err)
	}

	_, err := db.GetVisualBaseline(ctx, "vb-del-1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestCreateVisualDiff(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	b := models.VisualBaseline{
		ID:             "vb-diff-base",
		Name:           "Base",
		URL:            "https://example.com",
		ScreenshotPath: "/tmp/base.png",
		Width:          1920,
		Height:         1080,
		CreatedAt:      time.Now().Truncate(time.Second),
	}
	if err := db.CreateVisualBaseline(ctx, b); err != nil {
		t.Fatalf("CreateVisualBaseline: %v", err)
	}

	d := models.VisualDiff{
		ID:             "vd-1",
		BaselineID:     "vb-diff-base",
		TaskID:         "task-diff-1",
		ScreenshotPath: "/tmp/new.png",
		DiffImagePath:  "/tmp/diff.png",
		DiffPercent:    2.5,
		PixelCount:     500,
		Threshold:      5.0,
		Passed:         true,
		Width:          1920,
		Height:         1080,
		CreatedAt:      time.Now().Truncate(time.Second),
	}

	if err := db.CreateVisualDiff(ctx, d); err != nil {
		t.Fatalf("CreateVisualDiff: %v", err)
	}

	got, err := db.GetVisualDiff(ctx, "vd-1")
	if err != nil {
		t.Fatalf("GetVisualDiff: %v", err)
	}
	if got.DiffPercent != 2.5 {
		t.Errorf("DiffPercent: got %.2f, want 2.50", got.DiffPercent)
	}
	if got.PixelCount != 500 {
		t.Errorf("PixelCount: got %d, want 500", got.PixelCount)
	}
	if !got.Passed {
		t.Error("expected Passed=true")
	}
	if got.BaselineID != "vb-diff-base" {
		t.Errorf("BaselineID: got %q, want %q", got.BaselineID, "vb-diff-base")
	}
}

func TestListVisualDiffs(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	b := models.VisualBaseline{
		ID:             "vb-listdiff",
		Name:           "Base",
		URL:            "https://example.com",
		ScreenshotPath: "/tmp/base.png",
		Width:          800,
		Height:         600,
		CreatedAt:      time.Now().Truncate(time.Second),
	}
	if err := db.CreateVisualBaseline(ctx, b); err != nil {
		t.Fatalf("CreateVisualBaseline: %v", err)
	}

	for i := 0; i < 3; i++ {
		d := models.VisualDiff{
			ID:             fmt.Sprintf("vd-list-%d", i),
			BaselineID:     "vb-listdiff",
			TaskID:         fmt.Sprintf("task-list-%d", i),
			ScreenshotPath: fmt.Sprintf("/tmp/new-%d.png", i),
			DiffImagePath:  fmt.Sprintf("/tmp/diff-%d.png", i),
			DiffPercent:    float64(i),
			PixelCount:     int64(i * 100),
			Threshold:      5.0,
			Passed:         true,
			Width:          800,
			Height:         600,
			CreatedAt:      time.Now().Truncate(time.Second),
		}
		if err := db.CreateVisualDiff(ctx, d); err != nil {
			t.Fatalf("CreateVisualDiff %d: %v", i, err)
		}
	}

	diffs, err := db.ListVisualDiffs(ctx, "vb-listdiff")
	if err != nil {
		t.Fatalf("ListVisualDiffs: %v", err)
	}
	if len(diffs) != 3 {
		t.Errorf("expected 3 diffs, got %d", len(diffs))
	}
}

func TestListVisualDiffsByTask(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	b := models.VisualBaseline{
		ID:             "vb-bytask",
		Name:           "Base",
		URL:            "https://example.com",
		ScreenshotPath: "/tmp/base.png",
		Width:          800,
		Height:         600,
		CreatedAt:      time.Now().Truncate(time.Second),
	}
	if err := db.CreateVisualBaseline(ctx, b); err != nil {
		t.Fatalf("CreateVisualBaseline: %v", err)
	}

	for i := 0; i < 2; i++ {
		d := models.VisualDiff{
			ID:             fmt.Sprintf("vd-bytask-%d", i),
			BaselineID:     "vb-bytask",
			TaskID:         "shared-task",
			ScreenshotPath: fmt.Sprintf("/tmp/new-%d.png", i),
			DiffImagePath:  fmt.Sprintf("/tmp/diff-%d.png", i),
			DiffPercent:    1.0,
			PixelCount:     50,
			Threshold:      5.0,
			Passed:         true,
			Width:          800,
			Height:         600,
			CreatedAt:      time.Now().Truncate(time.Second),
		}
		if err := db.CreateVisualDiff(ctx, d); err != nil {
			t.Fatalf("CreateVisualDiff %d: %v", i, err)
		}
	}

	d3 := models.VisualDiff{
		ID:             "vd-bytask-other",
		BaselineID:     "vb-bytask",
		TaskID:         "other-task",
		ScreenshotPath: "/tmp/other.png",
		DiffImagePath:  "/tmp/other-diff.png",
		DiffPercent:    0.5,
		PixelCount:     10,
		Threshold:      5.0,
		Passed:         true,
		Width:          800,
		Height:         600,
		CreatedAt:      time.Now().Truncate(time.Second),
	}
	if err := db.CreateVisualDiff(ctx, d3); err != nil {
		t.Fatalf("CreateVisualDiff other: %v", err)
	}

	diffs, err := db.ListVisualDiffsByTask(ctx, "shared-task")
	if err != nil {
		t.Fatalf("ListVisualDiffsByTask: %v", err)
	}
	if len(diffs) != 2 {
		t.Errorf("expected 2 diffs for shared-task, got %d", len(diffs))
	}
}
