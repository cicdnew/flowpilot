<script lang="ts">
  import type { Task, BatchProgress } from '../lib/types';
  import { GetBatchProgress, RetryFailedBatch } from '../../wailsjs/go/main/App';

  export let task: Task | null = null;

  let progress: BatchProgress | null = null;
  let loading = false;
  let errorMessage = '';
  let retrying = false;

  $: batchId = task?.batchId;

  async function refresh() {
    if (!batchId) return;
    loading = true;
    try {
      errorMessage = '';
      progress = await GetBatchProgress(batchId);
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    } finally {
      loading = false;
    }
  }

  $: if (batchId) {
    refresh();
  } else {
    progress = null;
  }

  let interval: ReturnType<typeof setInterval> | null = null;
  $: if (batchId) {
    if (interval) {
      clearInterval(interval);
    }
    interval = setInterval(refresh, 5000);
  } else if (interval) {
    clearInterval(interval);
    interval = null;
  }

  async function retryFailed() {
    if (!batchId) return;
    retrying = true;
    try {
      errorMessage = '';
      await RetryFailedBatch(batchId);
      await refresh();
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    } finally {
      retrying = false;
    }
  }
</script>

{#if batchId}
  <div class="panel">
    <div class="panel-header">
      <h3>Batch Progress</h3>
      <div class="actions">
        <button class="btn-secondary btn-sm" on:click={refresh} disabled={loading}>Refresh</button>
        <button class="btn-primary btn-sm" on:click={retryFailed} disabled={retrying || !progress || progress.failed === 0}>
          {retrying ? 'Retrying...' : 'Retry Failed'}
        </button>
      </div>
    </div>
    {#if errorMessage}
      <div class="error-banner">{errorMessage}</div>
    {:else if progress}
      <div class="grid">
        <div><span class="label">Total</span>{progress.total}</div>
        <div><span class="label">Pending</span>{progress.pending}</div>
        <div><span class="label">Queued</span>{progress.queued}</div>
        <div><span class="label">Running</span>{progress.running}</div>
        <div><span class="label">Completed</span>{progress.completed}</div>
        <div><span class="label">Failed</span>{progress.failed}</div>
      </div>
    {:else}
      <div class="hint">No batch progress available.</div>
    {/if}
  </div>
{/if}

<style>
  .panel {
    padding: 12px;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--bg-secondary);
  }
  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }
  .actions {
    display: flex;
    gap: 8px;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 6px 12px;
    font-size: 12px;
  }
  .label {
    display: block;
    font-size: 10px;
    text-transform: uppercase;
    color: var(--text-muted);
  }
</style>
