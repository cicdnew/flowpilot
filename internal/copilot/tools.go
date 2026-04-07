package copilot

import (
	"context"
	"fmt"
	"time"

	"flowpilot/internal/models"
)

// ── v2 tool handlers ───────────────────────────────────────────────────────

// toolGetTask retrieves full details for a single task by ID.
// Returns: {id, name, status, url, error, duration_ms, steps_count}
func (c *CopilotFlow) toolGetTask(ctx context.Context, args map[string]any) (any, error) {
	taskID, ok := args["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("task_id must be a non-empty string")
	}

	task, err := c.db.GetTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	var durationMs int64
	if task.StartedAt != nil && task.CompletedAt != nil {
		durationMs = task.CompletedAt.Sub(*task.StartedAt).Milliseconds()
	}

	return map[string]any{
		"id":          task.ID,
		"name":        task.Name,
		"status":      string(task.Status),
		"url":         task.URL,
		"error":       task.Error,
		"duration_ms": durationMs,
		"steps_count": len(task.Steps),
	}, nil
}

// toolCancelTask cancels a pending, queued, or running task.
// Delegates to queue.Cancel which handles both in-flight cancellation and DB update.
// Returns: {task_id, message}
func (c *CopilotFlow) toolCancelTask(ctx context.Context, args map[string]any) (any, error) {
	taskID, ok := args["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("task_id must be a non-empty string")
	}

	if err := c.queue.Cancel(taskID); err != nil {
		return nil, fmt.Errorf("cancel task %s: %w", taskID, err)
	}

	return map[string]any{
		"task_id": taskID,
		"message": "task cancelled successfully",
	}, nil
}

// toolRetryTask resets a failed or cancelled task back to pending and re-queues it.
// Returns: {task_id, status}
func (c *CopilotFlow) toolRetryTask(ctx context.Context, args map[string]any) (any, error) {
	taskID, ok := args["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("task_id must be a non-empty string")
	}

	// Verify the task exists before attempting any mutation.
	if _, err := c.db.GetTask(ctx, taskID); err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	// Reset retry counter so the full retry budget is available again.
	if err := c.db.ResetRetryCount(ctx, taskID); err != nil {
		return nil, fmt.Errorf("reset retry count: %w", err)
	}

	// Transition status back to pending so the scheduler accepts it.
	if err := c.db.UpdateTaskStatus(ctx, taskID, models.TaskStatusPending, ""); err != nil {
		return nil, fmt.Errorf("reset task status to pending: %w", err)
	}

	// Fetch the refreshed task and re-submit it to the work queue.
	task, err := c.db.GetTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get updated task: %w", err)
	}

	if err := c.queue.Submit(ctx, *task); err != nil {
		return nil, fmt.Errorf("submit task to queue: %w", err)
	}

	return map[string]any{
		"task_id": taskID,
		"status":  string(models.TaskStatusPending),
	}, nil
}

// toolGetBatchProgress returns aggregate status counters for all tasks in a batch.
// Returns: {batch_id, total, pending, running, completed, failed}
func (c *CopilotFlow) toolGetBatchProgress(ctx context.Context, args map[string]any) (any, error) {
	batchID, ok := args["batch_id"].(string)
	if !ok || batchID == "" {
		return nil, fmt.Errorf("batch_id must be a non-empty string")
	}

	progress, err := c.db.GetBatchProgress(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("get batch progress: %w", err)
	}

	return map[string]any{
		"batch_id":  progress.BatchID,
		"total":     progress.Total,
		"pending":   progress.Pending,
		"running":   progress.Running,
		"completed": progress.Completed,
		"failed":    progress.Failed,
	}, nil
}

