<script lang="ts">
  import { CreateBatchFromFlow } from '../../wailsjs/go/main/App';
  import type { RecordedFlow } from '../lib/types';
  import { createEventDispatcher } from 'svelte';

  export let flow: RecordedFlow | null = null;
  const dispatch = createEventDispatcher();

  let urlList = '';
  let namingTemplate = 'Task \{\{index\}\} - \{\{domain\}\}';
  let priority = 5;
  let autoStart = true;
  let errorMessage = '';
  let submitting = false;

  function parseUrls(): string[] {
    return urlList
      .split('\n')
      .map(u => u.trim())
      .filter(Boolean);
  }

  async function submit() {
    if (!flow) return;
    const urls = parseUrls();
    if (urls.length === 0) return;
    submitting = true;
    try {
      errorMessage = '';
      await CreateBatchFromFlow({
        flowId: flow.id,
        urls,
        namingTemplate,
        priority,
        proxy: { server: '', username: '', password: '', geo: '' },
        tags: [],
        autoStart,
      } as any);
      dispatch('created');
      dispatch('close');
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    } finally {
      submitting = false;
    }
  }

  async function importCSV(event: Event) {
    const input = event.target as HTMLInputElement;
    if (!input.files || input.files.length === 0) return;
    const file = input.files[0];
    const text = await file.text();
    const lines = text.split(/\r?\n/).map(line => line.split(',')[0]?.trim()).filter(Boolean);
    if (lines.length === 0) return;
    const existing = parseUrls();
    const merged = Array.from(new Set([...existing, ...lines]));
    urlList = merged.join('\n');
    input.value = '';
  }
</script>

<div class="modal-overlay" role="button" tabindex="0" on:click={() => dispatch('close')} on:keydown={(e) => e.key === 'Escape' && dispatch('close')}>
  <div class="modal" role="dialog" tabindex="-1" on:click|stopPropagation on:keydown={(e) => e.key === 'Escape' && dispatch('close')}>
    <div class="modal-header">
      <h2>Create Batch from Flow</h2>
      <button class="btn-secondary btn-sm" on:click={() => dispatch('close')}>x</button>
    </div>
    <div class="modal-body">
      {#if flow}
        <div class="flow-info">Flow: <strong>{flow.name}</strong></div>
      {/if}
      <div class="form-group">
        <label for="batch-urls">URLs (one per line)</label>
        <textarea id="batch-urls" bind:value={urlList} rows="8" placeholder="https://example.com"></textarea>
        <div class="hint">
          Import CSV (first column URLs):
          <input type="file" accept=".csv" on:change={importCSV} />
        </div>
      </div>
      <div class="form-group">
        <label for="batch-name-template">Naming Template</label>
        <input id="batch-name-template" bind:value={namingTemplate} placeholder="Task {`{{index}}`} - {`{{domain}}`}" />
      </div>
      <div class="form-row">
        <div class="form-group">
          <label for="batch-priority">Priority</label>
          <select id="batch-priority" bind:value={priority}>
            <option value={1}>Low</option>
            <option value={5}>Normal</option>
            <option value={10}>High</option>
          </select>
        </div>
        <div class="form-group">
        <label for="batch-auto-start">Auto Start</label>
        <label class="checkbox">
          <input id="batch-auto-start" type="checkbox" bind:checked={autoStart} />
          Start immediately
        </label>
        </div>
      </div>
    </div>

    {#if errorMessage}
      <div class="error-banner">{errorMessage}</div>
    {/if}

    <div class="modal-footer">
      <button class="btn-secondary" on:click={() => dispatch('close')}>Cancel</button>
      <button class="btn-primary" on:click={submit} disabled={submitting || parseUrls().length === 0}>
        {submitting ? 'Creating...' : 'Create Batch'}
      </button>
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
  .modal-body {
    padding: 20px;
    overflow-y: auto;
  }
  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 16px 20px;
    border-top: 1px solid var(--border);
  }
  textarea {
    width: 100%;
  }
  .flow-info {
    margin-bottom: 10px;
  }
</style>
