package main

import (
	"context"
	"log/slog"
	"runtime"
	"time"

	"flowpilot/internal/monitoring"
)

// GetMonitoringMetrics returns comprehensive monitoring metrics.
func (a *App) GetMonitoringMetrics() monitoring.RepeatTaskMetrics {
	if a.monitor == nil {
		return monitoring.RepeatTaskMetrics{}
	}
	return a.monitor.GetRepeatTaskMetrics()
}

// GetSystemHealth returns system health status.
func (a *App) GetSystemHealth() monitoring.HealthStatus {
	if a.monitor == nil {
		return monitoring.HealthStatus{
			Status:    "unknown",
			Timestamp: time.Now(),
		}
	}
	
	components := map[string]func() monitoring.Component{
		"database": func() monitoring.Component {
			if a.db == nil {
				return monitoring.Component{Status: "unhealthy", Message: "Database not initialized"}
			}
			// Try a simple query
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err := a.db.ListTasks(ctx)
			if err != nil {
				return monitoring.Component{Status: "unhealthy", Message: err.Error()}
			}
			return monitoring.Component{Status: "healthy", Message: "Connected"}
		},
		"queue": func() monitoring.Component {
			if a.queue == nil {
				return monitoring.Component{Status: "unhealthy", Message: "Queue not initialized"}
			}
			metrics := a.queue.Metrics()
			if metrics.Running < 0 || metrics.Queued < 0 {
				return monitoring.Component{Status: "degraded", Message: "Invalid metrics"}
			}
			return monitoring.Component{Status: "healthy", Message: "Running"}
		},
		"browser_pool": func() monitoring.Component {
			if a.pool == nil {
				return monitoring.Component{Status: "degraded", Message: "Browser pool not initialized"}
			}
			stats := a.pool.Stats()
			if stats.TotalBrowsers > stats.MaxBrowsers {
				return monitoring.Component{Status: "degraded", Message: "Browser pool over capacity"}
			}
			return monitoring.Component{Status: "healthy"}
		},
		"proxy_manager": func() monitoring.Component {
			if a.proxyManager == nil {
				return monitoring.Component{Status: "degraded", Message: "Proxy manager not available"}
			}
			return monitoring.Component{Status: "healthy"}
		},
	}
	
	return a.monitor.GetHealthStatus(context.Background(), components)
}

// GetRecentLogs returns recent log entries.
func (a *App) GetRecentLogs(limit int) []monitoring.LogEntry {
	if a.logger == nil {
		return []monitoring.LogEntry{}
	}
	return a.logger.GetRecentLogs(limit)
}

// GetLogsByLevel returns logs filtered by level.
func (a *App) GetLogsByLevel(level string, limit int) []monitoring.LogEntry {
	if a.logger == nil {
		return []monitoring.LogEntry{}
	}
	return a.logger.GetLogsByLevel(monitoring.LogLevel(level), limit)
}

// GetLogsByTaskID returns logs for a specific task.
func (a *App) GetLogsByTaskID(taskID string, limit int) []monitoring.LogEntry {
	if a.logger == nil {
		return []monitoring.LogEntry{}
	}
	return a.logger.GetLogsByTaskID(taskID, limit)
}

// GetLogsByBatchID returns logs for a specific batch.
func (a *App) GetLogsByBatchID(batchID string, limit int) []monitoring.LogEntry {
	if a.logger == nil {
		return []monitoring.LogEntry{}
	}
	return a.logger.GetLogsByBatchID(batchID, limit)
}

// GetLogStats returns log statistics.
func (a *App) GetLogStats() monitoring.LogStats {
	if a.logger == nil {
		return monitoring.LogStats{}
	}
	return a.logger.GetLogStats()
}

// ClearLogs clears in-memory logs.
func (a *App) ClearLogs() {
	if a.logger != nil {
		a.logger.Clear()
	}
}

// startMonitoringLoop starts background monitoring updates.
func (a *App) startMonitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.updateSystemMetrics()
			a.checkAlerts()
		}
	}
}

// updateSystemMetrics updates system-level metrics.
func (a *App) updateSystemMetrics() {
	if a.monitor == nil {
		return
	}
	
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	memoryMB := float64(m.Alloc) / 1024 / 1024
	goroutines := runtime.NumGoroutine()
	
	// Database connections - estimate based on queue metrics
	dbConns := 0
	if a.queue != nil {
		metrics := a.queue.Metrics()
		dbConns = metrics.Running + 1 // Running tasks + main connection
	}
	
	a.monitor.UpdateSystemMetrics(memoryMB, goroutines, dbConns)
}

