package copilot

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"flowpilot/internal/batch"
	"flowpilot/internal/browser"
	"flowpilot/internal/crypto"
	"flowpilot/internal/database"
	"flowpilot/internal/models"
	"flowpilot/internal/proxy"
	"flowpilot/internal/queue"
)

// CopilotFlow is a standalone agent that provides natural language interface
// to all FlowPilot features with multi-model LLM support.
type CopilotFlow struct {
	db           *database.DB
	runner       *browser.Runner
	queue        *queue.Queue
	proxyManager *proxy.Manager
	batchEngine  *batch.Engine
	dataDir      string
	providerMu   sync.RWMutex
	provider     LLMProvider
	tools        map[string]Tool
	config       Config
	history      *ConversationHistory // multi-turn conversation context

	pollInterval  time.Duration
	pollingCancel context.CancelFunc
	pollingMu     sync.Mutex
}

// Config holds copilot agent configuration.
type Config struct {
	DataDir        string
	MaxConcurrency int
	ModelProvider  string
	APIKey         string
	BaseURL        string
	ModelName      string

	PollInterval        time.Duration // interval between pending-task polls (default 30s)
	HealthCheckInterval int           // proxy health check interval in seconds (default 300)
	MaxProxyFailures    int           // max proxy failures before marking unhealthy (default 3)
	Headless            bool          // when true, force all tasks headless; false = let task.Headless decide
}

// Tool represents a callable function that the LLM can invoke.
type Tool struct {
	Name        string
	Description string
	Schema      map[string]any
	Handler     func(ctx context.Context, args map[string]any) (any, error)
}

// LLMProvider defines the interface for all model providers.
type LLMProvider interface {
	ChatCompletion(ctx context.Context, messages []Message, tools []ToolDefinition) (ChatResponse, error)
	StreamChatCompletion(ctx context.Context, messages []Message, tools []ToolDefinition) <-chan StreamChunk
	ListModels(ctx context.Context) ([]Model, error)
	SupportsFunctionCalling() bool
	Model() string
	Provider() string
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolDefinition defines the schema for function calling.
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ChatResponse is the response from the LLM provider.
type ChatResponse struct {
	Content   string
	ToolCalls []ToolCall
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	Name      string
	Arguments map[string]any
}

// New creates a new CopilotFlow agent.
func New(cfg Config) (*CopilotFlow, error) {
	if cfg.DataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		cfg.DataDir = filepath.Join(home, ".flowpilot")
	}

	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	if err := crypto.InitKey(cfg.DataDir); err != nil {
		return nil, fmt.Errorf("init encryption: %w", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "tasks.db")
	db, err := database.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}

	screenshotDir := filepath.Join(cfg.DataDir, "screenshots")
	runner, err := browser.NewRunner(screenshotDir)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("init browser runner: %w", err)
	}
	runner.SetForceHeadless(cfg.Headless)

	if cfg.MaxConcurrency <= 0 {
		cfg.MaxConcurrency = 10
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 30 * time.Second
	}
	if cfg.HealthCheckInterval <= 0 {
		cfg.HealthCheckInterval = 300
	}
	if cfg.MaxProxyFailures <= 0 {
		cfg.MaxProxyFailures = 3
	}

	q := queue.New(db, runner, cfg.MaxConcurrency, func(event models.TaskEvent) {
		log.Printf("[copilot] task %s -> %s", event.TaskID, event.Status)
	})

	pm := proxy.NewManager(db, models.ProxyPoolConfig{
		Strategy:            models.RotationRoundRobin,
		HealthCheckInterval: cfg.HealthCheckInterval,
		MaxFailures:         cfg.MaxProxyFailures,
	})
	q.SetProxyManager(pm)

	batchEngine := batch.New(db)

	c := &CopilotFlow{
		db:           db,
		runner:       runner,
		queue:        q,
		proxyManager: pm,
		batchEngine:  batchEngine,
		dataDir:      cfg.DataDir,
		config:       cfg,
		tools:        make(map[string]Tool),
		pollInterval: cfg.PollInterval,
		history:      &ConversationHistory{},
	}

	c.registerTools()

	if cfg.ModelProvider != "" && cfg.APIKey != "" {
		if err := c.Connect(cfg.ModelProvider, cfg.APIKey, cfg.BaseURL, cfg.ModelName); err != nil {
			q.Stop()
			db.Close()
			return nil, err
		}
	}

	return c, nil
}

