<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { CreateRepeatedTask } from '../../wailsjs/go/main/App';
  import type { TaskStep, ProxyConfig, TaskLoggingPolicy } from '../lib/types';

  export let visible = false;

  const dispatch = createEventDispatcher();

  let name = '';
  let url = '';
  let steps: TaskStep[] = [{ action: 'navigate', selector: '', value: '', timeout: 0 }];
  
  // Repeat configuration
  let repeatMode: 'counter' | 'range' | 'list' = 'counter';
  let varName = 'counter';
  let startVal = 1;
  let endVal = 10;
  let step = 1;
  let valuesList = '';
  
  let priority = 5;
  let autoStart = true;
  let tags = '';
  let timeout = 0;
  let headless = true;
  
  let errorMessage = '';
  let isSubmitting = false;

  function addStep() {
    steps = [...steps, { action: 'click', selector: '', value: '', timeout: 0 }];
  }

  function removeStep(index: number) {
    steps = steps.filter((_, i) => i !== index);
  }

  function parseValues(): string[] {
    return valuesList
      .split('\n')
      .map(v => v.trim())
      .filter(v => v !== '');
  }

  async function handleSubmit() {
    errorMessage = '';
    
    if (!name.trim()) {
      errorMessage = 'Task name is required';
      return;
    }
    
    if (!url.trim()) {
      errorMessage = 'URL is required';
      return;
    }

    if (!varName.trim()) {
      errorMessage = 'Variable name is required';
      return;
    }

    if (repeatMode !== 'list' && startVal > endVal) {
      errorMessage = 'Start value must be <= end value';
      return;
    }

    if (repeatMode !== 'list' && step <= 0) {
      errorMessage = 'Step must be > 0';
      return;
    }

    if (repeatMode === 'list' && !valuesList.trim()) {
      errorMessage = 'Values list is required for list mode';
      return;
    }

    isSubmitting = true;

    try {
      const repeatConfig = {
        mode: repeatMode,
        varName: varName,
        startVal: repeatMode !== 'list' ? startVal : 0,
        endVal: repeatMode !== 'list' ? endVal : 0,
        step: repeatMode !== 'list' ? step : 1,
        values: repeatMode === 'list' ? parseValues() : [],
        batchSize: 0
      };

      const tagsArray = tags.split(',').map(t => t.trim()).filter(t => t);
      const proxyConfig: ProxyConfig = {
        server: '',
        protocol: 'http',
        username: '',
        password: '',
        geo: '',
        fallback: 'strict'
      };

      const loggingPolicy: TaskLoggingPolicy | null = null;

      await CreateRepeatedTask({
        name,
        url,
        steps,
        repeat: repeatConfig,
        proxy: proxyConfig,
        priority,
        autoStart,
        tags: tagsArray,
        timeout,
        loggingPolicy,
        headless: headless
      });

      dispatch('success');
      resetForm();
    } catch (err: any) {
      errorMessage = err?.message || String(err);
    } finally {
      isSubmitting = false;
    }
  }

  function resetForm() {
    name = '';
    url = '';
    steps = [{ action: 'navigate', selector: '', value: '', timeout: 0 }];
    repeatMode = 'counter';
    varName = 'counter';
    startVal = 1;
    endVal = 10;
    step = 1;
    valuesList = '';
    priority = 5;
    autoStart = true;
    tags = '';
    timeout = 0;
    headless = true;
    errorMessage = '';
  }
</script>

