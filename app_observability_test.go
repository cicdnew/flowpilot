package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"flowpilot/internal/monitoring"
	"flowpilot/internal/plugin"
)

func TestObservabilitySnapshotAndEndpoints(t *testing.T) {
	app := &App{config: DefaultAppConfig(), monitor: monitoring.New(), plugins: plugin.NewRegistry()}
	app.config.MetricsAddr = "127.0.0.1:0"
	app.monitor.Steps().SetSlowThreshold(50 * time.Millisecond)
	app.monitor.RecordStep("task-a", "navigate", 0, 20*time.Millisecond, nil)
	app.monitor.RecordStep("task-a", "click", 1, 120*time.Millisecond, context.DeadlineExceeded)
	p := &plugin.Builtin{Spec: plugin.Manifest{ID: "obs", Name: "obs", Version: "1", Actions: []plugin.ActionSpec{{Name: "obs_action"}}}}
	if err := app.plugins.Register(context.Background(), p, ""); err != nil {
		t.Fatal(err)
	}

	snap := app.GetObservabilitySnapshot()
	if len(snap.Steps) != 2 || len(snap.SlowSteps) != 1 || snap.SlowSteps[0].TaskID != "task-a" {
		t.Fatalf("snapshot steps: %+v slow: %+v", snap.Steps, snap.SlowSteps)
	}
	if len(snap.Plugins.Plugins) != 1 || snap.Runtime.Goroutines == 0 || snap.Runtime.HeapMB <= 0 {
		t.Fatalf("snapshot plugins/runtime: %+v %+v", snap.Plugins, snap.Runtime)
	}

	app.startMetricsServer(context.Background())
	t.Cleanup(func() { app.stopMetricsServer(context.Background()) })
	base := "http://" + app.metricsAddress()

	resp, err := http.Get(base + "/api/observability")
	if err != nil {
		t.Fatal(err)
	}
	var decoded ObservabilitySnapshot
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(decoded.Steps) != 2 || decoded.Plugins.Plugins[0].ID != "obs" {
		t.Fatalf("/api/observability: %+v", decoded)
	}

	resp, err = http.Get(base + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	for _, want := range []string{
		`flowpilot_step_duration_ms_count{action="click"} 1`,
		`flowpilot_step_errors_total{action="click"} 1`,
		`flowpilot_step_slow_total{action="click"} 1`,
		"flowpilot_plugins_loaded 1",
		"flowpilot_persistence_backpressure_total 0",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("/metrics missing %q", want)
		}
	}
}
