<script lang="ts">
  import { onMount } from 'svelte';
  import { proxies } from '../lib/store';
  import { AddProxy, ListProxies, DeleteProxy, ListProxyCountryStats, CreateProxyRoutingPreset, ListProxyRoutingPresets, DeleteProxyRoutingPreset, GetLocalProxyGatewayStats } from '../../wailsjs/go/main/App';
  import type { ProxyCountryStats, ProxyRoutingPreset, ProxyRoutingFallback, LocalProxyGatewayStats } from '../lib/types';

  let server = '';
  let protocol = 'http';
  let username = '';
  let password = '';
  let geo = '';
  let maxRequestsPerMinute = 0;
  let errorMessage = '';
  let adding = false;
  let countryStats: ProxyCountryStats[] = [];
  let routingPresets: ProxyRoutingPreset[] = [];
  let presetName = '';
  let presetCountry = '';
  let presetFallback: ProxyRoutingFallback = 'strict';
  let presetRandomByCountry = true;
  let gatewayStats: LocalProxyGatewayStats = {
    activeEndpoints: 0,
    endpointCreations: 0,
    endpointReuses: 0,
    authFailures: 0,
    upstreamFailures: 0,
  };

  $: countrySummary = countryStats.length;

  onMount(() => {
    refresh();
  });

  let loadError = '';

  export async function refresh() {
    try {
      loadError = '';
      const [list, stats, presets, gateway] = await Promise.all([
        ListProxies(),
        ListProxyCountryStats(),
        ListProxyRoutingPresets(),
        GetLocalProxyGatewayStats(),
      ]);
      proxies.set(list || []);
      countryStats = (stats || []) as ProxyCountryStats[];
      routingPresets = (presets || []) as ProxyRoutingPreset[];
      gatewayStats = (gateway || gatewayStats) as LocalProxyGatewayStats;
    } catch (err: any) {
      loadError = `Failed to load proxies: ${err?.message || err}`;
    }
  }

  async function savePreset() {
    if (!presetName.trim()) return;
    try {
      loadError = '';
      await CreateProxyRoutingPreset(presetName.trim(), presetCountry.trim().toUpperCase(), presetFallback, presetRandomByCountry);
      presetName = '';
      presetCountry = '';
      presetFallback = 'strict';
      presetRandomByCountry = true;
      await refresh();
    } catch (err: any) {
      loadError = `Failed to save preset: ${err?.message || err}`;
    }
  }

  async function removePreset(id: string) {
    try {
      loadError = '';
      await DeleteProxyRoutingPreset(id);
      await refresh();
    } catch (err: any) {
      loadError = `Failed to delete preset: ${err?.message || err}`;
    }
  }

  async function addProxy() {
    if (!server) return;
    adding = true;
    try {
      errorMessage = '';
      const app = (window as Window & { go?: { main?: { App?: { AddProxyWithRateLimit?: (...args: any[]) => Promise<any> } } } }).go?.main?.App;
      if (app?.AddProxyWithRateLimit) {
        await app.AddProxyWithRateLimit(server, protocol, username, password, geo.trim().toUpperCase(), Math.max(0, maxRequestsPerMinute || 0));
      } else {
        await AddProxy(server, protocol, username, password, geo.trim().toUpperCase());
      }
      server = '';
      username = '';
      password = '';
      geo = '';
      maxRequestsPerMinute = 0;
      await refresh();
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    } finally {
      adding = false;
    }
  }

  async function removeProxy(id: string) {
    if (!confirm('Delete this proxy?')) return;
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

  <div class="country-summary">
    <div class="country-summary-header">Local Gateway</div>
    <div class="country-summary-grid">
      <div class="country-chip">active {gatewayStats.activeEndpoints}</div>
      <div class="country-chip">created {gatewayStats.endpointCreations}</div>
      <div class="country-chip">reused {gatewayStats.endpointReuses}</div>
      <div class="country-chip">auth failures {gatewayStats.authFailures}</div>
      <div class="country-chip">upstream failures {gatewayStats.upstreamFailures}</div>
    </div>
    {#if gatewayStats.lastError}
      <div class="summary-hint">Last error: {gatewayStats.lastError}</div>
    {/if}
  </div>

  {#if routingPresets.length || true}
    <div class="country-summary">
      <div class="country-summary-header">Routing Presets</div>
      <div class="form-row">
        <input bind:value={presetName} placeholder="Preset name" />
        <input bind:value={presetCountry} placeholder="Country" style="max-width:90px" />
      </div>
      <div class="form-row mt-2">
        <select bind:value={presetFallback}>
          <option value="strict">Strict country only</option>
          <option value="any_healthy">Fallback to any healthy proxy</option>
          <option value="direct">Fallback to direct connection</option>
        </select>
        <label class="checkbox"><input type="checkbox" bind:checked={presetRandomByCountry} /> Random by country</label>
        <button class="btn-primary btn-sm" on:click={savePreset} disabled={!presetName.trim()}>Save Preset</button>
      </div>
      {#if routingPresets.length}
        <div class="country-summary-grid" style="margin-top:8px">
          {#each routingPresets as preset}
            <div class="country-chip">{preset.name} · {preset.country || 'ANY'} · {preset.fallback || 'strict'} <button class="btn-danger btn-sm" on:click={() => removePreset(preset.id)}>x</button></div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}

  {#if countryStats.length}
    <div class="country-summary">
      <div class="country-summary-header">Country Pools ({countrySummary})</div>
      <div class="country-summary-grid">
        {#each countryStats as stat}
          <div class="country-chip">{stat.country}: {stat.healthy}/{stat.total} healthy · {stat.activeReservations} reserved · {stat.fallbackAssignments} fallbacks · {stat.activeLocalEndpoints} local</div>
        {/each}
      </div>
      <div class="summary-hint">Tasks can use random healthy proxies by country by leaving the proxy server blank and setting the country code in task configuration.</div>
    </div>
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
      <input type="number" min="0" bind:value={maxRequestsPerMinute} placeholder="Req/min" style="width: 100px" />
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
            {#if proxy.maxRequestsPerMinute}
              | limit {proxy.maxRequestsPerMinute}/min
            {/if}
            {#if proxy.localEndpointOn}
              | local {proxy.localEndpoint} ({proxy.activeLocalUsers || 0} active){proxy.localAuthEnabled ? ' auth' : ''}
            {/if}
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
  .country-summary {
    background: var(--bg-secondary);
    padding: 12px;
    border-radius: var(--radius);
    border: 1px solid var(--border);
    margin-bottom: 16px;
  }
  .country-summary-header {
    font-size: 13px;
    font-weight: 600;
    margin-bottom: 8px;
  }
  .country-summary-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-bottom: 6px;
  }
  .country-chip {
    font-size: 11px;
    padding: 4px 8px;
    background: var(--bg-primary);
    border: 1px solid var(--border);
    border-radius: 999px;
  }
  .summary-hint {
    font-size: 11px;
    color: var(--text-muted);
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