// Connect configures the LLM provider for the copilot.
func (c *CopilotFlow) Connect(provider string, apiKey string, baseURL string, modelName string) error {
	p, err := NewProvider(provider, apiKey, baseURL, modelName)
	if err != nil {
		return err
	}
	c.providerMu.Lock()
	c.provider = p
	c.providerMu.Unlock()
	log.Printf("[copilot] connected to %s provider", provider)
	return nil
}

// SetModel switches to a different model.
func (c *CopilotFlow) SetModel(modelID string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to any provider")
	}
	return c.Connect(c.CurrentProvider(), c.config.APIKey, c.config.BaseURL, modelID)
}

// ListModels returns available models from the connected provider.
func (c *CopilotFlow) ListModels(ctx context.Context) ([]Model, error) {
	c.providerMu.RLock()
	p := c.provider
	c.providerMu.RUnlock()
	if p == nil {
		return nil, fmt.Errorf("not connected to any provider")
	}
	return p.ListModels(ctx)
}

// IsConnected returns true if the copilot is connected to an LLM provider.
func (c *CopilotFlow) IsConnected() bool {
	c.providerMu.RLock()
	defer c.providerMu.RUnlock()
	return c.provider != nil
}

// RunTask creates a task with the given name, URL, and headless flag, persists it
// to the database, submits it to the execution queue, and returns the task ID.
func (c *CopilotFlow) RunTask(ctx context.Context, name, url string, headless bool) (string, error) {
	if name == "" {
		name = "tui-task"
	}
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	task := models.Task{
		ID:         fmt.Sprintf("tui-%d", time.Now().UnixNano()),
		Name:       name,
		URL:        url,
		Steps:      []models.TaskStep{{Action: models.ActionNavigate, Value: url}},
		Status:     models.TaskStatusPending,
		Priority:   models.PriorityNormal,
		MaxRetries: 1,
		Headless:   headless,
		CreatedAt:  time.Now(),
	}

	if err := c.db.CreateTask(ctx, task); err != nil {
		return "", fmt.Errorf("create task: %w", err)
	}
	if err := c.queue.Submit(ctx, task); err != nil {
		return "", fmt.Errorf("submit task: %w", err)
	}
	return task.ID, nil
}

// SetHeadless updates the global headless flag on the browser runner at runtime.
// When true, all tasks are forced into headless mode regardless of their own Headless field.
// When false, each task controls its own headless mode via task.Headless.
func (c *CopilotFlow) SetHeadless(headless bool) {
	c.runner.SetForceHeadless(headless)
}

// CurrentModel returns the current model name.
func (c *CopilotFlow) CurrentModel() string {
	c.providerMu.RLock()
	defer c.providerMu.RUnlock()
	if c.provider == nil {
		return ""
	}
	return c.provider.Model()
}

// CurrentProvider returns the current provider name.
func (c *CopilotFlow) CurrentProvider() string {
	c.providerMu.RLock()
	defer c.providerMu.RUnlock()
	if c.provider == nil {
		return ""
	}
	return c.provider.Provider()
}

// Stop shuts down the copilot agent cleanly.
func (c *CopilotFlow) Stop() {
	c.StopPolling()
	if c.queue != nil {
		c.queue.Stop()
	}
	if c.proxyManager != nil {
		c.proxyManager.Stop()
	}
	if c.db != nil {
		c.db.Close()
	}
}

// StartPolling begins the background task-execution loop. Safe to call
// multiple times — stops any existing loop before starting a new one.
func (c *CopilotFlow) StartPolling(ctx context.Context) {
	c.StopPolling() // cancel any prior loop

	c.pollingMu.Lock()
	pollingCtx, cancel := context.WithCancel(ctx)
	c.pollingCancel = cancel
	c.pollingMu.Unlock()

	go func() {
		log.Printf("[copilot] background worker started (poll interval: %v)", c.pollInterval)
		ticker := time.NewTicker(c.pollInterval)
		defer ticker.Stop()
		c.processPending(pollingCtx)
		for {
			select {
			case <-ticker.C:
				c.processPending(pollingCtx)
			case <-pollingCtx.Done():
				log.Println("[copilot] background worker stopped")
				return
			}
		}
	}()
}

