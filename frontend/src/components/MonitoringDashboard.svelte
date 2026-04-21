<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    GetMonitoringMetrics,
    GetSystemHealth,
    GetRecentLogs,
    GetLogStats,
    GetQueueMetrics
  } from '../../wailsjs/go/main/App';
  import type { QueueMetrics } from '../lib/types';

  interface RepeatTaskMetrics {
    totalRepeatedBatches: number;
    totalRepeatedTasks: number;
    activeRepeatedBatches: number;
    completedRepeatedTasks: number;
    failedRepeatedTasks: number;
    avgTasksPerBatch: number;
    avgBatchCompletionTimeMs: number;
    counterModeBatches: number;
    rangeModeBatches: number;
    listModeBatches: number;
  }

  interface SystemMetrics {
    uptimeSeconds: number;
    totalRequests: number;
    totalErrors: number;
    memoryUsageMb: number;
    goroutineCount: number;
    databaseConnections: number;
  }

  interface HealthStatus {
    status: string;
    timestamp: string;
    uptime: number;
    components: Record<string, { status: string; message?: string }>;
    repeatTaskMetrics: RepeatTaskMetrics;
    systemMetrics: SystemMetrics;
  }

  interface LogEntry {
    timestamp: string;
    level: string;
    message: string;
    taskId?: string;
    batchId?: string;
    error?: string;
  }

  interface LogStats {
    totalEntries: number;
    byLevel: Record<string, number>;
  }

  let repeatMetrics: RepeatTaskMetrics | null = null;
  let health: HealthStatus | null = null;
  let queueMetrics: QueueMetrics | null = null;
  let recentLogs: LogEntry[] = [];
  let logStats: LogStats | null = null;
  let activeAlerts: any[] = []; // TODO: define proper AlertFiring type
  let refreshInterval: number;
  let loading = true;
  let error = '';

  async function loadData() {
    try {
      const [metrics, healthData, logs, stats, qMetrics] = await Promise.all([
        GetMonitoringMetrics(),
        GetSystemHealth(),
        GetRecentLogs(50),
        GetLogStats(),
        GetQueueMetrics()
      ]);
      
      repeatMetrics = metrics;
      health = healthData;
      recentLogs = logs;
      logStats = stats;
      queueMetrics = qMetrics;
      error = '';
    } catch (err: any) {
      error = err?.message || String(err);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadData();
    refreshInterval = window.setInterval(loadData, 10000); // Refresh every 10s
  });

  onDestroy(() => {
    if (refreshInterval) {
      clearInterval(refreshInterval);
    }
  });

  function formatUptime(seconds: number): string {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = Math.floor(seconds % 60);
    return `${hours}h ${minutes}m ${secs}s`;
  }

  function formatMemory(mb: number): string {
    if (mb > 1024) {
      return `${(mb / 1024).toFixed(2)} GB`;
    }
    return `${mb.toFixed(2)} MB`;
  }

  function formatTimestamp(ts: string): string {
    const date = new Date(ts);
    return date.toLocaleTimeString();
  }

  function getStatusColor(status: string): string {
    switch (status) {
      case 'healthy': return '#10b981';
      case 'degraded': return '#f59e0b';
      case 'unhealthy': return '#ef4444';
      default: return '#6b7280';
    }
  }

  function getLogLevelColor(level: string): string {
    switch (level) {
      case 'debug': return '#6b7280';
      case 'info': return '#3b82f6';
      case 'warning': return '#f59e0b';
      case 'error': return '#ef4444';
      default: return '#6b7280';
    }
  }
</script>

