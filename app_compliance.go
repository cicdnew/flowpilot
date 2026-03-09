package main

import (
	"strings"

	"flowpilot/internal/batch"
	"flowpilot/internal/models"
)

func (a *App) GetAuditTrail(taskID string, limit int) ([]models.TaskLifecycleEvent, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.db.ListAuditTrail(a.ctx, taskID, limit)
}

func (a *App) PurgeOldData(retentionDays int) (int64, error) {
	if err := a.ready(); err != nil {
		return 0, err
	}
	if retentionDays <= 0 {
		retentionDays = a.config.RetentionDays
	}
	return a.db.PurgeOldRecords(a.ctx, retentionDays)
}

func (a *App) ParseBatchURLs(input string, isCSV bool) ([]string, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if isCSV {
		return batch.ParseCSVURLs(strings.NewReader(input))
	}
	return batch.ParseURLList(input)
}
