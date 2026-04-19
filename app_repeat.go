package main

import (
	"fmt"

	"flowpilot/internal/models"
	"flowpilot/internal/validation"
)

// CreateRepeatedTask creates multiple task instances from a single task definition with parameter substitution.
func (a *App) CreateRepeatedTask(input models.RepeatTaskInput) (models.BatchGroup, error) {
	if err := a.ready(); err != nil {
		return models.BatchGroup{}, err
	}
	if a.repeatEngine == nil {
		return models.BatchGroup{}, fmt.Errorf("repeat engine unavailable")
	}

	// Validate base task properties
	if err := validation.ValidateTask(input.Name, input.URL, input.Steps, models.TaskPriority(input.Priority), false); err != nil {
		return models.BatchGroup{}, fmt.Errorf("validate task: %w", err)
	}
	if err := validation.ValidateTags(input.Tags); err != nil {
		return models.BatchGroup{}, fmt.Errorf("validate tags: %w", err)
	}
	if err := validation.ValidateTimeout(input.Timeout); err != nil {
		return models.BatchGroup{}, fmt.Errorf("validate timeout: %w", err)
	}
	if err := validation.ValidateProxyConfig(input.ProxyConfig); err != nil {
		return models.BatchGroup{}, fmt.Errorf("validate proxy: %w", err)
	}
	if err := validation.ValidateTaskLoggingPolicy(input.LoggingPolicy); err != nil {
		return models.BatchGroup{}, fmt.Errorf("validate logging policy: %w", err)
	}

	// Validate repeat configuration
	if err := input.Repeat.Validate(); err != nil {
		return models.BatchGroup{}, fmt.Errorf("validate repeat config: %w", err)
	}

	group, tasks, err := a.repeatEngine.CreateRepeatedTasks(a.ctx, input)
	if err != nil {
		return models.BatchGroup{}, err
	}

	// Log and record metrics
	a.logRepeatTaskCreation(group.ID, len(tasks), string(input.Repeat.Mode))

	if input.AutoStart {
		if err := a.queue.SubmitBatch(a.ctx, tasks); err != nil {
			return group, fmt.Errorf("submit repeated tasks: %w", err)
		}
	}

	return group, nil
}
