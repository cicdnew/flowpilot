package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"flowpilot/internal/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpBatchInput struct {
	Tasks     []mcpTaskInput `json:"tasks" jsonschema:"tasks to create as one batch"`
	AutoStart bool           `json:"autoStart,omitempty" jsonschema:"queue every task immediately"`
}

type mcpBatchFromFlowInput struct {
	FlowID         string   `json:"flowId" jsonschema:"recorded flow id"`
	URLs           []string `json:"urls" jsonschema:"one task is created per URL"`
	NamingTemplate string   `json:"namingTemplate,omitempty" jsonschema:"e.g. {{index}} - {{domain}}"`
	Priority       int      `json:"priority,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	AutoStart      bool     `json:"autoStart,omitempty"`
	Headless       *bool    `json:"headless,omitempty" jsonschema:"default true"`
	Timeout        int      `json:"timeout,omitempty"`
}

type mcpFlowTaskInput struct {
	FlowID    string   `json:"flowId" jsonschema:"recorded flow id"`
	Name      string   `json:"name,omitempty"`
	URL       string   `json:"url,omitempty" jsonschema:"override start URL"`
	Priority  int      `json:"priority,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	AutoStart bool     `json:"autoStart,omitempty"`
}

type mcpPlayFlowInput struct {
	FlowID   string `json:"flowId" jsonschema:"recorded flow id"`
	URL      string `json:"url,omitempty" jsonschema:"override start URL"`
	Headless *bool  `json:"headless,omitempty" jsonschema:"default true"`
	Timeout  int    `json:"timeout,omitempty"`
}

type mcpPathInput struct {
	Path string `json:"path" jsonschema:"absolute path to an exported flow or task JSON file"`
}

type mcpScheduleInput struct {
	Name     string   `json:"name"`
	CronExpr string   `json:"cronExpr" jsonschema:"5-field cron expression, e.g. */15 * * * *"`
	FlowID   string   `json:"flowId" jsonschema:"recorded flow to run"`
	URL      string   `json:"url,omitempty"`
	Priority int      `json:"priority,omitempty"`
	Headless *bool    `json:"headless,omitempty" jsonschema:"default true"`
	Tags     []string `json:"tags,omitempty"`
}

type mcpToggleInput struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

type mcpProxyInput struct {
	Server               string `json:"server" jsonschema:"host:port"`
	Protocol             string `json:"protocol,omitempty" jsonschema:"http, https or socks5 (default http)"`
	Username             string `json:"username,omitempty"`
	Password             string `json:"password,omitempty"`
	Geo                  string `json:"geo,omitempty" jsonschema:"ISO country code"`
	MaxRequestsPerMinute int    `json:"maxRequestsPerMinute,omitempty"`
}

type mcpLogsInput struct {
	TaskID  string `json:"taskId,omitempty" jsonschema:"only logs for this task"`
	BatchID string `json:"batchId,omitempty" jsonschema:"only logs for this batch"`
	Level   string `json:"level,omitempty" jsonschema:"debug, info, warning or error"`
	Limit   int    `json:"limit,omitempty" jsonschema:"max entries (default 100)"`
}

type mcpLimitInput struct {
	Limit int `json:"limit,omitempty"`
}

type mcpAlertInput struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Metric      string  `json:"metric" jsonschema:"metric name, e.g. queue_depth, failure_rate, memory_mb"`
	Condition   string  `json:"condition" jsonschema:"gt, lt or rate_gt"`
	Threshold   float64 `json:"threshold"`
	WindowSecs  int     `json:"windowSecs,omitempty"`
	CooldownSec int     `json:"cooldownSec,omitempty"`
	Severity    string  `json:"severity,omitempty" jsonschema:"info, warning or critical"`
	WebhookURL  string  `json:"webhookUrl,omitempty"`
}

