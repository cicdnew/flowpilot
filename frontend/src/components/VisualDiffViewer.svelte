<script lang="ts">
  import { onMount } from 'svelte';
  import { visualBaselines } from '../lib/store';
  import type { VisualDiff, Task } from '../lib/types';
  import { ListVisualBaselines, CreateVisualBaseline, CompareVisual, ListVisualDiffs, DeleteVisualBaseline, ListTasks } from '../../wailsjs/go/main/App';

  let errorMessage = '';
  let baselineName = '';
  let baselineTaskId = '';
  let baselineScreenshotPath = '';
  let creating = false;

  let compareBaselineId = '';
  let compareTaskId = '';
  let compareThreshold = 0.05;
  let comparing = false;
  let diffResult: VisualDiff | null = null;

  let selectedBaselineId = '';
  let diffs: VisualDiff[] = [];
  let loadingDiffs = false;

  let completedTasks: Task[] = [];

  onMount(async () => {
    await refresh();
    await loadCompletedTasks();
  });

  async function refresh() {
    try {
      errorMessage = '';
      const list = await ListVisualBaselines();
      visualBaselines.set(list || []);
    } catch (err: any) {
      errorMessage = `Failed to load baselines: ${err?.message || err}`;
    }
  }

  async function loadCompletedTasks() {
    try {
      const all = await ListTasks();
      completedTasks = ((all || []) as Task[]).filter(t => t.status === 'completed');
    } catch (_) {}
  }

  async function createBaseline() {
    if (!baselineName || !baselineTaskId || !baselineScreenshotPath) return;
    creating = true;
    try {
      errorMessage = '';
      await CreateVisualBaseline(baselineName, baselineTaskId, baselineScreenshotPath);
      baselineName = '';
      baselineTaskId = '';
      baselineScreenshotPath = '';
      await refresh();
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    } finally {
      creating = false;
    }
  }

  async function compare() {
    if (!compareBaselineId || !compareTaskId) return;
    comparing = true;
    diffResult = null;
    try {
      errorMessage = '';
      const result = await CompareVisual({ baselineId: compareBaselineId, taskId: compareTaskId, threshold: compareThreshold });
      diffResult = result;
    } catch (err: any) {
      errorMessage = `Compare failed: ${err?.message || err}`;
    } finally {
      comparing = false;
    }
  }

  async function loadDiffs(baselineId: string) {
    selectedBaselineId = baselineId;
    loadingDiffs = true;
    try {
      errorMessage = '';
      const list = await ListVisualDiffs(baselineId);
      diffs = list || [];
    } catch (err: any) {
      errorMessage = `Failed to load diffs: ${err?.message || err}`;
    } finally {
      loadingDiffs = false;
    }
  }

  async function removeBaseline(id: string) {
    try {
      errorMessage = '';
      await DeleteVisualBaseline(id);
      if (selectedBaselineId === id) {
        selectedBaselineId = '';
        diffs = [];
      }
      await refresh();
    } catch (err: any) {
      errorMessage = `Failed to delete: ${err?.message || err}`;
    }
  }
</script>

