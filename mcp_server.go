package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"flowpilot/internal/database"
	"flowpilot/internal/models"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpServerName         = "flowpilot"
	defaultWaitTimeoutSec = 300
	maxWaitTimeoutSec     = 3600
	waitPollInterval      = 250 * time.Millisecond
)

type mcpEmpty struct{}

type mcpID struct {
	ID string `json:"id" jsonschema:"identifier of the object"`
}

func mcpErrorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}
}

func portableSchema(sc *jsonschema.Schema) {
	if sc == nil {
		return
	}
	if len(sc.Types) > 0 {
		kept := make([]string, 0, len(sc.Types))
		for _, t := range sc.Types {
			if t != "null" {
				kept = append(kept, t)
			}
		}
		if len(kept) == 1 {
			sc.Type, sc.Types = kept[0], nil
		} else {
			sc.Types = kept
		}
	}
	for _, child := range sc.Properties {
		portableSchema(child)
	}
	for _, child := range sc.Defs {
		portableSchema(child)
	}
	for _, list := range [][]*jsonschema.Schema{sc.AnyOf, sc.PrefixItems} {
		for _, child := range list {
			portableSchema(child)
		}
	}
	portableSchema(sc.Items)
	portableSchema(sc.AdditionalProperties)
}

func addMCPTool[In any](s *mcp.Server, name, description string, fn func(ctx context.Context, in In) (any, error)) {
	inputSchema, err := jsonschema.For[In](nil)
	if err != nil {
		panic(fmt.Sprintf("mcp tool %s: infer input schema: %v", name, err))
	}
	portableSchema(inputSchema)
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: description, InputSchema: inputSchema}, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		out, err := fn(ctx, in)
		if err != nil {
			return mcpErrorResult(err), nil, nil
		}
		return nil, map[string]any{"result": out}, nil
	})
}

func isTerminalStatus(s models.TaskStatus) bool {
	return s == models.TaskStatusCompleted || s == models.TaskStatusFailed || s == models.TaskStatusCancelled
}

