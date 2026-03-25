package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"flowpilot/internal/models"
	"flowpilot/internal/scheduler"
	"flowpilot/internal/validation"

	"github.com/google/uuid"
)

func (a *App) CreateSchedule(name, cronExpr, flowID, url string, proxyConfig models.ProxyConfig, priority int, headless bool, tags []string) (*models.Schedule, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if err := validation.ValidateTaskName(name); err != nil {
		return nil, fmt.Errorf("create schedule: %w", err)
	}
	if strings.TrimSpace(cronExpr) == "" {
		return nil, fmt.Errorf("create schedule: cron expression is required")
	}
	cronSched, err := scheduler.ParseCron(cronExpr)
	if err != nil {
		return nil, fmt.Errorf("create schedule: %w", err)
	}
	if strings.TrimSpace(flowID) == "" {
		return nil, fmt.Errorf("create schedule: flowId is required")
	}
	if err := validation.ValidateTaskURL(url); err != nil {
		return nil, fmt.Errorf("create schedule: %w", err)
	}
	if err := validation.ValidatePriority(models.TaskPriority(priority)); err != nil {
		return nil, fmt.Errorf("create schedule: %w", err)
	}
	if err := validation.ValidateTags(tags); err != nil {
		return nil, fmt.Errorf("create schedule: %w", err)
	}
	if err := validation.ValidateProxyConfig(proxyConfig); err != nil {
		return nil, fmt.Errorf("create schedule: %w", err)
	}

	now := time.Now()
	nextRun := cronSched.Next(now)

	sched := models.Schedule{
		ID:          uuid.New().String(),
		Name:        name,
		CronExpr:    cronExpr,
		FlowID:      flowID,
		URL:         url,
		ProxyConfig: proxyConfig,
		Priority:    models.TaskPriority(priority),
		Headless:    headless,
		Tags:        tags,
		Enabled:     true,
		NextRunAt:   &nextRun,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := a.db.CreateSchedule(a.ctx, sched); err != nil {
		return nil, fmt.Errorf("create schedule: %w", err)
	}
	return &sched, nil
}

func (a *App) GetSchedule(id string) (*models.Schedule, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if id == "" {
		return nil, fmt.Errorf("get schedule: id is required")
	}
	return a.db.GetSchedule(a.ctx, id)
}

func (a *App) ListSchedules() ([]models.Schedule, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.db.ListSchedules(a.ctx)
}

func (a *App) UpdateSchedule(id, name, cronExpr, flowID, url string, proxyConfig models.ProxyConfig, priority int, headless bool, tags []string, enabled bool) error {
	if err := a.ready(); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("update schedule: id is required")
	}
	if err := validation.ValidateTaskName(name); err != nil {
		return fmt.Errorf("update schedule: %w", err)
	}
	if strings.TrimSpace(cronExpr) == "" {
		return fmt.Errorf("update schedule: cron expression is required")
	}
	cronSched, err := scheduler.ParseCron(cronExpr)
	if err != nil {
		return fmt.Errorf("update schedule: %w", err)
	}
	if strings.TrimSpace(flowID) == "" {
		return fmt.Errorf("update schedule: flowId is required")
	}
	if err := validation.ValidateTaskURL(url); err != nil {
		return fmt.Errorf("update schedule: %w", err)
	}
	if err := validation.ValidatePriority(models.TaskPriority(priority)); err != nil {
		return fmt.Errorf("update schedule: %w", err)
	}
	if err := validation.ValidateTags(tags); err != nil {
		return fmt.Errorf("update schedule: %w", err)
	}
	if err := validation.ValidateProxyConfig(proxyConfig); err != nil {
		return fmt.Errorf("update schedule: %w", err)
	}

	now := time.Now()
	var nextRun *time.Time
	if enabled {
		nr := cronSched.Next(now)
		nextRun = &nr
	}

	sched := models.Schedule{
		ID:          id,
		Name:        name,
		CronExpr:    cronExpr,
		FlowID:      flowID,
		URL:         url,
		ProxyConfig: proxyConfig,
		Priority:    models.TaskPriority(priority),
		Headless:    headless,
		Tags:        tags,
		Enabled:     enabled,
		NextRunAt:   nextRun,
		UpdatedAt:   now,
	}
	return a.db.UpdateSchedule(a.ctx, sched)
}

func (a *App) DeleteSchedule(id string) error {
	if err := a.ready(); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("delete schedule: id is required")
	}
	return a.db.DeleteSchedule(a.ctx, id)
}

func (a *App) ToggleSchedule(id string, enabled bool) error {
	if err := a.ready(); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("toggle schedule: id is required")
	}

	existing, err := a.db.GetSchedule(a.ctx, id)
	if err != nil {
		return fmt.Errorf("toggle schedule: %w", err)
	}

	existing.Enabled = enabled
	existing.UpdatedAt = time.Now()

	if enabled {
		cronSched, err := scheduler.ParseCron(existing.CronExpr)
		if err != nil {
			return fmt.Errorf("toggle schedule: %w", err)
		}
		nr := cronSched.Next(time.Now())
		existing.NextRunAt = &nr
	} else {
		existing.NextRunAt = nil
	}

	return a.db.UpdateSchedule(a.ctx, *existing)
}

func (a *App) SubmitScheduledTask(ctx context.Context, sched models.Schedule) error {
	flow, err := a.db.GetRecordedFlow(ctx, sched.FlowID)
	if err != nil {
		return fmt.Errorf("get flow for scheduled task: %w", err)
	}

	steps := models.FlowToTaskSteps(*flow)
	if len(steps) > 0 && steps[0].Action == models.ActionNavigate && steps[0].Value == "" {
		steps[0].Value = sched.URL
	}

	task := models.Task{
		ID:         uuid.New().String(),
		Name:       fmt.Sprintf("[sched] %s", sched.Name),
		URL:        sched.URL,
		Steps:      steps,
		Proxy:      sched.ProxyConfig,
		Priority:   sched.Priority,
		Status:     models.TaskStatusPending,
		MaxRetries: 3,
		Tags:       sched.Tags,
		FlowID:     sched.FlowID,
		Headless:   sched.Headless,
		CreatedAt:  time.Now(),
	}

	if err := a.db.CreateTask(ctx, task); err != nil {
		return fmt.Errorf("create scheduled task: %w", err)
	}

	if a.queue != nil {
		if err := a.queue.Submit(ctx, task); err != nil {
			return fmt.Errorf("submit scheduled task: %w", err)
		}
	}

	return nil
}