// StopPolling cancels the background task-execution loop.
func (c *CopilotFlow) StopPolling() {
	c.pollingMu.Lock()
	cancel := c.pollingCancel
	c.pollingCancel = nil
	c.pollingMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// IsPolling reports whether the background worker loop is currently running.
func (c *CopilotFlow) IsPolling() bool {
	c.pollingMu.Lock()
	defer c.pollingMu.Unlock()
	return c.pollingCancel != nil
}

// processPending fetches all pending tasks from the DB and submits them to the queue.
func (c *CopilotFlow) processPending(ctx context.Context) {
	tasks, err := c.db.ListTasksByStatus(ctx, models.TaskStatusPending)
	if err != nil {
		log.Printf("[copilot] list pending tasks: %v", err)
		return
	}
	for _, task := range tasks {
		if err := c.queue.Submit(ctx, task); err != nil {
			log.Printf("[copilot] submit task %s: %v", task.ID, err)
		}
	}
}

// registerTools registers all FlowPilot features as callable tools.
func (c *CopilotFlow) registerTools() {
	// Batch operations
	c.tools["create_batch"] = Tool{
		Name:        "create_batch",
		Description: "Create a batch of tasks from a recorded flow with a list of URLs",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"flow_id": map[string]any{
					"type":        "string",
					"description": "ID of the recorded flow to use as template",
				},
				"urls": map[string]any{
					"type":        "array",
					"description": "List of URLs to process",
					"items": map[string]any{
						"type": "string",
					},
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Name for the batch (optional)",
				},
			},
			"required": []string{"flow_id", "urls"},
		},
		Handler: c.toolCreateBatch,
	}

	// Task operations
	c.tools["list_tasks"] = Tool{
		Name:        "list_tasks",
		Description: "List tasks with optional status filter",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status": map[string]any{
					"type":        "string",
					"description": "Filter by task status (pending, running, completed, failed)",
					"enum":        []string{"pending", "running", "completed", "failed", "cancelled"},
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of tasks to return",
					"default":     50,
				},
			},
		},
		Handler: c.toolListTasks,
	}

	// Proxy management
	c.tools["list_proxies"] = Tool{
		Name:        "list_proxies",
		Description: "List all configured proxies and their health status",
		Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: c.toolListProxies,
	}

	// Flow management
	c.tools["list_flows"] = Tool{
		Name:        "list_flows",
		Description: "List all recorded flows",
		Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: c.toolListFlows,
	}

	// System operations
	c.tools["system_status"] = Tool{
		Name:        "system_status",
		Description: "Get current system status and statistics",
		Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: c.toolSystemStatus,
	}

	c.tools["run_task"] = Tool{
		Name:        "run_task",
		Description: "Create and immediately run a single browser automation task with specific steps",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Human-readable task name",
				},
				"url": map[string]any{
					"type":        "string",
					"description": "Starting URL for the task",
				},
				"steps": map[string]any{
					"type":        "array",
					"description": "List of browser steps to execute",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"action":   map[string]any{"type": "string"},
							"selector": map[string]any{"type": "string"},
							"value":    map[string]any{"type": "string"},
						},
						"required": []string{"action"},
					},
				},
				"headless": map[string]any{
					"type":        "boolean",
					"description": "Run in headless mode (no visible browser window). Default false = show browser.",
					"default":     false,
				},
			},
			"required": []string{"name", "url", "steps"},
		},
		Handler: c.toolRunTask,
	}

	// ── v2 tools ────────────────────────────────────────────────────────────

	// Task introspection and management
	c.tools["get_task"] = Tool{
		Name:        "get_task",
		Description: "Retrieve full details for a specific task by its ID",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "The task ID to look up",
				},
			},
			"required": []string{"task_id"},
		},
		Handler: c.toolGetTask,
	}

	c.tools["cancel_task"] = Tool{
		Name:        "cancel_task",
		Description: "Cancel a pending, queued, or running task by ID",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "ID of the task to cancel",
				},
			},
			"required": []string{"task_id"},
		},
		Handler: c.toolCancelTask,
	}

	c.tools["retry_task"] = Tool{
		Name:        "retry_task",
		Description: "Retry a failed or cancelled task by resetting its status and re-queuing it",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "ID of the task to retry",
				},
			},
			"required": []string{"task_id"},
		},
		Handler: c.toolRetryTask,
	}

	// Batch management
	c.tools["get_batch_progress"] = Tool{
		Name:        "get_batch_progress",
		Description: "Get aggregate progress statistics (pending/running/completed/failed) for a batch",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"batch_id": map[string]any{
					"type":        "string",
					"description": "The batch ID to inspect",
				},
			},
			"required": []string{"batch_id"},
		},
		Handler: c.toolGetBatchProgress,
	}

	c.tools["cancel_batch"] = Tool{
		Name:        "cancel_batch",
		Description: "Cancel all pending, queued, and running tasks that belong to a batch",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"batch_id": map[string]any{
					"type":        "string",
					"description": "ID of the batch to cancel",
				},
			},
			"required": []string{"batch_id"},
		},
		Handler: c.toolCancelBatch,
	}

	// Step-level logging
	c.tools["get_task_logs"] = Tool{
		Name:        "get_task_logs",
		Description: "Retrieve step execution logs for a task",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "ID of the task whose logs to retrieve",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of log entries to return (default 50)",
					"default":     50,
				},
			},
			"required": []string{"task_id"},
		},
		Handler: c.toolGetTaskLogs,
	}

	// Proxy management
	c.tools["add_proxy"] = Tool{
		Name:        "add_proxy",
		Description: "Add a new proxy server to the proxy pool",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"server": map[string]any{
					"type":        "string",
					"description": "Proxy address in host:port format",
				},
				"protocol": map[string]any{
					"type":        "string",
					"description": "Proxy protocol: http, https, or socks5",
					"enum":        []string{"http", "https", "socks5"},
				},
				"username": map[string]any{
					"type":        "string",
					"description": "Optional proxy username for authentication",
				},
				"password": map[string]any{
					"type":        "string",
					"description": "Optional proxy password for authentication",
				},
				"geo": map[string]any{
					"type":        "string",
					"description": "Optional ISO country code for geo-targeting (e.g. US, GB)",
				},
			},
			"required": []string{"server"},
		},
		Handler: c.toolAddProxy,
	}

	c.tools["delete_proxy"] = Tool{
		Name:        "delete_proxy",
		Description: "Remove a proxy server from the pool by its ID",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"proxy_id": map[string]any{
					"type":        "string",
					"description": "ID of the proxy to delete",
				},
			},
			"required": []string{"proxy_id"},
		},
		Handler: c.toolDeleteProxy,
	}

	// Schedule management
	c.tools["list_schedules"] = Tool{
		Name:        "list_schedules",
		Description: "List all configured recurring task schedules",
		Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: c.toolListSchedules,
	}

	c.tools["create_schedule"] = Tool{
		Name:        "create_schedule",
		Description: "Create a new recurring schedule that runs a recorded flow on a cron expression",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"flow_id": map[string]any{
					"type":        "string",
					"description": "ID of the recorded flow to run on schedule",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Human-readable name for this schedule",
				},
				"cron": map[string]any{
					"type":        "string",
					"description": "Cron expression (e.g. '0 * * * *' for hourly, '0 9 * * 1-5' for weekday mornings)",
				},
			},
			"required": []string{"flow_id", "name", "cron"},
		},
		Handler: c.toolCreateSchedule,
	}
}

