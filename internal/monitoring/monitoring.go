package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Monitor collects and aggregates monitoring metrics.
type Monitor struct {
	mu                sync.RWMutex
	repeatTaskMetrics RepeatTaskMetrics
	systemMetrics     SystemMetrics
	alertRules        []AlertRule
	alertCallbacks    []AlertCallback
}

// RepeatTaskMetrics tracks metrics specific to repeated tasks.
type RepeatTaskMetrics struct {
	TotalRepeatedBatches     int64     `json:"totalRepeatedBatches"`
	TotalRepeatedTasks       int64     `json:"totalRepeatedTasks"`
	ActiveRepeatedBatches    int       `json:"activeRepeatedBatches"`
	CompletedRepeatedTasks   int64     `json:"completedRepeatedTasks"`
	FailedRepeatedTasks      int64     `json:"failedRepeatedTasks"`
	AvgTasksPerBatch         float64   `json:"avgTasksPerBatch"`
	AvgBatchCompletionTimeMs int64     `json:"avgBatchCompletionTimeMs"`
	LastBatchCreatedAt       time.Time `json:"lastBatchCreatedAt"`
	LastBatchID              string    `json:"lastBatchId"`
	
	// Per-mode breakdown
	CounterModeBatches int64 `json:"counterModeBatches"`
	RangeModeBatches   int64 `json:"rangeModeBatches"`
	ListModeBatches    int64 `json:"listModeBatches"`
}

// SystemMetrics tracks overall system health.
type SystemMetrics struct {
	Uptime              time.Duration `json:"uptimeSeconds"`
	StartTime           time.Time     `json:"startTime"`
	TotalRequests       int64         `json:"totalRequests"`
	TotalErrors         int64         `json:"totalErrors"`
	MemoryUsageMB       float64       `json:"memoryUsageMb"`
	GoroutineCount      int           `json:"goroutineCount"`
	DatabaseConnections int           `json:"databaseConnections"`
	
	// Rate metrics (per minute)
	RequestsPerMinute float64 `json:"requestsPerMinute"`
	ErrorsPerMinute   float64 `json:"errorsPerMinute"`
	TasksPerMinute    float64 `json:"tasksPerMinute"`
}

// AlertRule defines a condition that triggers an alert.
type AlertRule struct {
	ID          string
	Name        string
	Description string
	Condition   func(*Monitor) bool
	Severity    AlertSeverity
	Cooldown    time.Duration
	lastFired   time.Time
}

// AlertSeverity defines alert importance levels.
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// Alert represents a triggered alert.
type Alert struct {
	RuleID      string        `json:"ruleId"`
	RuleName    string        `json:"ruleName"`
	Description string        `json:"description"`
	Severity    AlertSeverity `json:"severity"`
	Timestamp   time.Time     `json:"timestamp"`
	Value       interface{}   `json:"value,omitempty"`
}

// AlertCallback is called when an alert is triggered.
type AlertCallback func(Alert)

// New creates a new monitoring instance.
func New() *Monitor {
	return &Monitor{
		systemMetrics: SystemMetrics{
			StartTime: time.Now(),
		},
		alertRules: make([]AlertRule, 0),
		alertCallbacks: make([]AlertCallback, 0),
	}
}

// RecordRepeatBatchCreated records a new repeated batch creation.
func (m *Monitor) RecordRepeatBatchCreated(batchID string, taskCount int, mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.repeatTaskMetrics.TotalRepeatedBatches++
	m.repeatTaskMetrics.TotalRepeatedTasks += int64(taskCount)
	m.repeatTaskMetrics.ActiveRepeatedBatches++
	m.repeatTaskMetrics.LastBatchCreatedAt = time.Now()
	m.repeatTaskMetrics.LastBatchID = batchID
	
	switch mode {
	case "counter":
		m.repeatTaskMetrics.CounterModeBatches++
	case "range":
		m.repeatTaskMetrics.RangeModeBatches++
	case "list":
		m.repeatTaskMetrics.ListModeBatches++
	}
	
	// Update average
	if m.repeatTaskMetrics.TotalRepeatedBatches > 0 {
		m.repeatTaskMetrics.AvgTasksPerBatch = float64(m.repeatTaskMetrics.TotalRepeatedTasks) / 
			float64(m.repeatTaskMetrics.TotalRepeatedBatches)
	}
}

// RecordRepeatBatchCompleted records completion of a repeated batch.
func (m *Monitor) RecordRepeatBatchCompleted(batchID string, completionTimeMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.repeatTaskMetrics.ActiveRepeatedBatches > 0 {
		m.repeatTaskMetrics.ActiveRepeatedBatches--
	}
	
	// Update average completion time (simple moving average)
	if m.repeatTaskMetrics.AvgBatchCompletionTimeMs == 0 {
		m.repeatTaskMetrics.AvgBatchCompletionTimeMs = completionTimeMs
	} else {
		m.repeatTaskMetrics.AvgBatchCompletionTimeMs = 
			(m.repeatTaskMetrics.AvgBatchCompletionTimeMs + completionTimeMs) / 2
	}
}

