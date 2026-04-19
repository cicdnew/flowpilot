# Monitoring and Logging Guide - FlowPilot

## 📊 Overview

FlowPilot includes comprehensive monitoring and logging capabilities to track system health, performance, and repeated task execution.

---

## Table of Contents

1. [Architecture](#architecture)
2. [Metrics Collection](#metrics-collection)
3. [Structured Logging](#structured-logging)
4. [Health Checks](#health-checks)
5. [Alerting System](#alerting-system)
6. [Prometheus Integration](#prometheus-integration)
7. [Monitoring Dashboard](#monitoring-dashboard)
8. [API Reference](#api-reference)

---

## Architecture

### Components

```
┌─────────────────────────────────────────────────────┐
│                    FlowPilot App                    │
├─────────────────────────────────────────────────────┤
│                                                     │
│  ┌──────────────┐  ┌───────────────────────────┐  │
│  │   Monitor    │  │  StructuredLogger         │  │
│  │              │  │                           │  │
│  │ - Metrics    │  │ - Log Entries (10k max)   │  │
│  │ - Alerts     │  │ - Level Filtering         │  │
│  │ - Health     │  │ - Context Tagging         │  │
│  └──────────────┘  └───────────────────────────┘  │
│         │                      │                   │
│         └──────────┬───────────┘                   │
│                    │                               │
│         ┌──────────▼──────────┐                    │
│         │   Monitoring APIs   │                    │
│         │                     │                    │
│         │ - GetMonitoringMetrics()               │
│         │ - GetSystemHealth()                    │
│         │ - GetRecentLogs()                      │
│         │ - GetEnhancedPrometheusMetrics()       │
│         └─────────────────────┘                    │
└─────────────────────────────────────────────────────┘
```

---

## Metrics Collection

### Repeat Task Metrics

The monitoring system tracks comprehensive metrics for repeated tasks:

```go
type RepeatTaskMetrics struct {
    TotalRepeatedBatches     int64   // Total batches created
    TotalRepeatedTasks       int64   // Total tasks created
    ActiveRepeatedBatches    int     // Currently running batches
    CompletedRepeatedTasks   int64   // Successfully completed tasks
    FailedRepeatedTasks      int64   // Failed tasks
    AvgTasksPerBatch         float64 // Average tasks per batch
    AvgBatchCompletionTimeMs int64   // Average completion time
    
    // Per-mode breakdown
    CounterModeBatches int64
    RangeModeBatches   int64
    ListModeBatches    int64
}
```

### System Metrics

System-wide health and performance metrics:

```go
type SystemMetrics struct {
    Uptime              time.Duration // System uptime
    TotalRequests       int64         // Total API requests
    TotalErrors         int64         // Total errors
    MemoryUsageMB       float64       // Memory usage in MB
    GoroutineCount      int           // Active goroutines
    DatabaseConnections int           // DB connection count
}
```

### Recording Metrics

Metrics are automatically recorded when:

```go
// On repeated batch creation
monitor.RecordRepeatBatchCreated(batchID, taskCount, mode)

// On batch completion
monitor.RecordRepeatBatchCompleted(batchID, completionTimeMs)

// On task completion/failure
monitor.RecordRepeatTaskCompleted()
monitor.RecordRepeatTaskFailed()

// On requests/errors
monitor.RecordRequest()
monitor.RecordError()
```

---

## Structured Logging

### Log Levels

FlowPilot supports four log levels:

- **Debug**: Detailed debugging information
- **Info**: General informational messages
- **Warning**: Warning messages (degraded state)
- **Error**: Error messages (failures)

### Creating Logs

```go
// Basic logging
logger.Info(ctx, "Task created", 
    slog.String("taskId", taskID),
    slog.Int("priority", 5))

logger.Error(ctx, "Task failed", err,
    slog.String("taskId", taskID),
    slog.String("reason", "timeout"))

// Contextual logging
taskLogger := logger.WithTaskID(taskID)
taskLogger.Info(ctx, "Processing step 1")
taskLogger.Info(ctx, "Processing step 2")

batchLogger := logger.WithBatchID(batchID)
batchLogger.Info(ctx, "Batch started")
```

### Log Entry Structure

```json
{
  "timestamp": "2026-04-19T11:52:00Z",
  "level": "info",
  "message": "Repeated batch created",
  "context": {
    "mode": "range",
    "taskCount": "100"
  },
  "batchId": "batch-abc123"
}
```

### Retrieving Logs

```go
// Get recent logs
recentLogs := app.GetRecentLogs(50)

// Filter by level
errorLogs := app.GetLogsByLevel("error", 20)

// Filter by task
taskLogs := app.GetLogsByTaskID(taskID, 100)

// Filter by batch
batchLogs := app.GetLogsByBatchID(batchID, 100)

// Get statistics
stats := app.GetLogStats()
```

### Log Retention

- In-memory storage: **10,000 entries** (configurable)
- Oldest entries automatically removed when limit reached
- Logs persisted to stderr via slog (can be redirected to file)

---

## Health Checks

### Health Status

The system provides comprehensive health status:

```go
type HealthStatus struct {
    Status            string                // "healthy", "degraded", "unhealthy"
    Timestamp         time.Time
    Uptime            time.Duration
    Components        map[string]Component
    RepeatTaskMetrics RepeatTaskMetrics
    SystemMetrics     SystemMetrics
}
```

### Component Health Checks

Four key components are monitored:

1. **Database**: Connection health and query responsiveness
2. **Queue**: Worker status and metrics validity
3. **Browser Pool**: Resource utilization
4. **Proxy Manager**: Availability

### Checking Health

```go
// Via API
health := app.GetSystemHealth()

// Check status
if health.Status == "healthy" {
    // All components healthy
} else if health.Status == "degraded" {
    // Some components unhealthy
}

// Check individual components
dbHealth := health.Components["database"]
if dbHealth.Status != "healthy" {
    log.Printf("Database issue: %s", dbHealth.Message)
}
```

### Health Endpoint

**HTTP endpoint** (when metrics server enabled):

```bash
GET /health
```

Response:
```json
{
  "status": "healthy",
  "timestamp": "2026-04-19T11:52:00Z",
  "uptime": 3600000000000,
  "components": {
    "database": {
      "status": "healthy",
      "message": "Connected"
    },
    "queue": {
      "status": "healthy",
      "message": "Running"
    }
  }
}
```

---

## Alerting System

### Built-in Alerts

FlowPilot includes default alert rules:

1. **High Error Rate**: >10% error rate
2. **High Memory Usage**: >1GB memory
3. **High Goroutine Count**: >1000 goroutines
4. **Repeat Task High Failure Rate**: >20% failure rate

### Alert Structure

```go
type Alert struct {
    RuleID      string        // Unique rule identifier
    RuleName    string        // Human-readable name
    Description string        // Alert description
    Severity    AlertSeverity // info, warning, critical
    Timestamp   time.Time     // When alert fired
}
```

### Alert Severities

- **Info**: Informational alerts
- **Warning**: Potentially problematic conditions
- **Critical**: Serious issues requiring attention

### Creating Custom Alerts

```go
monitor.AddAlertRule(monitoring.AlertRule{
    ID:          "custom-alert",
    Name:        "Custom Alert",
    Description: "Custom condition detected",
    Severity:    monitoring.AlertSeverityWarning,
    Cooldown:    5 * time.Minute,
    Condition: func(m *monitoring.Monitor) bool {
        metrics := m.GetRepeatTaskMetrics()
        return metrics.ActiveRepeatedBatches > 10
    },
})
```

### Alert Callbacks

Register callbacks to handle alerts:

```go
monitor.RegisterAlertCallback(func(alert monitoring.Alert) {
    // Send notification
    fmt.Printf("ALERT: %s - %s\n", alert.RuleName, alert.Description)
    
    // Send to external service
    // sendToSlack(alert)
    // sendToEmail(alert)
})
```

### Alert Cooldown

Alerts include cooldown periods to prevent alert spam:

```go
Cooldown: 5 * time.Minute  // Won't fire again for 5 minutes
```

---

## Prometheus Integration

### Metrics Export

FlowPilot exports metrics in Prometheus format:

```bash
GET /metrics
```

### Available Metrics

#### Repeat Task Metrics
```prometheus
# HELP flowpilot_repeat_batches_total Total repeated batches created
# TYPE flowpilot_repeat_batches_total counter
flowpilot_repeat_batches_total 42

# HELP flowpilot_repeat_tasks_total Total tasks from repeated batches
# TYPE flowpilot_repeat_tasks_total counter
flowpilot_repeat_tasks_total 4200

# HELP flowpilot_repeat_batches_active Active repeated batches
# TYPE flowpilot_repeat_batches_active gauge
flowpilot_repeat_batches_active 5

# HELP flowpilot_repeat_tasks_completed_total Completed repeated tasks
# TYPE flowpilot_repeat_tasks_completed_total counter
flowpilot_repeat_tasks_completed_total 3800

# HELP flowpilot_repeat_tasks_failed_total Failed repeated tasks
# TYPE flowpilot_repeat_tasks_failed_total counter
flowpilot_repeat_tasks_failed_total 150

# HELP flowpilot_repeat_batches_by_mode_total Batches by mode
# TYPE flowpilot_repeat_batches_by_mode_total counter
flowpilot_repeat_batches_by_mode_total{mode="counter"} 10
flowpilot_repeat_batches_by_mode_total{mode="range"} 25
flowpilot_repeat_batches_by_mode_total{mode="list"} 7
```

#### System Metrics
```prometheus
# HELP flowpilot_system_uptime_seconds System uptime
# TYPE flowpilot_system_uptime_seconds gauge
flowpilot_system_uptime_seconds 3600

# HELP flowpilot_system_memory_mb Memory usage in megabytes
# TYPE flowpilot_system_memory_mb gauge
flowpilot_system_memory_mb 512.50

# HELP flowpilot_system_goroutines Goroutine count
# TYPE flowpilot_system_goroutines gauge
flowpilot_system_goroutines 95
```

### Prometheus Configuration

Add to `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'flowpilot'
    static_configs:
      - targets: ['localhost:9090']  # Default metrics server port
    scrape_interval: 30s
```

### Grafana Dashboard

Sample Grafana queries:

```promql
# Repeat task success rate
100 * (flowpilot_repeat_tasks_completed_total / 
       (flowpilot_repeat_tasks_completed_total + flowpilot_repeat_tasks_failed_total))

# Active batches over time
flowpilot_repeat_batches_active

# Memory usage trend
flowpilot_system_memory_mb

# Task completion rate (tasks/minute)
rate(flowpilot_repeat_tasks_completed_total[1m]) * 60
```

---

## Monitoring Dashboard

### Accessing the Dashboard

The monitoring dashboard is available in the FlowPilot UI:

1. Launch FlowPilot: `wails dev`
2. Navigate to **Monitoring** tab
3. View real-time metrics and logs

### Dashboard Sections

#### 1. System Health
- Overall health status indicator
- Component health cards
- Uptime display

#### 2. Repeated Task Metrics
- Total batches and tasks
- Active batch count
- Success/failure counts
- Average metrics
- Mode breakdown (counter/range/list)

#### 3. System Metrics
- Memory usage
- Goroutine count
- Database connections
- Request and error counts

#### 4. Recent Logs
- Color-coded log entries
- Level filtering
- Task/batch context
- Error details

### Auto-Refresh

Dashboard auto-refreshes every **10 seconds** to show current state.

---

## API Reference

### Monitoring APIs

```typescript
// Get repeat task metrics
GetMonitoringMetrics(): Promise<RepeatTaskMetrics>

// Get system health
GetSystemHealth(): Promise<HealthStatus>

// Get recent logs
GetRecentLogs(limit: number): Promise<LogEntry[]>

// Get logs by level
GetLogsByLevel(level: string, limit: number): Promise<LogEntry[]>

// Get logs by task ID
GetLogsByTaskID(taskID: string, limit: number): Promise<LogEntry[]>

// Get logs by batch ID
GetLogsByBatchID(batchID: string, limit: number): Promise<LogEntry[]>

// Get log statistics
GetLogStats(): Promise<LogStats>

// Clear in-memory logs
ClearLogs(): Promise<void>

// Get enhanced Prometheus metrics
GetEnhancedPrometheusMetrics(): Promise<string>
```

### Go API

```go
// App methods
func (a *App) GetMonitoringMetrics() monitoring.RepeatTaskMetrics
func (a *App) GetSystemHealth() monitoring.HealthStatus
func (a *App) GetRecentLogs(limit int) []monitoring.LogEntry
func (a *App) GetLogsByLevel(level string, limit int) []monitoring.LogEntry
func (a *App) GetLogsByTaskID(taskID string, limit int) []monitoring.LogEntry
func (a *App) GetLogsByBatchID(batchID string, limit int) []monitoring.LogEntry
func (a *App) GetLogStats() monitoring.LogStats
func (a *App) ClearLogs()
func (a *App) GetEnhancedPrometheusMetrics() string

// Monitor methods
func (m *Monitor) RecordRepeatBatchCreated(batchID string, taskCount int, mode string)
func (m *Monitor) RecordRepeatBatchCompleted(batchID string, completionTimeMs int64)
func (m *Monitor) RecordRepeatTaskCompleted()
func (m *Monitor) RecordRepeatTaskFailed()
func (m *Monitor) AddAlertRule(rule AlertRule)
func (m *Monitor) RegisterAlertCallback(cb AlertCallback)

// Logger methods
func (l *StructuredLogger) Info(ctx context.Context, message string, attrs ...slog.Attr)
func (l *StructuredLogger) Warning(ctx context.Context, message string, attrs ...slog.Attr)
func (l *StructuredLogger) Error(ctx context.Context, message string, err error, attrs ...slog.Attr)
func (l *StructuredLogger) WithTaskID(taskID string) *ContextLogger
func (l *StructuredLogger) WithBatchID(batchID string) *ContextLogger
```

---

## Best Practices

### 1. Log Appropriately

```go
// Use appropriate log levels
logger.Debug(ctx, "Detailed debug info")      // Development only
logger.Info(ctx, "Normal operation")           // General events
logger.Warning(ctx, "Potential issue")         // Warnings
logger.Error(ctx, "Failure occurred", err)     // Errors
```

### 2. Add Context

```go
// Always include context
logger.Info(ctx, "Batch created",
    slog.String("batchId", batchID),
    slog.Int("taskCount", count),
    slog.String("mode", mode))
```

### 3. Monitor Critical Metrics

Watch for:
- High failure rates (>20%)
- Memory growth trends
- Goroutine leaks
- Slow batch completion times

### 4. Set Up Alerts

Configure alerts for:
- System health degradation
- Resource exhaustion
- Repeated task failures
- Queue backlogs

### 5. Use Dashboards

- Monitor real-time in UI
- Export to Grafana for historical analysis
- Set up alert notifications

---

## Troubleshooting

### High Memory Usage

```go
// Check metrics
metrics := app.GetSystemHealth()
if metrics.SystemMetrics.MemoryUsageMB > 1024 {
    // Investigate memory leak
    // Check goroutine count
    // Review active tasks
}
```

### Logs Not Appearing

1. Check log level configuration
2. Verify logger initialization
3. Check in-memory limit (10k entries)
4. Review stderr output

### Alerts Not Firing

1. Check alert rule conditions
2. Verify cooldown periods haven't expired
3. Check alert callbacks are registered
4. Review metrics to confirm conditions

### Dashboard Not Updating

1. Verify auto-refresh is enabled (10s interval)
2. Check API connectivity
3. Review browser console for errors
4. Restart FlowPilot if needed

---

## Next Steps

- Configure Prometheus for long-term metrics storage
- Set up Grafana dashboards for visualization
- Configure alert notifications (Slack, email, etc.)
- Implement custom metrics for specific use cases
- Export logs to external logging system (ELK, Loki, etc.)

---

**The monitoring system provides complete visibility into FlowPilot operations!** 📊

