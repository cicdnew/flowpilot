package batch

import (
	"context"
	"testing"

	"flowpilot/internal/models"
)

func TestCreateBatchFromFlow_PropagatesTimeoutAndLoggingPolicy(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)

	captureStepLogs := false
	captureNetworkLogs := true
	captureScreenshots := false
	loggingPolicy := &models.TaskLoggingPolicy{
		CaptureStepLogs:    &captureStepLogs,
		CaptureNetworkLogs: &captureNetworkLogs,
		CaptureScreenshots: &captureScreenshots,
		MaxExecutionLogs:   77,
	}

	flow := makeFlow()
	group, tasks, err := engine.CreateBatchFromFlow(context.Background(), flow, models.AdvancedBatchInput{
		FlowID:        flow.ID,
		URLs:          []string{"https://one.example.com"},
		Priority:      int(models.PriorityNormal),
		Timeout:       90,
		LoggingPolicy: loggingPolicy,
	})
	if err != nil {
		t.Fatalf("CreateBatchFromFlow: %v", err)
	}
	if group.Total != 1 || len(tasks) != 1 {
		t.Fatalf("created %d tasks for group total %d, want 1", len(tasks), group.Total)
	}
	if tasks[0].Timeout != 90 {
		t.Fatalf("Timeout = %d, want 90", tasks[0].Timeout)
	}
	if tasks[0].LoggingPolicy == nil || tasks[0].LoggingPolicy.MaxExecutionLogs != 77 {
		t.Fatalf("LoggingPolicy = %#v, want propagated policy", tasks[0].LoggingPolicy)
	}
}
