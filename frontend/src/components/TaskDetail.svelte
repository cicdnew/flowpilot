<script lang="ts">
  import { selectedTask, tasks } from '../lib/store';
  import { UpdateTask, GetTask } from '../../wailsjs/go/main/App';
  import type { Task, TaskStep, ProxyConfig } from '../lib/types';

  let editing = false;
  let editName = '';
  let editUrl = '';
  let editPriority = 5;
  let editSteps: TaskStep[] = [];
  let editProxyServer = '';
  let editProxyUsername = '';
  let editProxyPassword = '';
  let editProxyGeo = '';
  let editTags = '';
  let editError = '';
  let saving = false;

  const actions = ['navigate', 'click', 'type', 'wait', 'screenshot', 'extract', 'scroll', 'select'];

  function startEdit() {
    if (!$selectedTask) return;
    editName = $selectedTask.name;
    editUrl = $selectedTask.url;
    editPriority = $selectedTask.priority;
    editSteps = ($selectedTask.steps || []).map(s => ({ ...s }));
    editProxyServer = $selectedTask.proxy?.server || '';
    editProxyUsername = $selectedTask.proxy?.username || '';
    editProxyPassword = $selectedTask.proxy?.password || '';
    editProxyGeo = $selectedTask.proxy?.geo || '';
    editTags = ($selectedTask.tags ?? []).join(', ');
    editError = '';
    editing = true;
  }

  function cancelEdit() {
    editing = false;
    editError = '';
  }

  function addEditStep() {
    editSteps = [...editSteps, { action: 'click', selector: '', value: '' }];
  }

  function removeEditStep(i: number) {
    editSteps = editSteps.filter((_, idx) => idx !== i);
  }

  async function saveEdit() {
    if (!$selectedTask) return;
    saving = true;
    const proxyConfig: ProxyConfig = {
      server: editProxyServer,
      username: editProxyUsername,
      password: editProxyPassword,
      geo: editProxyGeo,
    };
    try {
      editError = '';
      const tags = editTags.split(',').map(t => t.trim()).filter(t => t.length > 0);
      await UpdateTask($selectedTask.id, editName, editUrl, editSteps, proxyConfig, editPriority, tags);
      const updated = await GetTask($selectedTask.id) as unknown as Task;
      tasks.update(list => list.map(t => t.id === updated.id ? updated : t));
      editing = false;
    } catch (err: any) {
      editError = err?.message || String(err);
    } finally {
      saving = false;
    }
  }

  $: canEdit = $selectedTask && ($selectedTask.status === 'pending' || $selectedTask.status === 'failed');

  function formatDuration(ns: number): string {
    if (!ns) return '-';
    const ms = ns / 1000000;
    if (ms < 1000) return `${ms.toFixed(0)}ms`;
    return `${(ms / 1000).toFixed(1)}s`;
  }
</script>

