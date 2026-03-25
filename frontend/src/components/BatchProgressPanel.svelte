<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { Task, BatchProgress, BatchGroup } from '../lib/types';
  import { GetBatchProgress, RetryFailedBatch, ListBatchGroups } from '../../wailsjs/go/main/App';

  export let task: Task | null = null;

  let progress: BatchProgress | null = null;
  let loading = false;
  let errorMessage = '';
  let retrying = false;
  let showBatchList = false;
  let batchGroups: BatchGroup[] = [];
  let loadingGroups = false;
  let selectedBatchId: string | undefined = undefined;

  $: batchId = selectedBatchId || task?.batchId;

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

  onDestroy(() => {
    if (interval) {
      clearInterval(interval);
      interval = null;
    }
  });

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

  async function toggleBatchList() {
    showBatchList = !showBatchList;
    if (showBatchList && batchGroups.length === 0) {
      await loadBatchGroups();
    }
  }

  async function loadBatchGroups() {
    loadingGroups = true;
    try {
      errorMessage = '';
      batchGroups = await ListBatchGroups();
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    } finally {
      loadingGroups = false;
    }
  }

  function selectBatch(id: string) {
    selectedBatchId = id;
  }
</script>

<div class="panel">
  <div class="panel-header">
    <h3>Batch Progress</h3>
    <div class="actions">
      <button class="btn-secondary btn-sm" on:click={toggleBatchList}>
        {showBatchList ? 'Hide Batches' : 'Browse Batches'}
      </button>
      {#if batchId}
        <button class="btn-secondary btn-sm" on:click={refresh} disabled={loading}>Refresh</button>
        <button class="btn-primary btn-sm" on:click={retryFailed} disabled={retrying || !progress || progress.failed === 0}>
          {retrying ? 'Retrying...' : 'Retry Failed'}
        </button>
      {/if}
    </div>
  </div>

  {#if showBatchList}
    <div class="batch-list">
      <h4>Batch History</h4>
      {#if loadingGroups}
        <p class="hint">Loading...</p>
      {:else}
        {#each batchGroups as group}
          <button class="batch-item" class:active={batchId === group.id} on:click={() => selectBatch(group.id)}>
            <span class="batch-name">{group.name}</span>
            <span class="batch-info">{group.total} tasks &middot; {new Date(group.createdAt).toLocaleDateString()}</span>
          </button>
        {/each}
        {#if batchGroups.length === 0}
          <p class="empty">No batch groups yet</p>
        {/if}
      {/if}
    </div>
  {/if}

  {#if errorMessage}
    <div class="error-banner">{errorMessage}</div>
  {:else if batchId && progress}
    <div class="grid">
      <div><span class="label">Total</span>{progress.total}</div>
      <div><span class="label">Pending</span>{progress.pending}</div>
      <div><span class="label">Queued</span>{progress.queued}</div>
      <div><span class="label">Running</span>{progress.running}</div>
      <div><span class="label">Completed</span>{progress.completed}</div>
      <div><span class="label">Failed</span>{progress.failed}</div>
    </div>
  {:else if !batchId}
    <div class="hint">Select a batch to view progress.</div>
  {:else}
    <div class="hint">No batch progress available.</div>
  {/if}
</div>

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
  .batch-list {
    margin-bottom: 12px;
    padding-bottom: 12px;
    border-bottom: 1px solid var(--border);
  }
  .batch-list h4 {
    font-size: 12px;
    color: var(--text-muted);
    margin: 0 0 8px 0;
  }
  .batch-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
    padding: 8px 10px;
    margin-bottom: 4px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: transparent;
    cursor: pointer;
    font-size: 12px;
    color: inherit;
    text-align: left;
  }
  .batch-item:hover {
    background: var(--bg-secondary);
    border-color: var(--text-muted);
  }
  .batch-item.active {
    border-color: var(--accent, #4f8ff7);
    background: var(--bg-secondary);
  }
  .batch-name {
    font-weight: 500;
  }
  .batch-info {
    color: var(--text-muted);
    font-size: 11px;
  }
  .empty {
    color: var(--text-muted);
    font-size: 12px;
    text-align: center;
    padding: 12px 0;
    margin: 0;
  }
  .hint {
    color: var(--text-muted);
    font-size: 12px;
  }
</style>
