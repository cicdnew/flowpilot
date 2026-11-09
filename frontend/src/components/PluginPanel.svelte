<script lang="ts">
  import { onMount } from 'svelte';
  import { ListPlugins, ReloadPlugins } from '../../wailsjs/go/main/App';
  import { normalizePluginStatus, pluginActionCount } from '../lib/plugins';
  import { refreshStepActionState } from '../lib/step-actions';
  import type { PluginStatus } from '../lib/types';

  let status: PluginStatus = { dir: '', plugins: [], errors: [] };
  let loading = false;
  let errorMessage = '';

  $: actionCount = pluginActionCount(status);

  onMount(refresh);

  async function refresh() {
    await run(() => ListPlugins());
  }

  async function reload() {
    await run(() => ReloadPlugins());
    await refreshStepActionState();
  }

  async function run(fetcher: () => Promise<unknown>) {
    loading = true;
    errorMessage = '';
    try {
      status = normalizePluginStatus(await fetcher());
    } catch (err) {
      errorMessage = err instanceof Error ? err.message : String(err);
    } finally {
      loading = false;
    }
  }
</script>

<div class="plugin-panel">
  <div class="plugin-header">
    <div>
      <h3>Plugins</h3>
      <p class="muted">
        {status.plugins.length} loaded · {actionCount} custom actions
        {#if status.dir}· <code>{status.dir}</code>{/if}
      </p>
    </div>
    <button class="btn-secondary btn-sm" on:click={reload} disabled={loading}>
      {loading ? 'Reloading…' : 'Reload plugins'}
    </button>
  </div>

  {#if errorMessage}
    <div class="error-text" role="alert">{errorMessage}</div>
  {/if}

  {#if status.errors.length > 0}
    <div class="load-errors" role="alert">
      <strong>Failed to load</strong>
      <ul>
        {#each status.errors as err}
          <li>{err}</li>
        {/each}
      </ul>
    </div>
  {/if}

  {#if status.plugins.length === 0 && !loading}
    <p class="muted empty">
      No plugins installed. Add a folder containing <code>plugin.json</code> and its executable to the plugins
      directory, then reload.
    </p>
  {/if}

  {#each status.plugins as p (p.id)}
    <article class="plugin-card">
      <header>
        <span class="plugin-name">{p.name}</span>
        <span class="badge">v{p.version}</span>
        <span class="badge">{p.type}</span>
        {#if p.status}
          <span class="badge" class:down={!p.status.running}>{p.status.running ? 'running' : 'stopped'}</span>
          {#if p.status.restarts > 0}<span class="badge">restarts: {p.status.restarts}</span>{/if}
        {/if}
        <code class="plugin-id">{p.id}</code>
      </header>
      {#if p.status?.lastError}<p class="error-text">Last error: {p.status.lastError}</p>{/if}
      {#if p.description}<p class="description">{p.description}</p>{/if}

      {#if p.actions.length > 0}
        <div class="section-label">Actions</div>
        <ul class="actions">
          {#each p.actions as action}
            <li>
              <code>{action.name}</code>
              {#if action.selector_required}<span class="tag">selector</span>{/if}
              {#if action.value_required}<span class="tag">value</span>{/if}
              {#if action.description}<span class="muted"> — {action.description}</span>{/if}
            </li>
          {/each}
        </ul>
      {/if}

      <div class="meta">
        <span><span class="section-label">Hooks</span> {p.hooks.length ? p.hooks.join(', ') : 'none'}</span>
        <span><span class="section-label">Capabilities</span> {p.capabilities.length ? p.capabilities.join(', ') : 'none'}</span>
      </div>
    </article>
  {/each}
</div>

<style>
  .plugin-panel {
    padding: 16px;
    max-width: 860px;
  }
  .plugin-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 12px;
    margin-bottom: 12px;
  }
  .plugin-header h3 {
    font-size: 16px;
    margin-bottom: 4px;
  }
  .muted {
    font-size: 12px;
    color: var(--text-muted);
  }
  .empty {
    margin-top: 8px;
  }
  .plugin-card,
  .load-errors {
    background: var(--bg-secondary);
    padding: 12px;
    border-radius: var(--radius);
    border: 1px solid var(--border);
    margin-bottom: 12px;
  }
  .load-errors {
    border-color: var(--danger, #ef4444);
    color: var(--danger, #ef4444);
    font-size: 12px;
  }
  .load-errors ul {
    margin: 6px 0 0 16px;
  }
  .plugin-card header {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    margin-bottom: 6px;
  }
  .plugin-name {
    font-weight: 600;
  }
  .plugin-id {
    margin-left: auto;
    font-size: 11px;
    color: var(--text-muted);
  }
  .badge,
  .tag {
    font-size: 11px;
    padding: 1px 6px;
    border-radius: 999px;
    border: 1px solid var(--border);
    color: var(--text-secondary);
  }
  .badge.down {
    border-color: var(--danger, #ef4444);
    color: var(--danger, #ef4444);
  }
  .description {
    font-size: 12px;
    color: var(--text-secondary);
    margin-bottom: 8px;
  }
  .section-label {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .actions {
    list-style: none;
    margin: 4px 0 8px;
    padding: 0;
    font-size: 12px;
  }
  .actions li {
    margin-bottom: 4px;
  }
  .meta {
    display: flex;
    gap: 16px;
    flex-wrap: wrap;
    font-size: 12px;
    color: var(--text-secondary);
  }
  .error-text {
    color: var(--danger, #ef4444);
    font-size: 12px;
    margin-bottom: 8px;
  }
</style>
