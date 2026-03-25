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
  import {
    tasks,
    activeTab,
    updateTaskInStore,
    replaceTaskInStore,
    selectedTask,
    statusFilter,
    tagFilter,
    isRecording,
    recordedFlows,
  } from './lib/store';
  import type { Task, RecordedFlow } from './lib/types';
  import { ListTasksPaginated, GetTask, IsRecording, ListRecordedFlows } from '../wailsjs/go/main/App';
  import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';

  type TabId = 'tasks' | 'proxies' | 'recorder' | 'schedules' | 'visual' | 'settings';

  const tabs: Array<{ id: TabId; label: string; description: string }> = [
    { id: 'tasks', label: 'Tasks', description: 'Monitor runs, triage failures, and inspect logs.' },
    { id: 'proxies', label: 'Proxies', description: 'Manage routing pools and credentials securely.' },
    { id: 'recorder', label: 'Recorder', description: 'Capture browser flows and turn them into reusable automations.' },
    { id: 'schedules', label: 'Schedules', description: 'Automate recurring submissions and queue timing.' },
    { id: 'visual', label: 'Visual', description: 'Review baselines and detect UI drift quickly.' },
    { id: 'settings', label: 'Settings', description: 'Configure captcha and runtime integrations.' },
  ];

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
  let refreshRequestSeq = 0;
  let unsubscribeTaskEvents: (() => void) | null = null;

  $: activeTabMeta = tabs.find((tab) => tab.id === $activeTab) ?? tabs[0];

  async function refreshTasks() {
    pendingTaskRefresh = false;
    const requestSeq = ++refreshRequestSeq;
    loading = true;
    try {
      loadError = '';
      const result = await ListTasksPaginated(
        currentPage,
        pageSize,
        $statusFilter === 'all' ? 'all' : $statusFilter,
        $tagFilter,
      );
      if (requestSeq !== refreshRequestSeq) {
        return;
      }
      tasks.set((result.tasks || []) as Task[]);
      totalPages = result.totalPages || 1;
      totalItems = result.total || 0;
    } catch (err: any) {
      if (requestSeq !== refreshRequestSeq) {
        return;
      }
      loadError = `Failed to load tasks: ${err?.message || err}`;
    } finally {
      if (requestSeq === refreshRequestSeq) {
        loading = false;
      }
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
    if (page < 1 || page > totalPages) {
      return;
    }

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

    unsubscribeTaskEvents = EventsOn('task:event', async (event: any) => {
      updateTaskInStore(event);
      if (['completed', 'failed', 'cancelled'].includes(event.status)) {
        try {
          const full = (await GetTask(event.taskId)) as Task;
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
    if (unsubscribeTaskEvents) {
      unsubscribeTaskEvents();
      unsubscribeTaskEvents = null;
    } else {
      EventsOff('task:event');
    }
  });
</script>

<div class="app-shell">
  <div class="app-layout">
    <Header />

    <section class="workspace-bar">
      <div class="workspace-intro">
        <span class="workspace-kicker">Operations Console</span>
        <div class="workspace-copy">
          <h2>{activeTabMeta.label}</h2>
          <p>{activeTabMeta.description}</p>
        </div>
      </div>

      <nav class="tabs" aria-label="Primary sections">
        {#each tabs as tab}
          <button
            type="button"
            class="tab"
            class:active={$activeTab === tab.id}
            aria-pressed={$activeTab === tab.id}
            on:click={() => $activeTab = tab.id}
          >
            <span class="tab-label">{tab.label}</span>
            <span class="tab-description">{tab.description}</span>
          </button>
        {/each}
      </nav>
    </section>

    {#if loading}
      <div class="loading-banner">Refreshing task data…</div>
    {/if}

    {#if loadError}
      <div class="load-error" role="alert">{loadError}</div>
    {/if}

    <section class="workspace-surface">
      {#if $activeTab === 'tasks'}
        <TaskToolbar on:create={() => showCreateModal = true} on:batchCreate={() => showBatchModal = true} />
        <div class="main-content main-content--tasks">
          <div class="task-list-area">
            <TaskTable on:refresh={refreshTasks} />
            <div class="pagination">
              <button
                type="button"
                class="pagination-btn"
                disabled={currentPage <= 1}
                on:click={() => goToPage(currentPage - 1)}
              >
                ← Prev
              </button>
              <span class="pagination-info">
                Page {currentPage} of {totalPages} · {totalItems} task{totalItems === 1 ? '' : 's'}
              </span>
              <button
                type="button"
                class="pagination-btn"
                disabled={currentPage >= totalPages}
                on:click={() => goToPage(currentPage + 1)}
              >
                Next →
              </button>
            </div>
          </div>
          <aside class="side-panel">
            <TaskDetail />
            <BatchProgressPanel task={$selectedTask} />
            <LogViewer task={$selectedTask} />
          </aside>
        </div>
      {:else if $activeTab === 'proxies'}
        <div class="panel-view">
          <ProxyPanel />
        </div>
      {:else if $activeTab === 'recorder'}
        <div class="main-content main-content--recorder">
          <div class="panel-view panel-view--recorder">
            <RecorderPanel on:saved={refreshFlows} />
          </div>
          <aside class="side-panel side-panel--wide">
            <FlowManager on:use={(e) => { selectedFlow = e.detail; showBatchFromFlow = true; }} />
          </aside>
        </div>
      {:else if $activeTab === 'schedules'}
        <div class="panel-view">
          <SchedulePanel />
        </div>
      {:else if $activeTab === 'visual'}
        <div class="panel-view">
          <VisualDiffViewer />
        </div>
      {:else if $activeTab === 'settings'}
        <div class="panel-view">
          <CaptchaSettings />
        </div>
      {/if}
    </section>
  </div>
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
  .app-shell {
    min-height: 100vh;
    padding: 20px;
    background:
      radial-gradient(circle at top, rgba(59, 130, 246, 0.15), transparent 30%),
      linear-gradient(180deg, rgba(15, 23, 42, 0.96), rgba(3, 7, 18, 0.98));
  }

  .app-layout {
    display: flex;
    flex-direction: column;
    min-height: calc(100vh - 40px);
    border: 1px solid rgba(148, 163, 184, 0.18);
    border-radius: 24px;
    background: rgba(15, 23, 42, 0.72);
    box-shadow: 0 24px 80px rgba(2, 6, 23, 0.45);
    overflow: hidden;
    backdrop-filter: blur(20px);
  }

  .workspace-bar {
    display: grid;
    grid-template-columns: minmax(0, 360px) minmax(0, 1fr);
    gap: 20px;
    align-items: center;
    padding: 20px 24px;
    border-bottom: 1px solid rgba(148, 163, 184, 0.14);
    background: linear-gradient(180deg, rgba(15, 23, 42, 0.82), rgba(15, 23, 42, 0.58));
  }

  .workspace-intro {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .workspace-kicker {
    display: inline-flex;
    width: fit-content;
    padding: 6px 10px;
    border-radius: 999px;
    border: 1px solid rgba(96, 165, 250, 0.28);
    background: rgba(59, 130, 246, 0.1);
    color: #93c5fd;
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .workspace-copy h2 {
    margin: 0;
    font-size: 26px;
    line-height: 1.1;
  }

  .workspace-copy p {
    margin: 8px 0 0;
    max-width: 56ch;
    color: var(--text-secondary);
    font-size: 14px;
  }

  .tabs {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
  }

  .tab {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
    min-height: 92px;
    padding: 16px;
    text-align: left;
    color: var(--text-secondary);
    background: rgba(15, 23, 42, 0.45);
    border: 1px solid rgba(148, 163, 184, 0.14);
    border-radius: 18px;
  }

  .tab:hover {
    color: var(--text-primary);
    border-color: rgba(96, 165, 250, 0.28);
    background: rgba(30, 41, 59, 0.8);
    transform: translateY(-1px);
  }

  .tab.active {
    color: var(--text-primary);
    border-color: rgba(59, 130, 246, 0.5);
    background: linear-gradient(180deg, rgba(37, 99, 235, 0.28), rgba(30, 41, 59, 0.92));
    box-shadow: inset 0 1px 0 rgba(191, 219, 254, 0.16);
  }

  .tab-label {
    font-size: 14px;
    font-weight: 700;
  }

  .tab-description {
    font-size: 12px;
    line-height: 1.45;
    color: var(--text-muted);
  }

  .tab.active .tab-description {
    color: rgba(226, 232, 240, 0.88);
  }

  .loading-banner,
  .load-error {
    margin: 0 24px;
    padding: 10px 14px;
    border-radius: 14px;
    font-size: 12px;
  }

  .loading-banner {
    margin-top: 16px;
    border: 1px solid rgba(59, 130, 246, 0.22);
    background: rgba(59, 130, 246, 0.12);
    color: #93c5fd;
  }

  .load-error {
    margin-top: 16px;
    border: 1px solid rgba(239, 68, 68, 0.28);
    background: rgba(127, 29, 29, 0.34);
    color: #fca5a5;
  }

  .workspace-surface {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
    padding: 0 20px 20px;
  }

  .main-content {
    display: flex;
    flex: 1;
    min-height: 0;
    gap: 16px;
  }

  .main-content--tasks,
  .main-content--recorder {
    padding-top: 16px;
  }

  .task-list-area,
  .panel-view,
  .side-panel {
    min-height: 0;
    border: 1px solid rgba(148, 163, 184, 0.14);
    border-radius: 20px;
    background: rgba(15, 23, 42, 0.62);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.03);
  }

  .task-list-area,
  .panel-view {
    display: flex;
    flex: 1;
    flex-direction: column;
    overflow: hidden;
  }

  .panel-view {
    padding: 16px;
  }

  .panel-view--recorder {
    padding: 0;
  }

  .side-panel {
    display: flex;
    flex-direction: column;
    gap: 12px;
    width: 340px;
    padding: 12px;
    overflow-y: auto;
  }

  .side-panel--wide {
    width: 380px;
  }

  .pagination {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 12px 14px;
    border-top: 1px solid rgba(148, 163, 184, 0.14);
    background: rgba(15, 23, 42, 0.85);
  }

  .pagination-btn {
    padding: 7px 12px;
    font-size: 12px;
    font-weight: 600;
    color: var(--text-primary);
    background: rgba(30, 41, 59, 0.88);
    border: 1px solid rgba(148, 163, 184, 0.16);
    border-radius: 10px;
  }

  .pagination-btn:hover:not(:disabled) {
    background: rgba(37, 99, 235, 0.9);
    border-color: rgba(96, 165, 250, 0.45);
  }

  .pagination-btn:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .pagination-info {
    color: var(--text-muted);
    font-size: 12px;
  }

  @media (max-width: 1200px) {
    .workspace-bar {
      grid-template-columns: 1fr;
    }

    .tabs {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .side-panel,
    .side-panel--wide {
      width: 320px;
    }
  }

  @media (max-width: 960px) {
    .app-shell {
      padding: 12px;
    }

    .app-layout {
      min-height: calc(100vh - 24px);
      border-radius: 18px;
    }

    .tabs {
      grid-template-columns: 1fr;
    }

    .main-content {
      flex-direction: column;
    }

    .side-panel,
    .side-panel--wide {
      width: 100%;
      max-height: 42vh;
    }
  }
</style>
