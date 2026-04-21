package monitoring

import (
	"testing"
	"time"
)

func TestRecordStepDuration(t *testing.T) {
	m := New()

	// Record some step durations
	durations := []int64{100, 150, 200, 120, 180}
	for _, d := range durations {
		m.RecordStepDuration(d)
	}

	avgDuration := m.GetAvgStepDuration()
	expectedAvg := 150.0 // (100+150+200+120+180)/5

	if avgDuration != expectedAvg {
		t.Errorf("got avg duration %v, want %v", avgDuration, expectedAvg)
	}
}

func TestGetAvgStepDurationEmpty(t *testing.T) {
	m := New()

	avgDuration := m.GetAvgStepDuration()
	if avgDuration != 0 {
		t.Errorf("got avg duration %v for empty monitor, want 0", avgDuration)
	}
}

func TestRecordStepDurationBounded(t *testing.T) {
	m := New()
	m.maxStepSamples = 10 // Small limit for testing

	// Record more samples than the limit
	for i := 0; i < 20; i++ {
		m.RecordStepDuration(int64(i * 10))
	}

	// Verify we only keep the most recent 10 samples
	m.mu.RLock()
	samplesCount := len(m.stepDurations)
	m.mu.RUnlock()

	if samplesCount != 10 {
		t.Errorf("got %d samples, want 10 (bounded by maxStepSamples)", samplesCount)
	}
}

func TestRecordErrorContext(t *testing.T) {
	m := New()

	errorContexts := []string{
		`{"taskId":"task-1","errorCode":"TIMEOUT"}`,
		`{"taskId":"task-2","errorCode":"SELECTOR_NOT_FOUND"}`,
		`{"taskId":"task-3","errorCode":"NETWORK_ERROR"}`,
	}

	for _, errCtx := range errorContexts {
		m.RecordErrorContext(errCtx)
	}

	recentErrors := m.GetRecentErrors(10)
	if len(recentErrors) != 3 {
		t.Errorf("got %d recent errors, want 3", len(recentErrors))
	}

	// Verify the last error is the most recent one
	if recentErrors[len(recentErrors)-1] != errorContexts[2] {
		t.Errorf("most recent error mismatch")
	}
}

func TestGetRecentErrorsLimited(t *testing.T) {
	m := New()

	errorContexts := []string{
		`{"taskId":"task-1"}`,
		`{"taskId":"task-2"}`,
		`{"taskId":"task-3"}`,
		`{"taskId":"task-4"}`,
		`{"taskId":"task-5"}`,
	}

	for _, errCtx := range errorContexts {
		m.RecordErrorContext(errCtx)
	}

	// Request only the 2 most recent
	recentErrors := m.GetRecentErrors(2)
	if len(recentErrors) != 2 {
		t.Errorf("got %d recent errors, want 2", len(recentErrors))
	}

	// Verify it contains the last 2
	if recentErrors[0] != errorContexts[3] || recentErrors[1] != errorContexts[4] {
		t.Errorf("recent errors mismatch")
	}
}

func TestRecordErrorContextBounded(t *testing.T) {
	m := New()
	m.maxErrorSamples = 5 // Small limit for testing

	// Record more errors than the limit
	for i := 0; i < 10; i++ {
		m.RecordErrorContext(`{"taskId":"task-` + string(rune(i+'0')) + `"}`)
	}

	// Verify we only keep the most recent 5 errors
	m.mu.RLock()
	errorsCount := len(m.errorContexts)
	m.mu.RUnlock()

	if errorsCount != 5 {
		t.Errorf("got %d errors, want 5 (bounded by maxErrorSamples)", errorsCount)
	}
}

func TestGetRecentErrorsEmpty(t *testing.T) {
	m := New()

	recentErrors := m.GetRecentErrors(10)
	if recentErrors != nil {
		t.Errorf("got %v for empty monitor, want nil", recentErrors)
	}
}

func TestGetRecentErrorsNegativeLimit(t *testing.T) {
	m := New()
	m.RecordErrorContext(`{"taskId":"task-1"}`)

	recentErrors := m.GetRecentErrors(-1)
	if recentErrors != nil {
		t.Errorf("got %v for negative limit, want nil", recentErrors)
	}
}

func TestStepDurationConcurrency(t *testing.T) {
	m := New()
	done := make(chan bool)

	// Record durations from multiple goroutines
	for i := 0; i < 10; i++ {
		go func(val int64) {
			m.RecordStepDuration(val)
			done <- true
		}(int64(i * 10))
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	avgDuration := m.GetAvgStepDuration()
	if avgDuration == 0 {
		t.Errorf("got zero avg duration from concurrent recording")
	}

	// Verify count
	m.mu.RLock()
	count := len(m.stepDurations)
	m.mu.RUnlock()

	if count != 10 {
		t.Errorf("got %d recorded durations, want 10", count)
	}
}

func TestErrorContextConcurrency(t *testing.T) {
	m := New()
	done := make(chan bool)

	// Record errors from multiple goroutines
	for i := 0; i < 5; i++ {
		go func(val int) {
			m.RecordErrorContext(`{"taskId":"task"}`)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	recentErrors := m.GetRecentErrors(10)
	if len(recentErrors) != 5 {
		t.Errorf("got %d errors from concurrent recording, want 5", len(recentErrors))
	}
}

func TestMonitoringMetricsConsistency(t *testing.T) {
	m := New()

	// Simulate a sequence of step executions and errors
	for i := 0; i < 5; i++ {
		// Each step takes 100-500ms
		duration := int64((i + 1) * 100)
		m.RecordStepDuration(duration)

		// Some steps fail
		if i%2 == 0 {
			m.RecordErrorContext(`{"taskId":"task-` + string(rune(i+'0')) + `"}`)
		}
	}

	// Verify both metrics are tracked independently
	avgDuration := m.GetAvgStepDuration()
	if avgDuration == 0 {
		t.Errorf("avg duration should be tracked")
	}

	recentErrors := m.GetRecentErrors(10)
	if len(recentErrors) != 3 { // 3 failures (i=0,2,4)
		t.Errorf("got %d errors, want 3", len(recentErrors))
	}
}

func TestRecordStepDurationNegative(t *testing.T) {
	m := New()
	m.RecordStepDuration(-100) // Edge case: negative duration

	m.mu.RLock()
	if len(m.stepDurations) != 1 {
		m.mu.RUnlock()
		t.Errorf("negative duration not recorded")
		return
	}
	m.mu.RUnlock()

	avgDuration := m.GetAvgStepDuration()
	if avgDuration != -100 {
		t.Errorf("got avg duration %v for negative input, want -100", avgDuration)
	}
}

func TestRecordStepDurationZero(t *testing.T) {
	m := New()
	m.RecordStepDuration(0)

	avgDuration := m.GetAvgStepDuration()
	if avgDuration != 0 {
		t.Errorf("got avg duration %v, want 0", avgDuration)
	}
}

func TestMonitorCreationTime(t *testing.T) {
	before := time.Now()
	m := New()
	after := time.Now()

	sysMetrics := m.GetSystemMetrics()
	if sysMetrics.StartTime.Before(before) || sysMetrics.StartTime.After(after) {
		t.Errorf("StartTime not within expected range")
	}
}