// GetToolDefinitions returns the tool definitions for function calling.
func (c *CopilotFlow) GetToolDefinitions() []ToolDefinition {
	var defs []ToolDefinition
	for _, tool := range c.tools {
		defs = append(defs, ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Schema,
			},
		})
	}
	return defs
}

// Process processes a natural language request and returns the response.
// Prior conversation turns are prepended automatically for multi-turn context.
func (c *CopilotFlow) Process(ctx context.Context, input string) (string, error) {
	c.providerMu.RLock()
	p := c.provider
	c.providerMu.RUnlock()
	if p == nil {
		return "", fmt.Errorf("not connected to any LLM provider. Use /connect command first")
	}

	// Trim and include prior conversation turns for multi-turn context.
	if c.history != nil {
		c.history.Trim(20)
	}
	prior := make([]Message, 0)
	if c.history != nil {
		prior = c.history.Messages()
	}
	messages := make([]Message, 0, len(prior)+2)
	messages = append(messages, Message{Role: "system", Content: systemPrompt})
	messages = append(messages, prior...)
	messages = append(messages, Message{Role: "user", Content: input})

	response, err := p.ChatCompletion(ctx, messages, c.GetToolDefinitions())
	if err != nil {
		return "", fmt.Errorf("chat completion failed: %w", err)
	}

	// Persist this turn so the next call has full context.
	if c.history != nil {
		c.history.Append("user", input)
		c.history.Append("assistant", response.Content)
	}

	if len(response.ToolCalls) > 0 {
		var results []string
		for _, call := range response.ToolCalls {
			tool, ok := c.tools[call.Name]
			if !ok {
				results = append(results, fmt.Sprintf("Unknown tool: %s", call.Name))
				continue
			}

			result, err := tool.Handler(ctx, call.Arguments)
			if err != nil {
				results = append(results, fmt.Sprintf("Error in %s: %v", call.Name, err))
				continue
			}

			results = append(results, fmt.Sprintf("%s result: %v", call.Name, result))
		}

		if response.Content != "" {
			return fmt.Sprintf("%s\n\n%s", response.Content, strings.Join(results, "\n")), nil
		}
		return strings.Join(results, "\n"), nil
	}

	return response.Content, nil
}

