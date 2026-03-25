<script lang="ts">
  import { onDestroy } from 'svelte';
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

  const metricCards = [
    { label: 'Total Tasks', value: () => $taskStats.total, tone: 'neutral' },
    { label: 'Running', value: () => metrics.running, tone: 'running' },
    { label: 'Queued', value: () => metrics.queued, tone: 'queued' },
    { label: 'Proxy Slots', value: () => `${metrics.runningProxied}/${metrics.proxyConcurrencyLimit || '-'}`, tone: 'info' },
    { label: 'Write Buffer', value: () => `${metrics.persistenceQueueDepth}/${metrics.persistenceQueueCapacity || '-'}`, tone: 'info' },
    { label: 'Completed', value: () => $taskStats.completed, tone: 'success' },
    { label: 'Failed', value: () => $taskStats.failed, tone: 'danger' },
  ];

  async function exportPrometheusMetrics() {
    try {
      const app = (window as Window & { go?: { main?: { App?: { GetPrometheusMetrics?: () => Promise<string> } } } }).go?.main?.App;
      const body = await app?.GetPrometheusMetrics?.();
      if (!body) {
        return;
      }
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(body);
        return;
      }
      const blob = new Blob([body], { type: 'text/plain;charset=utf-8' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = 'flowpilot-metrics.prom';
      link.click();
      URL.revokeObjectURL(url);
    } catch (_) {}
  }

  const interval = setInterval(async () => {
    try {
      metrics = await GetQueueMetrics();
    } catch (_) {}
  }, 2000);

  onDestroy(() => clearInterval(interval));
</script>

<header class="header-shell">
  <div class="brand-block">
    <div class="brand-mark">FP</div>
    <div class="brand-copy">
      <span class="eyebrow">Desktop automation control plane</span>
      <div class="title-row">
        <h1>FlowPilot</h1>
        <span class="subtitle">Go + Wails + chromedp</span>
      </div>
      <p>
        Observe queue throughput, inspect failures, and operate browser workflows from one
        focused workspace.
      </p>
    </div>
  </div>

  <div class="header-stats">
    <button class="metric-export" on:click={exportPrometheusMetrics}>Prometheus</button>
    {#each metricCards as card}
      <div class={`stat stat--${card.tone}`}>
        <span class="stat-label">{card.label}</span>
        <span class="stat-value">{card.value()}</span>
      </div>
    {/each}
  </div>
</header>

<style>
  .header-shell {
    display: grid;
    grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr);
    gap: 18px;
    align-items: stretch;
    padding: 24px;
    border-bottom: 1px solid rgba(148, 163, 184, 0.14);
    background:
      radial-gradient(circle at left top, rgba(59, 130, 246, 0.16), transparent 38%),
      linear-gradient(180deg, rgba(15, 23, 42, 0.94), rgba(15, 23, 42, 0.72));
  }

  .brand-block {
    display: flex;
    gap: 16px;
    align-items: flex-start;
  }

  .brand-mark {
    display: grid;
    place-items: center;
    width: 52px;
    height: 52px;
    border-radius: 16px;
    background: linear-gradient(180deg, rgba(59, 130, 246, 0.9), rgba(37, 99, 235, 0.75));
    color: white;
    font-size: 16px;
    font-weight: 800;
    letter-spacing: 0.08em;
    box-shadow: 0 16px 40px rgba(37, 99, 235, 0.28);
  }

  .brand-copy {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .eyebrow {
    color: #93c5fd;
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .title-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 10px;
  }

  h1 {
    margin: 0;
    font-size: 30px;
    line-height: 1;
    font-weight: 800;
    letter-spacing: -0.03em;
  }

  .subtitle {
    padding: 4px 10px;
    border-radius: 999px;
    background: rgba(148, 163, 184, 0.12);
    color: var(--text-secondary);
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .brand-copy p {
    margin: 0;
    max-width: 60ch;
    color: var(--text-secondary);
    font-size: 14px;
    line-height: 1.6;
  }

  .header-stats {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 12px;
  }

  .metric-export {
    grid-column: span 4;
    justify-self: end;
    padding: 8px 12px;
    border-radius: 10px;
    border: 1px solid rgba(148, 163, 184, 0.25);
    background: rgba(15, 23, 42, 0.65);
    color: var(--text-primary);
    font-size: 12px;
    font-weight: 700;
    cursor: pointer;
  }

  .stat {
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    min-height: 82px;
    padding: 14px 16px;
    border-radius: 18px;
    border: 1px solid rgba(148, 163, 184, 0.12);
    background: rgba(15, 23, 42, 0.6);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.03);
  }

  .stat-label {
    font-size: 11px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-weight: 700;
  }

  .stat-value {
    margin-top: 8px;
    font-size: 22px;
    line-height: 1.1;
    font-weight: 800;
    color: var(--text-primary);
  }

  .stat--running .stat-value { color: #60a5fa; }
  .stat--queued .stat-value { color: #fbbf24; }
  .stat--success .stat-value { color: #34d399; }
  .stat--danger .stat-value { color: #f87171; }
  .stat--info .stat-value { color: #e2e8f0; font-size: 19px; }

  @media (max-width: 1200px) {
    .header-shell {
      grid-template-columns: 1fr;
    }

    .header-stats {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }
  }

  @media (max-width: 760px) {
    .header-shell {
      padding: 18px;
    }

    h1 {
      font-size: 24px;
    }

    .header-stats {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }
</style>
