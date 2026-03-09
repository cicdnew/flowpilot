package main

import (
	"fmt"

	"flowpilot/internal/models"
)

func (a *App) CreateBatchFromFlow(input models.AdvancedBatchInput) (models.BatchGroup, error) {
	if err := a.ready(); err != nil {
		return models.BatchGroup{}, err
	}
	if a.batchEngine == nil {
		return models.BatchGroup{}, fmt.Errorf("batch engine unavailable")
	}
	flow, err := a.db.GetRecordedFlow(a.ctx, input.FlowID)
	if err != nil {
		return models.BatchGroup{}, fmt.Errorf("get flow: %w", err)
	}
	group, tasks, err := a.batchEngine.CreateBatchFromFlow(a.ctx, *flow, input)
	if err != nil {
		return models.BatchGroup{}, err
	}
	if input.AutoStart {
		if err := a.queue.SubmitBatch(a.ctx, tasks); err != nil {
			return group, fmt.Errorf("submit batch: %w", err)
		}
	}
	return group, nil
}

func (a *App) GetBatchProgress(batchID string) (models.BatchProgress, error) {
	if err := a.ready(); err != nil {
		return models.BatchProgress{}, err
	}
	return a.db.GetBatchProgress(a.ctx, batchID)
}

func (a *App) ListBatchGroups() ([]models.BatchGroup, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.db.ListBatchGroups(a.ctx)
}

func (a *App) ListTasksByBatch(batchID string) ([]models.Task, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.db.ListTasksByBatch(a.ctx, batchID)
}

func (a *App) RetryFailedBatch(batchID string) ([]models.Task, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if batchID == "" {
		return nil, fmt.Errorf("retry batch: batchID is required")
	}
	failed, err := a.db.ListTasksByBatchStatus(a.ctx, batchID, models.TaskStatusFailed)
	if err != nil {
		return nil, fmt.Errorf("retry batch: %w", err)
	}
	if len(failed) == 0 {
		return failed, nil
	}
	for i, task := range failed {
		if err := a.db.ResetRetryCount(a.ctx, task.ID); err != nil {
			return failed, fmt.Errorf("retry batch reset retry: %w", err)
		}
		if err := a.db.UpdateTaskStatus(a.ctx, task.ID, models.TaskStatusPending, "retry batch"); err != nil {
			return failed, fmt.Errorf("retry batch update: %w", err)
		}
		failed[i].RetryCount = 0
		failed[i].Status = models.TaskStatusPending
	}
	if err := a.queue.SubmitBatch(a.ctx, failed); err != nil {
		return failed, fmt.Errorf("retry batch submit: %w", err)
	}
	return failed, nil
}