// ProcessStream processes a natural language request with streaming response.
// Calls onToken for each content chunk received. Returns error if stream fails.
// Prior conversation turns are prepended automatically for multi-turn context.
func (c *CopilotFlow) ProcessStream(ctx context.Context, input string, onToken func(string)) error {
	c.providerMu.RLock()
	p := c.provider
	c.providerMu.RUnlock()

	if p == nil {
		return fmt.Errorf("not connected to any LLM provider. Use /connect command first")
	}

	// Trim and include prior conversation turns for multi-turn context.
	if c.history != nil {
		c.history.Trim(20)
	}
	prior := make([]Message, 0)
	if c.history != nil {
		prior = c.history.Messages()
	}
	messages := make([]Message, 0, len(prior)+2)
	messages = append(messages, Message{Role: "system", Content: systemPrompt})
	messages = append(messages, prior...)
	messages = append(messages, Message{Role: "user", Content: input})

	stream := p.StreamChatCompletion(ctx, messages, c.GetToolDefinitions())

	var toolCallAccum *ToolCall
	var fullContent strings.Builder

	for chunk := range stream {
		if chunk.Error != nil {
			return chunk.Error
		}

		if chunk.Content != "" {
			fullContent.WriteString(chunk.Content)
			onToken(chunk.Content)
		}

		if chunk.ToolCall != nil {
			toolCallAccum = chunk.ToolCall
		}

		if chunk.Done {
			break
		}
	}

	// Execute tool call if accumulated.
	if toolCallAccum != nil {
		tool, ok := c.tools[toolCallAccum.Name]
		if !ok {
			onToken(fmt.Sprintf("\n[Error: Unknown tool: %s]", toolCallAccum.Name))
			return nil
		}

		result, err := tool.Handler(ctx, toolCallAccum.Arguments)
		if err != nil {
			onToken(fmt.Sprintf("\n[Error in %s: %v]", toolCallAccum.Name, err))
			return nil
		}

		resultStr := fmt.Sprintf("\n[%s result: %v]", toolCallAccum.Name, result)
		onToken(resultStr)
		fullContent.WriteString(resultStr)
	}

	// Persist this turn to conversation history for subsequent calls.
	if c.history != nil {
		c.history.Append("user", input)
		c.history.Append("assistant", fullContent.String())
	}

	return nil
}

// ClearHistory resets the conversation history, starting a fresh context window.
func (c *CopilotFlow) ClearHistory() {
	if c.history != nil {
		c.history.Clear()
	}
}

// systemPrompt is the system prompt for the copilot.
const systemPrompt = `You are CopilotFlow, an AI assistant for FlowPilot automation platform.

You have access to tools that let you:
- Create and run batches of automation tasks
- Monitor task status and progress
- Manage proxy configurations
- Work with recorded automation flows
- Check system health and status

Help the user automate their browser workflows. Be concise and helpful.
When you need to perform actions, use the available tools.
If you need information that isn't available, ask the user for it.`