type mcpAlertFiringsInput struct {
	RuleID string `json:"ruleId,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type mcpRetentionInput struct {
	RetentionDays int `json:"retentionDays" jsonschema:"delete finished tasks older than this many days"`
}

func boolOr(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func registerBatchAndFlowTools(s *mcp.Server, a *App) {
	addMCPTool(s, "create_batch", "Create many tasks as one batch (headless by default per task).", func(_ context.Context, in mcpBatchInput) (any, error) {
		inputs := make([]models.BatchTaskInput, 0, len(in.Tasks))
		for _, t := range in.Tasks {
			p := t.params(false)
			inputs = append(inputs, models.BatchTaskInput{Name: p.Name, URL: p.URL, Steps: p.Steps, Proxy: p.ProxyConfig,
				Priority: p.Priority, Timeout: p.Timeout, Tags: p.Tags, LoggingPolicy: p.LoggingPolicy, Headless: p.Headless})
		}
		return a.CreateBatch(inputs, in.AutoStart)
	})
	addMCPTool(s, "create_batch_from_flow", "Create one task per URL from a recorded flow.", func(_ context.Context, in mcpBatchFromFlowInput) (any, error) {
		headless := boolOr(in.Headless, true)
		return a.CreateBatchFromFlow(models.AdvancedBatchInput{FlowID: in.FlowID, URLs: in.URLs, NamingTemplate: in.NamingTemplate,
			Priority: in.Priority, Tags: in.Tags, AutoStart: in.AutoStart, Headless: &headless, Timeout: in.Timeout})
	})
	addMCPTool(s, "list_batches", "List batch groups.", func(context.Context, mcpEmpty) (any, error) { return a.ListBatchGroups() })
	addMCPTool(s, "get_batch_progress", "Progress counters for a batch.", func(_ context.Context, in mcpID) (any, error) { return a.GetBatchProgress(in.ID) })
	addMCPTool(s, "pause_batch", "Pause queued tasks of a batch.", func(_ context.Context, in mcpID) (any, error) { return "paused", a.PauseBatch(in.ID) })
	addMCPTool(s, "resume_batch", "Resume a paused batch.", func(_ context.Context, in mcpID) (any, error) { return "resumed", a.ResumeBatch(in.ID) })
	addMCPTool(s, "retry_failed_batch", "Re-queue the failed tasks of a batch.", func(_ context.Context, in mcpID) (any, error) { return a.RetryFailedBatch(in.ID) })

	addMCPTool(s, "list_flows", "List recorded flows.", func(context.Context, mcpEmpty) (any, error) { return a.ListRecordedFlows() })
	addMCPTool(s, "get_flow", "Get a recorded flow with its steps.", func(_ context.Context, in mcpID) (any, error) { return a.GetRecordedFlow(in.ID) })
	addMCPTool(s, "delete_flow", "Delete a recorded flow.", func(_ context.Context, in mcpID) (any, error) { return "deleted", a.DeleteRecordedFlow(in.ID) })
	addMCPTool(s, "create_task_from_flow", "Create a task from a recorded flow.", func(_ context.Context, in mcpFlowTaskInput) (any, error) {
		return a.CreateTaskFromFlow(CreateTaskFromFlowParams{FlowID: in.FlowID, Name: in.Name, URL: in.URL, Priority: in.Priority, Tags: in.Tags, AutoStart: in.AutoStart})
	})
	addMCPTool(s, "play_flow", "Run a recorded flow immediately as a task.", func(_ context.Context, in mcpPlayFlowInput) (any, error) {
		return a.PlayRecordedFlow(in.FlowID, in.URL, boolOr(in.Headless, true), in.Timeout, nil)
	})
	addMCPTool(s, "import_flow", "Import an exported flow or task JSON file from disk.", func(_ context.Context, in mcpPathInput) (any, error) { return a.ImportFlowWithWarnings(in.Path) })
	addMCPTool(s, "export_flow", "Export a recorded flow to a JSON file and return its path.", func(_ context.Context, in mcpID) (any, error) { return a.ExportFlow(in.ID) })

	addMCPTool(s, "list_schedules", "List cron schedules.", func(context.Context, mcpEmpty) (any, error) { return a.ListSchedules() })
	addMCPTool(s, "create_schedule", "Schedule a recorded flow with a cron expression.", func(_ context.Context, in mcpScheduleInput) (any, error) {
		return a.CreateSchedule(ScheduleParams{Name: in.Name, CronExpr: in.CronExpr, FlowID: in.FlowID, URL: in.URL, Priority: in.Priority, Headless: boolOr(in.Headless, true), Tags: in.Tags})
	})
	addMCPTool(s, "toggle_schedule", "Enable or disable a schedule.", func(_ context.Context, in mcpToggleInput) (any, error) {
		return "ok", a.ToggleSchedule(in.ID, in.Enabled)
	})
	addMCPTool(s, "delete_schedule", "Delete a schedule.", func(_ context.Context, in mcpID) (any, error) { return "deleted", a.DeleteSchedule(in.ID) })
}

func registerInfraTools(s *mcp.Server, a *App) {
	addMCPTool(s, "list_proxies", "List configured proxies with health.", func(context.Context, mcpEmpty) (any, error) { return a.ListProxies() })
	addMCPTool(s, "add_proxy", "Add a proxy to the pool.", func(_ context.Context, in mcpProxyInput) (any, error) {
		protocol := in.Protocol
		if protocol == "" {
			protocol = "http"
		}
		return a.AddProxyWithRateLimit(in.Server, protocol, in.Username, in.Password, in.Geo, in.MaxRequestsPerMinute)
	})
	addMCPTool(s, "delete_proxy", "Remove a proxy.", func(_ context.Context, in mcpID) (any, error) { return "deleted", a.DeleteProxy(in.ID) })

	addMCPTool(s, "list_plugins", "Loaded plugins with their actions, hooks, capabilities and runtime status.", func(context.Context, mcpEmpty) (any, error) { return a.ListPlugins(), nil })
	addMCPTool(s, "reload_plugins", "Restart all plugins and load newly installed ones from the plugins directory.", func(context.Context, mcpEmpty) (any, error) { return a.ReloadPlugins() })

	addMCPTool(s, "get_observability", "One snapshot of everything: health, queue, browser pools, plugins, per-action step latency and errors, slow steps, recent errors, active alerts, runtime.", func(context.Context, mcpEmpty) (any, error) {
		return a.GetObservabilitySnapshot(), nil
	})
	addMCPTool(s, "get_queue_metrics", "Queue counters: running, queued, totals, persistence depth and backpressure.", func(context.Context, mcpEmpty) (any, error) { return a.GetQueueMetrics(), nil })
	addMCPTool(s, "get_system_health", "Overall health status.", func(context.Context, mcpEmpty) (any, error) { return a.GetSystemHealth(), nil })
	addMCPTool(s, "get_prometheus_metrics", "All metrics in Prometheus text format.", func(context.Context, mcpEmpty) (any, error) { return a.GetEnhancedPrometheusMetrics(), nil })
	addMCPTool(s, "get_logs", "Structured application logs, optionally filtered by task, batch or level.", func(_ context.Context, in mcpLogsInput) (any, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 100
		}
		switch {
		case in.TaskID != "":
			return a.GetLogsByTaskID(in.TaskID, limit), nil
		case in.BatchID != "":
			return a.GetLogsByBatchID(in.BatchID, limit), nil
		case in.Level != "":
			return a.GetLogsByLevel(in.Level, limit), nil
		default:
			return a.GetRecentLogs(limit), nil
		}
	})
	addMCPTool(s, "get_recent_errors", "Most recent step error contexts.", func(_ context.Context, in mcpLimitInput) (any, error) {
		return a.GetRecentErrors(max(in.Limit, 1)), nil
	})
	addMCPTool(s, "get_audit_trail", "Audit trail of lifecycle events for a task.", func(_ context.Context, in mcpID) (any, error) { return a.GetAuditTrail(in.ID, 200) })
	addMCPTool(s, "list_alert_rules", "List alert rules.", func(context.Context, mcpEmpty) (any, error) { return a.ListAlertRules() })
	addMCPTool(s, "create_alert_rule", "Create an alert rule evaluated by the monitor.", func(_ context.Context, in mcpAlertInput) (any, error) {
		severity := models.AlertSeverity(strings.ToLower(in.Severity))
		if severity == "" {
			severity = "warning"
		}
		rule := models.AlertRule{ID: fmt.Sprintf("mcp-%d", time.Now().UnixNano()), Name: in.Name, Description: in.Description, Metric: in.Metric,
			Cond: in.Condition, Threshold: in.Threshold, Window: in.WindowSecs, Cooldown: time.Duration(in.CooldownSec) * time.Second,
			Severity: severity, Enabled: true, WebhookURL: in.WebhookURL}
		if err := a.CreateAlertRule(rule); err != nil {
			return nil, err
		}
		return rule, nil
	})
	addMCPTool(s, "delete_alert_rule", "Delete an alert rule.", func(_ context.Context, in mcpID) (any, error) { return "deleted", a.DeleteAlertRule(in.ID) })
	addMCPTool(s, "get_active_alerts", "Alerts currently firing.", func(context.Context, mcpEmpty) (any, error) { return a.GetActiveAlerts() })
	addMCPTool(s, "list_alert_firings", "History of alert firings.", func(_ context.Context, in mcpAlertFiringsInput) (any, error) {
		return a.ListAlertFirings(in.RuleID, max(in.Limit, 50))
	})
	addMCPTool(s, "purge_old_data", "Delete finished tasks older than the retention period.", func(_ context.Context, in mcpRetentionInput) (any, error) {
		return a.PurgeOldData(in.RetentionDays)
	})
}

func newMCPServer(a *App, version string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: mcpServerName, Title: "FlowPilot browser automation", Version: version}, &mcp.ServerOptions{
		Instructions: "FlowPilot runs browser automation tasks. Typical loop: list_step_actions -> run_task (or create_task + start_task + wait_for_task) -> get_task. " +
			"Tasks are headless by default; pass headless=false for a visible browser. Use get_observability for health, queue, pool, plugin and step-latency metrics.",
	})
	registerTaskTools(s, a)
	registerBatchAndFlowTools(s, a)
	registerInfraTools(s, a)
	s.AddResource(&mcp.Resource{URI: "flowpilot://observability", Name: "observability", Title: "Observability snapshot", MIMEType: "application/json"},
		jsonResource(func() any { return a.GetObservabilitySnapshot() }))
	s.AddResource(&mcp.Resource{URI: "flowpilot://metrics", Name: "metrics", Title: "Prometheus metrics", MIMEType: "text/plain"},
		func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: req.Params.URI, MIMEType: "text/plain", Text: a.GetEnhancedPrometheusMetrics()}}}, nil
		})
	return s
}
