package main

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"flowpilot/internal/models"
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

// checkAlerts evaluates alert rules and persists firings.
func (a *App) checkAlerts() {
	if a.monitor == nil || a.db == nil {
		return
	}
	ctx := context.Background()

	// Get current active alerts from DB
	activeDB, _ := a.db.ListActiveAlerts(ctx)
	activeDBMap := make(map[string]*models.AlertFiring)
	for _, f := range activeDB {
		activeDBMap[f.RuleID] = &f
	}

	// Evaluate using monitor (this updates in-memory firing state)
	a.monitor.CheckAlerts(ctx)

	// Get updated firing map
	currentFiring := a.monitor.GetFiring()

	// Persist new firings and resolve old ones
	now := time.Now()
	for ruleID, alert := range currentFiring {
		if _, ok := activeDBMap[ruleID]; ok {
			// Already in DB, skip
			continue
		}
		firing := models.AlertFiring{
			ID:        ruleID + "-" + now.Format("20060102150405.000"),
			RuleID:    ruleID,
			RuleName:  alert.RuleName,
			Severity:  models.AlertSeverity(alert.Severity),
			Value:     alert.Value,
			Threshold: alert.Threshold,
			FiredAt:   now,
			Notified:  false,
		}
		if err := a.db.SaveAlertFiring(ctx, firing); err != nil {
			a.logger.Error(ctx, "save alert firing: %v", err)
		} else {
			// Emit Wails event
			safeWailsEmit(a.ctx, "alert:fired", map[string]any{
				"ruleId":    ruleID,
				"ruleName":  alert.RuleName,
				"severity":  alert.Severity,
				"value":     alert.Value,
				"threshold": alert.Threshold,
			})
		}
	}

	// Resolve firings that are no longer active
	for ruleID, existing := range activeDBMap {
		if _, still := currentFiring[ruleID]; !still {
			now := time.Now()
			existing.ResolvedAt = &now
			if err := a.db.UpdateAlertFiring(ctx, *existing); err != nil {
				a.logger.Error(ctx, "resolve alert firing: %v", err)
			}
		}
	}
}

// setupDefaultAlerts loads persisted alert rules from DB and fall back to defaults if none.
func (a *App) setupDefaultAlerts() {
	if a.monitor == nil || a.db == nil {
		return
	}

	ctx := context.Background()
	rules, err := a.db.ListAlertRules(ctx)
	if err != nil {
		a.logger.Error(ctx, "list alert rules: %v", err)
	}

	if len(rules) == 0 {
		// Seed default rules
		defaults := []models.AlertRule{
			{
				ID:          "high-error-rate",
				Name:        "High Error Rate",
				Description: "Error rate exceeds 10% of total requests",
				Metric:      "totalErrors",
				Cond:        "rate_gt",
				Threshold:   0.1,
				Window:      60,
				Cooldown:    5 * time.Minute,
				Severity:    models.AlertSeverityWarning,
				Enabled:     true,
				CreatedAt:   time.Now(),
			},
			{
				ID:          "high-memory-usage",
				Name:        "High Memory Usage",
				Description: "Memory usage exceeds 1GB",
				Metric:      "memoryUsageMb",
				Cond:        "gt",
				Threshold:   1024,
				Window:      60,
				Cooldown:    10 * time.Minute,
				Severity:    models.AlertSeverityWarning,
				Enabled:     true,
				CreatedAt:   time.Now(),
			},
			{
				ID:          "high-goroutine-count",
				Name:        "High Goroutine Count",
				Description: "Goroutine count exceeds 1000",
				Metric:      "goroutineCount",
				Cond:        "gt",
				Threshold:   1000,
				Window:      60,
				Cooldown:    10 * time.Minute,
				Severity:    models.AlertSeverityWarning,
				Enabled:     true,
				CreatedAt:   time.Now(),
			},
			{
				ID:          "repeat-task-high-failure-rate",
				Name:        "Repeated Task High Failure Rate",
				Description: "Repeated task failure rate exceeds 20%",
				Metric:      "failedRepeatedTasks",
				Cond:        "rate_gt",
				Threshold:   0.2,
				Window:      300,
				Cooldown:    5 * time.Minute,
				Severity:    models.AlertSeverityCritical,
				Enabled:     true,
				CreatedAt:   time.Now(),
			},
		}
		for _, rule := range defaults {
			if err := a.db.CreateAlertRule(ctx, rule); err != nil {
				a.logger.Error(ctx, "seed alert rule: %v", err)
			}
			rules = append(rules, rule)
		}
	}

	// Convert DB rules to in-memory monitoring.AlertRule with Condition func
	monitorRules := make([]monitoring.AlertRule, 0, len(rules))
	for _, r := range rules {
		monitorRules = append(monitorRules, a.buildAlertRule(r))
	}
	a.monitor.SetAlertRules(monitorRules)

	// Register alert callback for logging and Wails events
	a.monitor.RegisterAlertCallback(func(alert monitoring.Alert, fired bool) {
		if fired {
			if a.logger != nil {
				a.logger.Warning(
					context.Background(),
					"Alert triggered: "+alert.RuleName,
					slog.String("alertId", alert.RuleID),
					slog.String("severity", string(alert.Severity)),
					slog.String("description", alert.Description),
					slog.String("metric", alert.Metric),
					slog.Float64("value", alert.Value),
					slog.Float64("threshold", alert.Threshold),
				)
			}
			safeWailsEmit(a.ctx, "alert:fired", map[string]any{
				"ruleId":    alert.RuleID,
				"ruleName":  alert.RuleName,
				"severity":  alert.Severity,
				"metric":    alert.Metric,
				"value":     alert.Value,
				"threshold": alert.Threshold,
			})
		}
	})
}