<div class="visual-panel">
  <h3>Visual Diff Testing</h3>
  {#if errorMessage}
    <div class="error-text">{errorMessage}</div>
  {/if}

  <div class="section">
    <h4>Create Baseline</h4>
    <div class="form-row">
      <div class="form-group">
        <label for="vb-name">Name</label>
        <input id="vb-name" bind:value={baselineName} placeholder="Homepage baseline" />
      </div>
      <div class="form-group">
        <label for="vb-task">Task</label>
        <select id="vb-task" bind:value={baselineTaskId}>
          <option value="">Select completed task...</option>
          {#each completedTasks as task}
            <option value={task.id}>{task.name}</option>
          {/each}
        </select>
      </div>
    </div>
    <div class="form-row">
      <div class="form-group">
        <label for="vb-path">Screenshot Path</label>
        <input id="vb-path" bind:value={baselineScreenshotPath} placeholder="/path/to/screenshot.png" />
      </div>
    </div>
    <div class="form-actions">
      <button class="btn-primary btn-sm" on:click={createBaseline} disabled={!baselineName || !baselineTaskId || !baselineScreenshotPath || creating}>
        {creating ? 'Creating...' : 'Create Baseline'}
      </button>
    </div>
  </div>

  <div class="section">
    <h4>Baselines</h4>
    <div class="baseline-list">
      {#each $visualBaselines as bl (bl.id)}
        <div class="baseline-item" class:selected={selectedBaselineId === bl.id}>
          <div class="baseline-info" role="button" tabindex="0" on:click={() => loadDiffs(bl.id)} on:keydown={(e) => e.key === 'Enter' && loadDiffs(bl.id)}>
            <span class="baseline-name">{bl.name}</span>
            <span class="baseline-meta">{bl.url} | {bl.width}x{bl.height}</span>
          </div>
          <button class="btn-danger btn-sm" on:click={() => removeBaseline(bl.id)}>Del</button>
        </div>
      {:else}
        <p class="text-muted empty-msg">No baselines created.</p>
      {/each}
    </div>
  </div>

  <div class="section">
    <h4>Compare</h4>
    <div class="form-row">
      <div class="form-group">
        <label for="cmp-baseline">Baseline</label>
        <select id="cmp-baseline" bind:value={compareBaselineId}>
          <option value="">Select baseline...</option>
          {#each $visualBaselines as bl}
            <option value={bl.id}>{bl.name}</option>
          {/each}
        </select>
      </div>
      <div class="form-group">
        <label for="cmp-task">Task</label>
        <select id="cmp-task" bind:value={compareTaskId}>
          <option value="">Select task...</option>
          {#each completedTasks as task}
            <option value={task.id}>{task.name}</option>
          {/each}
        </select>
      </div>
      <div class="form-group">
        <label for="cmp-threshold">Threshold</label>
        <input id="cmp-threshold" type="number" bind:value={compareThreshold} min="0" max="1" step="0.01" />
      </div>
    </div>
    <div class="form-actions">
      <button class="btn-primary btn-sm" on:click={compare} disabled={!compareBaselineId || !compareTaskId || comparing}>
        {comparing ? 'Comparing...' : 'Run Compare'}
      </button>
    </div>

    {#if diffResult}
      <div class="diff-result" class:diff-pass={diffResult.passed} class:diff-fail={!diffResult.passed}>
        <div class="diff-header">
          <span class="badge" class:badge-completed={diffResult.passed} class:badge-failed={!diffResult.passed}>
            {diffResult.passed ? 'PASS' : 'FAIL'}
          </span>
          <span>{diffResult.diffPercent.toFixed(4)}% diff ({diffResult.pixelCount} pixels)</span>
        </div>
        <div class="diff-meta">
          Threshold: {diffResult.threshold} | {diffResult.width}x{diffResult.height}
        </div>
        {#if diffResult.diffImagePath}
          <div class="diff-path font-mono">{diffResult.diffImagePath}</div>
        {/if}
      </div>
    {/if}
  </div>

  {#if selectedBaselineId && diffs.length > 0}
    <div class="section">
      <h4>Diff History</h4>
      {#if loadingDiffs}
        <p class="text-muted">Loading...</p>
      {:else}
        <div class="diff-list">
          {#each diffs as d (d.id)}
            <div class="diff-item">
              <span class="badge" class:badge-completed={d.passed} class:badge-failed={!d.passed}>
                {d.passed ? 'PASS' : 'FAIL'}
              </span>
              <span>{d.diffPercent.toFixed(4)}%</span>
              <span class="text-muted text-sm">{d.pixelCount}px</span>
              <span class="text-muted text-sm">{new Date(d.createdAt).toLocaleString()}</span>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .visual-panel {
    padding: 16px;
  }
  .visual-panel h3 {
    font-size: 16px;
    margin-bottom: 12px;
  }
  .section {
    background: var(--bg-secondary);
    padding: 12px;
    border-radius: var(--radius);
    border: 1px solid var(--border);
    margin-bottom: 12px;
  }
  .section h4 {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-muted);
    margin-bottom: 8px;
  }
  .form-row {
    display: flex;
    gap: 8px;
    margin-bottom: 8px;
  }
  .form-group {
    flex: 1;
  }
  .form-group label {
    display: block;
    font-size: 12px;
    font-weight: 600;
    color: var(--text-muted);
    margin-bottom: 4px;
  }
  .form-group input,
  .form-group select {
    width: 100%;
  }
  .form-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 4px;
  }
  .baseline-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .baseline-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 10px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    cursor: pointer;
    transition: background 0.15s ease;
  }
  .baseline-item:hover,
  .baseline-item.selected {
    background: var(--bg-tertiary);
  }
  .baseline-info {
    flex: 1;
  }
  .baseline-name {
    font-size: 13px;
    font-weight: 600;
  }
  .baseline-meta {
    display: block;
    font-size: 11px;
    color: var(--text-muted);
    margin-top: 2px;
  }
  .diff-result {
    margin-top: 8px;
    padding: 10px;
    border-radius: var(--radius);
    border: 1px solid var(--border);
  }
  .diff-pass {
    background: rgba(16, 185, 129, 0.08);
    border-color: rgba(16, 185, 129, 0.3);
  }
  .diff-fail {
    background: rgba(239, 68, 68, 0.08);
    border-color: rgba(239, 68, 68, 0.3);
  }
  .diff-header {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
  }
  .diff-meta {
    font-size: 11px;
    color: var(--text-muted);
    margin-top: 4px;
  }
  .diff-path {
    font-size: 11px;
    color: var(--text-muted);
    margin-top: 4px;
    word-break: break-all;
  }
  .diff-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .diff-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 8px;
    border-bottom: 1px solid var(--border);
    font-size: 12px;
  }
  .error-text {
    color: var(--danger, #ef4444);
    font-size: 12px;
    margin-bottom: 8px;
  }
  .empty-msg {
    text-align: center;
    padding: 12px;
  }
</style>
