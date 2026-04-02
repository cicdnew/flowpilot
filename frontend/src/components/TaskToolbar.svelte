<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { statusFilter, tagFilter, allTags } from '../lib/store';
  import { StartAllPending, ExportResultsJSON, ExportResultsCSV } from '../../wailsjs/go/main/App';

  const dispatch = createEventDispatcher();

  let exporting = false;
  let toolbarError = '';
  let exportSuccess = '';

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
      exportSuccess = '';
      const path = await ExportResultsJSON();
      exportSuccess = `Exported to: ${path}`;
      setTimeout(() => { exportSuccess = ''; }, 4000);
    } catch (err: unknown) {
      toolbarError = `Export failed: ${err instanceof Error ? err.message : String(err)}`;
    } finally {
      exporting = false;
    }
  }

  async function exportCSV() {
    exporting = true;
    try {
      toolbarError = '';
      exportSuccess = '';
      const path = await ExportResultsCSV();
      exportSuccess = `Exported to: ${path}`;
      setTimeout(() => { exportSuccess = ''; }, 4000);
    } catch (err: unknown) {
      toolbarError = `Export failed: ${err instanceof Error ? err.message : String(err)}`;
    } finally {
      exporting = false;
    }
  }
</script>

{#if toolbarError}
  <div class="toolbar-error" role="alert">{toolbarError}</div>
{/if}
{#if exportSuccess}
  <div class="toolbar-success" role="status">{exportSuccess}</div>
{/if}

<section class="toolbar-shell">
  <div class="toolbar-section toolbar-section--primary">
    <div class="section-copy">
      <span class="section-label">Actions</span>
      <p>Launch tasks, create new work, and export results from one control bar.</p>
    </div>
    <div class="action-row">
      <button type="button" class="btn-primary" on:click={() => dispatch('create')}>
        + New Task
      </button>
      <button type="button" class="btn-secondary" on:click={() => dispatch('batchCreate')}>
        + Batch Create
      </button>
      <button type="button" class="btn-success" on:click={startAll}>
        Start All Pending
      </button>
    </div>
  </div>

  <div class="toolbar-section toolbar-section--filters">
    <div class="section-copy">
      <span class="section-label">Filters</span>
      <p>Narrow the queue view by task state or tag.</p>
    </div>
    <div class="filter-row">
      <label>
        <span>Status</span>
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
      </label>
      <label>
        <span>Tag</span>
        <select bind:value={$tagFilter}>
          <option value="">All Tags</option>
          {#each $allTags as tag}
            <option value={tag}>{tag}</option>
          {/each}
        </select>
      </label>
    </div>
  </div>

  <div class="toolbar-section toolbar-section--exports">
    <div class="section-copy">
      <span class="section-label">Exports</span>
      <p>Capture queue output for analysis or sharing.</p>
    </div>
    <div class="action-row action-row--compact">
      <button type="button" class="btn-secondary btn-sm" on:click={exportJSON} disabled={exporting}>
        Export JSON
      </button>
      <button type="button" class="btn-secondary btn-sm" on:click={exportCSV} disabled={exporting}>
        Export CSV
      </button>
    </div>
  </div>
</section>

<style>
  .toolbar-shell {
    display: grid;
    grid-template-columns: 1.4fr 1fr 0.85fr;
    gap: 14px;
    padding-top: 16px;
  }

  .toolbar-section {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 16px 18px;
    border: 1px solid rgba(148, 163, 184, 0.14);
    border-radius: 18px;
    background: rgba(15, 23, 42, 0.62);
  }

  .section-copy {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .section-label {
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: #93c5fd;
  }

  .section-copy p {
    margin: 0;
    color: var(--text-secondary);
    font-size: 13px;
  }

  .action-row,
  .filter-row {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    align-items: end;
  }

  .action-row--compact {
    align-items: center;
  }

  label {
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-width: 0;
    color: var(--text-muted);
    font-size: 12px;
    font-weight: 600;
  }

  select {
    min-width: 160px;
  }

  .toolbar-error {
    margin-top: 16px;
    padding: 10px 14px;
    border-radius: 14px;
    border: 1px solid rgba(239, 68, 68, 0.28);
    background: rgba(127, 29, 29, 0.34);
    color: #fca5a5;
    font-size: 12px;
  }

  .toolbar-success {
    margin-top: 16px;
    padding: 10px 14px;
    border-radius: 14px;
    border: 1px solid rgba(34, 197, 94, 0.28);
    background: rgba(20, 83, 45, 0.34);
    color: #86efac;
    font-size: 12px;
  }

  @media (max-width: 1200px) {
    .toolbar-shell {
      grid-template-columns: 1fr;
    }
  }
</style>
