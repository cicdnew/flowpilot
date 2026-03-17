<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Header from './components/Header.svelte';
  import TaskToolbar from './components/TaskToolbar.svelte';
  import TaskTable from './components/TaskTable.svelte';
  import TaskDetail from './components/TaskDetail.svelte';
  import CreateTaskModal from './components/CreateTaskModal.svelte';
  import BatchCreateModal from './components/BatchCreateModal.svelte';
  import ProxyPanel from './components/ProxyPanel.svelte';
  import RecorderPanel from './components/RecorderPanel.svelte';
  import FlowManager from './components/FlowManager.svelte';
  import BatchFromFlow from './components/BatchFromFlow.svelte';
  import LogViewer from './components/LogViewer.svelte';
  import BatchProgressPanel from './components/BatchProgressPanel.svelte';
  import SchedulePanel from './components/SchedulePanel.svelte';
  import CaptchaSettings from './components/CaptchaSettings.svelte';
  import VisualDiffViewer from './components/VisualDiffViewer.svelte';
  import { tasks, activeTab, updateTaskInStore, replaceTaskInStore, selectedTask, statusFilter, tagFilter, isRecording, recordedFlows} from './lib/store';
  import type { Task, RecordedFlow } from './lib/types';
  import { ListTasksPaginated, GetTask, IsRecording, ListRecordedFlows } from '../wailsjs/go/main/App';
  import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';

  let showCreateModal = false;
  let showBatchModal = false;
  let loadError = '';
  let loading = false;
  let selectedFlow: RecordedFlow | null = null;
  let showBatchFromFlow = false;
  let currentPage = 1;
  let pageSize = 50;
  let totalPages = 1;
  let totalItems = 0;
  let refreshTimer: ReturnType<typeof setTimeout> | null = null;
  let pendingTaskRefresh = false;

  async function refreshTasks() {
    pendingTaskRefresh = false;
    loading = true;
    try {
      loadError = '';
      const result = await ListTasksPaginated(currentPage, pageSize, $statusFilter === 'all' ? 'all' : $statusFilter, $tagFilter);
      tasks.set((result.tasks || []) as Task[]);
      totalPages = result.totalPages || 1;
      totalItems = result.total || 0;
    } catch (err: any) {
      loadError = `Failed to load tasks: ${err?.message || err}`;
    } finally {
      loading = false;
    }
  }

  function scheduleRefresh(delay = 150) {
    if ($activeTab !== 'tasks') {
      pendingTaskRefresh = true;
      return;
    }
    if (refreshTimer) {
      clearTimeout(refreshTimer);
    }
    refreshTimer = setTimeout(() => {
      refreshTimer = null;
      refreshTasks();
    }, delay);
  }

  let lastFilterKey = '';

  $: {
    const nextFilterKey = `${$statusFilter}|${$tagFilter}`;
    if (nextFilterKey !== lastFilterKey) {
      lastFilterKey = nextFilterKey;
      currentPage = 1;
      refreshTasks();
    }
  }

  $: if ($activeTab === 'tasks' && pendingTaskRefresh && !refreshTimer && !loading) {
    scheduleRefresh(0);
  }

  function goToPage(page: number) {
    if (page < 1 || page > totalPages) return;
    currentPage = page;
    refreshTasks();
  }

  async function refreshFlows() {
    try {
      const flows = await ListRecordedFlows();
      recordedFlows.set(flows || []);
    } catch (_) {}
  }

  onMount(async () => {
    try {
      const recording = await IsRecording();
      isRecording.set(recording);
    } catch (_) {}

    EventsOn('task:event', async (event: any) => {
      updateTaskInStore(event);
      if (['completed', 'failed', 'cancelled'].includes(event.status)) {
        try {
          const full = await GetTask(event.taskId) as Task;
          replaceTaskInStore(full);
        } catch (_) {}
        pendingTaskRefresh = true;
        scheduleRefresh();
      }
    });
  });

  onDestroy(() => {
    if (refreshTimer) {
      clearTimeout(refreshTimer);
      refreshTimer = null;
    }
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
    <button
      class="tab"
      class:active={$activeTab === 'recorder'}
      on:click={() => $activeTab = 'recorder'}
    >
      Recorder
    </button>
    <button
      class="tab"
      class:active={$activeTab === 'schedules'}
      on:click={() => $activeTab = 'schedules'}
    >
      Schedules
    </button>
    <button
      class="tab"
      class:active={$activeTab === 'visual'}
      on:click={() => $activeTab = 'visual'}
    >
      Visual
    </button>
    <button
      class="tab"
      class:active={$activeTab === 'settings'}
      on:click={() => $activeTab = 'settings'}
    >
      Settings
    </button>
  </nav>

  {#if loading}
    <div class="loading-bar">Loading...</div>
  {/if}
  {#if loadError}
    <div class="load-error">{loadError}</div>
  {/if}

  {#if $activeTab === 'tasks'}
    <TaskToolbar on:create={() => showCreateModal = true} on:batchCreate={() => showBatchModal = true} />
    <div class="main-content">
      <div class="task-list-area">
        <TaskTable on:refresh={refreshTasks} />
        <div class="pagination">
          <button
            class="pagination-btn"
            disabled={currentPage <= 1}
            on:click={() => goToPage(currentPage - 1)}
          >
            ← Prev
          </button>
          <span class="pagination-info">
            Page {currentPage} of {totalPages} ({totalItems} total)
          </span>
          <button
            class="pagination-btn"
            disabled={currentPage >= totalPages}
            on:click={() => goToPage(currentPage + 1)}
          >
            Next →
          </button>
        </div>
      </div>
      <div class="side-panel">
        <TaskDetail />
        <BatchProgressPanel task={$selectedTask} />
        <LogViewer task={$selectedTask} />
      </div>
    </div>
  {:else if $activeTab === 'proxies'}
    <ProxyPanel />
  {:else if $activeTab === 'recorder'}
    <div class="main-content">
      <RecorderPanel on:saved={refreshFlows} />
      <div class="side-panel">
        <FlowManager on:use={(e) => { selectedFlow = e.detail; showBatchFromFlow = true; }} />
      </div>
    </div>
  {:else if $activeTab === 'schedules'}
    <SchedulePanel />
  {:else if $activeTab === 'visual'}
    <VisualDiffViewer />
  {:else if $activeTab === 'settings'}
    <CaptchaSettings />
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

{#if showBatchFromFlow}
  <BatchFromFlow
    flow={selectedFlow}
    on:close={() => showBatchFromFlow = false}
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
  .task-list-area {
    display: flex;
    flex-direction: column;
    flex: 1;
    overflow: hidden;
  }
  .side-panel {
    display: flex;
    flex-direction: column;
    gap: 12px;
    width: 320px;
    padding: 12px;
    border-left: 1px solid var(--border);
    overflow-y: auto;
  }
  .loading-bar {
    padding: 6px 20px;
    background: rgba(59, 130, 246, 0.1);
    color: var(--accent, #3b82f6);
    font-size: 12px;
    border-bottom: 1px solid rgba(59, 130, 246, 0.2);
    text-align: center;
  }
  .load-error {
    padding: 8px 20px;
    background: rgba(239, 68, 68, 0.1);
    color: var(--danger, #ef4444);
    font-size: 12px;
    border-bottom: 1px solid rgba(239, 68, 68, 0.2);
  }
  .pagination {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 8px 12px;
    border-top: 1px solid var(--border);
    background: var(--bg-secondary);
    flex-shrink: 0;
  }
  .pagination-btn {
    padding: 4px 12px;
    font-size: 12px;
    font-weight: 500;
    background: var(--bg-primary);
    color: var(--text-primary);
    border: 1px solid var(--border);
    border-radius: 4px;
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .pagination-btn:hover:not(:disabled) {
    background: var(--accent);
    color: #fff;
    border-color: var(--accent);
  }
  .pagination-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .pagination-info {
    font-size: 12px;
    color: var(--text-muted);
  }
</style>
