package monitoring

import (
	"context"
	"testing"
	"time"
)

func TestMonitorRecordRepeatBatch(t *testing.T) {
	m := New()

	m.RecordRepeatBatchCreated("batch-1", 100, "range")

	metrics := m.GetRepeatTaskMetrics()

	if metrics.TotalRepeatedBatches != 1 {
		t.Errorf("TotalRepeatedBatches = %d, want 1", metrics.TotalRepeatedBatches)
	}

	if metrics.TotalRepeatedTasks != 100 {
		t.Errorf("TotalRepeatedTasks = %d, want 100", metrics.TotalRepeatedTasks)
	}

	if metrics.ActiveRepeatedBatches != 1 {
		t.Errorf("ActiveRepeatedBatches = %d, want 1", metrics.ActiveRepeatedBatches)
	}

	if metrics.RangeModeBatches != 1 {
		t.Errorf("RangeModeBatches = %d, want 1", metrics.RangeModeBatches)
	}

	if metrics.AvgTasksPerBatch != 100.0 {
		t.Errorf("AvgTasksPerBatch = %f, want 100.0", metrics.AvgTasksPerBatch)
	}
}

func TestMonitorRecordMultipleBatches(t *testing.T) {
	m := New()

	m.RecordRepeatBatchCreated("batch-1", 100, "range")
	m.RecordRepeatBatchCreated("batch-2", 50, "counter")
	m.RecordRepeatBatchCreated("batch-3", 25, "list")

	metrics := m.GetRepeatTaskMetrics()

	if metrics.TotalRepeatedBatches != 3 {
		t.Errorf("TotalRepeatedBatches = %d, want 3", metrics.TotalRepeatedBatches)
	}

	if metrics.TotalRepeatedTasks != 175 {
		t.Errorf("TotalRepeatedTasks = %d, want 175", metrics.TotalRepeatedTasks)
	}

	expectedAvg := 175.0 / 3.0
	if metrics.AvgTasksPerBatch != expectedAvg {
		t.Errorf("AvgTasksPerBatch = %f, want %f", metrics.AvgTasksPerBatch, expectedAvg)
	}

	if metrics.RangeModeBatches != 1 {
		t.Errorf("RangeModeBatches = %d, want 1", metrics.RangeModeBatches)
	}

	if metrics.CounterModeBatches != 1 {
		t.Errorf("CounterModeBatches = %d, want 1", metrics.CounterModeBatches)
	}

	if metrics.ListModeBatches != 1 {
		t.Errorf("ListModeBatches = %d, want 1", metrics.ListModeBatches)
	}
}

func TestMonitorBatchCompletion(t *testing.T) {
	m := New()

	m.RecordRepeatBatchCreated("batch-1", 100, "range")
	m.RecordRepeatBatchCompleted("batch-1", 5000)

	metrics := m.GetRepeatTaskMetrics()

	if metrics.ActiveRepeatedBatches != 0 {
		t.Errorf("ActiveRepeatedBatches = %d, want 0", metrics.ActiveRepeatedBatches)
	}

	if metrics.AvgBatchCompletionTimeMs != 5000 {
		t.Errorf("AvgBatchCompletionTimeMs = %d, want 5000", metrics.AvgBatchCompletionTimeMs)
	}
}

func TestMonitorTaskCompletion(t *testing.T) {
	m := New()

	m.RecordRepeatTaskCompleted()
	m.RecordRepeatTaskCompleted()
	m.RecordRepeatTaskFailed()

	metrics := m.GetRepeatTaskMetrics()

	if metrics.CompletedRepeatedTasks != 2 {
		t.Errorf("CompletedRepeatedTasks = %d, want 2", metrics.CompletedRepeatedTasks)
	}

	if metrics.FailedRepeatedTasks != 1 {
		t.Errorf("FailedRepeatedTasks = %d, want 1", metrics.FailedRepeatedTasks)
	}
}

func TestMonitorSystemMetrics(t *testing.T) {
	m := New()

	m.UpdateSystemMetrics(512.5, 100, 5)

	metrics := m.GetSystemMetrics()

	if metrics.MemoryUsageMB != 512.5 {
		t.Errorf("MemoryUsageMB = %f, want 512.5", metrics.MemoryUsageMB)
	}

	if metrics.GoroutineCount != 100 {
		t.Errorf("GoroutineCount = %d, want 100", metrics.GoroutineCount)
	}

	if metrics.DatabaseConnections != 5 {
		t.Errorf("DatabaseConnections = %d, want 5", metrics.DatabaseConnections)
	}
}

func TestMonitorRequestTracking(t *testing.T) {
	m := New()

	m.RecordRequest()
	m.RecordRequest()
	m.RecordError()

	metrics := m.GetSystemMetrics()

	if metrics.TotalRequests != 2 {
		t.Errorf("TotalRequests = %d, want 2", metrics.TotalRequests)
	}

	if metrics.TotalErrors != 1 {
		t.Errorf("TotalErrors = %d, want 1", metrics.TotalErrors)
	}
}