<div class="detail-panel">
  {#if $selectedTask}
    <div class="detail-header">
      <h3>{$selectedTask.name}</h3>
      <div class="detail-header-right">
        <span class="badge badge-{$selectedTask.status}">{$selectedTask.status}</span>
        {#if canEdit && !editing}
          <button class="btn-secondary btn-sm" on:click={startEdit}>Edit</button>
        {/if}
      </div>
    </div>

    {#if editing}
      <div class="edit-form">
        <div class="form-group">
          <label for="edit-name">Name</label>
          <input id="edit-name" bind:value={editName} />
        </div>
        <div class="form-group">
          <label for="edit-url">URL</label>
          <input id="edit-url" bind:value={editUrl} />
        </div>
        <div class="form-group">
          <label for="edit-priority">Priority</label>
          <select id="edit-priority" bind:value={editPriority}>
            <option value={1}>Low</option>
            <option value={5}>Normal</option>
            <option value={10}>High</option>
          </select>
        </div>
        <h4>Proxy</h4>
        <div class="form-group">
          <input bind:value={editProxyServer} placeholder="host:port" />
        </div>
        <div class="form-row-sm">
          <input bind:value={editProxyUsername} placeholder="Username" />
          <input type="password" bind:value={editProxyPassword} placeholder="Password" />
          <input bind:value={editProxyGeo} placeholder="Geo" style="width:50px" />
        </div>
        <div class="form-group">
          <label for="edit-tags">Tags</label>
          <input id="edit-tags" bind:value={editTags} placeholder="tag1, tag2, tag3" />
          <span style="font-size:11px;color:var(--text-muted)">Comma-separated</span>
        </div>
        <h4>Steps</h4>
        {#each editSteps as step, i}
          <div class="step-row-edit">
            <select bind:value={step.action}>
              {#each actions as a}<option value={a}>{a}</option>{/each}
            </select>
            {#if step.action !== 'navigate' && step.action !== 'screenshot'}
              <input bind:value={step.selector} placeholder="Selector" class="flex-1" />
            {/if}
            <input bind:value={step.value} placeholder="Value" class="flex-1" />
            <button class="btn-danger btn-sm" on:click={() => removeEditStep(i)} disabled={editSteps.length <= 1}>-</button>
          </div>
        {/each}
        <button class="btn-secondary btn-sm mt-2" on:click={addEditStep}>+ Step</button>
        {#if editError}
          <div class="error-text">{editError}</div>
        {/if}
        <div class="edit-actions">
          <button class="btn-secondary btn-sm" on:click={cancelEdit}>Cancel</button>
          <button class="btn-primary btn-sm" on:click={saveEdit} disabled={!editName || !editUrl || saving}>{saving ? "Saving..." : "Save"}</button>
        </div>
      </div>
    {/if}

    <div class="detail-section">
      <h4>Info</h4>
      <div class="detail-grid">
        <div class="detail-item">
          <span class="label">ID</span>
          <span class="value font-mono">{$selectedTask.id}</span>
        </div>
        <div class="detail-item">
          <span class="label">URL</span>
          <span class="value">{$selectedTask.url}</span>
        </div>
        <div class="detail-item">
          <span class="label">Priority</span>
          <span class="value">{$selectedTask.priority}</span>
        </div>
        <div class="detail-item">
          <span class="label">Retries</span>
          <span class="value">{$selectedTask.retryCount} / {$selectedTask.maxRetries}</span>
        </div>
        {#if $selectedTask.error}
          <div class="detail-item error">
            <span class="label">Error</span>
            <span class="value">{$selectedTask.error}</span>
          </div>
        {/if}
      </div>
    </div>

    {#if $selectedTask.tags?.length}
      <div class="detail-section">
        <h4>Tags</h4>
        <div class="tag-list">
          {#each $selectedTask.tags as tag}
            <span class="tag-badge">{tag}</span>
          {/each}
        </div>
      </div>
    {/if}

    {#if $selectedTask.proxy?.server}
      <div class="detail-section">
        <h4>Proxy</h4>
        <div class="detail-grid">
          <div class="detail-item">
            <span class="label">Server</span>
            <span class="value font-mono">{$selectedTask.proxy.server}</span>
          </div>
          {#if $selectedTask.proxy.geo}
            <div class="detail-item">
              <span class="label">Geo</span>
              <span class="value">{$selectedTask.proxy.geo}</span>
            </div>
          {/if}
        </div>
      </div>
    {/if}

    {#if $selectedTask.steps?.length}
      <div class="detail-section">
        <h4>Steps ({$selectedTask.steps.length})</h4>
        <div class="steps-list">
          {#each $selectedTask.steps as step, i}
            <div class="step">
              <span class="step-num">{i + 1}</span>
              <span class="step-action">{step.action}</span>
              {#if step.selector}
                <span class="step-selector font-mono">{step.selector}</span>
              {/if}
              {#if step.value}
                <span class="step-value">= {step.value}</span>
              {/if}
            </div>
          {/each}
        </div>
      </div>
    {/if}

    {#if $selectedTask.result}
      <div class="detail-section">
        <h4>Result</h4>
        <div class="detail-grid">
          <div class="detail-item">
            <span class="label">Duration</span>
            <span class="value">{formatDuration($selectedTask.result.duration)}</span>
          </div>
          <div class="detail-item">
            <span class="label">Success</span>
            <span class="value">{$selectedTask.result.success ? 'Yes' : 'No'}</span>
          </div>
        </div>

        {#if $selectedTask.result.extractedData && Object.keys($selectedTask.result.extractedData).length}
          <h4 class="mt-2">Extracted Data</h4>
          <div class="extracted-data font-mono">
            {#each Object.entries($selectedTask.result.extractedData) as [key, val]}
              <div><strong>{key}:</strong> {val}</div>
            {/each}
          </div>
        {/if}

        {#if $selectedTask.result.logs?.length}
          <h4 class="mt-2">Logs</h4>
          <div class="log-viewer font-mono">
            {#each $selectedTask.result.logs as log}
              <div class="log-entry log-{log.level}">
                <span class="log-level">[{log.level}]</span>
                {log.message}
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  {:else}
    <div class="empty-detail">
      <p>Select a task to view details</p>
    </div>
  {/if}
</div>

<style>
  .detail-panel {
    width: 380px;
    background: var(--bg-secondary);
    border-left: 1px solid var(--border);
    overflow-y: auto;
    padding: 16px;
    flex-shrink: 0;
  }
  .detail-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
  }
  .detail-header h3 {
    font-size: 16px;
    font-weight: 600;
    margin: 0;
  }
  .detail-header-right {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .edit-form {
    margin-bottom: 16px;
    padding: 12px;
    background: var(--bg-tertiary);
    border-radius: var(--radius);
  }
  .edit-form .form-group { margin-bottom: 8px; }
  .edit-form label {
    display: block;
    font-size: 11px;
    font-weight: 600;
    color: var(--text-muted);
    margin-bottom: 2px;
  }
  .edit-form input, .edit-form select { width: 100%; }
  .edit-form h4 {
    font-size: 11px;
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    margin: 8px 0 4px;
  }
  .form-row-sm { display: flex; gap: 4px; margin-bottom: 8px; }
  .form-row-sm input { flex: 1; }
  .step-row-edit {
    display: flex;
    gap: 4px;
    align-items: center;
    margin-bottom: 4px;
  }
  .step-row-edit select { min-width: 90px; }
  .edit-actions {
    display: flex;
    justify-content: flex-end;
    gap: 6px;
    margin-top: 12px;
  }
  .error-text {
    color: var(--danger, #ef4444);
    font-size: 11px;
    margin-top: 6px;
  }
  .detail-section {
    margin-bottom: 16px;
  }
  .detail-section h4 {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-bottom: 8px;
  }
  .detail-grid {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .detail-item {
    display: flex;
    gap: 8px;
  }
  .detail-item .label {
    color: var(--text-muted);
    min-width: 70px;
    font-size: 12px;
  }
  .detail-item .value {
    font-size: 12px;
    word-break: break-all;
  }
  .detail-item.error .value {
    color: var(--danger);
  }
  .steps-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .step {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 8px;
    background: var(--bg-tertiary);
    border-radius: 4px;
    font-size: 12px;
  }
  .step-num {
    color: var(--text-muted);
    min-width: 16px;
  }
  .step-action {
    color: var(--accent);
    font-weight: 600;
  }
  .step-selector {
    color: var(--warning);
  }
  .step-value {
    color: var(--text-secondary);
  }
  .extracted-data {
    padding: 8px;
    background: var(--bg-tertiary);
    border-radius: 4px;
    font-size: 11px;
  }
  .log-viewer {
    max-height: 200px;
    overflow-y: auto;
    padding: 8px;
    background: var(--bg-primary);
    border-radius: 4px;
  }
  .log-entry {
    font-size: 11px;
    padding: 2px 0;
  }
  .log-level {
    font-weight: 600;
  }
  .log-info .log-level { color: var(--accent); }
  .log-warn .log-level { color: var(--warning); }
  .log-error .log-level { color: var(--danger); }
  .empty-detail {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--text-muted);
  }
  .tag-list {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }
  .tag-badge {
    padding: 2px 8px;
    background: rgba(59, 130, 246, 0.15);
    color: var(--accent);
    border-radius: 10px;
    font-size: 11px;
    font-weight: 500;
  }
</style>
