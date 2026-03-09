<script lang="ts">
  import { CreateTask } from '../../wailsjs/go/main/App';
  import type { TaskStep, ProxyConfig } from '../lib/types';
  import { createEventDispatcher } from 'svelte';

  const dispatch = createEventDispatcher();

  let name = '';
  let url = '';
  let priority = 5;
  let autoStart = true;
  let proxyServer = '';
  let proxyProtocol = 'http';
  let proxyUsername = '';
  let proxyPassword = '';
  let proxyGeo = '';

  let taskTimeout = 0;
  let tagsInput = '';
  let steps: TaskStep[] = [{ action: 'navigate', value: '', selector: '' }];
  let errorMessage = '';
  let submitting = false;

  const actions = ['navigate', 'click', 'type', 'wait', 'screenshot', 'extract', 'scroll', 'select'];

  function addStep() {
    steps = [...steps, { action: 'click', selector: '', value: '' }];
  }

  function removeStep(i: number) {
    steps = steps.filter((_, idx) => idx !== i);
  }

  async function submit() {
    if (!name || !url) return;
    submitting = true;

    const proxyConfig: ProxyConfig = {
      server: proxyServer,
      protocol: proxyProtocol,
      username: proxyUsername,
      password: proxyPassword,
      geo: proxyGeo,
    };

    // Set first step's value to URL if it's a navigate step
    const taskSteps = steps.map(s => {
      if (s.action === 'navigate' && !s.value) {
        return { ...s, value: url };
      }
      return s;
    });

    const tags = tagsInput
      .split(',')
      .map(t => t.trim())
      .filter(t => t.length > 0);

    try {
      errorMessage = '';
      await CreateTask(name, url, taskSteps, proxyConfig, priority, autoStart, tags, taskTimeout);
      dispatch('created');
      dispatch('close');
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    } finally {
      submitting = false;
    }
  }
</script>

<div class="modal-overlay" role="button" tabindex="0" on:click={() => dispatch('close')} on:keydown={(e) => e.key === 'Escape' && dispatch('close')}>
  <div class="modal" role="dialog" tabindex="-1" on:click|stopPropagation on:keydown={(e) => e.key === 'Escape' && dispatch('close')}>
    <div class="modal-header">
      <h2>Create Task</h2>
      <button class="btn-secondary btn-sm" on:click={() => dispatch('close')}>x</button>
    </div>

    <div class="modal-body">
      <div class="form-group">
        <label for="task-name">Name</label>
        <input id="task-name" bind:value={name} placeholder="My automation task" />
      </div>

      <div class="form-group">
        <label for="task-url">URL</label>
        <input id="task-url" bind:value={url} placeholder="https://example.com" />
      </div>

      <div class="form-row">
        <div class="form-group">
          <label for="task-priority">Priority</label>
          <select id="task-priority" bind:value={priority}>
            <option value={1}>Low</option>
            <option value={5}>Normal</option>
            <option value={10}>High</option>
          </select>
        </div>
        <div class="form-group">
          <label for="task-auto-start">Auto Start</label>
          <label class="checkbox">
            <input id="task-auto-start" type="checkbox" bind:checked={autoStart} />
            Start immediately
          </label>
        </div>
      </div>

      <div class="form-group">
        <label for="task-tags">Tags</label>
        <input id="task-tags" bind:value={tagsInput} placeholder="scraping, production, daily" />
        <span class="hint">Comma-separated</span>
      </div>

      <div class="form-group">
        <label for="task-timeout">Timeout (seconds)</label>
        <input id="task-timeout" type="number" bind:value={taskTimeout} min="0" max="3600" placeholder="0 = default (5 min)" />
        <span class="hint">0 = use default (5 min). Max 3600s.</span>
      </div>

      <h4>Proxy (Optional)</h4>
      <div class="form-row">
        <div class="form-group">
          <label for="proxy-protocol">Protocol</label>
          <select id="proxy-protocol" bind:value={proxyProtocol}>
            <option value="http">http</option>
            <option value="https">https</option>
            <option value="socks5">socks5</option>
          </select>
        </div>
        <div class="form-group">
          <label for="proxy-server">Server</label>
          <input id="proxy-server" bind:value={proxyServer} placeholder="host:port" />
        </div>
        <div class="form-group">
          <label for="proxy-geo">Geo</label>
          <input id="proxy-geo" bind:value={proxyGeo} placeholder="US" />
        </div>
      </div>
      <div class="form-row">
        <div class="form-group">
          <label for="proxy-username">Username</label>
          <input id="proxy-username" bind:value={proxyUsername} placeholder="optional" />
        </div>
        <div class="form-group">
          <label for="proxy-password">Password</label>
          <input id="proxy-password" type="password" bind:value={proxyPassword} placeholder="optional" />
        </div>
      </div>

      <h4>Steps</h4>
      {#each steps as step, i}
        <div class="step-row">
          <select bind:value={step.action}>
            {#each actions as action}
              <option value={action}>{action}</option>
            {/each}
          </select>
          {#if step.action !== 'navigate' && step.action !== 'screenshot'}
            <input bind:value={step.selector} placeholder="CSS selector" class="flex-1" />
          {/if}
          <input bind:value={step.value} placeholder={step.action === 'navigate' ? 'URL' : 'Value'} class="flex-1" />
          <button class="btn-danger btn-sm" on:click={() => removeStep(i)} disabled={steps.length <= 1}>-</button>
        </div>
      {/each}
      <button class="btn-secondary btn-sm mt-2" on:click={addStep}>+ Add Step</button>
    </div>

    {#if errorMessage}
      <div class="error-banner">{errorMessage}</div>
    {/if}

    <div class="modal-footer">
      <button class="btn-secondary" on:click={() => dispatch('close')}>Cancel</button>
      <button class="btn-primary" on:click={submit} disabled={!name || !url || submitting}>{submitting ? "Creating..." : "Create Task"}</button>
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
    width: 600px;
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
  .modal-body h4 {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-muted);
    margin: 16px 0 8px;
  }
  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 16px 20px;
    border-top: 1px solid var(--border);
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
  .form-group input, .form-group select {
    width: 100%;
  }
  .form-row {
    display: flex;
    gap: 12px;
  }
  .form-row .form-group {
    flex: 1;
  }
  .step-row {
    display: flex;
    gap: 8px;
    align-items: center;
    margin-bottom: 8px;
  }
  .step-row select {
    min-width: 110px;
  }
  .checkbox {
    display: flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
  }
  .checkbox input[type="checkbox"] {
    width: auto;
    padding: 0;
  }
  .hint {
    font-size: 11px;
    color: var(--text-muted);
    margin-top: 2px;
    display: block;
  }
  .error-banner {
    padding: 8px 20px;
    background: rgba(239, 68, 68, 0.1);
    color: var(--danger, #ef4444);
    font-size: 12px;
    border-top: 1px solid rgba(239, 68, 68, 0.2);
  }
</style>