{#if visible}
  <div class="modal-overlay" on:click={() => dispatch('close')} on:keydown={(e) => e.key === 'Escape' && dispatch('close')}>
    <div class="modal" on:click|stopPropagation on:keydown={(e) => e.key === 'Escape' && dispatch('close')}>
      <div class="modal-header">
        <h2>Create Repeated Task</h2>
        <button class="close-btn" on:click={() => dispatch('close')}>&times;</button>
      </div>

      <div class="modal-body">
        <div class="form-group">
          <label for="name">Task Name (use &#123;&#123;varName&#125;&#125; for substitution)</label>
          <input id="name" type="text" bind:value={name} placeholder="Task &#123;&#123;counter&#125;&#125;" />
        </div>

        <div class="form-group">
          <label for="url">URL (use &#123;&#123;varName&#125;&#125; for substitution)</label>
          <input id="url" type="text" bind:value={url} placeholder="https://chhotu-bin.infy.uk/page/&#123;&#123;counter&#125;&#125;" />
        </div>

        <div class="form-section">
          <h3>Repeat Configuration</h3>
          
          <div class="form-row">
            <div class="form-group">
              <label for="repeatMode">Repeat Mode</label>
              <select id="repeatMode" bind:value={repeatMode}>
                <option value="counter">Counter (1, 2, 3...)</option>
                <option value="range">Range (custom start/end/step)</option>
                <option value="list">List (custom values)</option>
              </select>
            </div>

            <div class="form-group">
              <label for="varName">Variable Name</label>
              <input id="varName" type="text" bind:value={varName} placeholder="counter" />
            </div>
          </div>

          {#if repeatMode !== 'list'}
            <div class="form-row">
              <div class="form-group">
                <label for="startVal">Start Value</label>
                <input id="startVal" type="number" bind:value={startVal} />
              </div>

              <div class="form-group">
                <label for="endVal">End Value</label>
                <input id="endVal" type="number" bind:value={endVal} />
              </div>

              <div class="form-group">
                <label for="step">Step</label>
                <input id="step" type="number" bind:value={step} min="1" />
              </div>
            </div>

            <div class="info-box">
              Will create {Math.floor((endVal - startVal) / step) + 1} tasks
            </div>
          {:else}
            <div class="form-group">
              <label for="valuesList">Values (one per line)</label>
              <textarea id="valuesList" bind:value={valuesList} rows="5" placeholder="apple&#10;banana&#10;cherry"></textarea>
            </div>

            <div class="info-box">
              Will create {parseValues().length} tasks
            </div>
          {/if}
        </div>

        <div class="form-section">
          <h3>Steps</h3>
          {#each steps as step, i}
            <div class="step-row">
              <select bind:value={step.action}>
                <option value="navigate">Navigate</option>
                <option value="click">Click</option>
                <option value="type">Type</option>
                <option value="wait">Wait</option>
                <option value="extract">Extract</option>
                <option value="screenshot">Screenshot</option>
              </select>
              <input type="text" bind:value={step.selector} placeholder="Selector" />
               <input type="text" bind:value={step.value} placeholder="Value (use &#123;&#123;varName&#125;&#125;)" />
              <button type="button" class="remove-btn" on:click={() => removeStep(i)}>×</button>
            </div>
          {/each}
          <button type="button" class="add-step-btn" on:click={addStep}>+ Add Step</button>
        </div>

        <div class="form-row">
          <div class="form-group">
            <label for="priority">Priority</label>
            <input id="priority" type="number" bind:value={priority} min="1" max="10" />
          </div>

          <div class="form-group">
            <label for="timeout">Timeout (seconds, 0=default)</label>
            <input id="timeout" type="number" bind:value={timeout} min="0" />
          </div>
        </div>

        <div class="form-group">
          <label for="tags">Tags (comma-separated)</label>
          <input id="tags" type="text" bind:value={tags} placeholder="tag1, tag2" />
        </div>

        <div class="form-row">
          <div class="form-group checkbox-group">
            <label>
              <input type="checkbox" bind:checked={autoStart} />
              Auto-start tasks
            </label>
          </div>

          <div class="form-group checkbox-group">
            <label>
              <input type="checkbox" bind:checked={headless} />
              Headless mode
            </label>
          </div>
        </div>
      </div>

      {#if errorMessage}
        <div class="error-message">{errorMessage}</div>
      {/if}

      <div class="modal-footer">
        <button class="btn-secondary" on:click={() => dispatch('close')} disabled={isSubmitting}>Cancel</button>
        <button class="btn-primary" on:click={handleSubmit} disabled={isSubmitting}>
          {isSubmitting ? 'Creating...' : 'Create Tasks'}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(0, 0, 0, 0.7);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 1000;
  }

  .modal {
    background: #1e293b;
    border-radius: 8px;
    padding: 24px;
    max-width: 700px;
    width: 90%;
    max-height: 90vh;
    overflow-y: auto;
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.4);
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
  }

  .modal-header h2 {
    margin: 0;
    color: #f1f5f9;
  }

  .close-btn {
    background: none;
    border: none;
    font-size: 28px;
    color: #94a3b8;
    cursor: pointer;
    padding: 0;
    width: 32px;
    height: 32px;
  }

  .close-btn:hover {
    color: #f1f5f9;
  }

  .modal-body {
    margin-bottom: 20px;
  }

  .form-section {
    margin-bottom: 24px;
    padding: 16px;
    background: #0f172a;
    border-radius: 6px;
  }

  .form-section h3 {
    margin: 0 0 16px 0;
    font-size: 16px;
    color: #60a5fa;
  }

  .form-group {
    margin-bottom: 16px;
  }

  .form-group label {
    display: block;
    margin-bottom: 6px;
    color: #cbd5e1;
    font-size: 14px;
  }

  .form-group input,
  .form-group select,
  .form-group textarea {
    width: 100%;
    padding: 8px 12px;
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 4px;
    color: #f1f5f9;
    font-size: 14px;
  }

  .form-group textarea {
    resize: vertical;
    font-family: monospace;
  }

  .form-row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: 12px;
    margin-bottom: 16px;
  }

  .checkbox-group label {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
  }

  .checkbox-group input[type="checkbox"] {
    width: auto;
  }

  .info-box {
    padding: 12px;
    background: #1e293b;
    border-left: 3px solid #60a5fa;
    color: #cbd5e1;
    font-size: 14px;
    margin-top: 12px;
  }

  .step-row {
    display: grid;
    grid-template-columns: 120px 1fr 1fr 40px;
    gap: 8px;
    margin-bottom: 8px;
  }

  .step-row select,
  .step-row input {
    padding: 6px 8px;
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 4px;
    color: #f1f5f9;
    font-size: 13px;
  }

  .remove-btn {
    background: #dc2626;
    border: none;
    border-radius: 4px;
    color: white;
    font-size: 18px;
    cursor: pointer;
    padding: 0;
  }

  .remove-btn:hover {
    background: #b91c1c;
  }

  .add-step-btn {
    width: 100%;
    padding: 8px;
    background: #334155;
    border: 1px dashed #475569;
    border-radius: 4px;
    color: #cbd5e1;
    cursor: pointer;
    font-size: 14px;
  }

  .add-step-btn:hover {
    background: #475569;
    border-color: #64748b;
  }

  .error-message {
    padding: 12px;
    background: #7f1d1d;
    border: 1px solid #991b1b;
    border-radius: 4px;
    color: #fecaca;
    margin-bottom: 16px;
    font-size: 14px;
  }

  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
  }

  .btn-primary,
  .btn-secondary {
    padding: 8px 16px;
    border-radius: 4px;
    border: none;
    cursor: pointer;
    font-size: 14px;
    font-weight: 500;
  }

  .btn-primary {
    background: #3b82f6;
    color: white;
  }

  .btn-primary:hover:not(:disabled) {
    background: #2563eb;
  }

  .btn-secondary {
    background: #475569;
    color: #e2e8f0;
  }

  .btn-secondary:hover:not(:disabled) {
    background: #64748b;
  }

  .btn-primary:disabled,
  .btn-secondary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
