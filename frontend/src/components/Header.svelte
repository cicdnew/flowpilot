<script lang="ts">
  import { taskStats } from '../lib/store';
  import { GetQueueMetrics } from '../../wailsjs/go/main/App';
  import type { QueueMetrics } from '../lib/types';

  let metrics: QueueMetrics = {
    running: 0,
    queued: 0,
    pending: 0,
    totalSubmitted: 0,
    totalCompleted: 0,
    totalFailed: 0,
    runningProxied: 0,
    proxyConcurrencyLimit: 0,
    persistenceQueueDepth: 0,
    persistenceQueueCapacity: 0,
    persistenceBatchSize: 0,
  };

  const interval = setInterval(async () => {
    try {
      metrics = await GetQueueMetrics();
    } catch (_) {}
  }, 2000);

  import { onDestroy } from 'svelte';
  onDestroy(() => clearInterval(interval));
</script>

<header>
  <div class="header-left">
    <h1>FlowPilot</h1>
    <span class="subtitle">Go + Wails + chromedp</span>
  </div>
  <div class="header-stats">
    <div class="stat">
      <span class="stat-value">{$taskStats.total}</span>
      <span class="stat-label">Total</span>
    </div>
    <div class="stat running">
      <span class="stat-value">{metrics.running}</span>
      <span class="stat-label">Running</span>
    </div>
    <div class="stat queued">
      <span class="stat-value">{metrics.queued}</span>
      <span class="stat-label">Queued</span>
    </div>
    <div class="stat info">
      <span class="stat-value">{metrics.runningProxied}/{metrics.proxyConcurrencyLimit || '-'}</span>
      <span class="stat-label">Proxy Slots</span>
    </div>
    <div class="stat info">
      <span class="stat-value">{metrics.persistenceQueueDepth}/{metrics.persistenceQueueCapacity || '-'}</span>
      <span class="stat-label">Write Buffer</span>
    </div>
    <div class="stat success">
      <span class="stat-value">{$taskStats.completed}</span>
      <span class="stat-label">Done</span>
    </div>
    <div class="stat danger">
      <span class="stat-value">{$taskStats.failed}</span>
      <span class="stat-label">Failed</span>
    </div>
  </div>
</header>

<style>
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 20px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }
  .header-left {
    display: flex;
    align-items: baseline;
    gap: 12px;
  }
  h1 {
    font-size: 18px;
    font-weight: 700;
    margin: 0;
  }
  .subtitle {
    font-size: 12px;
    color: var(--text-muted);
  }
  .header-stats {
    display: flex;
    gap: 20px;
  }
  .stat {
    text-align: center;
  }
  .stat-value {
    display: block;
    font-size: 20px;
    font-weight: 700;
  }
  .stat-label {
    font-size: 11px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .running .stat-value { color: var(--accent); }
  .success .stat-value { color: var(--success); }
  .danger .stat-value { color: var(--danger); }
  .queued .stat-value { color: var(--warning); }
  .info .stat-value { color: var(--text-primary); }
</style>
