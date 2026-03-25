<script lang="ts">
  import { onMount } from 'svelte';
  import { captchaConfig } from '../lib/store';

  import { GetCaptchaConfig, SaveCaptchaConfig, TestCaptchaConfig, DeleteCaptchaConfig } from '../../wailsjs/go/main/App';

  let provider = '2captcha';
  let apiKey = '';
  let errorMessage = '';
  let successMessage = '';
  let saving = false;
  let testing = false;

  onMount(async () => {
    await refresh();
  });

  async function refresh() {
    try {
      errorMessage = '';
      const config = await GetCaptchaConfig();
      captchaConfig.set(config || null);
      if (config) {
        provider = config.provider;
        apiKey = config.apiKey;
      }
    } catch (_) {
      captchaConfig.set(null);
    }
  }

  async function save() {
    if (!apiKey) return;
    saving = true;
    try {
      errorMessage = '';
      successMessage = '';
      const config = await SaveCaptchaConfig(provider, apiKey);
      captchaConfig.set(config);
      successMessage = 'Configuration saved';
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    } finally {
      saving = false;
    }
  }

  async function test() {
    if (!$captchaConfig) return;
    testing = true;
    try {
      errorMessage = '';
      const balance = await TestCaptchaConfig($captchaConfig.id);
      successMessage = `Balance: $${balance.toFixed(4)}`;
    } catch (err: any) {
      errorMessage = `Test failed: ${err?.message || err}`;
    } finally {
      testing = false;
    }
  }

  async function remove() {
    if (!$captchaConfig) return;
    try {
      errorMessage = '';
      successMessage = '';
      await DeleteCaptchaConfig($captchaConfig.id);
      captchaConfig.set(null);
      provider = '2captcha';
      apiKey = '';
    } catch (err: any) {
      errorMessage = `Failed to delete: ${err?.message || err}`;
    }
  }
</script>

<div class="captcha-panel">
  <h3>CAPTCHA Settings</h3>
  {#if errorMessage}
    <div class="error-text">{errorMessage}</div>
  {/if}
  {#if successMessage}
    <div class="success-text">{successMessage}</div>
  {/if}

  {#if $captchaConfig}
    <div class="current-config">
      <div class="config-header">
        <span class="config-label">Active Provider</span>
        <span class="badge badge-running">{$captchaConfig.provider}</span>
      </div>
      {#if $captchaConfig.balance !== undefined && $captchaConfig.balance !== null}
        <div class="config-detail">
          <span>Balance: ${$captchaConfig.balance.toFixed(4)}</span>
        </div>
      {/if}
      <div class="config-actions">
        <button class="btn-secondary btn-sm" on:click={test} disabled={testing}>
          {testing ? 'Testing...' : 'Test Connection'}
        </button>
        <button class="btn-danger btn-sm" on:click={remove}>Remove</button>
      </div>
    </div>
  {/if}

  <div class="config-form">
    <div class="form-group">
      <label for="captcha-provider">Provider</label>
      <select id="captcha-provider" bind:value={provider}>
        <option value="2captcha">2Captcha</option>
        <option value="anticaptcha">Anti-Captcha</option>
      </select>
    </div>
    <div class="form-group">
      <label for="captcha-key">API Key</label>
      <input id="captcha-key" type="password" bind:value={apiKey} placeholder="Enter API key" />
    </div>
    <div class="form-actions">
      <button class="btn-primary btn-sm" on:click={save} disabled={!apiKey || saving}>
        {saving ? 'Saving...' : 'Save Configuration'}
      </button>
    </div>
  </div>
</div>

<style>
  .captcha-panel {
    padding: 16px;
    max-width: 500px;
  }
  .captcha-panel h3 {
    font-size: 16px;
    margin-bottom: 12px;
  }
  .current-config {
    background: var(--bg-secondary);
    padding: 12px;
    border-radius: var(--radius);
    border: 1px solid var(--border);
    margin-bottom: 16px;
  }
  .config-header {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 6px;
  }
  .config-label {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-muted);
  }
  .config-detail {
    font-size: 12px;
    color: var(--text-secondary);
    margin-bottom: 8px;
  }
  .config-actions {
    display: flex;
    gap: 6px;
  }
  .config-form {
    background: var(--bg-secondary);
    padding: 12px;
    border-radius: var(--radius);
    border: 1px solid var(--border);
  }
  .form-group {
    margin-bottom: 12px;
  }
  .form-group label {
    display: block;
    font-size: 12px;
    font-weight: 600;
    color: var(--text-muted);
    margin-bottom: 4px;
  }
  .form-group input,
  .form-group select {
    width: 100%;
  }
  .form-actions {
    display: flex;
    justify-content: flex-end;
  }
  .error-text {
    color: var(--danger, #ef4444);
    font-size: 12px;
    margin-bottom: 8px;
  }
  .success-text {
    color: var(--success, #10b981);
    font-size: 12px;
    margin-bottom: 8px;
  }
</style>
