package main

import (
	"fmt"
	"strings"
	"time"

	"flowpilot/internal/models"

	"github.com/google/uuid"
)

func (a *App) CreateRecordedFlow(name, description, originURL string, steps []models.RecordedStep) (*models.RecordedFlow, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("create flow: name is required")
	}
	flow := models.RecordedFlow{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Steps:       steps,
		OriginURL:   originURL,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := a.db.CreateRecordedFlow(a.ctx, flow); err != nil {
		return nil, fmt.Errorf("create flow: %w", err)
	}
	return &flow, nil
}

func (a *App) ListRecordedFlows() ([]models.RecordedFlow, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.db.ListRecordedFlows(a.ctx)
}

func (a *App) GetRecordedFlow(id string) (*models.RecordedFlow, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if id == "" {
		return nil, fmt.Errorf("get flow: id is required")
	}
	return a.db.GetRecordedFlow(a.ctx, id)
}

func (a *App) DeleteRecordedFlow(id string) error {
	if err := a.ready(); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("delete flow: id is required")
	}
	return a.db.DeleteRecordedFlow(a.ctx, id)
}

func (a *App) UpdateRecordedFlow(flow models.RecordedFlow) error {
	if err := a.ready(); err != nil {
		return err
	}
	if flow.ID == "" {
		return fmt.Errorf("update recorded flow: id is required")
	}
	flow.UpdatedAt = time.Now()
	return a.db.UpdateRecordedFlow(a.ctx, flow)
}

func (a *App) SaveDOMSnapshot(snapshot models.DOMSnapshot) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.db.CreateDOMSnapshot(a.ctx, snapshot)
}

func (a *App) ListDOMSnapshots(flowID string) ([]models.DOMSnapshot, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.db.ListDOMSnapshots(a.ctx, flowID)
}

func (a *App) CreateTaskFromFlow(flowID, name, url string, proxyConfig models.ProxyConfig, priority int, autoStart bool, tags []string) (*models.Task, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	flow, err := a.db.GetRecordedFlow(a.ctx, flowID)
	if err != nil {
		return nil, fmt.Errorf("create task from flow: %w", err)
	}
	steps := models.FlowToTaskSteps(*flow)
	if len(steps) > 0 && steps[0].Action == models.ActionNavigate && steps[0].Value == "" {
		steps[0].Value = url
	}
	return a.CreateTask(name, url, steps, proxyConfig, priority, autoStart, tags, flow.Timeout, flow.LoggingPolicy)
}
