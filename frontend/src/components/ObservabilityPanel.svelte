<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { GetObservabilitySnapshot } from '../../wailsjs/go/main/App';
  import {
    formatDuration,
    formatUptime,
    healthTone,
    normalizeObservability,
    successRate,
    type ObservabilityView,
  } from '../lib/observability';

  const REFRESH_MS = 3000;

  let view: ObservabilityView = normalizeObservability(undefined);
  let errorMessage = '';
  let autoRefresh = true;
  let timer: ReturnType<typeof setInterval> | undefined;

  $: rate = successRate(view.queue);
  $: tone = healthTone(view.health);

  async function refresh() {
    try {
      view = normalizeObservability(await GetObservabilitySnapshot());
      errorMessage = '';
    } catch (err) {
      errorMessage = err instanceof Error ? err.message : String(err);
    }
  }

  function schedule() {
    clearInterval(timer);
    timer = autoRefresh ? setInterval(refresh, REFRESH_MS) : undefined;
  }

  $: autoRefresh, schedule();

  onMount(refresh);
  onDestroy(() => clearInterval(timer));
</script>

<div class="obs">
  <div class="obs-header">
    <div>
      <h3>Observability</h3>
      <p class="muted">
        {view.generatedAt ? `Updated ${new Date(view.generatedAt).toLocaleTimeString()}` : 'Loading…'}
      </p>
    </div>
    <div class="controls">
      <label class="muted"><input type="checkbox" bind:checked={autoRefresh} /> Auto-refresh</label>
      <button class="btn-secondary btn-sm" on:click={refresh}>Refresh</button>
    </div>
  </div>

  {#if errorMessage}<div class="error-text" role="alert">{errorMessage}</div>{/if}

  <div class="cards">
    <div class="card">
      <div class="label">Health</div>
      <div class="value tone-{tone}">{view.health}</div>
      <div class="muted">Uptime {formatUptime(view.runtime.uptimeSec)}</div>
    </div>
    <div class="card">
      <div class="label">Queue</div>
      <div class="value">{view.queue.running ?? 0} running · {view.queue.queued ?? 0} queued</div>
      <div class="muted">
        {view.queue.totalCompleted ?? 0} done · {view.queue.totalFailed ?? 0} failed · {view.queue.totalCancelled ?? 0} cancelled
        {#if rate !== null}· {(rate * 100).toFixed(1)}% success{/if}
      </div>
    </div>
    <div class="card">
      <div class="label">Browser pool</div>
      <div class="value">{view.pool.totalBrowsers}/{view.pool.maxBrowsers} browsers</div>
      <div class="muted">{view.pool.activeTabs} active tabs · {view.pool.idleBrowsers} idle</div>
    </div>
    <div class="card">
      <div class="label">Persistence</div>
      <div class="value">{view.queue.persistenceQueueDepth ?? 0}/{view.queue.persistenceQueueCapacity ?? 0}</div>
      <div class="muted">{view.queue.persistenceBackpressure ?? 0} backpressure waits</div>
    </div>
    <div class="card">
      <div class="label">Plugins</div>
      <div class="value">{view.plugins.running}/{view.plugins.loaded} running</div>
      <div class="muted">{view.plugins.restarts} restarts · {view.plugins.errors} load errors</div>
    </div>
    <div class="card">
      <div class="label">Runtime</div>
      <div class="value">{view.runtime.heapMB.toFixed(1)} MB heap</div>
      <div class="muted">{view.runtime.goroutines} goroutines · {view.runtime.numGC} GCs</div>
    </div>
    <div class="card wide">
      <div class="label">MCP server</div>
      {#if view.mcp.enabled}
        <div class="value"><code>{view.mcp.url}</code></div>
        <div class="muted">{view.mcp.auth ? 'Bearer token required' : 'No authentication (localhost only)'}</div>
      {:else}
        <div class="value">disabled</div>
        <div class="muted">Set FLOWPILOT_MCP_ADDR (e.g. 127.0.0.1:8765) before launching to let AI agents control this app.</div>
      {/if}
    </div>
  </div>

  {#if view.activeAlerts.length > 0}
    <section>
      <h4>Active alerts</h4>
      <ul class="list">
        {#each view.activeAlerts as alert (alert.id)}
          <li class="sev-{alert.severity}">
            <strong>{alert.rule_name}</strong> — {alert.current_value} (threshold {alert.threshold}) · {alert.severity}
          </li>
        {/each}
      </ul>
    </section>
  {/if}

  <section>
    <h4>Step latency by action</h4>
    {#if view.steps.length === 0}
      <p class="muted">No steps executed yet.</p>
    {:else}
      <table>
        <thead>
          <tr><th>Action</th><th>Runs</th><th>p50</th><th>p95</th><th>Max</th><th>Errors</th><th>Slow</th></tr>
        </thead>
        <tbody>
          {#each view.steps as s (s.action)}
            <tr>
              <td><code>{s.action}</code></td>
              <td>{s.count}</td>
              <td>{formatDuration(s.p50Ms)}</td>
              <td>{formatDuration(s.p95Ms)}</td>
              <td>{formatDuration(s.maxMs)}</td>
              <td class:bad={s.errors > 0}>{s.errors} ({(s.errorRate * 100).toFixed(1)}%)</td>
              <td>{s.slowCount}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </section>

  <div class="two-col">
    <section>
      <h4>Slow steps</h4>
      {#if view.slowSteps.length === 0}
        <p class="muted">None over the slow-step threshold.</p>
      {:else}
        <ul class="list">
          {#each view.slowSteps as s}
            <li>
              <code>{s.action}</code> {formatDuration(s.durationMs)} · task {s.taskId.slice(0, 8)} step {s.stepIndex + 1}
              {#if s.error}<span class="bad"> — {s.error}</span>{/if}
            </li>
          {/each}
        </ul>
      {/if}
    </section>
    <section>
      <h4>Recent errors</h4>
      {#if view.recentErrors.length === 0}
        <p class="muted">No recent errors.</p>
      {:else}
        <ul class="list">
          {#each view.recentErrors as err}
            <li class="bad">{err}</li>
          {/each}
        </ul>
      {/if}
    </section>
  </div>
</div>

<style>
  .obs {
    padding: 16px;
  }
  .obs-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 12px;
  }
  .obs-header h3 {
    font-size: 16px;
    margin-bottom: 4px;
  }
  .controls {
    display: flex;
    gap: 12px;
    align-items: center;
  }
  .muted {
    font-size: 12px;
    color: var(--text-muted);
  }
  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 10px;
    margin-bottom: 16px;
  }
  .card {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 10px 12px;
  }
  .card.wide {
    grid-column: span 2;
  }
  .label {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--text-muted);
    margin-bottom: 4px;
  }
  .value {
    font-size: 15px;
    font-weight: 600;
    margin-bottom: 2px;
  }
  .tone-ok {
    color: var(--success, #22c55e);
  }
  .tone-warn {
    color: var(--warning, #f59e0b);
  }
  .tone-bad,
  .bad {
    color: var(--danger, #ef4444);
  }
  section {
    margin-bottom: 16px;
  }
  h4 {
    font-size: 13px;
    margin-bottom: 6px;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 12px;
  }
  th,
  td {
    text-align: left;
    padding: 6px 8px;
    border-bottom: 1px solid var(--border);
  }
  th {
    color: var(--text-muted);
    font-weight: 600;
  }
  .list {
    list-style: none;
    padding: 0;
    margin: 0;
    font-size: 12px;
    max-height: 220px;
    overflow: auto;
  }
  .list li {
    padding: 4px 0;
    border-bottom: 1px solid var(--border);
  }
  .two-col {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }
  .error-text {
    color: var(--danger, #ef4444);
    font-size: 12px;
    margin-bottom: 8px;
  }
</style>
