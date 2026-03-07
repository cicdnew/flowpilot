<script lang="ts">
  import { StartRecording, StopRecording, CreateRecordedFlow } from '../../wailsjs/go/main/App';
  import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
  import { isRecording, recordingSteps, webSocketLogs } from '../lib/store';
  import type { RecordedStep, WebSocketLog } from '../lib/types';
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';

  const dispatch = createEventDispatcher();

  let flowName = '';
  let flowDescription = '';
  let originUrl = '';
  let errorMessage = '';
  let saving = false;
  let starting = false;

  async function toggleRecording() {
    errorMessage = '';
    if ($isRecording) {
      await stopRecording();
    } else {
      await startRecording();
    }
  }

  async function startRecording() {
    if (!originUrl.trim()) {
      errorMessage = 'URL is required to start recording';
      return;
    }
    starting = true;
    try {
      recordingSteps.set([]);
      webSocketLogs.set([]);
      await StartRecording(originUrl);
      isRecording.set(true);
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    } finally {
      starting = false;
    }
  }

  async function stopRecording() {
    try {
      const steps = await StopRecording();
      isRecording.set(false);
      recordingSteps.set(steps || []);
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    }
  }

  function handleRecorderStep(step: RecordedStep) {
    recordingSteps.update(list => [...list, step]);
  }

  function handleWebSocketEvent(log: WebSocketLog) {
    webSocketLogs.update(list => [...list, log]);
  }

  async function saveFlow() {
    if (!flowName || $recordingSteps.length === 0) return;
    saving = true;
    try {
      errorMessage = '';
      await CreateRecordedFlow(flowName, flowDescription, originUrl, $recordingSteps);
      dispatch('saved');
      flowName = '';
      flowDescription = '';
      recordingSteps.set([]);
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    } finally {
      saving = false;
    }
  }

  onMount(() => {
    EventsOn('recorder:step', handleRecorderStep);
    EventsOn('recorder:websocket', handleWebSocketEvent);
  });

  onDestroy(() => {
    EventsOff('recorder:step');
    EventsOff('recorder:websocket');
  });
</script>

<div class="panel">
  <div class="panel-header">
    <h2>Live Recorder</h2>
    <button
      class="btn-primary"
      class:btn-danger={$isRecording}
      disabled={starting || (!$isRecording && !originUrl.trim())}
      on:click={toggleRecording}
    >
      {#if starting}
        Launching...
      {:else if $isRecording}
        ⏹ Stop Recording
      {:else}
        ⏺ Start Recording
      {/if}
    </button>
  </div>

  <div class="panel-body">
    <div class="form-group">
      <label for="origin-url">URL</label>
      <input
        id="origin-url"
        bind:value={originUrl}
        placeholder="https://example.com"
        disabled={$isRecording}
      />
    </div>
    <div class="form-group">
      <label for="flow-name">Flow Name</label>
      <input id="flow-name" bind:value={flowName} placeholder="Checkout flow" />
    </div>
    <div class="form-group">
      <label for="flow-desc">Description</label>
      <input id="flow-desc" bind:value={flowDescription} placeholder="Optional" />
    </div>

    {#if $isRecording}
      <div class="recording-indicator">
        <span class="pulse"></span> Recording — interact with the browser window
      </div>
    {/if}

    <div class="steps">
      <h4>Recorded Steps ({$recordingSteps.length})</h4>
      {#if $recordingSteps.length === 0}
        <div class="empty">No steps recorded yet.</div>
      {:else}
        <ul>
          {#each $recordingSteps as step}
            <li>
              <strong>{step.action}</strong>
              {#if step.selector} <span class="muted">{step.selector}</span> {/if}
              {#if step.value} <span class="value">= {step.value}</span> {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </div>

    <div class="ws-logs">
      <h4>WebSocket Events ({$webSocketLogs.length})</h4>
      {#if $webSocketLogs.length === 0}
        <div class="empty">No WebSocket activity captured.</div>
      {:else}
        <ul>
          {#each $webSocketLogs as log}
            <li class="ws-entry">
              <span class="ws-badge ws-{log.eventType}">{log.eventType}</span>
              {#if log.direction}
                <span class="ws-direction">{log.direction}</span>
              {/if}
              <span class="muted ws-url">{log.url}</span>
              {#if log.payloadSize > 0}
                <span class="ws-size">{log.payloadSize}B</span>
              {/if}
              {#if log.errorMessage}
                <span class="ws-error">{log.errorMessage}</span>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>

  {#if errorMessage}
    <div class="error-banner">{errorMessage}</div>
  {/if}

  <div class="panel-footer">
    <button class="btn-secondary" on:click={() => dispatch('close')}>Close</button>
    <button
      class="btn-primary"
      disabled={!flowName || $recordingSteps.length === 0 || saving || $isRecording}
      on:click={saveFlow}
    >
      {saving ? 'Saving...' : 'Save Recorded Flow'}
    </button>
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
  .panel-body {
    margin-top: 12px;
  }
  .recording-indicator {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    margin-top: 12px;
    background: rgba(239, 68, 68, 0.08);
    border: 1px solid rgba(239, 68, 68, 0.2);
    border-radius: 8px;
    color: var(--danger, #ef4444);
    font-size: 13px;
    font-weight: 500;
  }
  .pulse {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--danger, #ef4444);
    animation: pulse-anim 1.2s ease-in-out infinite;
  }
  @keyframes pulse-anim {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.3; }
  }
  .steps {
    margin-top: 16px;
  }
  .steps ul {
    list-style: none;
    padding: 0;
  }
  .steps li {
    padding: 6px 0;
    border-bottom: 1px solid var(--border);
  }
  .empty {
    font-size: 12px;
    color: var(--text-muted);
  }
  .muted {
    color: var(--text-muted);
    font-size: 11px;
  }
  .value {
    color: var(--text-muted);
    font-size: 11px;
    font-style: italic;
  }
  .panel-footer {
    margin-top: 16px;
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
  .ws-logs {
    margin-top: 16px;
  }
  .ws-logs ul {
    list-style: none;
    padding: 0;
  }
  .ws-entry {
    padding: 5px 0;
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }
  .ws-badge {
    display: inline-block;
    padding: 1px 6px;
    border-radius: 4px;
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    color: #fff;
    background: var(--text-muted, #888);
  }
  .ws-created { background: #3b82f6; }
  .ws-handshake { background: #8b5cf6; }
  .ws-frame_sent { background: #f59e0b; }
  .ws-frame_received { background: #10b981; }
  .ws-closed { background: #6b7280; }
  .ws-error { background: var(--danger, #ef4444); }
  .ws-direction {
    font-size: 11px;
    font-weight: 500;
    color: var(--text-secondary, #666);
  }
  .ws-url {
    max-width: 200px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ws-size {
    font-size: 11px;
    color: var(--text-muted);
    font-family: monospace;
  }
  .ws-error {
    font-size: 11px;
    color: var(--danger, #ef4444);
  }
  .btn-danger {
    background: var(--danger, #ef4444) !important;
    border-color: var(--danger, #ef4444) !important;
  }
</style>