// RecordRepeatTaskCompleted records a repeated task completion.
func (m *Monitor) RecordRepeatTaskCompleted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.repeatTaskMetrics.CompletedRepeatedTasks++
}

// RecordRepeatTaskFailed records a repeated task failure.
func (m *Monitor) RecordRepeatTaskFailed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.repeatTaskMetrics.FailedRepeatedTasks++
}

// RecordRequest records an API request.
func (m *Monitor) RecordRequest() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.systemMetrics.TotalRequests++
}

// RecordError records an error.
func (m *Monitor) RecordError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.systemMetrics.TotalErrors++
}

// UpdateSystemMetrics updates system-level metrics.
func (m *Monitor) UpdateSystemMetrics(memoryMB float64, goroutines int, dbConns int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.systemMetrics.MemoryUsageMB = memoryMB
	m.systemMetrics.GoroutineCount = goroutines
	m.systemMetrics.DatabaseConnections = dbConns
	m.systemMetrics.Uptime = time.Since(m.systemMetrics.StartTime)
}

// GetRepeatTaskMetrics returns a snapshot of repeat task metrics.
func (m *Monitor) GetRepeatTaskMetrics() RepeatTaskMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.repeatTaskMetrics
}

// GetSystemMetrics returns a snapshot of system metrics.
func (m *Monitor) GetSystemMetrics() SystemMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.systemMetrics
}

// AddAlertRule adds an alert rule to the monitor.
func (m *Monitor) AddAlertRule(rule AlertRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertRules = append(m.alertRules, rule)
}

// RegisterAlertCallback registers a callback for alerts.
func (m *Monitor) RegisterAlertCallback(cb AlertCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertCallbacks = append(m.alertCallbacks, cb)
}

// CheckAlerts evaluates all alert rules and triggers callbacks.
func (m *Monitor) CheckAlerts(ctx context.Context) {
	m.mu.Lock()
	
	now := time.Now()
	alertsToFire := []Alert{}
	
	for i := range m.alertRules {
		rule := &m.alertRules[i]
		
		// Check cooldown
		if !rule.lastFired.IsZero() && now.Sub(rule.lastFired) < rule.Cooldown {
			continue
		}
		
		// Evaluate condition (unlock temporarily to avoid deadlock)
		m.mu.Unlock()
		shouldFire := rule.Condition(m)
		m.mu.Lock()
		
		if shouldFire {
			alert := Alert{
				RuleID:      rule.ID,
				RuleName:    rule.Name,
				Description: rule.Description,
				Severity:    rule.Severity,
				Timestamp:   now,
			}
			
			alertsToFire = append(alertsToFire, alert)
			rule.lastFired = now
		}
	}
	
	// Get callbacks while still locked
	callbacks := make([]AlertCallback, len(m.alertCallbacks))
	copy(callbacks, m.alertCallbacks)
	
	m.mu.Unlock()
	
	// Trigger callbacks outside of lock
	for _, alert := range alertsToFire {
		for _, cb := range callbacks {
			go cb(alert)
		}
	}
}

