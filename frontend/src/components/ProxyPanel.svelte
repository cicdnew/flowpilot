<script lang="ts">
  import { onMount } from 'svelte';
  import { proxies } from '../lib/store';
  import { AddProxy, ListProxies, DeleteProxy } from '../../wailsjs/go/main/App';

  let server = '';
  let protocol = 'http';
  let username = '';
  let password = '';
  let geo = '';
  let errorMessage = '';
  let adding = false;

  onMount(() => {
    refresh();
  });

  let loadError = '';

  export async function refresh() {
    try {
      loadError = '';
      const list = await ListProxies();
      proxies.set(list || []);
    } catch (err: any) {
      loadError = `Failed to load proxies: ${err?.message || err}`;
    }
  }

  async function addProxy() {
    if (!server) return;
    adding = true;
    try {
      errorMessage = '';
      await AddProxy(server, protocol, username, password, geo);
      server = '';
      username = '';
      password = '';
      geo = '';
      await refresh();
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    } finally {
      adding = false;
    }
  }

  async function removeProxy(id: string) {
    try {
      loadError = '';
      await DeleteProxy(id);
      await refresh();
    } catch (err: any) {
      loadError = `Failed to delete proxy: ${err?.message || err}`;
    }
  }

  function statusColor(status: string): string {
    if (status === 'healthy') return 'var(--success)';
    if (status === 'unhealthy') return 'var(--danger)';
    return 'var(--text-muted)';
  }
</script>

<div class="proxy-panel">
  <h3>Proxy Pool</h3>
  {#if loadError}
    <div class="error-text">{loadError}</div>
  {/if}

  <div class="add-proxy">
    <div class="form-row">
      <input bind:value={server} placeholder="host:port" />
      <select bind:value={protocol}>
        <option value="http">HTTP</option>
        <option value="https">HTTPS</option>
        <option value="socks5">SOCKS5</option>
      </select>
      <input bind:value={geo} placeholder="Geo" style="width: 60px" />
    </div>
    <div class="form-row mt-2">
      <input bind:value={username} placeholder="Username" />
      <input type="password" bind:value={password} placeholder="Password" />
      <button class="btn-primary btn-sm" on:click={addProxy} disabled={!server || adding}>{adding ? "..." : "Add"}</button>
    </div>
    {#if errorMessage}
      <div class="error-text">{errorMessage}</div>
    {/if}
  </div>

  <div class="proxy-list">
    {#each $proxies as proxy (proxy.id)}
      <div class="proxy-item">
        <div class="proxy-info">
          <span class="proxy-server font-mono">{proxy.server}</span>
          <span class="proxy-meta">
            <span style="color: {statusColor(proxy.status)}">{proxy.status}</span>
            {#if proxy.geo}| {proxy.geo}{/if}
            | {proxy.latency}ms
            | {(proxy.successRate * 100).toFixed(0)}% success
            | used {proxy.totalUsed}x
          </span>
        </div>
        <button class="btn-danger btn-sm" on:click={() => removeProxy(proxy.id)}>Del</button>
      </div>
    {:else}
      <p class="text-muted" style="text-align: center; padding: 20px;">
        No proxies configured. Add proxies above.
      </p>
    {/each}
  </div>
</div>

<style>
  .proxy-panel {
    padding: 16px;
  }
  .proxy-panel h3 {
    font-size: 16px;
    margin-bottom: 12px;
  }
  .add-proxy {
    background: var(--bg-secondary);
    padding: 12px;
    border-radius: var(--radius);
    border: 1px solid var(--border);
    margin-bottom: 16px;
  }
  .form-row {
    display: flex;
    gap: 8px;
  }
  .form-row input, .form-row select {
    flex: 1;
  }
  .proxy-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .proxy-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 12px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }
  .proxy-server {
    font-size: 13px;
  }
  .proxy-meta {
    font-size: 11px;
    color: var(--text-muted);
    margin-left: 8px;
  }
  .error-text {
    color: var(--danger, #ef4444);
    font-size: 12px;
    margin-top: 8px;
  }
</style>
