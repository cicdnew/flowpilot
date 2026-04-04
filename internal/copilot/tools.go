package copilot

import (
	"context"
	"fmt"

	"flowpilot/internal/models"
)

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
	var err error

	if status != "" {
		tasks, err = c.db.ListTasksByStatus(ctx, models.TaskStatus(status))
	} else {
		paginated, err := c.db.ListTasksPaginated(ctx, 1, int(limit), "all", "")
		if err != nil {
			return nil, fmt.Errorf("list tasks paginated: %w", err)
		}
		tasks = paginated.Tasks
	}

	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
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