func (a *App) waitForTask(ctx context.Context, id string, timeoutSec int) (*models.Task, bool, error) {
	if timeoutSec <= 0 {
		timeoutSec = defaultWaitTimeoutSec
	}
	timeoutSec = min(timeoutSec, maxWaitTimeoutSec)
	deadline := time.NewTimer(time.Duration(timeoutSec) * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(waitPollInterval)
	defer ticker.Stop()
	for {
		task, err := a.GetTask(id)
		if err != nil {
			return nil, false, err
		}
		if isTerminalStatus(task.Status) {
			return task, true, nil
		}
		select {
		case <-ctx.Done():
			return task, false, ctx.Err()
		case <-deadline.C:
			return task, false, nil
		case <-ticker.C:
		}
	}
}

type mcpTaskInput struct {
	Name          string                    `json:"name" jsonschema:"human readable task name"`
	URL           string                    `json:"url" jsonschema:"start URL (http or https)"`
	Steps         []models.TaskStep         `json:"steps" jsonschema:"ordered browser steps; call list_step_actions for valid action names"`
	Headless      *bool                     `json:"headless,omitempty" jsonschema:"run without a visible browser window (default true); set false for a headed run"`
	Priority      int                       `json:"priority,omitempty" jsonschema:"1 low, 5 normal (default), 10 high"`
	Tags          []string                  `json:"tags,omitempty" jsonschema:"labels used for filtering"`
	Timeout       int                       `json:"timeout,omitempty" jsonschema:"task timeout in seconds (0 uses the default)"`
	Proxy         *models.ProxyConfig       `json:"proxy,omitempty" jsonschema:"optional proxy configuration"`
	LoggingPolicy *models.TaskLoggingPolicy `json:"loggingPolicy,omitempty" jsonschema:"optional per-task logging policy"`
}

func (in mcpTaskInput) params(autoStart bool) CreateTaskParams {
	p := CreateTaskParams{
		Name: in.Name, URL: in.URL, Steps: in.Steps, Priority: in.Priority, Tags: in.Tags,
		Timeout: in.Timeout, LoggingPolicy: in.LoggingPolicy, AutoStart: autoStart, Headless: true,
	}
	if p.Priority == 0 {
		p.Priority = int(models.PriorityNormal)
	}
	if in.Headless != nil {
		p.Headless = *in.Headless
	}
	if in.Proxy != nil {
		p.ProxyConfig = *in.Proxy
	}
	return p
}

type mcpCreateTaskInput struct {
	mcpTaskInput
	AutoStart bool `json:"autoStart,omitempty" jsonschema:"queue the task immediately after creating it"`
}

type mcpRunTaskInput struct {
	mcpTaskInput
	WaitTimeoutSec int `json:"waitTimeoutSec,omitempty" jsonschema:"seconds to wait for the task to finish (default 300, max 3600)"`
}

type mcpWaitInput struct {
	ID             string `json:"id" jsonschema:"task id"`
	WaitTimeoutSec int    `json:"waitTimeoutSec,omitempty" jsonschema:"seconds to wait (default 300, max 3600)"`
}

type mcpListTasksInput struct {
	Page     int    `json:"page,omitempty" jsonschema:"1-based page number (default 1)"`
	PageSize int    `json:"pageSize,omitempty" jsonschema:"page size (default 50, max 500)"`
	Status   string `json:"status,omitempty" jsonschema:"filter by status: pending, queued, running, completed, failed, cancelled, retrying"`
	Tag      string `json:"tag,omitempty" jsonschema:"filter by tag"`
	BatchID  string `json:"batchId,omitempty" jsonschema:"list every task in this batch instead of paginating"`
}

type mcpUpdateTaskInput struct {
	ID       string            `json:"id" jsonschema:"task id"`
	Name     string            `json:"name" jsonschema:"new name"`
	URL      string            `json:"url" jsonschema:"new start URL"`
	Steps    []models.TaskStep `json:"steps" jsonschema:"replacement steps"`
	Priority int               `json:"priority,omitempty" jsonschema:"new priority"`
	Tags     []string          `json:"tags,omitempty" jsonschema:"replacement tags"`
}

type mcpWaitResult struct {
	Finished bool         `json:"finished"`
	Task     *models.Task `json:"task"`
}

func registerTaskTools(s *mcp.Server, a *App) {
	addMCPTool(s, "list_step_actions", "List every step action the runner accepts, including actions added by plugins.", func(context.Context, mcpEmpty) (any, error) {
		return a.GetSupportedStepActions(), nil
	})
	addMCPTool(s, "create_task", "Create a browser automation task. Headless by default; pass headless=false for a visible browser.", func(_ context.Context, in mcpCreateTaskInput) (any, error) {
		return a.CreateTask(in.params(in.AutoStart))
	})
	addMCPTool(s, "run_task", "Create a task, start it and wait until it finishes. Returns the final task with its result, logs and extracted data.", func(ctx context.Context, in mcpRunTaskInput) (any, error) {
		task, err := a.CreateTask(in.params(true))
		if err != nil {
			return nil, err
		}
		final, finished, err := a.waitForTask(ctx, task.ID, in.WaitTimeoutSec)
		if err != nil {
			return nil, err
		}
		return mcpWaitResult{Finished: finished, Task: final}, nil
	})
	addMCPTool(s, "start_task", "Queue an existing pending or failed task for execution.", func(_ context.Context, in mcpID) (any, error) {
		return "started", a.StartTask(in.ID)
	})
	addMCPTool(s, "start_all_pending", "Queue every pending task.", func(context.Context, mcpEmpty) (any, error) {
		return "started", a.StartAllPending()
	})
	addMCPTool(s, "cancel_task", "Cancel a queued, running or retrying task.", func(_ context.Context, in mcpID) (any, error) {
		return "cancelled", a.CancelTask(in.ID)
	})
	addMCPTool(s, "delete_task", "Delete a task that is not running.", func(_ context.Context, in mcpID) (any, error) {
		return "deleted", a.DeleteTask(in.ID)
	})
	addMCPTool(s, "update_task", "Edit a pending task's name, URL, steps, tags and priority.", func(_ context.Context, in mcpUpdateTaskInput) (any, error) {
		current, err := a.GetTask(in.ID)
		if err != nil {
			return nil, err
		}
		priority := in.Priority
		if priority == 0 {
			priority = int(current.Priority)
		}
		if err := a.UpdateTask(in.ID, updateParams(current, in), priority); err != nil {
			return nil, err
		}
		return a.GetTask(in.ID)
	})
	addMCPTool(s, "get_task", "Get a task with its status, error, result, extracted data and execution logs.", func(_ context.Context, in mcpID) (any, error) {
		return a.GetTask(in.ID)
	})
	addMCPTool(s, "wait_for_task", "Block until a task reaches completed, failed or cancelled, or the timeout elapses.", func(ctx context.Context, in mcpWaitInput) (any, error) {
		task, finished, err := a.waitForTask(ctx, in.ID, in.WaitTimeoutSec)
		if err != nil {
			return nil, err
		}
		return mcpWaitResult{Finished: finished, Task: task}, nil
	})
	addMCPTool(s, "list_tasks", "List tasks with pagination and optional status, tag or batch filters.", func(_ context.Context, in mcpListTasksInput) (any, error) {
		if in.BatchID != "" {
			return a.ListTasksByBatch(in.BatchID)
		}
		page, size := max(in.Page, 1), in.PageSize
		if size <= 0 {
			size = 50
		}
		return a.ListTasksPaginated(page, min(size, 500), in.Status, in.Tag)
	})
	addMCPTool(s, "get_task_stats", "Count tasks by status.", func(context.Context, mcpEmpty) (any, error) {
		return a.GetTaskStats()
	})
	addMCPTool(s, "get_task_events", "Lifecycle events (status transitions) recorded for a task.", func(_ context.Context, in mcpID) (any, error) {
		return a.ListTaskEvents(in.ID)
	})
	addMCPTool(s, "export_task", "Export a task definition to a JSON file and return its path.", func(_ context.Context, in mcpID) (any, error) {
		return a.ExportTask(in.ID)
	})
}

func updateParams(current *models.Task, in mcpUpdateTaskInput) database.TaskUpdateParams {
	p := database.TaskUpdateParams{
		Name: current.Name, URL: current.URL, Steps: current.Steps, Tags: current.Tags,
		ProxyConfig: current.Proxy, Timeout: current.Timeout, LoggingPolicy: current.LoggingPolicy,
	}
	if in.Name != "" {
		p.Name = in.Name
	}
	if in.URL != "" {
		p.URL = in.URL
	}
	if len(in.Steps) > 0 {
		p.Steps = in.Steps
	}
	if in.Tags != nil {
		p.Tags = in.Tags
	}
	return p
}

func jsonResource(get func() any) mcp.ResourceHandler {
	return func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		raw, err := json.MarshalIndent(get(), "", "  ")
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: req.Params.URI, MIMEType: "application/json", Text: string(raw)}}}, nil
	}
}
