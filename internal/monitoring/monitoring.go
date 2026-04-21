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
	firing            map[string]*Alert

	// Step duration tracking (P0)
	stepDurations  []int64
	maxStepSamples int

	// Error tracking (P0)
	errorContexts   []string // Serialized ErrorContext for storage
	maxErrorSamples int
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
	Metric      string
	Cond        string
	Threshold   float64
	Window      int
	Cooldown    time.Duration
	Enabled     bool
	WebhookURL  string
	Severity    AlertSeverity
	Condition   func(*Monitor) bool
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
	Metric      string        `json:"metric"`
	Condition   string        `json:"condition"`
	Threshold   float64       `json:"threshold"`
	Value       float64       `json:"value"`
}

// AlertCallback is called when an alert is triggered or resolved.
type AlertCallback func(alert Alert, fired bool)

// New creates a new monitoring instance.
func New() *Monitor {
	return &Monitor{
		systemMetrics: SystemMetrics{
			StartTime: time.Now(),
		},
		alertRules:      make([]AlertRule, 0),
		alertCallbacks:  make([]AlertCallback, 0),
		firing:          make(map[string]*Alert),
		stepDurations:   make([]int64, 0, 1000),
		maxStepSamples:  1000,
		errorContexts:   make([]string, 0, 100),
		maxErrorSamples: 100,
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

// RecordStepDuration records the duration of a single step execution (P0).
// This populates the AvgStepDurationMs field in QueueMetrics.
func (m *Monitor) RecordStepDuration(durationMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stepDurations = append(m.stepDurations, durationMs)

	// Keep only the most recent samples to avoid unbounded growth
	if len(m.stepDurations) > m.maxStepSamples {
		m.stepDurations = m.stepDurations[len(m.stepDurations)-m.maxStepSamples:]
	}
}

// RecordErrorContext records an error with detailed context information (P0).
// This is called whenever an error occurs during task execution.
func (m *Monitor) RecordErrorContext(errCtxJSON string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errorContexts = append(m.errorContexts, errCtxJSON)

	// Keep only the most recent error samples
	if len(m.errorContexts) > m.maxErrorSamples {
		m.errorContexts = m.errorContexts[len(m.errorContexts)-m.maxErrorSamples:]
	}
}

// GetAvgStepDuration returns the average step duration in milliseconds.
// Returns 0 if no samples have been recorded.
func (m *Monitor) GetAvgStepDuration() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.stepDurations) == 0 {
		return 0
	}

	var sum int64
	for _, d := range m.stepDurations {
		sum += d
	}
	return float64(sum) / float64(len(m.stepDurations))
}

// GetRecentErrors returns the most recent error contexts (up to limit).
func (m *Monitor) GetRecentErrors(limit int) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || len(m.errorContexts) == 0 {
		return nil
	}

	start := len(m.errorContexts) - limit
	if start < 0 {
		start = 0
	}

	result := make([]string, len(m.errorContexts[start:]))
	copy(result, m.errorContexts[start:])
	return result
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
// For backward compatibility, rules with Enabled=false are treated as enabled by default.
func (m *Monitor) AddAlertRule(rule AlertRule) {
	if !rule.Enabled {
		rule.Enabled = true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertRules = append(m.alertRules, rule)
}

// SetAlertRules replaces all alert rules atomically.
func (m *Monitor) SetAlertRules(rules []AlertRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertRules = rules
}

// GetAlertRules returns a copy of current alert rules.
func (m *Monitor) GetAlertRules() []AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rules := make([]AlertRule, len(m.alertRules))
	copy(rules, m.alertRules)
	return rules
}

// RemoveAlertRule removes a rule by ID.
func (m *Monitor) RemoveAlertRule(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, rule := range m.alertRules {
		if rule.ID == id {
			m.alertRules = append(m.alertRules[:i], m.alertRules[i+1:]...)
			break
		}
	}
}

// RegisterAlertCallback registers a callback for alerts.
func (m *Monitor) RegisterAlertCallback(cb AlertCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertCallbacks = append(m.alertCallbacks, cb)
}

