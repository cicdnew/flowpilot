package main

import (
	"fmt"
	"time"

	"flowpilot/internal/database"
	"flowpilot/internal/models"
	"flowpilot/internal/validation"

	"github.com/google/uuid"
)

const errCreateTask = "create task: %w"

// CreateTaskParams holds parameters for creating a task.
type CreateTaskParams struct {
	Name          string
	URL           string
	Steps         []models.TaskStep
	ProxyConfig   models.ProxyConfig
	Priority      int
	AutoStart     bool
	Tags          []string
	Timeout       int
	LoggingPolicy *models.TaskLoggingPolicy
}

func (a *App) CreateTask(p CreateTaskParams) (*models.Task, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if err := validation.ValidateTask(p.Name, p.URL, p.Steps, models.TaskPriority(p.Priority), false); err != nil {
		return nil, fmt.Errorf(errCreateTask, err)
	}
	if err := validation.ValidateTags(p.Tags); err != nil {
		return nil, fmt.Errorf(errCreateTask, err)
	}
	if err := validation.ValidateTimeout(p.Timeout); err != nil {
		return nil, fmt.Errorf(errCreateTask, err)
	}
	if err := validation.ValidateProxyConfig(p.ProxyConfig); err != nil {
		return nil, fmt.Errorf(errCreateTask, err)
	}
	if err := validation.ValidateTaskLoggingPolicy(p.LoggingPolicy); err != nil {
		return nil, fmt.Errorf(errCreateTask, err)
	}

	task := models.Task{
		ID:            uuid.New().String(),
		Name:          p.Name,
		URL:           p.URL,
		Steps:         p.Steps,
		Proxy:         p.ProxyConfig,
		Priority:      models.TaskPriority(p.Priority),
		Status:        models.TaskStatusPending,
		MaxRetries:    models.DefaultMaxRetries,
		Timeout:       p.Timeout,
		Tags:          p.Tags,
		CreatedAt:     time.Now(),
		LoggingPolicy: p.LoggingPolicy,
	}

	if err := a.db.CreateTask(a.ctx, task); err != nil {
		return nil, fmt.Errorf(errCreateTask, err)
	}

	if p.AutoStart {
		if err := a.queue.Submit(a.ctx, task); err != nil {
			return nil, fmt.Errorf("submit task: %w", err)
		}
	}

	return &task, nil
}

func (a *App) GetTask(id string) (*models.Task, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if id == "" {
		return nil, fmt.Errorf("get task: id is required")
	}
	return a.db.GetTask(a.ctx, id)
}

func (a *App) ListTasks() ([]models.Task, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.db.ListTasks(a.ctx)
}

func (a *App) ListTasksPaginated(page, pageSize int, status, tag string) (models.PaginatedTasks, error) {
	if err := a.ready(); err != nil {
		return models.PaginatedTasks{}, err
	}
	if err := validation.ValidatePagination(page, pageSize, status, tag); err != nil {
		return models.PaginatedTasks{}, fmt.Errorf("list tasks paginated: %w", err)
	}
	return a.db.ListTasksPaginated(a.ctx, page, pageSize, status, tag)
}

func (a *App) ListTasksByStatus(status string) ([]models.Task, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if err := validation.ValidateStatus(status); err != nil {
		return nil, fmt.Errorf("list tasks by status: %w", err)
	}
	return a.db.ListTasksByStatus(a.ctx, models.TaskStatus(status))
}

func (a *App) StartTask(id string) error {
	if err := a.ready(); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("start task: id is required")
	}
	task, err := a.db.GetTask(a.ctx, id)
	if err != nil {
		return fmt.Errorf("get task for start: %w", err)
	}
	return a.queue.Submit(a.ctx, *task)
}

func (a *App) StartAllPending() error {
	if err := a.ready(); err != nil {
		return err
	}
	tasks, err := a.db.ListTasksByStatus(a.ctx, models.TaskStatusPending)
	if err != nil {
		return fmt.Errorf("list pending tasks: %w", err)
	}
	return a.queue.SubmitBatch(a.ctx, tasks)
}

func (a *App) CancelTask(id string) error {
	if err := a.ready(); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("cancel task: id is required")
	}
	return a.queue.Cancel(id)
}

func (a *App) UpdateTask(id string, p database.TaskUpdateParams, priority int) error {
	if err := a.ready(); err != nil {
		return err
	}
	if err := validation.ValidateTask(p.Name, p.URL, p.Steps, models.TaskPriority(priority), false); err != nil {
		return fmt.Errorf(errUpdateTask, err)
	}
	if err := validation.ValidateTags(p.Tags); err != nil {
		return fmt.Errorf(errUpdateTask, err)
	}
	if err := validation.ValidateTimeout(p.Timeout); err != nil {
		return fmt.Errorf(errUpdateTask, err)
	}
	if err := validation.ValidateProxyConfig(p.ProxyConfig); err != nil {
		return fmt.Errorf(errUpdateTask, err)
	}
	if err := validation.ValidateTaskLoggingPolicy(p.LoggingPolicy); err != nil {
		return fmt.Errorf(errUpdateTask, err)
	}
	p.Priority = models.TaskPriority(priority)
	return a.db.UpdateTask(a.ctx, id, p)
}

func (a *App) DeleteTask(id string) error {
	if err := a.ready(); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("delete task: id is required")
	}
	if a.queue != nil {
		if err := a.queue.Cancel(id); err != nil {
			logWarningf(a.ctx, "cancel before delete for task %s: %v", id, err)
		}
	}
	return a.db.DeleteTask(a.ctx, id)
}

func (a *App) GetTaskStats() (map[string]int, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.db.GetTaskStats(a.ctx)
}

func (a *App) GetRunningCount() int {
	if a.initErr != nil {
		return 0
	}
	if a.queue == nil {
		return 0
	}
	return a.queue.RunningCount()
}

func (a *App) GetQueueMetrics() models.QueueMetrics {
	if a.initErr != nil {
		return models.QueueMetrics{}
	}
	if a.queue == nil {
		return models.QueueMetrics{}
	}
	return a.queue.Metrics()
}

func (a *App) CreateBatch(inputs []models.BatchTaskInput, autoStart bool) ([]models.Task, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	for i, input := range inputs {
		if err := validation.ValidateTask(input.Name, input.URL, input.Steps, models.TaskPriority(input.Priority), false); err != nil {
			return nil, fmt.Errorf("task %d: %w", i, err)
		}
		if err := validation.ValidateProxyConfig(input.Proxy); err != nil {
			return nil, fmt.Errorf("task %d: %w", i, err)
		}
	}

	created := make([]models.Task, 0, len(inputs))
	for _, input := range inputs {
		task := models.Task{
			ID:            uuid.New().String(),
			Name:          input.Name,
			URL:           input.URL,
			Steps:         input.Steps,
			Proxy:         input.Proxy,
			Priority:      models.TaskPriority(input.Priority),
			Status:        models.TaskStatusPending,
			MaxRetries:    models.DefaultMaxRetries,
			Timeout:       input.Timeout,
			Tags:          input.Tags,
			LoggingPolicy: input.LoggingPolicy,
			Headless:      input.Headless,
			CreatedAt:     time.Now(),
		}
		if err := a.db.CreateTask(a.ctx, task); err != nil {
			return created, fmt.Errorf("create task %d: %w", len(created), err)
		}
		created = append(created, task)
	}

	if autoStart && a.queue != nil {
		if err := a.queue.SubmitBatch(a.ctx, created); err != nil {
			return created, fmt.Errorf("submit batch: %w", err)
		}
	}

	return created, nil
}
