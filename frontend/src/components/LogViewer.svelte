<script lang="ts">
  import { ExportTaskLogs, ExportBatchLogs } from '../../wailsjs/go/main/App';
  import type { Task } from '../lib/types';

  export let task: Task | null = null;

  let exporting = false;
  let exportMessage = '';

  async function exportTask() {
    if (!task) return;
    exporting = true;
    try {
      const zipPath = await ExportTaskLogs(task.id);
      exportMessage = `Exported: ${zipPath}`;
    } catch (err: any) {
      exportMessage = err?.message || String(err);
    } finally {
      exporting = false;
    }
  }

  async function exportBatch() {
    if (!task?.batchId) return;
    exporting = true;
    try {
      const zipPath = await ExportBatchLogs(task.batchId);
      exportMessage = `Batch export: ${zipPath}`;
    } catch (err: any) {
      exportMessage = err?.message || String(err);
    } finally {
      exporting = false;
    }
  }
</script>

<div class="panel">
  <div class="panel-header">
    <h3>Advanced Logs Export</h3>
  </div>
  <div class="panel-body">
    <button class="btn-secondary" on:click={exportTask} disabled={!task || exporting}>Export Task Logs</button>
    {#if task?.batchId}
      <button class="btn-secondary" on:click={exportBatch} disabled={exporting}>Export Batch Logs (ZIP)</button>
    {/if}
    {#if exportMessage}
      <div class="hint">{exportMessage}</div>
    {/if}
  </div>
</div>

<style>
  .panel {
    padding: 12px;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--bg-secondary);
  }
  .panel-body {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .hint {
    font-size: 11px;
    color: var(--text-muted);
  }
</style>
