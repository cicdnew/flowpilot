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

const (
	errCreateSchedule = "create schedule: %w"
	errUpdateSchedule = "update schedule: %w"
)

// ScheduleParams holds parameters for creating or updating a schedule.
type ScheduleParams struct {
	Name        string
	CronExpr    string
	FlowID      string
	URL         string
	ProxyConfig models.ProxyConfig
	Priority    int
	Headless    bool
	Tags        []string
}

// validateAndParseCronExpr validates and parses cron expression (S3776)
func (a *App) validateAndParseCronExpr(cronExpr string) (scheduler.Schedule, error) {
	if strings.TrimSpace(cronExpr) == "" {
		return nil, fmt.Errorf("create schedule: cron expression is required")
	}
	sched, err := scheduler.ParseCron(cronExpr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}
	if sched == nil {
		return nil, fmt.Errorf("invalid cron expression: parser returned nil")
	}
	return sched, nil
}

// validateCreateScheduleParams validates all create schedule parameters (S3776)
func (a *App) validateCreateScheduleParams(p ScheduleParams, cronSched scheduler.Schedule) error {
	checks := []struct {
		name string
		err  error
	}{
		{"name", validation.ValidateTaskName(p.Name)},
		{"flowId", nil}, // Check flowId manually
		{"url", validation.ValidateTaskURL(p.URL)},
		{"priority", validation.ValidatePriority(models.TaskPriority(p.Priority))},
		{"tags", validation.ValidateTags(p.Tags)},
		{"proxy", validation.ValidateProxyConfig(p.ProxyConfig)},
	}
	
	if strings.TrimSpace(p.FlowID) == "" {
		return fmt.Errorf("create schedule: flowId is required")
	}
	
	for _, check := range checks {
		if check.err != nil {
			return fmt.Errorf(errCreateSchedule, check.err)
		}
	}
	return nil
}

func (a *App) CreateSchedule(p ScheduleParams) (*models.Schedule, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	
	cronSched, err := a.validateAndParseCronExpr(p.CronExpr)
	if err != nil {
		return nil, fmt.Errorf(errCreateSchedule, err)
	}
	
	if err := a.validateCreateScheduleParams(p, cronSched); err != nil {
		return nil, err
	}

	now := time.Now()
	nextRun := cronSched.Next(now)

	sched := models.Schedule{
		ID:          uuid.New().String(),
		Name:        p.Name,
		CronExpr:    p.CronExpr,
		FlowID:      p.FlowID,
		URL:         p.URL,
		ProxyConfig: p.ProxyConfig,
		Priority:    models.TaskPriority(p.Priority),
		Headless:    p.Headless,
		Tags:        p.Tags,
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

// validateUpdateScheduleParams validates all update schedule parameters (S3776)
func (a *App) validateUpdateScheduleParams(p ScheduleParams, cronSched scheduler.Schedule) error {
	checks := []struct {
		name string
		err  error
	}{
		{"name", validation.ValidateTaskName(p.Name)},
		{"flowId", nil}, // Check flowId manually
		{"url", validation.ValidateTaskURL(p.URL)},
		{"priority", validation.ValidatePriority(models.TaskPriority(p.Priority))},
		{"tags", validation.ValidateTags(p.Tags)},
		{"proxy", validation.ValidateProxyConfig(p.ProxyConfig)},
	}
	
	if strings.TrimSpace(p.FlowID) == "" {
		return fmt.Errorf("update schedule: flowId is required")
	}
	
	for _, check := range checks {
		if check.err != nil {
			return fmt.Errorf(errUpdateSchedule, check.err)
		}
	}
	return nil
}

func (a *App) UpdateSchedule(id string, p ScheduleParams, enabled bool) error {
	if err := a.ready(); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("update schedule: id is required")
	}
	
	cronSched, err := a.validateAndParseCronExpr(p.CronExpr)
	if err != nil {
		return fmt.Errorf(errUpdateSchedule, err)
	}
	
	if err := a.validateUpdateScheduleParams(p, cronSched); err != nil {
		return err
	}

	now := time.Now()
	var nextRun *time.Time
	if enabled {
		nr := cronSched.Next(now)
		nextRun = &nr
	}

	sched := models.Schedule{
		ID:          id,
		Name:        p.Name,
		CronExpr:    p.CronExpr,
		FlowID:      p.FlowID,
		URL:         p.URL,
		ProxyConfig: p.ProxyConfig,
		Priority:    models.TaskPriority(p.Priority),
		Headless:    p.Headless,
		Tags:        p.Tags,
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
		MaxRetries: models.DefaultMaxRetries,
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
