package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"flowpilot/internal/models"
	"flowpilot/internal/recorder"
	"flowpilot/internal/validation"

	"github.com/google/uuid"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) StartRecording(url string) error {
	if err := a.ready(); err != nil {
		return err
	}
	a.recorderMu.Lock()
	defer a.recorderMu.Unlock()

	if a.activeRecorder != nil {
		return fmt.Errorf("recording already in progress")
	}

	if strings.TrimSpace(url) == "" {
		return fmt.Errorf("start recording: url is required")
	}

	flowID := uuid.New().String()
	recCtx, recCancel := context.WithCancel(a.ctx)

	a.recordedSteps = nil
	a.activeRecorder = recorder.New(recCtx, flowID, func(step models.RecordedStep) {
		a.recorderMu.Lock()
		a.recordedSteps = append(a.recordedSteps, step)
		a.recorderMu.Unlock()
		wailsRuntime.EventsEmit(a.ctx, "recorder:step", step)
	})
	a.recorderCancel = recCancel

	snapshotDir := filepath.Join(a.dataDir, "snapshots", flowID)
	if snapshotter, err := recorder.NewSnapshotter(snapshotDir); err != nil {
		logWarningf(a.ctx, "snapshot init failed: %v", err)
	} else {
		a.activeRecorder.SetSnapshotter(snapshotter)
	}

	if err := a.activeRecorder.Start(url); err != nil {
		a.activeRecorder = nil
		recCancel()
		return fmt.Errorf("start recording: %w", err)
	}

	a.activeRecorder.SetWSCallback(func(log models.WebSocketLog) {
		wailsRuntime.EventsEmit(a.ctx, "recorder:websocket", log)
	})

	logInfof(a.ctx, "Recording started for flow %s at %s", flowID, url)
	return nil
}

func (a *App) StopRecording() ([]models.RecordedStep, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	a.recorderMu.Lock()
	defer a.recorderMu.Unlock()

	if a.activeRecorder == nil {
		return nil, fmt.Errorf("no active recording session")
	}

	netLogs := a.activeRecorder.NetworkLogs()
	wsLogs := a.activeRecorder.WebSocketLogs()
	flowID := a.activeRecorder.FlowID()

	a.activeRecorder.Stop()

	if a.recorderCancel != nil {
		a.recorderCancel()
	}

	steps := make([]models.RecordedStep, len(a.recordedSteps))
	copy(steps, a.recordedSteps)

	a.activeRecorder = nil
	a.recorderCancel = nil
	a.recordedSteps = nil

	if len(netLogs) > 0 && a.db != nil {
		if err := a.db.InsertNetworkLogs(a.ctx, flowID, netLogs); err != nil {
			logWarningf(a.ctx, "failed to persist network logs: %v", err)
		}
	}

	if len(wsLogs) > 0 && a.db != nil {
		if err := a.db.InsertWebSocketLogs(a.ctx, flowID, wsLogs); err != nil {
			logWarningf(a.ctx, "failed to persist websocket logs: %v", err)
		}
	}

	logInfof(a.ctx, "Recording stopped, captured %d steps, %d network requests, %d websocket events", len(steps), len(netLogs), len(wsLogs))
	return steps, nil
}

func (a *App) PlayRecordedFlow(flowID, url string, headless bool, timeout int, loggingPolicy *models.TaskLoggingPolicy) (*models.Task, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	flow, err := a.db.GetRecordedFlow(a.ctx, flowID)
	if err != nil {
		return nil, fmt.Errorf("play flow: %w", err)
	}
	steps := models.FlowToTaskSteps(*flow)
	if len(steps) > 0 && steps[0].Action == models.ActionNavigate && steps[0].Value == "" {
		steps[0].Value = url
	}
	if url == "" && flow.OriginURL != "" {
		url = flow.OriginURL
	}

	if err := validation.ValidateTask(flow.Name, url, steps, models.PriorityNormal, false); err != nil {
		return nil, fmt.Errorf("play flow: %w", err)
	}

	task := models.Task{
		ID:         uuid.New().String(),
		Name:       "Playback: " + flow.Name,
		URL:        url,
		Steps:      steps,
		Priority:   models.PriorityNormal,
		Status:     models.TaskStatusPending,
		MaxRetries: models.DefaultMaxRetries,
		Headless:   headless,
		FlowID:     flowID,
		Timeout:    timeout,
		CreatedAt:  time.Now(),
		LoggingPolicy: func() *models.TaskLoggingPolicy {
			if loggingPolicy != nil {
				return loggingPolicy
			}
			return &models.TaskLoggingPolicy{
				CaptureStepLogs:    &a.config.CaptureStepLogs,
				CaptureNetworkLogs: &a.config.CaptureNetworkLogs,
				CaptureScreenshots: &a.config.CaptureScreenshots,
				MaxExecutionLogs:   a.config.MaxExecutionLogs,
			}
		}(),
	}

	if err := a.db.CreateTask(a.ctx, task); err != nil {
		return nil, fmt.Errorf("play flow create task: %w", err)
	}
	if err := a.queue.Submit(a.ctx, task); err != nil {
		return nil, fmt.Errorf("play flow submit: %w", err)
	}
	return &task, nil
}

func (a *App) IsRecording() bool {
	a.recorderMu.Lock()
	defer a.recorderMu.Unlock()
	return a.activeRecorder != nil
}