func TestMonitorAlerts(t *testing.T) {
	m := New()

	alertTriggered := make(chan bool, 1)

	m.RegisterAlertCallback(func(alert Alert, fired bool) {
		if fired {
			select {
			case alertTriggered <- true:
			default:
			}
		}
	})

	m.AddAlertRule(AlertRule{
		ID:          "test-alert",
		Name:        "Test Alert",
		Description: "Test alert for high error rate",
		Severity:    AlertSeverityWarning,
		Cooldown:    1 * time.Second,
		Enabled:     true,
		Condition: func(monitor *Monitor) bool {
			metrics := monitor.GetSystemMetrics()
			return metrics.TotalErrors > 0
		},
	})

	// Record error to trigger alert
	m.RecordError()

	// Check alerts
	m.CheckAlerts(context.Background())

	// Wait for callback with timeout
	select {
	case <-alertTriggered:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Alert was not triggered within timeout")
	}
}

func TestMonitorAlertCooldown(t *testing.T) {
	m := New()

	alertCount := 0
	alertChan := make(chan bool, 10)

	m.RegisterAlertCallback(func(alert Alert, fired bool) {
		if fired {
			alertCount++
			select {
			case alertChan <- true:
			default:
			}
		}
	})

	m.AddAlertRule(AlertRule{
		ID:          "cooldown-test",
		Name:        "Cooldown Test",
		Description: "Test cooldown",
		Severity:    AlertSeverityInfo,
		Cooldown:    100 * time.Millisecond,
		Enabled:     true,
		Condition: func(monitor *Monitor) bool {
			return true // Always true
		},
	})

	// Check multiple times rapidly
	m.CheckAlerts(context.Background())
	select {
	case <-alertChan:
	case <-time.After(50 * time.Millisecond):
	}

	m.CheckAlerts(context.Background())
	m.CheckAlerts(context.Background())

	// Wait a bit for any errant callbacks
	time.Sleep(50 * time.Millisecond)

	// Should only trigger once due to cooldown
	if alertCount != 1 {
		t.Errorf("alertCount = %d, want 1 (cooldown should prevent multiple triggers)", alertCount)
	}

	// Wait for cooldown to expire
	time.Sleep(100 * time.Millisecond)

	// Should trigger again
	m.CheckAlerts(context.Background())
	select {
	case <-alertChan:
	case <-time.After(50 * time.Millisecond):
	}

	if alertCount != 2 {
		t.Errorf("alertCount = %d, want 2 (after cooldown expires)", alertCount)
	}
}

func TestMonitorPrometheusMetrics(t *testing.T) {
	m := New()

	m.RecordRepeatBatchCreated("batch-1", 100, "range")
	m.RecordRepeatTaskCompleted()
	m.RecordRepeatTaskFailed()
	m.UpdateSystemMetrics(512.5, 100, 5)

	metrics := m.GetPrometheusMetrics()

	// Check for expected metric lines
	expectedMetrics := []string{
		"flowpilot_repeat_batches_total 1",
		"flowpilot_repeat_tasks_total 100",
		"flowpilot_repeat_tasks_completed_total 1",
		"flowpilot_repeat_tasks_failed_total 1",
		"flowpilot_system_memory_mb 512.50",
		"flowpilot_system_goroutines 100",
	}

	for _, expected := range expectedMetrics {
		if !contains(metrics, expected) {
			t.Errorf("Prometheus metrics missing expected line: %s", expected)
		}
	}
}

func TestMonitorHealthStatus(t *testing.T) {
	m := New()

	components := map[string]func() Component{
		"database": func() Component {
			return Component{Status: "healthy", Message: "Connected"}
		},
		"queue": func() Component {
			return Component{Status: "healthy", Message: "Running"}
		},
	}

	status := m.GetHealthStatus(context.Background(), components)

	if status.Status != "healthy" {
		t.Errorf("Status = %s, want healthy", status.Status)
	}

	if len(status.Components) != 2 {
		t.Errorf("Components count = %d, want 2", len(status.Components))
	}

	if status.Components["database"].Status != "healthy" {
		t.Error("Database component not healthy")
	}
}

func TestMonitorHealthStatusDegraded(t *testing.T) {
	m := New()

	components := map[string]func() Component{
		"database": func() Component {
			return Component{Status: "healthy"}
		},
		"queue": func() Component {
			return Component{Status: "unhealthy", Message: "Queue full"}
		},
	}

	status := m.GetHealthStatus(context.Background(), components)

	if status.Status != "degraded" {
		t.Errorf("Status = %s, want degraded", status.Status)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