// toolCancelBatch cancels all pending, queued, and running tasks that belong to
// the given batch. Already-completed or already-failed tasks are left untouched.
// Returns: {batch_id, cancelled_count}
func (c *CopilotFlow) toolCancelBatch(ctx context.Context, args map[string]any) (any, error) {
	batchID, ok := args["batch_id"].(string)
	if !ok || batchID == "" {
		return nil, fmt.Errorf("batch_id must be a non-empty string")
	}

	tasks, err := c.db.ListTasksByBatch(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("list batch tasks: %w", err)
	}

	cancelled := 0
	for _, task := range tasks {
		switch task.Status {
		case models.TaskStatusPending, models.TaskStatusQueued, models.TaskStatusRunning:
			if err := c.queue.Cancel(task.ID); err != nil {
				// Log the individual failure but continue cancelling remaining tasks.
				continue
			}
			cancelled++
		}
	}

	return map[string]any{
		"batch_id":        batchID,
		"cancelled_count": cancelled,
	}, nil
}

// toolGetTaskLogs fetches step-level execution logs for a task.
// An optional limit caps the number of returned entries (default 50).
// Returns: [{step, action, status, message, duration_ms}]
func (c *CopilotFlow) toolGetTaskLogs(ctx context.Context, args map[string]any) (any, error) {
	taskID, ok := args["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("task_id must be a non-empty string")
	}

	limit := 50
	if raw, ok := args["limit"].(float64); ok && raw > 0 {
		limit = int(raw)
	}

	logs, err := c.db.ListStepLogs(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("list step logs: %w", err)
	}

	// Apply limit — keep the first `limit` entries (chronological order from DB).
	if len(logs) > limit {
		logs = logs[:limit]
	}

	result := make([]map[string]any, 0, len(logs))
	for _, l := range logs {
		// Derive a human-readable status: "ok" when no error code is set.
		status := "ok"
		if l.ErrorCode != "" {
			status = l.ErrorCode
		}

		result = append(result, map[string]any{
			"step":        l.StepIndex,
			"action":      string(l.Action),
			"status":      status,
			"message":     l.ErrorMsg,
			"duration_ms": l.DurationMs,
		})
	}
	return result, nil
}

// toolAddProxy adds a new proxy server to the proxy pool.
// Returns: {proxy_id, server}
func (c *CopilotFlow) toolAddProxy(ctx context.Context, args map[string]any) (any, error) {
	server, ok := args["server"].(string)
	if !ok || server == "" {
		return nil, fmt.Errorf("server must be a non-empty string (host:port)")
	}

	protocol, _ := args["protocol"].(string)
	if protocol == "" {
		protocol = "http"
	}
	username, _ := args["username"].(string)
	password, _ := args["password"].(string)
	geo, _ := args["geo"].(string)

	proxyID := fmt.Sprintf("proxy-%d", time.Now().UnixNano())

	proxy := models.Proxy{
		ID:        proxyID,
		Server:    server,
		Protocol:  models.ProxyProtocol(protocol),
		Username:  username,
		Password:  password,
		Geo:       geo,
		Status:    models.ProxyStatusUnknown,
		CreatedAt: time.Now(),
	}

	if err := c.db.CreateProxy(ctx, proxy); err != nil {
		return nil, fmt.Errorf("create proxy: %w", err)
	}

	return map[string]any{
		"proxy_id": proxyID,
		"server":   server,
	}, nil
}

// toolDeleteProxy removes a proxy from the pool by its ID.
// Returns: {proxy_id, message}
func (c *CopilotFlow) toolDeleteProxy(ctx context.Context, args map[string]any) (any, error) {
	proxyID, ok := args["proxy_id"].(string)
	if !ok || proxyID == "" {
		return nil, fmt.Errorf("proxy_id must be a non-empty string")
	}

	if err := c.db.DeleteProxy(ctx, proxyID); err != nil {
		return nil, fmt.Errorf("delete proxy %s: %w", proxyID, err)
	}

	return map[string]any{
		"proxy_id": proxyID,
		"message":  "proxy deleted successfully",
	}, nil
}