// GetPrometheusMetrics generates Prometheus-compatible metrics for repeated tasks.
func (m *Monitor) GetPrometheusMetrics() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	metrics := ""
	
	// Repeat task metrics
	metrics += "# HELP flowpilot_repeat_batches_total Total repeated batches created\n"
	metrics += "# TYPE flowpilot_repeat_batches_total counter\n"
	metrics += "flowpilot_repeat_batches_total " + formatInt64(m.repeatTaskMetrics.TotalRepeatedBatches) + "\n"
	
	metrics += "# HELP flowpilot_repeat_tasks_total Total tasks created from repeated batches\n"
	metrics += "# TYPE flowpilot_repeat_tasks_total counter\n"
	metrics += "flowpilot_repeat_tasks_total " + formatInt64(m.repeatTaskMetrics.TotalRepeatedTasks) + "\n"
	
	metrics += "# HELP flowpilot_repeat_batches_active Active repeated batches\n"
	metrics += "# TYPE flowpilot_repeat_batches_active gauge\n"
	metrics += "flowpilot_repeat_batches_active " + formatInt(m.repeatTaskMetrics.ActiveRepeatedBatches) + "\n"
	
	metrics += "# HELP flowpilot_repeat_tasks_completed_total Completed tasks from repeated batches\n"
	metrics += "# TYPE flowpilot_repeat_tasks_completed_total counter\n"
	metrics += "flowpilot_repeat_tasks_completed_total " + formatInt64(m.repeatTaskMetrics.CompletedRepeatedTasks) + "\n"
	
	metrics += "# HELP flowpilot_repeat_tasks_failed_total Failed tasks from repeated batches\n"
	metrics += "# TYPE flowpilot_repeat_tasks_failed_total counter\n"
	metrics += "flowpilot_repeat_tasks_failed_total " + formatInt64(m.repeatTaskMetrics.FailedRepeatedTasks) + "\n"
	
	metrics += "# HELP flowpilot_repeat_avg_tasks_per_batch Average tasks per repeated batch\n"
	metrics += "# TYPE flowpilot_repeat_avg_tasks_per_batch gauge\n"
	metrics += "flowpilot_repeat_avg_tasks_per_batch " + formatFloat(m.repeatTaskMetrics.AvgTasksPerBatch) + "\n"
	
	metrics += "# HELP flowpilot_repeat_batch_completion_ms Average batch completion time in milliseconds\n"
	metrics += "# TYPE flowpilot_repeat_batch_completion_ms gauge\n"
	metrics += "flowpilot_repeat_batch_completion_ms " + formatInt64(m.repeatTaskMetrics.AvgBatchCompletionTimeMs) + "\n"
	
	// Mode breakdown
	metrics += "# HELP flowpilot_repeat_batches_by_mode_total Repeated batches by mode\n"
	metrics += "# TYPE flowpilot_repeat_batches_by_mode_total counter\n"
	metrics += "flowpilot_repeat_batches_by_mode_total{mode=\"counter\"} " + formatInt64(m.repeatTaskMetrics.CounterModeBatches) + "\n"
	metrics += "flowpilot_repeat_batches_by_mode_total{mode=\"range\"} " + formatInt64(m.repeatTaskMetrics.RangeModeBatches) + "\n"
	metrics += "flowpilot_repeat_batches_by_mode_total{mode=\"list\"} " + formatInt64(m.repeatTaskMetrics.ListModeBatches) + "\n"
	
	// System metrics
	metrics += "# HELP flowpilot_system_uptime_seconds System uptime in seconds\n"
	metrics += "# TYPE flowpilot_system_uptime_seconds gauge\n"
	metrics += "flowpilot_system_uptime_seconds " + formatInt64(int64(m.systemMetrics.Uptime.Seconds())) + "\n"
	
	metrics += "# HELP flowpilot_system_requests_total Total API requests\n"
	metrics += "# TYPE flowpilot_system_requests_total counter\n"
	metrics += "flowpilot_system_requests_total " + formatInt64(m.systemMetrics.TotalRequests) + "\n"
	
	metrics += "# HELP flowpilot_system_errors_total Total errors\n"
	metrics += "# TYPE flowpilot_system_errors_total counter\n"
	metrics += "flowpilot_system_errors_total " + formatInt64(m.systemMetrics.TotalErrors) + "\n"
	
	metrics += "# HELP flowpilot_system_memory_mb Memory usage in megabytes\n"
	metrics += "# TYPE flowpilot_system_memory_mb gauge\n"
	metrics += "flowpilot_system_memory_mb " + formatFloat(m.systemMetrics.MemoryUsageMB) + "\n"
	
	metrics += "# HELP flowpilot_system_goroutines Goroutine count\n"
	metrics += "# TYPE flowpilot_system_goroutines gauge\n"
	metrics += "flowpilot_system_goroutines " + formatInt(m.systemMetrics.GoroutineCount) + "\n"
	
	return metrics
}

// Helper formatting functions
func formatInt64(v int64) string {
	return fmt.Sprintf("%d", v)
}

func formatInt(v int) string {
	return fmt.Sprintf("%d", v)
}

func formatFloat(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

// HealthStatus represents the system health status.
type HealthStatus struct {
	Status            string                `json:"status"`
	Timestamp         time.Time             `json:"timestamp"`
	Uptime            time.Duration         `json:"uptime"`
	Components        map[string]Component  `json:"components"`
	RepeatTaskMetrics RepeatTaskMetrics     `json:"repeatTaskMetrics"`
	SystemMetrics     SystemMetrics         `json:"systemMetrics"`
}

// Component represents a health check component.
type Component struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// GetHealthStatus returns comprehensive health status.
func (m *Monitor) GetHealthStatus(ctx context.Context, components map[string]func() Component) HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	status := HealthStatus{
		Status:            "healthy",
		Timestamp:         time.Now(),
		Uptime:            time.Since(m.systemMetrics.StartTime),
		Components:        make(map[string]Component),
		RepeatTaskMetrics: m.repeatTaskMetrics,
		SystemMetrics:     m.systemMetrics,
	}
	
	// Check components
	for name, check := range components {
		comp := check()
		status.Components[name] = comp
		if comp.Status != "healthy" {
			status.Status = "degraded"
		}
	}
	
	return status
}
