<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Header from './components/Header.svelte';
  import TaskToolbar from './components/TaskToolbar.svelte';
  import TaskTable from './components/TaskTable.svelte';
  import TaskDetail from './components/TaskDetail.svelte';
  import CreateTaskModal from './components/CreateTaskModal.svelte';
  import BatchCreateModal from './components/BatchCreateModal.svelte';
  import ProxyPanel from './components/ProxyPanel.svelte';
  import { tasks, activeTab, updateTaskInStore } from './lib/store';
  import { ListTasks } from '../wailsjs/go/main/App';
  import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';

  let showCreateModal = false;
  let showBatchModal = false;
  let loadError = '';

  async function refreshTasks() {
    try {
      loadError = '';
      const list = await ListTasks();
      tasks.set(list || []);
    } catch (err: any) {
      loadError = `Failed to load tasks: ${err?.message || err}`;
    }
  }

  onMount(() => {
    refreshTasks();

    EventsOn('task:event', (event: any) => {
      updateTaskInStore(event);
      if (['completed', 'failed', 'cancelled'].includes(event.status)) {
        refreshTasks();
      }
    });
  });

  onDestroy(() => {
    EventsOff('task:event');
  });
</script>

<div class="app-layout">
  <Header />

  <nav class="tabs">
    <button
      class="tab"
      class:active={$activeTab === 'tasks'}
      on:click={() => $activeTab = 'tasks'}
    >
      Tasks
    </button>
    <button
      class="tab"
      class:active={$activeTab === 'proxies'}
      on:click={() => $activeTab = 'proxies'}
    >
      Proxies
    </button>
  </nav>

  {#if loadError}
    <div class="load-error">{loadError}</div>
  {/if}

  {#if $activeTab === 'tasks'}
    <TaskToolbar on:create={() => showCreateModal = true} on:batchCreate={() => showBatchModal = true} />
    <div class="main-content">
      <TaskTable on:refresh={refreshTasks} />
      <TaskDetail />
    </div>
  {:else if $activeTab === 'proxies'}
    <ProxyPanel />
  {/if}
</div>

{#if showCreateModal}
  <CreateTaskModal
    on:close={() => showCreateModal = false}
    on:created={refreshTasks}
  />
{/if}

{#if showBatchModal}
  <BatchCreateModal
    on:close={() => showBatchModal = false}
    on:created={refreshTasks}
  />
{/if}

<style>
  .app-layout {
    display: flex;
    flex-direction: column;
    height: 100vh;
  }

  .tabs {
    display: flex;
    gap: 0;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
    padding: 0 20px;
    flex-shrink: 0;
  }
  .tab {
    padding: 10px 20px;
    background: none;
    color: var(--text-muted);
    font-size: 13px;
    font-weight: 500;
    border-radius: 0;
    border-bottom: 2px solid transparent;
    transition: all 0.15s ease;
  }
  .tab:hover {
    color: var(--text-primary);
  }
  .tab.active {
    color: var(--accent);
    border-bottom-color: var(--accent);
  }

  .main-content {
    display: flex;
    flex: 1;
    overflow: hidden;
  }
  .load-error {
    padding: 8px 20px;
    background: rgba(239, 68, 68, 0.1);
    color: var(--danger, #ef4444);
    font-size: 12px;
    border-bottom: 1px solid rgba(239, 68, 68, 0.2);
  }
</style>
