<script lang="ts">
  import { CreateBatch } from '../../wailsjs/go/main/App';
  import type { TaskStep } from '../lib/types';
  import { createEventDispatcher } from 'svelte';

  const dispatch = createEventDispatcher();

  interface BatchEntry {
    name: string;
    url: string;
    priority: number;
    steps: TaskStep[];
  }

  let autoStart = true;
  let errorMessage = '';
  let entries: BatchEntry[] = [
    { name: '', url: '', priority: 5, steps: [{ action: 'navigate', value: '', selector: '' }] },
  ];

  function addEntry() {
    entries = [...entries, { name: '', url: '', priority: 5, steps: [{ action: 'navigate', value: '', selector: '' }] }];
  }

  function removeEntry(i: number) {
    entries = entries.filter((_, idx) => idx !== i);
  }

  function canSubmit(): boolean {
    return entries.length > 0 && entries.every(e => e.name && e.url);
  }

  async function submit() {
    if (!canSubmit()) return;

    const inputs = entries.map(e => ({
      name: e.name,
      url: e.url,
      steps: e.steps.map(s => {
        if (s.action === 'navigate' && !s.value) {
          return { ...s, value: e.url };
        }
        return s;
      }),
      proxy: { server: '', username: '', password: '', geo: '' },
      priority: e.priority,
    }));

    try {
      errorMessage = '';
      await CreateBatch(inputs, autoStart);
      dispatch('created');
      dispatch('close');
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    }
  }
</script>

<div class="modal-overlay" on:click={() => dispatch('close')}>
  <div class="modal" on:click|stopPropagation>
    <div class="modal-header">
      <h2>Batch Create Tasks</h2>
      <button class="btn-secondary btn-sm" on:click={() => dispatch('close')}>x</button>
    </div>

    <div class="modal-body">
      <div class="form-group">
        <label class="checkbox">
          <input type="checkbox" bind:checked={autoStart} />
          Auto-start all tasks
        </label>
      </div>

      {#each entries as entry, i}
        <div class="batch-entry">
          <div class="entry-header">
            <span class="entry-num">#{i + 1}</span>
            <button class="btn-danger btn-sm" on:click={() => removeEntry(i)} disabled={entries.length <= 1}>Remove</button>
          </div>
          <div class="form-row">
            <div class="form-group">
              <label>Name</label>
              <input bind:value={entry.name} placeholder="Task name" />
            </div>
            <div class="form-group">
              <label>URL</label>
              <input bind:value={entry.url} placeholder="https://example.com" />
            </div>
            <div class="form-group" style="max-width: 100px">
              <label>Priority</label>
              <select bind:value={entry.priority}>
                <option value={1}>Low</option>
                <option value={5}>Normal</option>
                <option value={10}>High</option>
              </select>
            </div>
          </div>
        </div>
      {/each}

      <button class="btn-secondary btn-sm mt-2" on:click={addEntry}>+ Add Task</button>
    </div>

    {#if errorMessage}
      <div class="error-banner">{errorMessage}</div>
    {/if}

    <div class="modal-footer">
      <span class="text-muted text-sm">{entries.length} task{entries.length !== 1 ? 's' : ''}</span>
      <div class="footer-buttons">
        <button class="btn-secondary" on:click={() => dispatch('close')}>Cancel</button>
        <button class="btn-primary" on:click={submit} disabled={!canSubmit()}>Create {entries.length} Task{entries.length !== 1 ? 's' : ''}</button>
      </div>
    </div>
  </div>
</div>

<style>
  .modal-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }
  .modal {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 12px;
    width: 700px;
    max-height: 80vh;
    display: flex;
    flex-direction: column;
  }
  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border);
  }
  .modal-header h2 {
    font-size: 16px;
    margin: 0;
  }
  .modal-body {
    padding: 20px;
    overflow-y: auto;
  }
  .modal-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
    padding: 16px 20px;
    border-top: 1px solid var(--border);
  }
  .footer-buttons {
    display: flex;
    gap: 8px;
  }
  .batch-entry {
    padding: 12px;
    background: var(--bg-tertiary);
    border-radius: var(--radius);
    margin-bottom: 8px;
  }
  .entry-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }
  .entry-num {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-muted);
  }
  .form-group {
    flex: 1;
    margin-bottom: 0;
  }
  .form-group label {
    display: block;
    font-size: 11px;
    font-weight: 600;
    color: var(--text-muted);
    margin-bottom: 4px;
  }
  .form-group input, .form-group select {
    width: 100%;
  }
  .form-row {
    display: flex;
    gap: 8px;
  }
  .checkbox {
    display: flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
    font-size: 13px;
  }
  .checkbox input[type="checkbox"] {
    width: auto;
    padding: 0;
  }
  .error-banner {
    padding: 8px 20px;
    background: rgba(239, 68, 68, 0.1);
    color: var(--danger, #ef4444);
    font-size: 12px;
    border-top: 1px solid rgba(239, 68, 68, 0.2);
  }
</style>