// buildAlertRule builds an in-memory monitoring.AlertRule from persisted models.AlertRule.
func (a *App) buildAlertRule(r models.AlertRule) monitoring.AlertRule {
	condition := func(m *monitoring.Monitor) bool {
		val, ok := m.GetMetricValue(r.Metric)
		if !ok {
			return false
		}
		switch r.Cond {
		case "gt":
			return val > r.Threshold
		case "lt":
			return val < r.Threshold
		case "eq":
			return val == r.Threshold
		case "rate_gt":
			return val > r.Threshold
		default:
			return false
		}
	}
	return monitoring.AlertRule{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Metric:      r.Metric,
		Cond:        r.Cond,
		Threshold:   r.Threshold,
		Window:      r.Window,
		Cooldown:    r.Cooldown,
		Enabled:     r.Enabled,
		WebhookURL:  r.WebhookURL,
		Severity:    monitoring.AlertSeverity(r.Severity),
		Condition:   condition,
	}
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
	baseMetrics := a.GetPrometheusMetrics()
	monitoringMetrics := ""
	if a.monitor != nil {
		monitoringMetrics = a.monitor.GetPrometheusMetrics()
	}
	return baseMetrics + "\n" + monitoringMetrics
}

// — Alert CRUD API —

// CreateAlertRule creates a new alert rule.
func (a *App) CreateAlertRule(rule models.AlertRule) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	ctx := context.Background()
	rule.CreatedAt = time.Now()
	if err := a.db.CreateAlertRule(ctx, rule); err != nil {
		return err
	}
	// Reload monitor rules
	a.reloadAlertRules()
	return nil
}

// ListAlertRules returns all alert rules.
func (a *App) ListAlertRules() ([]models.AlertRule, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	ctx := context.Background()
	return a.db.ListAlertRules(ctx)
}

// UpdateAlertRule updates an existing rule.
func (a *App) UpdateAlertRule(rule models.AlertRule) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	ctx := context.Background()
	if err := a.db.UpdateAlertRule(ctx, rule); err != nil {
		return err
	}
	a.reloadAlertRules()
	return nil
}

// DeleteAlertRule removes a rule by ID.
func (a *App) DeleteAlertRule(id string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	ctx := context.Background()
	if err := a.db.DeleteAlertRule(ctx, id); err != nil {
		return err
	}
	a.reloadAlertRules()
	return nil
}

// GetAlertRule retrieves a single rule.
func (a *App) GetAlertRule(id string) (*models.AlertRule, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	ctx := context.Background()
	return a.db.GetAlertRule(ctx, id)
}

// ListAlertFirings returns firings for a rule.
func (a *App) ListAlertFirings(ruleID string, limit int) ([]models.AlertFiring, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	ctx := context.Background()
	return a.db.ListAlertFirings(ctx, ruleID, limit)
}

// GetActiveAlerts returns all currently firing alerts.
func (a *App) GetActiveAlerts() ([]models.AlertFiring, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	ctx := context.Background()
	return a.db.ListActiveAlerts(ctx)
}

// reloadAlertRules reloads rules from DB and updates monitor.
func (a *App) reloadAlertRules() {
	if a.db == nil || a.monitor == nil {
		return
	}
	ctx := context.Background()
	rules, err := a.db.ListAlertRules(ctx)
	if err != nil {
		a.logger.Error(ctx, "reload alert rules: %v", err)
		return
	}
	monitorRules := make([]monitoring.AlertRule, 0, len(rules))
	for _, r := range rules {
		monitorRules = append(monitorRules, a.buildAlertRule(r))
	}
	a.monitor.SetAlertRules(monitorRules)
}
