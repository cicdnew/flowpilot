<script lang="ts">
  import { statusFilter, tagFilter, allTags } from '../lib/store';
  import { StartAllPending, ExportResultsJSON, ExportResultsCSV } from '../../wailsjs/go/main/App';
  import { createEventDispatcher } from 'svelte';

  const dispatch = createEventDispatcher();

  let exporting = false;
  let toolbarError = '';

  async function startAll() {
    try {
      toolbarError = '';
      await StartAllPending();
    } catch (err: any) {
      toolbarError = `Failed to start tasks: ${err?.message || err}`;
    }
  }

  async function exportJSON() {
    exporting = true;
    try {
      toolbarError = '';
      const path = await ExportResultsJSON();
      alert(`Exported to: ${path}`);
    } catch (err: any) {
      toolbarError = `Export failed: ${err?.message || err}`;
    }
    exporting = false;
  }

  async function exportCSV() {
    exporting = true;
    try {
      toolbarError = '';
      const path = await ExportResultsCSV();
      alert(`Exported to: ${path}`);
    } catch (err: any) {
      toolbarError = `Export failed: ${err?.message || err}`;
    }
    exporting = false;
  }
</script>

{#if toolbarError}
  <div class="toolbar-error">{toolbarError}</div>
{/if}
<div class="toolbar">
  <div class="toolbar-left">
    <button class="btn-primary" on:click={() => dispatch('create')}>
      + New Task
    </button>
    <button class="btn-secondary" on:click={() => dispatch('batchCreate')}>
      + Batch Create
    </button>
    <button class="btn-success" on:click={startAll}>
      Start All Pending
    </button>
  </div>

  <div class="toolbar-center">
    <select bind:value={$statusFilter}>
      <option value="all">All Status</option>
      <option value="pending">Pending</option>
      <option value="queued">Queued</option>
      <option value="running">Running</option>
      <option value="completed">Completed</option>
      <option value="failed">Failed</option>
      <option value="cancelled">Cancelled</option>
      <option value="retrying">Retrying</option>
    </select>
    <select bind:value={$tagFilter}>
      <option value="">All Tags</option>
      {#each $allTags as tag}
        <option value={tag}>{tag}</option>
      {/each}
    </select>
  </div>

  <div class="toolbar-right">
    <button class="btn-secondary btn-sm" on:click={exportJSON} disabled={exporting}>
      Export JSON
    </button>
    <button class="btn-secondary btn-sm" on:click={exportCSV} disabled={exporting}>
      Export CSV
    </button>
  </div>
</div>

<style>
  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 20px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }
  .toolbar-left, .toolbar-right, .toolbar-center {
    display: flex;
    gap: 8px;
  }
  select {
    min-width: 150px;
  }
  .toolbar-error {
    padding: 6px 20px;
    background: rgba(239, 68, 68, 0.1);
    color: var(--danger, #ef4444);
    font-size: 12px;
    border-bottom: 1px solid rgba(239, 68, 68, 0.2);
  }
</style>
