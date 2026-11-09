package main

import (
	"context"
	"runtime"
	"time"

	"flowpilot/internal/browser"
	"flowpilot/internal/models"
	"flowpilot/internal/monitoring"
)

type RuntimeStats struct {
	Goroutines int     `json:"goroutines"`
	HeapMB     float64 `json:"heapMB"`
	NumGC      uint32  `json:"numGC"`
	UptimeSec  int64   `json:"uptimeSec"`
}

type ObservabilitySnapshot struct {
	GeneratedAt  time.Time                    `json:"generatedAt"`
	Health       monitoring.HealthStatus      `json:"health"`
	Queue        models.QueueMetrics          `json:"queue"`
	BrowserPool  browser.PoolStats            `json:"browserPool"`
	ProxyPools   browser.ProxyPoolMetrics     `json:"proxyPools"`
	Plugins      PluginStatus                 `json:"plugins"`
	Steps        []monitoring.ActionStepStats `json:"steps"`
	SlowSteps    []monitoring.SlowStep        `json:"slowSteps"`
	RecentErrors []string                     `json:"recentErrors"`
	ActiveAlerts []models.AlertFiring         `json:"activeAlerts"`
	Runtime      RuntimeStats                 `json:"runtime"`
	MCP          MCPStatus                    `json:"mcp"`
}

type MCPStatus struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url,omitempty"`
	Auth    bool   `json:"auth"`
}

var processStart = time.Now()

func (a *App) GetObservabilitySnapshot() ObservabilitySnapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	snap := ObservabilitySnapshot{
		GeneratedAt:  time.Now().UTC(),
		Health:       a.GetSystemHealth(),
		Queue:        a.GetQueueMetrics(),
		BrowserPool:  a.getSharedPoolMetrics(),
		ProxyPools:   a.getProxyPoolMetrics(),
		Plugins:      a.ListPlugins(),
		Steps:        []monitoring.ActionStepStats{},
		SlowSteps:    []monitoring.SlowStep{},
		RecentErrors: []string{},
		ActiveAlerts: []models.AlertFiring{},
		Runtime: RuntimeStats{
			Goroutines: runtime.NumGoroutine(),
			HeapMB:     float64(mem.HeapInuse) / (1 << 20),
			NumGC:      mem.NumGC,
			UptimeSec:  int64(time.Since(processStart).Seconds()),
		},
	}
	if a.mcpHTTP != nil {
		snap.MCP = MCPStatus{Enabled: true, URL: "http://" + a.mcpHTTP.Addr() + mcpPath, Auth: a.mcpHTTP.auth}
	}
	if a.monitor != nil {
		snap.Steps = a.monitor.Steps().Stats()
		snap.SlowSteps = a.monitor.Steps().SlowSteps(20)
		if errs := a.monitor.GetRecentErrors(20); errs != nil {
			snap.RecentErrors = errs
		}
	}
	if a.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if firing, err := a.db.ListActiveAlerts(ctx); err == nil && firing != nil {
			snap.ActiveAlerts = firing
		}
	}
	return snap
}
