<script lang="ts">
  import { tasks, selectedTaskId } from '../lib/store';
  import { StartTask, CancelTask, DeleteTask } from '../../wailsjs/go/main/App';
  import type { Task } from '../lib/types';
  import { createEventDispatcher } from 'svelte';

  const dispatch = createEventDispatcher();

  let actionError = '';
  let busyTaskIds: string[] = [];

  function markBusy(id: string) {
    busyTaskIds = [...busyTaskIds, id];
  }

  function unmarkBusy(id: string) {
    busyTaskIds = busyTaskIds.filter(x => x !== id);
  }

  function selectTask(task: Task) {
    selectedTaskId.set(task.id);
  }

  function clearError() {
    actionError = '';
  }

  async function startTask(id: string) {
    markBusy(id);
    try {
      clearError();
      await StartTask(id);
    } catch (err: any) {
      actionError = `Start failed: ${err?.message || err}`;
    } finally {
      unmarkBusy(id);
    }
    dispatch('refresh');
  }

  async function cancelTask(id: string) {
    markBusy(id);
    try {
      clearError();
      await CancelTask(id);
    } catch (err: any) {
      actionError = `Cancel failed: ${err?.message || err}`;
    } finally {
      unmarkBusy(id);
    }
    dispatch('refresh');
  }

  async function deleteTask(id: string) {
    if (!confirm('Delete this task?')) return;
    markBusy(id);
    try {
      clearError();
      await DeleteTask(id);
    } catch (err: any) {
      actionError = `Delete failed: ${err?.message || err}`;
    } finally {
      unmarkBusy(id);
    }
    dispatch('refresh');
  }

  function formatDate(dateStr: string): string {
    if (!dateStr) return '-';
    const d = new Date(dateStr);
    const now = new Date();
    if (d.toDateString() === now.toDateString()) {
      return d.toLocaleTimeString();
    }
    return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }) + ' ' + d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
  }

  function shortId(id: string): string {
    return id.substring(0, 8);
  }
</script>

{#if actionError}
  <div class="action-error">{actionError}</div>
{/if}
<div class="table-container">
  <table>
    <thead>
      <tr>
        <th>ID</th>
        <th>Name</th>
        <th>URL</th>
        <th>Status</th>
        <th>Tags</th>
        <th>Priority</th>
        <th>Retries</th>
        <th>Created</th>
        <th>Actions</th>
      </tr>
    </thead>
    <tbody>
      {#each $tasks as task (task.id)}
        <tr
          class:selected={$selectedTaskId === task.id}
          on:click={() => selectTask(task)}
        >
          <td class="font-mono">{shortId(task.id)}</td>
          <td class="truncate" style="max-width: 200px">{task.name}</td>
          <td class="truncate text-muted" style="max-width: 250px">{task.url}</td>
          <td>
            <span class="badge badge-{task.status}">{task.status}</span>
          </td>
          <td>
            <div class="tag-list">
              {#each task.tags ?? [] as tag}
                <span class="tag-badge">{tag}</span>
              {/each}
            </div>
          </td>
          <td>{task.priority}</td>
          <td>{task.retryCount}/{task.maxRetries}</td>
          <td class="text-sm text-muted">{formatDate(task.createdAt)}</td>
          <td>
            <div class="actions">
              {#if task.status === 'pending' || task.status === 'failed'}
                <button class="btn-primary btn-sm" on:click|stopPropagation={() => startTask(task.id)}>
                  Start
                </button>
              {/if}
              {#if task.status === 'running' || task.status === 'queued'}
                <button class="btn-danger btn-sm" on:click|stopPropagation={() => cancelTask(task.id)}>
                  Cancel
                </button>
              {/if}
              {#if task.status !== 'running' && task.status !== 'queued'}
                <button class="btn-secondary btn-sm" on:click|stopPropagation={() => deleteTask(task.id)}>
                  Del
                </button>
              {/if}
            </div>
          </td>
        </tr>
      {:else}
        <tr>
          <td colspan="9" style="text-align: center; padding: 40px; color: var(--text-muted);">
            No tasks found. Click "+ New Task" to get started.
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  .table-container {
    flex: 1;
    overflow-y: auto;
  }
  tr.selected {
    background: rgba(59, 130, 246, 0.1) !important;
    border-left: 3px solid var(--accent);
  }
  .actions {
    display: flex;
    gap: 4px;
  }
  tbody tr {
    cursor: pointer;
  }
  .tag-list {
    display: flex;
    flex-wrap: wrap;
    gap: 2px;
  }
  .tag-badge {
    padding: 1px 6px;
    background: rgba(59, 130, 246, 0.15);
    color: var(--accent);
    border-radius: 8px;
    font-size: 10px;
    font-weight: 500;
    white-space: nowrap;
  }
  .action-error {
    padding: 6px 12px;
    background: rgba(239, 68, 68, 0.1);
    color: var(--danger, #ef4444);
    font-size: 12px;
    border-bottom: 1px solid rgba(239, 68, 68, 0.2);
  }
</style>