// GetMetricValue returns the current numeric value of a named metric.
// Supported metric names: totalRequests, totalErrors, memoryUsageMb, goroutineCount,
// totalRepeatedBatches, totalRepeatedTasks, completedRepeatedTasks, failedRepeatedTasks,
// avgStepDurationMs, etc.
func (m *Monitor) GetMetricValue(metric string) (float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch metric {
	// Repeat task metrics
	case "totalRepeatedBatches":
		return float64(m.repeatTaskMetrics.TotalRepeatedBatches), true
	case "totalRepeatedTasks":
		return float64(m.repeatTaskMetrics.TotalRepeatedTasks), true
	case "activeRepeatedBatches":
		return float64(m.repeatTaskMetrics.ActiveRepeatedBatches), true
	case "completedRepeatedTasks":
		return float64(m.repeatTaskMetrics.CompletedRepeatedTasks), true
	case "failedRepeatedTasks":
		return float64(m.repeatTaskMetrics.FailedRepeatedTasks), true
	case "avgTasksPerBatch":
		return m.repeatTaskMetrics.AvgTasksPerBatch, true
	case "avgBatchCompletionTimeMs":
		return float64(m.repeatTaskMetrics.AvgBatchCompletionTimeMs), true
	// System metrics
	case "uptimeSeconds":
		return m.systemMetrics.Uptime.Seconds(), true
	case "totalRequests":
		return float64(m.systemMetrics.TotalRequests), true
	case "totalErrors":
		return float64(m.systemMetrics.TotalErrors), true
	case "memoryUsageMb":
		return m.systemMetrics.MemoryUsageMB, true
	case "goroutineCount":
		return float64(m.systemMetrics.GoroutineCount), true
	case "databaseConnections":
		return float64(m.systemMetrics.DatabaseConnections), true
	case "requestsPerMinute":
		return m.systemMetrics.RequestsPerMinute, true
	case "errorsPerMinute":
		return m.systemMetrics.ErrorsPerMinute, true
	case "tasksPerMinute":
		return m.systemMetrics.TasksPerMinute, true
	case "avgStepDurationMs":
		if len(m.stepDurations) == 0 {
			return 0, true
		}
		var sum int64
		for _, d := range m.stepDurations {
			sum += d
		}
		return float64(sum) / float64(len(m.stepDurations)), true
	default:
		return 0, false
	}
}

// CheckAlerts evaluates all alert rules and triggers callbacks.
func (m *Monitor) CheckAlerts(ctx context.Context) {
	m.mu.Lock()
	now := time.Now()
	newFirings := make(map[string]*Alert) // ruleID -> alert

	// Evaluate each rule
	for i := range m.alertRules {
		rule := &m.alertRules[i]
		if !rule.Enabled {
			continue
		}
		// Cooldown check
		if !rule.lastFired.IsZero() && now.Sub(rule.lastFired) < rule.Cooldown {
			continue
		}
		// Evaluate condition
		m.mu.Unlock()
		shouldFire := rule.Condition(m)
		m.mu.Lock()

		if shouldFire {
			// Get metric value for this rule
			val, ok := m.GetMetricValue(rule.Metric)
			if !ok {
				// Cannot compute metric, skip
				continue
			}
			alert := Alert{
				RuleID:      rule.ID,
				RuleName:    rule.Name,
				Description: rule.Description,
				Severity:    rule.Severity,
				Timestamp:   now,
				Metric:      rule.Metric,
				Condition:   rule.Cond,
				Threshold:   rule.Threshold,
				Value:       val,
			}
			newFirings[rule.ID] = &alert
			rule.lastFired = now
		}
	}

	// Determine resolved alerts
	resolved := []*Alert{}
	for ruleID, alert := range m.firing {
		if _, still := newFirings[ruleID]; !still {
			resolved = append(resolved, alert)
		}
	}

	// Update firing map
	m.firing = newFirings

	// Copy callbacks
	callbacks := make([]AlertCallback, len(m.alertCallbacks))
	copy(callbacks, m.alertCallbacks)

	m.mu.Unlock()

	// Fire new alerts
	for _, alert := range newFirings {
		for _, cb := range callbacks {
			go cb(*alert, true)
		}
	}
	// Fire resolved alerts
	for _, alert := range resolved {
		for _, cb := range callbacks {
			go cb(*alert, false)
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
	Status            string               `json:"status"`
	Timestamp         time.Time            `json:"timestamp"`
	Uptime            time.Duration        `json:"uptime"`
	Components        map[string]Component `json:"components"`
	RepeatTaskMetrics RepeatTaskMetrics    `json:"repeatTaskMetrics"`
	SystemMetrics     SystemMetrics        `json:"systemMetrics"`
}

// Component represents a health check component.
type Component struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// GetHealthStatus returns comprehensive health status.
func (m *Monitor) GetHealthStatus(ctx context.Context, checks map[string]func() Component) HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uptime := time.Since(m.systemMetrics.StartTime)
	components := make(map[string]Component, len(checks))

	status := "healthy"
	for name, check := range checks {
		comp := check()
		components[name] = comp
		if comp.Status == "unhealthy" {
			status = "unhealthy"
		} else if comp.Status == "degraded" && status != "unhealthy" {
			status = "degraded"
		}
	}

	return HealthStatus{
		Status:            status,
		Timestamp:         time.Now(),
		Uptime:            uptime,
		Components:        components,
		RepeatTaskMetrics: m.repeatTaskMetrics,
		SystemMetrics:     m.systemMetrics,
	}
}

// GetFiring returns a copy of the current firing map (ruleID -> alert).
func (m *Monitor) GetFiring() map[string]*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	copyMap := make(map[string]*Alert, len(m.firing))
	for k, v := range m.firing {
		alertCopy := *v
		copyMap[k] = &alertCopy
	}
	return copyMap
}
