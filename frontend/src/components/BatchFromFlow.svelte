<script lang="ts">
  import { CreateBatchFromFlow, ParseBatchURLs, ListProxyRoutingPresets } from '../../wailsjs/go/main/App';
  import type { RecordedFlow, ProxyRoutingFallback, ProxyRoutingPreset } from '../lib/types';
  import { createEventDispatcher } from 'svelte';

  export let flow: RecordedFlow | null = null;
  const dispatch = createEventDispatcher();

  let urlList = '';
  let namingTemplate = 'Task \{\{index\}\} - \{\{domain\}\}';
  let priority = 5;
  let autoStart = true;
  let useRandomCountryProxy = false;
  let proxyCountry = '';
  let proxyFallback: ProxyRoutingFallback = 'strict';
  let routingPresets: ProxyRoutingPreset[] = [];
  let selectedPresetId = '';
  let errorMessage = '';
  let submitting = false;

  ListProxyRoutingPresets().then((list) => {
    routingPresets = (list || []) as ProxyRoutingPreset[];
  }).catch(() => {});

  function applyPreset(id: string) {
    const preset = routingPresets.find(p => p.id === id);
    if (!preset) return;
    useRandomCountryProxy = preset.randomByCountry;
    proxyCountry = preset.country || '';
    proxyFallback = (preset.fallback as ProxyRoutingFallback) || 'strict';
  }

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
    if (useRandomCountryProxy && !proxyCountry.trim()) {
      errorMessage = 'Country code is required for batch random-by-country proxy routing.';
      return;
    }
    submitting = true;
    try {
      errorMessage = '';
      await CreateBatchFromFlow({
        flowId: flow.id,
        urls,
        namingTemplate,
        priority,
        proxy: {
          server: '',
          username: '',
          password: '',
          geo: useRandomCountryProxy ? proxyCountry.trim().toUpperCase() : '',
          fallback: proxyFallback,
        },
        proxyCountry: useRandomCountryProxy ? proxyCountry.trim().toUpperCase() : '',
        proxyFallback,
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
    try {
      const parsed = await ParseBatchURLs(text, true);
      const existing = parseUrls();
      const merged = Array.from(new Set([...existing, ...parsed]));
      urlList = merged.join('\n');
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    }
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
      <h4>Batch Proxy Routing</h4>
      {#if routingPresets.length}
        <div class="form-group">
          <label for="batch-routing-preset">Routing Preset</label>
          <select id="batch-routing-preset" bind:value={selectedPresetId} on:change={() => applyPreset(selectedPresetId)}>
            <option value="">Custom</option>
            {#each routingPresets as preset}
              <option value={preset.id}>{preset.name}</option>
            {/each}
          </select>
        </div>
      {/if}
      <label class="checkbox">
        <input type="checkbox" bind:checked={useRandomCountryProxy} />
        Random healthy proxy by country for all tasks in this batch
      </label>
      <div class="form-row">
        <div class="form-group">
          <label for="batch-proxy-country">Country</label>
          <input id="batch-proxy-country" bind:value={proxyCountry} placeholder="US" />
        </div>
        <div class="form-group">
          <label for="batch-proxy-fallback">Fallback</label>
          <select id="batch-proxy-fallback" bind:value={proxyFallback}>
            <option value="strict">Strict country only</option>
            <option value="any_healthy">Fallback to any healthy proxy</option>
            <option value="direct">Fallback to direct connection</option>
          </select>
        </div>
      </div>
      <div class="hint">When enabled, every generated task inherits the same country-random routing rule.</div>
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