// toolListSchedules returns all configured recurring task schedules.
// Returns: [{id, name, flow_id, cron, enabled, next_run}]
func (c *CopilotFlow) toolListSchedules(ctx context.Context, args map[string]any) (any, error) {
	schedules, err := c.db.ListSchedules(ctx)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}

	result := make([]map[string]any, 0, len(schedules))
	for _, s := range schedules {
		entry := map[string]any{
			"id":      s.ID,
			"name":    s.Name,
			"flow_id": s.FlowID,
			"cron":    s.CronExpr,
			"enabled": s.Enabled,
		}
		// next_run is nil until the scheduler computes the first run time.
		if s.NextRunAt != nil {
			entry["next_run"] = s.NextRunAt.Format(time.RFC3339)
		} else {
			entry["next_run"] = nil
		}
		result = append(result, entry)
	}
	return result, nil
}

// toolCreateSchedule creates a new recurring schedule that runs a recorded flow
// on the provided cron expression.
// Returns: {schedule_id, name, cron}
func (c *CopilotFlow) toolCreateSchedule(ctx context.Context, args map[string]any) (any, error) {
	flowID, ok := args["flow_id"].(string)
	if !ok || flowID == "" {
		return nil, fmt.Errorf("flow_id must be a non-empty string")
	}
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("name must be a non-empty string")
	}
	cronExpr, ok := args["cron"].(string)
	if !ok || cronExpr == "" {
		return nil, fmt.Errorf("cron must be a non-empty string")
	}

	scheduleID := fmt.Sprintf("sched-%d", time.Now().UnixNano())
	now := time.Now()

	schedule := models.Schedule{
		ID:        scheduleID,
		Name:      name,
		CronExpr:  cronExpr,
		FlowID:    flowID,
		Enabled:   true,
		Priority:  models.PriorityNormal,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := c.db.CreateSchedule(ctx, schedule); err != nil {
		return nil, fmt.Errorf("create schedule: %w", err)
	}

	return map[string]any{
		"schedule_id": scheduleID,
		"name":        name,
		"cron":        cronExpr,
	}, nil
}

// toolCreateBatch implements the create_batch tool.
func (c *CopilotFlow) toolCreateBatch(ctx context.Context, args map[string]any) (any, error) {
	flowID, ok := args["flow_id"].(string)
	if !ok {
		return nil, fmt.Errorf("flow_id must be a string")
	}

	urlsAny, ok := args["urls"].([]any)
	if !ok {
		return nil, fmt.Errorf("urls must be an array")
	}

	var urls []string
	for _, u := range urlsAny {
		if s, ok := u.(string); ok {
			urls = append(urls, s)
		}
	}

	name, _ := args["name"].(string)

	flow, err := c.db.GetRecordedFlow(ctx, flowID)
	if err != nil {
		return nil, fmt.Errorf("get flow: %w", err)
	}

	batch, tasks, err := c.batchEngine.CreateBatchFromFlow(ctx, *flow, models.AdvancedBatchInput{
		URLs:           urls,
		NamingTemplate: name,
	})
	if err != nil {
		return nil, fmt.Errorf("create batch: %w", err)
	}

	return map[string]any{
		"batch_id": batch.ID,
		"tasks":    len(tasks),
		"name":     batch.Name,
	}, nil
}

// toolListTasks implements the list_tasks tool.
func (c *CopilotFlow) toolListTasks(ctx context.Context, args map[string]any) (any, error) {
	status, _ := args["status"].(string)
	limit, _ := args["limit"].(float64)
	if limit == 0 {
		limit = 50
	}

	var tasks []models.Task

	if status != "" {
		var err error
		tasks, err = c.db.ListTasksByStatus(ctx, models.TaskStatus(status))
		if err != nil {
			return nil, fmt.Errorf("list tasks: %w", err)
		}
	} else {
		paginated, err := c.db.ListTasksPaginated(ctx, 1, int(limit), "all", "")
		if err != nil {
			return nil, fmt.Errorf("list tasks paginated: %w", err)
		}
		tasks = paginated.Tasks
	}

	var result []map[string]any
	for _, t := range tasks {
		result = append(result, map[string]any{
			"id":      t.ID,
			"name":    t.Name,
			"status":  t.Status,
			"url":     t.URL,
			"created": t.CreatedAt,
		})
	}

	return result, nil
}

