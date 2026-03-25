<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { tasks, selectedTaskId, removeTaskFromStore } from '../lib/store';
  import { StartTask, CancelTask, DeleteTask } from '../../wailsjs/go/main/App';
  import type { Task } from '../lib/types';

  const dispatch = createEventDispatcher();

  let actionError = '';
  let busyTaskIds: string[] = [];

  function markBusy(id: string) {
    busyTaskIds = [...busyTaskIds, id];
  }

  function unmarkBusy(id: string) {
    busyTaskIds = busyTaskIds.filter((x) => x !== id);
  }

  function selectTask(task: Task) {
    selectedTaskId.set(task.id);
  }

  function clearError() {
    actionError = '';
  }

  function isBusy(id: string) {
    return busyTaskIds.includes(id);
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
  }

  async function deleteTask(id: string) {
    if (!confirm('Delete this task?')) {
      return;
    }

    let deleted = false;
    markBusy(id);
    try {
      clearError();
      await DeleteTask(id);
      deleted = true;
    } catch (err: any) {
      actionError = `Delete failed: ${err?.message || err}`;
    } finally {
      unmarkBusy(id);
    }

    if (deleted) {
      removeTaskFromStore(id);
      dispatch('refresh');
    }
  }

  function formatDate(dateStr: string): string {
    if (!dateStr) {
      return '-';
    }

    const d = new Date(dateStr);
    const now = new Date();
    if (d.toDateString() === now.toDateString()) {
      return d.toLocaleTimeString();
    }

    return `${d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })} ${d.toLocaleTimeString(undefined, {
      hour: '2-digit',
      minute: '2-digit',
    })}`;
  }

  function shortId(id: string): string {
    return id.substring(0, 8);
  }
</script>

{#if actionError}
  <div class="action-error" role="alert">{actionError}</div>
{/if}

<div class="table-shell">
  <div class="table-header">
    <div>
      <span class="table-kicker">Queue Overview</span>
      <h3>Tasks</h3>
    </div>
    <p>Select a row to inspect details, logs, and batch progress.</p>
  </div>

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
            <td class="truncate task-name">{task.name}</td>
            <td class="truncate text-muted task-url">{task.url}</td>
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
                  <button
                    type="button"
                    class="btn-primary btn-sm"
                    disabled={isBusy(task.id)}
                    on:click|stopPropagation={() => startTask(task.id)}
                  >
                    {isBusy(task.id) ? 'Starting…' : 'Start'}
                  </button>
                {/if}
                {#if task.status === 'running' || task.status === 'queued'}
                  <button
                    type="button"
                    class="btn-danger btn-sm"
                    disabled={isBusy(task.id)}
                    on:click|stopPropagation={() => cancelTask(task.id)}
                  >
                    {isBusy(task.id) ? 'Stopping…' : 'Cancel'}
                  </button>
                {/if}
                {#if task.status !== 'running' && task.status !== 'queued'}
                  <button
                    type="button"
                    class="btn-secondary btn-sm"
                    disabled={isBusy(task.id)}
                    on:click|stopPropagation={() => deleteTask(task.id)}
                  >
                    Delete
                  </button>
                {/if}
              </div>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="9" class="empty-state">
              No tasks found. Click "+ New Task" to get started.
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>

<style>
  .table-shell {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
  }

  .table-header {
    display: flex;
    align-items: end;
    justify-content: space-between;
    gap: 12px;
    padding: 18px 20px 14px;
    border-bottom: 1px solid rgba(148, 163, 184, 0.12);
    background: rgba(15, 23, 42, 0.55);
  }

  .table-kicker {
    display: inline-block;
    margin-bottom: 6px;
    color: #93c5fd;
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .table-header h3 {
    margin: 0;
    font-size: 18px;
  }

  .table-header p {
    margin: 0;
    color: var(--text-secondary);
    font-size: 13px;
  }

  .table-container {
    flex: 1;
    overflow: auto;
  }

  tbody tr {
    cursor: pointer;
    transition: background-color 0.18s ease, box-shadow 0.18s ease;
  }

  tr.selected {
    background: rgba(37, 99, 235, 0.18) !important;
    box-shadow: inset 3px 0 0 var(--accent);
  }

  .task-name {
    max-width: 220px;
    font-weight: 600;
  }

  .task-url {
    max-width: 280px;
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .tag-list {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }

  .tag-badge {
    padding: 2px 8px;
    background: rgba(59, 130, 246, 0.12);
    color: #93c5fd;
    border: 1px solid rgba(96, 165, 250, 0.2);
    border-radius: 999px;
    font-size: 10px;
    font-weight: 700;
    white-space: nowrap;
  }

  .action-error {
    margin: 16px 16px 0;
    padding: 10px 14px;
    border-radius: 14px;
    border: 1px solid rgba(239, 68, 68, 0.28);
    background: rgba(127, 29, 29, 0.34);
    color: #fca5a5;
    font-size: 12px;
  }

  .empty-state {
    padding: 48px 16px;
    text-align: center;
    color: var(--text-muted);
  }

  @media (max-width: 920px) {
    .table-header {
      flex-direction: column;
      align-items: flex-start;
    }
  }
</style>
