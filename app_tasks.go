package main

import (
	"fmt"
	"time"

	"flowpilot/internal/models"
	"flowpilot/internal/validation"

	"github.com/google/uuid"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) CreateTask(name, url string, steps []models.TaskStep, proxyConfig models.ProxyConfig, priority int, autoStart bool, tags []string, timeout int) (*models.Task, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if err := validation.ValidateTask(name, url, steps, models.TaskPriority(priority), false); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	if err := validation.ValidateTags(tags); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	if err := validation.ValidateTimeout(timeout); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	task := models.Task{
		ID:         uuid.New().String(),
		Name:       name,
		URL:        url,
		Steps:      steps,
		Proxy:      proxyConfig,
		Priority:   models.TaskPriority(priority),
		Status:     models.TaskStatusPending,
		MaxRetries: 3,
		Timeout:    timeout,
		Tags:       tags,
		CreatedAt:  time.Now(),
	}

	if err := a.db.CreateTask(a.ctx, task); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	if autoStart {
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

func (a *App) UpdateTask(id, name, url string, steps []models.TaskStep, proxyConfig models.ProxyConfig, priority int, tags []string, timeout int) error {
	if err := a.ready(); err != nil {
		return err
	}
	if err := validation.ValidateTask(name, url, steps, models.TaskPriority(priority), false); err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	if err := validation.ValidateTags(tags); err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	if err := validation.ValidateTimeout(timeout); err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return a.db.UpdateTask(a.ctx, id, name, url, steps, proxyConfig, models.TaskPriority(priority), tags, timeout)
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
			wailsRuntime.LogWarningf(a.ctx, "cancel before delete for task %s: %v", id, err)
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
	}

	created := make([]models.Task, 0, len(inputs))
	for _, input := range inputs {
		task := models.Task{
			ID:         uuid.New().String(),
			Name:       input.Name,
			URL:        input.URL,
			Steps:      input.Steps,
			Proxy:      input.Proxy,
			Priority:   models.TaskPriority(input.Priority),
			Status:     models.TaskStatusPending,
			MaxRetries: 3,
			CreatedAt:  time.Now(),
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