// toolListProxies implements the list_proxies tool.
func (c *CopilotFlow) toolListProxies(ctx context.Context, args map[string]any) (any, error) {
	proxies, err := c.db.ListProxies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list proxies: %w", err)
	}

	var result []map[string]any
	for _, p := range proxies {
		result = append(result, map[string]any{
			"id":           p.ID,
			"server":       p.Server,
			"status":       p.Status,
			"latency":      p.Latency,
			"success_rate": p.SuccessRate,
			"used":         p.TotalUsed,
		})
	}

	return result, nil
}

// toolListFlows implements the list_flows tool.
func (c *CopilotFlow) toolListFlows(ctx context.Context, args map[string]any) (any, error) {
	flows, err := c.db.ListRecordedFlows(ctx)
	if err != nil {
		return nil, fmt.Errorf("list flows: %w", err)
	}

	var result []map[string]any
	for _, f := range flows {
		result = append(result, map[string]any{
			"id":         f.ID,
			"name":       f.Name,
			"steps":      len(f.Steps),
			"created_at": f.CreatedAt,
		})
	}

	return result, nil
}

// toolSystemStatus implements the system_status tool.
func (c *CopilotFlow) toolSystemStatus(ctx context.Context, args map[string]any) (any, error) {
	stats, err := c.db.GetTaskStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("get stats: %w", err)
	}

	proxies, err := c.db.ListProxies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list proxies: %w", err)
	}

	healthyProxies := 0
	for _, p := range proxies {
		if p.Status == models.ProxyStatusHealthy {
			healthyProxies++
		}
	}

	return map[string]any{
		"tasks": map[string]any{
			"total":     stats["total"],
			"pending":   stats["pending"],
			"running":   stats["running"],
			"completed": stats["completed"],
			"failed":    stats["failed"],
		},
		"proxies": map[string]any{
			"total":   len(proxies),
			"healthy": healthyProxies,
		},
		"queue_depth":   c.queue.Metrics().Pending,
		"running_tasks": c.queue.RunningCount(),
	}, nil
}

// toolRunTask implements the run_task tool — creates a Task and submits it to the queue immediately.
func (c *CopilotFlow) toolRunTask(ctx context.Context, args map[string]any) (any, error) {
	name, _ := args["name"].(string)
	if name == "" {
		name = "copilot-task"
	}
	url, _ := args["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}
	headless, _ := args["headless"].(bool)

	// Parse steps from the LLM arguments.
	var steps []models.TaskStep
	if stepsAny, ok := args["steps"].([]any); ok {
		for _, s := range stepsAny {
			stepMap, ok := s.(map[string]any)
			if !ok {
				continue
			}
			step := models.TaskStep{}
			if a, ok := stepMap["action"].(string); ok {
				step.Action = models.StepAction(a)
			}
			if sel, ok := stepMap["selector"].(string); ok {
				step.Selector = sel
			}
			if val, ok := stepMap["value"].(string); ok {
				step.Value = val
			}
			steps = append(steps, step)
		}
	}
	if len(steps) == 0 {
		// Default: just navigate to the URL.
		steps = []models.TaskStep{
			{Action: models.ActionNavigate, Value: url},
		}
	}

	task := models.Task{
		ID:         fmt.Sprintf("copilot-%d", time.Now().UnixNano()),
		Name:       name,
		URL:        url,
		Steps:      steps,
		Status:     models.TaskStatusPending,
		Priority:   models.PriorityNormal,
		MaxRetries: 1,
		Headless:   headless,
		CreatedAt:  time.Now(),
	}

	if err := c.db.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	if err := c.queue.Submit(ctx, task); err != nil {
		return nil, fmt.Errorf("submit task: %w", err)
	}

	mode := "non-headless (visible browser)"
	if headless {
		mode = "headless"
	}

	return map[string]any{
		"task_id": task.ID,
		"name":    task.Name,
		"url":     task.URL,
		"steps":   len(task.Steps),
		"mode":    mode,
		"status":  "queued",
	}, nil
}