<div class="monitoring-dashboard">
  {#if loading}
    <div class="loading">Loading monitoring data...</div>
  {:else if error}
    <div class="error-banner">{error}</div>
  {:else}
    <!-- Health Status -->
    {#if health}
      <div class="section">
        <h2>System Health</h2>
        <div class="health-card">
          <div class="health-status" style="border-color: {getStatusColor(health.status)}">
            <span class="status-indicator" style="background: {getStatusColor(health.status)}"></span>
            <span class="status-text">{health.status.toUpperCase()}</span>
          </div>
          <div class="health-info">
            <div class="info-item">
              <span class="label">Uptime:</span>
              <span class="value">{formatUptime(health.uptime / 1000000000)}</span>
            </div>
          </div>
        </div>

        <h3>Components</h3>
        <div class="components-grid">
          {#each Object.entries(health.components) as [name, component]}
            <div class="component-card">
              <div class="component-header">
                <span class="component-indicator" style="background: {getStatusColor(component.status)}"></span>
                <span class="component-name">{name}</span>
              </div>
              <div class="component-status">{component.status}</div>
              {#if component.message}
                <div class="component-message">{component.message}</div>
              {/if}
            </div>
          {/each}
        </div>
      </div>
    {/if}

    <!-- Repeat Task Metrics -->
    {#if repeatMetrics}
      <div class="section">
        <h2>Repeated Task Metrics</h2>
        <div class="metrics-grid">
          <div class="metric-card">
            <div class="metric-value">{repeatMetrics.totalRepeatedBatches}</div>
            <div class="metric-label">Total Batches</div>
          </div>
          <div class="metric-card">
            <div class="metric-value">{repeatMetrics.totalRepeatedTasks}</div>
            <div class="metric-label">Total Tasks</div>
          </div>
          <div class="metric-card">
            <div class="metric-value">{repeatMetrics.activeRepeatedBatches}</div>
            <div class="metric-label">Active Batches</div>
          </div>
          <div class="metric-card">
            <div class="metric-value">{repeatMetrics.completedRepeatedTasks}</div>
            <div class="metric-label">Completed</div>
          </div>
          <div class="metric-card">
            <div class="metric-value">{repeatMetrics.failedRepeatedTasks}</div>
            <div class="metric-label">Failed</div>
          </div>
          <div class="metric-card">
            <div class="metric-value">{repeatMetrics.avgTasksPerBatch.toFixed(1)}</div>
            <div class="metric-label">Avg Tasks/Batch</div>
          </div>
        </div>

        <h3>Batch Modes</h3>
        <div class="mode-stats">
          <div class="mode-item">
            <span class="mode-label">Counter:</span>
            <span class="mode-value">{repeatMetrics.counterModeBatches}</span>
          </div>
          <div class="mode-item">
            <span class="mode-label">Range:</span>
            <span class="mode-value">{repeatMetrics.rangeModeBatches}</span>
          </div>
          <div class="mode-item">
            <span class="mode-label">List:</span>
            <span class="mode-value">{repeatMetrics.listModeBatches}</span>
          </div>
        </div>
      </div>
    {/if}

    <!-- System Metrics -->
    {#if health?.systemMetrics}
      <div class="section">
        <h2>System Metrics</h2>
        <div class="metrics-grid">
          <div class="metric-card">
            <div class="metric-value">{formatMemory(health.systemMetrics.memoryUsageMb)}</div>
            <div class="metric-label">Memory Usage</div>
          </div>
          <div class="metric-card">
            <div class="metric-value">{health.systemMetrics.goroutineCount}</div>
            <div class="metric-label">Goroutines</div>
          </div>
          <div class="metric-card">
            <div class="metric-value">{health.systemMetrics.databaseConnections}</div>
            <div class="metric-label">DB Connections</div>
          </div>
          <div class="metric-card">
            <div class="metric-value">{health.systemMetrics.totalRequests}</div>
            <div class="metric-label">Total Requests</div>
          </div>
          <div class="metric-card">
            <div class="metric-value">{health.systemMetrics.totalErrors}</div>
            <div class="metric-label">Total Errors</div>
          </div>
        </div>
      </div>
     {/if}

     <!-- Queue Performance -->
     {#if health}
       <div class="section">
         <h2>Queue Performance</h2>
         <div class="metrics-grid">
           <div class="metric-card">
             <div class="metric-value">{health.systemMetrics?.avgStepDurationMs ? health.systemMetrics.avgStepDurationMs.toFixed(1) + ' ms' : 'N/A'}</div>
             <div class="metric-label">Avg Step Duration</div>
           </div>
           <div class="metric-card">
             <div class="metric-value">{health.systemMetrics?.workerUtilizationPercent ? health.systemMetrics.workerUtilizationPercent.toFixed(1) + '%' : 'N/A'}</div>
             <div class="metric-label">Worker Utilization</div>
           </div>
         </div>
       </div>
     {/if}

     <!-- Active Alerts -->
     {#if activeAlerts?.length}
       <div class="section">
         <h2>Active Alerts ({activeAlerts.length})</h2>
         <div class="alerts-list">
           {#each activeAlerts as alert}
             <div class="alert-item alert--{alert.severity}">
               <div class="alert-header">
                 <span class="alert-severity">{alert.severity.toUpperCase()}</span>
                 <span class="alert-rule">{alert.ruleName}</span>
               </div>
               <div class="alert-body">
                 <div>Value: <strong>{alert.value.toFixed(2)}</strong> / Threshold: {alert.threshold}</div>
                 <div class="alert-time">Fired at: {new Date(alert.fired_at).toLocaleString()}</div>
               </div>
             </div>
           {/each}
         </div>
       </div>
     {/if}

     <!-- Recent Logs -->
    <div class="section">
      <h2>Recent Logs</h2>
      {#if logStats}
        <div class="log-stats">
          <span class="stat-item">Total: {logStats.totalEntries}</span>
          {#each Object.entries(logStats.byLevel) as [level, count]}
            <span class="stat-item" style="color: {getLogLevelColor(level)}">
              {level}: {count}
            </span>
          {/each}
        </div>
      {/if}
      
      <div class="logs-container">
        {#each recentLogs as log}
          <div class="log-entry" style="border-left-color: {getLogLevelColor(log.level)}">
            <div class="log-header">
              <span class="log-timestamp">{formatTimestamp(log.timestamp)}</span>
              <span class="log-level" style="color: {getLogLevelColor(log.level)}">{log.level.toUpperCase()}</span>
            </div>
            <div class="log-message">{log.message}</div>
            {#if log.taskId}
              <div class="log-meta">Task: {log.taskId}</div>
            {/if}
            {#if log.batchId}
              <div class="log-meta">Batch: {log.batchId}</div>
            {/if}
            {#if log.error}
              <div class="log-error">{log.error}</div>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>

<style>
  .monitoring-dashboard {
    padding: 24px;
    max-width: 1400px;
    margin: 0 auto;
  }

  .loading {
    text-align: center;
    padding: 48px;
    color: #94a3b8;
  }

  .error-banner {
    padding: 16px;
    background: #7f1d1d;
    border: 1px solid #991b1b;
    border-radius: 6px;
    color: #fecaca;
    margin-bottom: 24px;
  }

  .section {
    margin-bottom: 32px;
  }

  h2 {
    font-size: 20px;
    font-weight: 600;
    color: #f1f5f9;
    margin-bottom: 16px;
  }

  h3 {
    font-size: 16px;
    font-weight: 500;
    color: #cbd5e1;
    margin: 20px 0 12px 0;
  }

  .health-card {
    background: #1e293b;
    border-radius: 8px;
    padding: 20px;
    margin-bottom: 20px;
  }

  .health-status {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px;
    border-left: 4px solid;
    background: #0f172a;
    border-radius: 4px;
    margin-bottom: 16px;
  }

  .status-indicator {
    width: 12px;
    height: 12px;
    border-radius: 50%;
  }

  .status-text {
    font-weight: 600;
    font-size: 18px;
    color: #f1f5f9;
  }

  .health-info {
    display: flex;
    flex-wrap: wrap;
    gap: 16px;
  }

  .info-item {
    display: flex;
    gap: 8px;
  }

  .info-item .label {
    color: #94a3b8;
  }

  .info-item .value {
    color: #e2e8f0;
    font-weight: 500;
  }

  .components-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
    gap: 16px;
  }

  .component-card {
    background: #1e293b;
    border-radius: 6px;
    padding: 16px;
  }

  .component-header {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
  }

  .component-indicator {
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }

  .component-name {
    font-weight: 500;
    color: #f1f5f9;
    text-transform: capitalize;
  }

  .component-status {
    color: #94a3b8;
    font-size: 14px;
    text-transform: capitalize;
  }

  .component-message {
    margin-top: 8px;
    font-size: 12px;
    color: #64748b;
  }

  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 16px;
  }

  .metric-card {
    background: #1e293b;
    border-radius: 6px;
    padding: 20px;
    text-align: center;
  }

  .metric-value {
    font-size: 32px;
    font-weight: 700;
    color: #60a5fa;
    margin-bottom: 8px;
  }

  .metric-label {
    font-size: 14px;
    color: #94a3b8;
  }

  .mode-stats {
    display: flex;
    gap: 24px;
    padding: 16px;
    background: #1e293b;
    border-radius: 6px;
  }

  .mode-item {
    display: flex;
    gap: 8px;
  }

  .mode-label {
    color: #94a3b8;
  }

  .mode-value {
    color: #60a5fa;
    font-weight: 600;
  }

  .log-stats {
    display: flex;
    gap: 16px;
    margin-bottom: 16px;
    padding: 12px;
    background: #1e293b;
    border-radius: 6px;
  }

  .stat-item {
    font-size: 14px;
    color: #cbd5e1;
  }

  .logs-container {
    background: #1e293b;
    border-radius: 6px;
    padding: 16px;
    max-height: 600px;
    overflow-y: auto;
  }

  .log-entry {
    padding: 12px;
    border-left: 3px solid;
    background: #0f172a;
    border-radius: 4px;
    margin-bottom: 12px;
  }

  .log-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 8px;
  }

  .log-timestamp {
    font-size: 12px;
    color: #64748b;
  }

  .log-level {
    font-size: 12px;
    font-weight: 600;
  }

  .log-message {
    color: #e2e8f0;
    font-size: 14px;
    margin-bottom: 4px;
  }

  .log-meta {
    font-size: 12px;
    color: #64748b;
    margin-top: 4px;
  }

  .log-error {
    margin-top: 8px;
    padding: 8px;
    background: #7f1d1d;
    border-radius: 4px;
    color: #fecaca;
    font-size: 12px;
    font-family: monospace;
  }
</style>