// checkAlerts evaluates alert rules.
func (a *App) checkAlerts() {
	if a.monitor == nil {
		return
	}
	a.monitor.CheckAlerts(context.Background())
}

// setupDefaultAlerts configures default alerting rules.
func (a *App) setupDefaultAlerts() {
	if a.monitor == nil {
		return
	}
	
	// High error rate alert
	a.monitor.AddAlertRule(monitoring.AlertRule{
		ID:          "high-error-rate",
		Name:        "High Error Rate",
		Description: "Error rate exceeds 10% of total requests",
		Severity:    monitoring.AlertSeverityWarning,
		Cooldown:    5 * time.Minute,
		Condition: func(m *monitoring.Monitor) bool {
			metrics := m.GetSystemMetrics()
			if metrics.TotalRequests == 0 {
				return false
			}
			errorRate := float64(metrics.TotalErrors) / float64(metrics.TotalRequests)
			return errorRate > 0.1
		},
	})
	
	// High memory usage alert
	a.monitor.AddAlertRule(monitoring.AlertRule{
		ID:          "high-memory-usage",
		Name:        "High Memory Usage",
		Description: "Memory usage exceeds 1GB",
		Severity:    monitoring.AlertSeverityWarning,
		Cooldown:    10 * time.Minute,
		Condition: func(m *monitoring.Monitor) bool {
			metrics := m.GetSystemMetrics()
			return metrics.MemoryUsageMB > 1024
		},
	})
	
	// High goroutine count alert
	a.monitor.AddAlertRule(monitoring.AlertRule{
		ID:          "high-goroutine-count",
		Name:        "High Goroutine Count",
		Description: "Goroutine count exceeds 1000",
		Severity:    monitoring.AlertSeverityWarning,
		Cooldown:    10 * time.Minute,
		Condition: func(m *monitoring.Monitor) bool {
			metrics := m.GetSystemMetrics()
			return metrics.GoroutineCount > 1000
		},
	})
	
	// Repeated task failure rate alert
	a.monitor.AddAlertRule(monitoring.AlertRule{
		ID:          "repeat-task-high-failure-rate",
		Name:        "Repeated Task High Failure Rate",
		Description: "Repeated task failure rate exceeds 20%",
		Severity:    monitoring.AlertSeverityCritical,
		Cooldown:    5 * time.Minute,
		Condition: func(m *monitoring.Monitor) bool {
			metrics := m.GetRepeatTaskMetrics()
			total := metrics.CompletedRepeatedTasks + metrics.FailedRepeatedTasks
			if total == 0 {
				return false
			}
			failureRate := float64(metrics.FailedRepeatedTasks) / float64(total)
			return failureRate > 0.2
		},
	})
	
	// Register alert callback for logging
	a.monitor.RegisterAlertCallback(func(alert monitoring.Alert) {
		if a.logger != nil {
			a.logger.Warning(
				context.Background(),
				"Alert triggered: "+alert.RuleName,
				slog.String("alertId", alert.RuleID),
				slog.String("severity", string(alert.Severity)),
				slog.String("description", alert.Description),
			)
		}
	})
}

// logRepeatTaskCreation logs repeated task creation.
func (a *App) logRepeatTaskCreation(batchID string, taskCount int, mode string) {
	if a.logger != nil {
		a.logger.Info(
			context.Background(),
			"Repeated batch created",
			slog.String("batchId", batchID),
			slog.Int("taskCount", taskCount),
			slog.String("mode", mode),
		)
	}
	
	if a.monitor != nil {
		a.monitor.RecordRepeatBatchCreated(batchID, taskCount, mode)
	}
}

// logRepeatBatchCompletion logs repeated batch completion.
func (a *App) logRepeatBatchCompletion(batchID string, completionTimeMs int64) {
	if a.logger != nil {
		a.logger.Info(
			context.Background(),
			"Repeated batch completed",
			slog.String("batchId", batchID),
			slog.Int64("completionTimeMs", completionTimeMs),
		)
	}
	
	if a.monitor != nil {
		a.monitor.RecordRepeatBatchCompleted(batchID, completionTimeMs)
	}
}

// GetEnhancedPrometheusMetrics returns combined Prometheus metrics.
func (a *App) GetEnhancedPrometheusMetrics() string {
	// Get base metrics
	baseMetrics := a.GetPrometheusMetrics()
	
	// Get monitoring metrics
	monitoringMetrics := ""
	if a.monitor != nil {
		monitoringMetrics = a.monitor.GetPrometheusMetrics()
	}
	
	return baseMetrics + "\n" + monitoringMetrics
}
