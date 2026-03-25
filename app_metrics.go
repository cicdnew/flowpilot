package main

import (
	"context"
	"fmt"
	"strings"

	"flowpilot/internal/browser"
	"flowpilot/internal/models"
)

func (a *App) GetPrometheusMetrics() string {
	queueMetrics := a.GetQueueMetrics()
	taskMetrics := a.GetTaskMetrics()
	proxyMetrics := a.getProxyMetrics()
	sharedPoolMetrics := a.getSharedPoolMetrics()
	proxyPoolMetrics := a.getProxyPoolMetrics()

	lines := make([]string, 0, 72)
	appendMetric := func(help, metricType, name string, value any) {
		lines = append(lines,
			fmt.Sprintf("# HELP %s %s", name, help),
			fmt.Sprintf("# TYPE %s %s", name, metricType),
			fmt.Sprintf("%s %v", name, value),
		)
	}

	appendMetric("Number of currently running tasks.", "gauge", "flowpilot_queue_running", queueMetrics.Running)
	appendMetric("Number of queued tasks.", "gauge", "flowpilot_queue_queued", queueMetrics.Queued)
	appendMetric("Number of pending tasks tracked by the queue.", "gauge", "flowpilot_queue_pending", queueMetrics.Pending)
	appendMetric("Number of running proxied tasks.", "gauge", "flowpilot_queue_running_proxied", queueMetrics.RunningProxied)
	appendMetric("Configured proxy concurrency limit.", "gauge", "flowpilot_queue_proxy_concurrency_limit", queueMetrics.ProxyConcurrencyLimit)
	appendMetric("Total completed tasks observed in memory.", "counter", "flowpilot_tasks_completed_total", taskMetrics.Completed)
	appendMetric("Total failed tasks observed in memory.", "counter", "flowpilot_tasks_failed_total", taskMetrics.Failed)
	appendMetric("Average completed task duration in milliseconds.", "gauge", "flowpilot_tasks_avg_duration_ms", taskMetrics.AvgDurationMs)
	appendMetric("Task queue depth snapshot from task metrics.", "gauge", "flowpilot_tasks_queue_depth", taskMetrics.QueueDepth)
	appendMetric("Current persistence writer queue depth.", "gauge", "flowpilot_persistence_queue_depth", queueMetrics.PersistenceQueueDepth)
	appendMetric("Persistence writer queue capacity.", "gauge", "flowpilot_persistence_queue_capacity", queueMetrics.PersistenceQueueCapacity)
	appendMetric("Total proxies in the configured pool.", "gauge", "flowpilot_proxies_total", proxyMetrics.Total)
	appendMetric("Total healthy proxies.", "gauge", "flowpilot_proxies_healthy", proxyMetrics.Healthy)
	appendMetric("Total unhealthy proxies.", "gauge", "flowpilot_proxies_unhealthy", proxyMetrics.Unhealthy)
	appendMetric("Total proxies with unknown health status.", "gauge", "flowpilot_proxies_unknown", proxyMetrics.Unknown)
	appendMetric("Shared browser pool browser count.", "gauge", "flowpilot_browser_pool_browsers", sharedPoolMetrics.TotalBrowsers)
	appendMetric("Shared browser pool browser capacity.", "gauge", "flowpilot_browser_pool_capacity", sharedPoolMetrics.MaxBrowsers)
	appendMetric("Shared browser pool active tab count.", "gauge", "flowpilot_browser_pool_active_tabs", sharedPoolMetrics.ActiveTabs)
	appendMetric("Shared browser pool idle browser count.", "gauge", "flowpilot_browser_pool_idle_browsers", sharedPoolMetrics.IdleBrowsers)
	appendMetric("Total proxy-keyed browser pools.", "gauge", "flowpilot_proxy_browser_pools", proxyPoolMetrics.Pools)
	appendMetric("Proxy-keyed browser count.", "gauge", "flowpilot_proxy_browser_pool_browsers", proxyPoolMetrics.TotalBrowsers)
	appendMetric("Proxy-keyed browser capacity.", "gauge", "flowpilot_proxy_browser_pool_capacity", proxyPoolMetrics.MaxBrowsers)
	appendMetric("Proxy-keyed active tab count.", "gauge", "flowpilot_proxy_browser_pool_active_tabs", proxyPoolMetrics.ActiveTabs)
	appendMetric("Proxy-keyed idle browser count.", "gauge", "flowpilot_proxy_browser_pool_idle_browsers", proxyPoolMetrics.IdleBrowsers)
	return strings.Join(lines, "\n") + "\n"
}

type proxyMetrics struct {
	Total     int
	Healthy   int
	Unhealthy int
	Unknown   int
}

func (a *App) getProxyMetrics() proxyMetrics {
	if a.db == nil {
		return proxyMetrics{}
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	proxies, err := a.db.ListProxies(ctx)
	if err != nil {
		return proxyMetrics{}
	}
	metrics := proxyMetrics{Total: len(proxies)}
	for _, proxy := range proxies {
		switch proxy.Status {
		case models.ProxyStatusHealthy:
			metrics.Healthy++
		case models.ProxyStatusUnhealthy:
			metrics.Unhealthy++
		default:
			metrics.Unknown++
		}
	}
	return metrics
}

func (a *App) getSharedPoolMetrics() browser.PoolStats {
	if a.pool == nil {
		return browser.PoolStats{}
	}
	return a.pool.Stats()
}

func (a *App) getProxyPoolMetrics() browser.ProxyPoolMetrics {
	if a.runner == nil {
		return browser.ProxyPoolMetrics{}
	}
	return a.runner.ProxyPoolMetrics()
}
