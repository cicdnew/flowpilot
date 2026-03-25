package main

import (
	"testing"

	"flowpilot/internal/models"
)

func TestPlayRecordedFlowPropagatesTimeoutAndLoggingPolicy(t *testing.T) {
	app := setupTestAppWithQueue(t)
	flow, err := app.CreateRecordedFlow("Flow", "", "https://example.com", []models.RecordedStep{{Action: models.ActionNavigate, Value: "https://example.com"}})
	if err != nil {
		t.Fatalf("CreateRecordedFlow: %v", err)
	}

	captureStepLogs := false
	captureNetworkLogs := true
	captureScreenshots := true
	loggingPolicy := &models.TaskLoggingPolicy{
		CaptureStepLogs:    &captureStepLogs,
		CaptureNetworkLogs: &captureNetworkLogs,
		CaptureScreenshots: &captureScreenshots,
		MaxExecutionLogs:   42,
	}

	task, err := app.PlayRecordedFlow(flow.ID, "https://override.example.com", true, 45, loggingPolicy)
	if err != nil {
		t.Fatalf("PlayRecordedFlow: %v", err)
	}
	if task.Timeout != 45 {
		t.Fatalf("Timeout = %d, want 45", task.Timeout)
	}
	if task.LoggingPolicy == nil || task.LoggingPolicy.MaxExecutionLogs != 42 {
		t.Fatalf("LoggingPolicy = %#v, want propagated policy", task.LoggingPolicy)
	}
	if task.MaxRetries != models.DefaultMaxRetries {
		t.Fatalf("MaxRetries = %d, want %d", task.MaxRetries, models.DefaultMaxRetries)
	}
}

func TestCreateTaskFromFlowPropagatesTimeoutAndLoggingPolicy(t *testing.T) {
	app := setupTestApp(t)
	captureStepLogs := true
	captureNetworkLogs := false
	captureScreenshots := true
	flow := models.RecordedFlow{
		Name:      "Flow",
		OriginURL: "https://example.com",
		Steps:     []models.RecordedStep{{Action: models.ActionNavigate, Value: "https://example.com"}},
		Timeout:   30,
		LoggingPolicy: &models.TaskLoggingPolicy{
			CaptureStepLogs:    &captureStepLogs,
			CaptureNetworkLogs: &captureNetworkLogs,
			CaptureScreenshots: &captureScreenshots,
			MaxExecutionLogs:   21,
		},
	}
	createdFlow, err := app.CreateRecordedFlow(flow.Name, "", flow.OriginURL, flow.Steps)
	if err != nil {
		t.Fatalf("CreateRecordedFlow: %v", err)
	}
	createdFlow.Timeout = flow.Timeout
	createdFlow.LoggingPolicy = flow.LoggingPolicy
	if err := app.UpdateRecordedFlow(*createdFlow); err != nil {
		t.Fatalf("UpdateRecordedFlow: %v", err)
	}

	task, err := app.CreateTaskFromFlow(createdFlow.ID, "Task", "https://target.example.com", models.ProxyConfig{}, 5, false, []string{"tag"})
	if err != nil {
		t.Fatalf("CreateTaskFromFlow: %v", err)
	}
	if task.Timeout != 30 {
		t.Fatalf("Timeout = %d, want 30", task.Timeout)
	}
	if task.LoggingPolicy == nil || task.LoggingPolicy.MaxExecutionLogs != 21 {
		t.Fatalf("LoggingPolicy = %#v, want propagated policy", task.LoggingPolicy)
	}
}
