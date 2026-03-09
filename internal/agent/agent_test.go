package agent

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"flowpilot/internal/crypto"
	"flowpilot/internal/models"
)

func TestNewWithTempDir(t *testing.T) {
	dir := t.TempDir()
	crypto.ResetForTest()
	t.Cleanup(func() { crypto.ResetForTest() })

	cfg := Config{
		DataDir:        filepath.Join(dir, "agent-data"),
		MaxConcurrency: 5,
		PollInterval:   1 * time.Second,
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer a.Stop()

	if a.db == nil {
		t.Error("db should not be nil")
	}
	if a.runner == nil {
		t.Error("runner should not be nil")
	}
	if a.queue == nil {
		t.Error("queue should not be nil")
	}
	if a.proxyManager == nil {
		t.Error("proxyManager should not be nil")
	}
	if a.dataDir != cfg.DataDir {
		t.Errorf("dataDir: got %q, want %q", a.dataDir, cfg.DataDir)
	}
	if a.pollInterval != 1*time.Second {
		t.Errorf("pollInterval: got %v, want %v", a.pollInterval, 1*time.Second)
	}
}

func TestNewConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	crypto.ResetForTest()
	t.Cleanup(func() { crypto.ResetForTest() })

	cfg := Config{
		DataDir: filepath.Join(dir, "agent-defaults"),
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer a.Stop()

	if a.pollInterval != 30*time.Second {
		t.Errorf("default pollInterval: got %v, want 30s", a.pollInterval)
	}
}

func TestNewInvalidDataDir(t *testing.T) {
	crypto.ResetForTest()
	t.Cleanup(func() { crypto.ResetForTest() })

	cfg := Config{
		DataDir: "/dev/null/invalid",
	}

	_, err := New(cfg)
	if err == nil {
		t.Fatal("expected error for invalid data dir")
	}
}

func TestStopNilFields(t *testing.T) {
	a := &Agent{}
	a.Stop()
}

func TestStopIdempotent(t *testing.T) {
	dir := t.TempDir()
	crypto.ResetForTest()
	t.Cleanup(func() { crypto.ResetForTest() })

	cfg := Config{
		DataDir: filepath.Join(dir, "agent-stop"),
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	a.Stop()
	a.Stop()
}

func TestRunRespondsToContextCancel(t *testing.T) {
	dir := t.TempDir()
	crypto.ResetForTest()
	t.Cleanup(func() { crypto.ResetForTest() })

	cfg := Config{
		DataDir:        filepath.Join(dir, "agent-run"),
		MaxConcurrency: 2,
		PollInterval:   100 * time.Millisecond,
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- a.Run(ctx)
	}()

	time.Sleep(300 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Run error: got %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}

func TestProcessPendingNoTasks(t *testing.T) {
	dir := t.TempDir()
	crypto.ResetForTest()
	t.Cleanup(func() { crypto.ResetForTest() })

	cfg := Config{
		DataDir: filepath.Join(dir, "agent-process"),
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer a.Stop()

	a.processPending(context.Background())
}

func TestProcessPendingWithTasks(t *testing.T) {
	dir := t.TempDir()
	crypto.ResetForTest()
	t.Cleanup(func() { crypto.ResetForTest() })

	cfg := Config{
		DataDir:        filepath.Join(dir, "agent-pending"),
		MaxConcurrency: 2,
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer a.Stop()

	task := models.Task{
		ID:         "agent-task-1",
		Name:       "Test Agent Task",
		URL:        "https://example.com",
		Status:     models.TaskStatusPending,
		Priority:   models.PriorityNormal,
		MaxRetries: 1,
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: "https://example.com"},
		},
		CreatedAt: time.Now(),
	}
	if err := a.db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	a.processPending(context.Background())

	time.Sleep(200 * time.Millisecond)

	got, err := a.db.GetTask(context.Background(), "agent-task-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status == models.TaskStatusPending {
		t.Error("task should no longer be pending after processPending")
	}
}

func TestNewWithNegativeConcurrency(t *testing.T) {
	dir := t.TempDir()
	crypto.ResetForTest()
	t.Cleanup(func() { crypto.ResetForTest() })

	cfg := Config{
		DataDir:        filepath.Join(dir, "agent-neg"),
		MaxConcurrency: -1,
		PollInterval:   -1,
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer a.Stop()

	if a.pollInterval != 30*time.Second {
		t.Errorf("pollInterval: got %v, want 30s", a.pollInterval)
	}
}
