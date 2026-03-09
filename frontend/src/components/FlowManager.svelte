<script lang="ts">
  import { ListRecordedFlows, DeleteRecordedFlow, PlayRecordedFlow, UpdateRecordedFlow } from '../../wailsjs/go/main/App';
  import { recordedFlows } from '../lib/store';
  import type { RecordedFlow } from '../lib/types';
  import { createEventDispatcher, onMount } from 'svelte';

  const dispatch = createEventDispatcher();

  let loading = false;
  let errorMessage = '';
  let playingFlowId = '';
  let headless = false;
  let editingFlowId = '';
  let editName = '';
  let editDescription = '';
  let editSaving = false;

  async function refresh() {
    loading = true;
    try {
      errorMessage = '';
      const flows = await ListRecordedFlows();
      recordedFlows.set(flows || []);
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    } finally {
      loading = false;
    }
  }

  async function removeFlow(id: string) {
    if (!confirm('Delete this recorded flow?')) return;
    try {
      await DeleteRecordedFlow(id);
      await refresh();
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    }
  }

  async function playFlow(id: string, originUrl: string) {
    playingFlowId = id;
    try {
      errorMessage = '';
      await PlayRecordedFlow(id, originUrl, headless);
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    } finally {
      playingFlowId = '';
    }
  }

  function startEditFlow(flow: RecordedFlow) {
    editingFlowId = flow.id;
    editName = flow.name;
    editDescription = flow.description || '';
  }

  function cancelEditFlow() {
    editingFlowId = '';
    editName = '';
    editDescription = '';
  }

  async function saveEditFlow(flow: RecordedFlow) {
    editSaving = true;
    try {
      errorMessage = '';
      await UpdateRecordedFlow({
        ...flow,
        name: editName,
        description: editDescription,
      } as any);
      cancelEditFlow();
      await refresh();
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    } finally {
      editSaving = false;
    }
  }

  onMount(() => {
    refresh();
  });
</script>

<div class="panel">
  <div class="panel-header">
    <h2>Recorded Flows</h2>
    <div class="header-controls">
      <label class="checkbox">
        <input type="checkbox" bind:checked={headless} />
        Headless
      </label>
      <button class="btn-secondary btn-sm" on:click={refresh} disabled={loading}>Refresh</button>
    </div>
  </div>

  {#if errorMessage}
    <div class="error-banner">{errorMessage}</div>
  {/if}

  <div class="panel-body">
    {#if $recordedFlows.length === 0}
      <div class="empty">No recorded flows yet.</div>
    {:else}
      {#each $recordedFlows as flow}
        <div class="flow-row">
          {#if editingFlowId === flow.id}
            <div class="edit-inline">
              <input bind:value={editName} placeholder="Flow name" />
              <input bind:value={editDescription} placeholder="Description (optional)" />
              <div class="edit-inline-actions">
                <button class="btn-primary btn-sm" on:click={() => saveEditFlow(flow)} disabled={!editName || editSaving}>{editSaving ? 'Saving...' : 'Save'}</button>
                <button class="btn-secondary btn-sm" on:click={cancelEditFlow}>Cancel</button>
              </div>
            </div>
          {:else}
            <div>
              <strong>{flow.name}</strong>
              {#if flow.description}<div class="muted">{flow.description}</div>{/if}
              <div class="muted">{flow.originUrl}</div>
            </div>
            <div class="actions">
              <button class="btn-primary btn-sm" on:click={() => dispatch('use', flow)}>Use</button>
              <button class="btn-secondary btn-sm" on:click={() => startEditFlow(flow)}>Edit</button>
              <button class="btn-success btn-sm" on:click={() => playFlow(flow.id, flow.originUrl)} disabled={playingFlowId === flow.id}>
                {playingFlowId === flow.id ? '...' : '▶ Play'}
              </button>
              <button class="btn-danger btn-sm" on:click={() => removeFlow(flow.id)}>Delete</button>
            </div>
          {/if}
        </div>
      {/each}
    {/if}
  </div>
</div>

<style>
  .panel {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 16px;
  }
  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .header-controls {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .checkbox {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    cursor: pointer;
  }
  .checkbox input[type="checkbox"] {
    width: auto;
  }
  .flow-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 0;
    border-bottom: 1px solid var(--border);
  }
  .muted {
    color: var(--text-muted);
    font-size: 11px;
  }
  .actions {
    display: flex;
    gap: 8px;
  }
  .edit-inline {
    display: flex;
    flex-direction: column;
    gap: 6px;
    flex: 1;
  }
  .edit-inline input {
    width: 100%;
  }
  .edit-inline-actions {
    display: flex;
    gap: 6px;
  }
</style>
