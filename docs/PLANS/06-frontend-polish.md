# Implementation Plan: Frontend Polish — Dashboard UX Improvements

## Overview

The FlowPilot frontend is functional but has several UX gaps that limit productivity at scale: no virtual scrolling (O(n) DOM at 1000+ tasks), no inline log viewer, missing visual diff viewer component, no keyboard shortcuts, no bulk operations, no cron expression helper, and form state lost on modal close. This plan addresses each gap in priority order, keeping all changes within the existing Svelte 3 + TypeScript + Wails stack with no new build dependencies required.

## Requirements

- Task table renders 10,000+ rows without frame drops (virtual scrolling)
- Inline log viewer with live tail, search, and level filtering
- Visual diff viewer to compare baseline vs current screenshots side-by-side
- Bulk select + bulk actions (start, cancel, delete) on the task table
- Cron expression builder with plain-English preview and validation
- Keyboard shortcuts for common actions (new task, start all, cancel selected)
- Form draft persistence (localStorage) so modal data survives close/reopen
- Schedule run history visible in the UI
- Responsive layout improvements for smaller screens (1024px min)
- Accessibility: ARIA labels, keyboard-navigable modals, focus trapping

---

## Implementation Steps

### Phase 1 — Task Table Overhaul (3–4 days, Medium complexity)

#### 1.1 Virtual Scrolling for TaskTable
**File:** `frontend/src/components/TaskTable.svelte`

The current `{#each $filteredTasks as task}` renders all rows. Replace with a windowed renderer:

```svelte
<script lang="ts">
  import { onMount } from 'svelte';

  const ROW_HEIGHT = 52;   // px — fixed height for virtualization
  const OVERSCAN   = 8;    // rows above/below viewport to pre-render

  let containerEl: HTMLDivElement;
  let containerHeight = 600;
  let scrollTop = 0;

  $: totalHeight  = $filteredTasks.length * ROW_HEIGHT;
  $: startIdx     = Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - OVERSCAN);
  $: endIdx       = Math.min(
      $filteredTasks.length,
      Math.ceil((scrollTop + containerHeight) / ROW_HEIGHT) + OVERSCAN
  );
  $: visibleTasks = $filteredTasks.slice(startIdx, endIdx);
  $: offsetY      = startIdx * ROW_HEIGHT;

  function onScroll(e: Event) {
    scrollTop = (e.target as HTMLElement).scrollTop;
  }

  onMount(() => {
    const ro = new ResizeObserver(([entry]) => {
      containerHeight = entry.contentRect.height;
    });
    ro.observe(containerEl);
    return () => ro.disconnect();
  });
</script>

<div class="task-table-scroll" bind:this={containerEl} on:scroll={onScroll}
     style="height: 100%; overflow-y: auto;">
  <div style="height: {totalHeight}px; position: relative;">
    <div style="transform: translateY({offsetY}px);">
      {#each visibleTasks as task (task.id)}
        <TaskRow {task} on:select on:action />
      {/each}
    </div>
  </div>
</div>
```

Also extract `<TaskRow>` into `frontend/src/components/TaskRow.svelte` to keep TaskTable.svelte < 200 lines.

#### 1.2 Bulk Selection and Actions
**File:** `frontend/src/components/TaskTable.svelte` and `TaskToolbar.svelte`

Add a checkbox column and bulk action bar:

```svelte
<!-- In store.ts — add selection state -->
export const selectedTaskIds = writable<Set<string>>(new Set());

export function toggleTaskSelection(id: string) {
  selectedTaskIds.update(s => {
    const next = new Set(s);
    next.has(id) ? next.delete(id) : next.add(id);
    return next;
  });
}

export function clearSelection() {
  selectedTaskIds.set(new Set());
}
```

```svelte
<!-- TaskToolbar.svelte — bulk action bar appears when selection non-empty -->
{#if $selectedCount > 0}
  <div class="bulk-bar" role="toolbar" aria-label="Bulk actions">
    <span>{$selectedCount} selected</span>
    <button on:click={bulkStart}   disabled={$bulkStarting}>Start</button>
    <button on:click={bulkCancel}  class="danger">Cancel</button>
    <button on:click={bulkDelete}  class="danger">Delete</button>
    <button on:click={clearSelection}>✕ Clear</button>
  </div>
{/if}
```

Backend already has `StartAllPending()` — add `BulkCancelTasks(ids: string[])` and `BulkDeleteTasks(ids: string[])` to `app_tasks.go`:

```go
// app_tasks.go
func (a *App) BulkCancelTasks(ids []string) error {
    for _, id := range ids {
        if err := a.CancelTask(id); err != nil {
            // log and continue, don't abort whole batch
        }
    }
    return nil
}

func (a *App) BulkDeleteTasks(ids []string) error {
    for _, id := range ids {
        if err := a.DeleteTask(id); err != nil {
            // log and continue
        }
    }
    return nil
}
```

#### 1.3 Column Sorting
**File:** `frontend/src/components/TaskTable.svelte`

```svelte
<script lang="ts">
  type SortKey = 'name' | 'status' | 'priority' | 'created_at' | 'duration';
  let sortKey: SortKey = 'created_at';
  let sortDir: 'asc' | 'desc' = 'desc';

  $: sortedTasks = [...$filteredTasks].sort((a, b) => {
    const mul = sortDir === 'asc' ? 1 : -1;
    if (sortKey === 'priority') return mul * (a.priority - b.priority);
    if (sortKey === 'created_at') return mul * (new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
    return mul * String(a[sortKey]).localeCompare(String(b[sortKey]));
  });

  function toggleSort(key: SortKey) {
    if (sortKey === key) sortDir = sortDir === 'asc' ? 'desc' : 'asc';
    else { sortKey = key; sortDir = 'desc'; }
  }
</script>

<th on:click={() => toggleSort('priority')} class:sorted={sortKey === 'priority'}>
  Priority {sortKey === 'priority' ? (sortDir === 'asc' ? '↑' : '↓') : ''}
</th>
```

---

### Phase 2 — Inline Log Viewer (2–3 days, Medium complexity)

#### 2.1 Rewrite LogViewer Component
**File:** `frontend/src/components/LogViewer.svelte`

Replace the export-only panel with a full inline viewer:

```svelte
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
  import { SearchTaskLogs, ExportTaskLogs } from '../../wailsjs/go/main/App';
  import type { StepLog } from '../lib/types';

  export let taskId: string;

  const MAX_BUFFER = 2000;
  let logs: StepLog[] = [];
  let search = '';
  let levelFilter = '';
  let liveTail = true;
  let logContainer: HTMLDivElement;

  // Historical load
  onMount(async () => {
    const history = await SearchTaskLogs({ taskID: taskId, limit: 500 });
    logs = history;
    if (liveTail) scrollToBottom();
  });

  // Live tail via Wails events
  onMount(() => {
    EventsOn('log:entry', (entry: StepLog) => {
      if (entry.task_id !== taskId) return;
      logs = [...logs.slice(-MAX_BUFFER + 1), entry];
      if (liveTail) scrollToBottom();
    });
    return () => EventsOff('log:entry');
  });

  $: filteredLogs = logs.filter(l =>
    (!levelFilter || l.level === levelFilter) &&
    (!search || l.message?.toLowerCase().includes(search.toLowerCase()) ||
                l.action?.toLowerCase().includes(search.toLowerCase()))
  );

  function scrollToBottom() {
    requestAnimationFrame(() => logContainer?.scrollTo({ top: logContainer.scrollHeight }));
  }
</script>

<div class="log-viewer">
  <div class="log-toolbar">
    <input bind:value={search} placeholder="Search logs…" />
    <select bind:value={levelFilter}>
      <option value="">All levels</option>
      <option value="info">Info</option>
      <option value="warn">Warn</option>
      <option value="error">Error</option>
    </select>
    <label><input type="checkbox" bind:checked={liveTail} /> Live tail</label>
    <button on:click={() => ExportTaskLogs(taskId)}>Export ZIP</button>
  </div>

  <div class="log-entries" bind:this={logContainer}>
    {#each filteredLogs as log (log.id)}
      <div class="log-entry level-{log.level}">
        <span class="ts">{formatTime(log.created_at)}</span>
        <span class="level">[{log.level?.toUpperCase().padEnd(5)}]</span>
        <span class="action">{log.action ?? ''}</span>
        <span class="msg">{log.message ?? log.error ?? ''}</span>
        {#if log.duration_ms}
          <span class="dur">{log.duration_ms}ms</span>
        {/if}
      </div>
    {/each}
  </div>
</div>
```

Backend: add `SearchTaskLogs(query LogSearchQuery)` to `app_export.go` (delegates to `db.SearchLogs()`).

#### 2.2 Network Log Tab
**File:** `frontend/src/components/LogViewer.svelte` — add tabbed interface

```
┌─ Step Logs ─┬─ Network ─┬─ WebSocket ─┐
│             │           │             │
```

Network tab shows `ListNetworkLogs(taskID)` results with method, URL, status code, size, duration columns. Filter by status code range (2xx/3xx/4xx/5xx).

---

### Phase 3 — Visual Diff Viewer (2–3 days, Medium complexity)

#### 3.1 VisualDiffViewer Component
**File:** `frontend/src/components/VisualDiffViewer.svelte` — this file exists but needs implementation

```svelte
<script lang="ts">
  import { ListVisualBaselines, CompareVisual, ListVisualDiffs } from '../../wailsjs/go/main/App';
  import type { VisualBaseline, VisualDiff } from '../lib/types';

  export let taskId: string = '';

  let baselines: VisualBaseline[] = [];
  let selectedBaselineId = '';
  let diffs: VisualDiff[] = [];
  let comparing = false;
  let threshold = 5.0;

  // Slider for overlay opacity in side-by-side mode
  let overlayMode: 'side-by-side' | 'overlay' | 'diff-only' = 'side-by-side';
  let overlayOpacity = 0.5;

  async function runComparison() {
    comparing = true;
    try {
      const result = await CompareVisual({ baseline_id: selectedBaselineId, task_id: taskId, threshold });
      diffs = [result, ...diffs];
    } finally {
      comparing = false;
    }
  }
</script>

<div class="diff-viewer">
  <div class="controls">
    <select bind:value={selectedBaselineId}>
      {#each baselines as b}
        <option value={b.id}>{b.name} ({b.width}×{b.height})</option>
      {/each}
    </select>
    <label>Threshold: <input type="range" min="0" max="20" step="0.5" bind:value={threshold} /> {threshold}%</label>
    <button on:click={runComparison} disabled={comparing || !selectedBaselineId}>
      {comparing ? 'Comparing…' : 'Compare'}
    </button>
  </div>

  {#if diffs.length > 0}
    {@const latest = diffs[0]}
    <div class="result" class:pass={latest.passed} class:fail={!latest.passed}>
      {latest.passed ? '✓ PASS' : '✗ FAIL'} — {latest.diff_percent.toFixed(2)}% diff
      (threshold: {latest.threshold}%)
    </div>

    <div class="view-toggle">
      <button class:active={overlayMode === 'side-by-side'} on:click={() => overlayMode = 'side-by-side'}>Side by Side</button>
      <button class:active={overlayMode === 'overlay'}      on:click={() => overlayMode = 'overlay'}>Overlay</button>
      <button class:active={overlayMode === 'diff-only'}    on:click={() => overlayMode = 'diff-only'}>Diff Only</button>
    </div>

    {#if overlayMode === 'side-by-side'}
      <div class="side-by-side">
        <div><h4>Baseline</h4><img src="data:image/png;base64,{latest.baseline_b64}" alt="baseline" /></div>
        <div><h4>Current</h4> <img src="data:image/png;base64,{latest.current_b64}"  alt="current"  /></div>
      </div>
    {:else if overlayMode === 'overlay'}
      <div class="overlay-container">
        <img src="data:image/png;base64,{latest.baseline_b64}" alt="baseline" />
        <img src="data:image/png;base64,{latest.current_b64}"  alt="current"
             style="opacity: {overlayOpacity}; position: absolute; top: 0; left: 0;" />
        <input type="range" min="0" max="1" step="0.05" bind:value={overlayOpacity} />
      </div>
    {:else}
      <img src="data:image/png;base64,{latest.diff_b64}" alt="diff" />
    {/if}
  {/if}
</div>
```

Backend: add base64 fields to `models.VisualDiff` or a new `VisualDiffDetail` response that includes the image bytes. Add `GetVisualDiffDetail(id string)` to `app_vision.go`.

---

### Phase 4 — Cron Expression Builder (1–2 days, Low complexity)

#### 4.1 CronBuilder Component
**New file:** `frontend/src/components/CronBuilder.svelte`

```svelte
<script lang="ts">
  export let value = '0 9 * * 1-5';   // bindable cron string

  // Parse into fields
  let fields = parseCron(value);    // { minute, hour, dom, month, dow }
  let preview = '';
  let valid = true;

  $: {
    value = `${fields.minute} ${fields.hour} ${fields.dom} ${fields.month} ${fields.dow}`;
    preview = describesCron(fields);   // "Every weekday at 09:00"
    valid = validateCron(value);
  }

  // Quick presets
  const presets = [
    { label: 'Every hour',         value: '0 * * * *'    },
    { label: 'Every day at 9am',   value: '0 9 * * *'    },
    { label: 'Every weekday 9am',  value: '0 9 * * 1-5'  },
    { label: 'Every Sunday midnight', value: '0 0 * * 0' },
    { label: 'Every 15 minutes',   value: '*/15 * * * *' },
  ];
</script>

<div class="cron-builder">
  <div class="presets">
    {#each presets as p}
      <button on:click={() => { value = p.value; fields = parseCron(p.value); }}>
        {p.label}
      </button>
    {/each}
  </div>

  <div class="fields">
    <label>Minute   <input bind:value={fields.minute}  placeholder="0-59 or *" /></label>
    <label>Hour     <input bind:value={fields.hour}    placeholder="0-23 or *" /></label>
    <label>Day/Month<input bind:value={fields.dom}     placeholder="1-31 or *" /></label>
    <label>Month    <input bind:value={fields.month}   placeholder="1-12 or *" /></label>
    <label>Weekday  <input bind:value={fields.dow}     placeholder="0-6 or *"  /></label>
  </div>

  <div class="preview" class:invalid={!valid}>
    {#if valid}
      📅 {preview}
    {:else}
      ⚠ Invalid cron expression
    {/if}
  </div>

  <code class="raw">{value}</code>
</div>
```

Wire into `SchedulePanel.svelte` to replace the raw text input.

---

### Phase 5 — Form Draft Persistence (1–2 days, Low complexity)

#### 5.1 useDraft Svelte Store Helper
**New file:** `frontend/src/lib/draft.ts`

```typescript
import { writable, get } from 'svelte/store';

/**
 * Creates a writable store that persists to localStorage.
 * Useful for preserving modal form state across close/reopen.
 */
export function draftStore<T>(key: string, initial: T) {
  const stored = localStorage.getItem(key);
  const data = stored ? JSON.parse(stored) : initial;
  const store = writable<T>(data);

  store.subscribe(value => {
    localStorage.setItem(key, JSON.stringify(value));
  });

  return {
    ...store,
    clear: () => {
      localStorage.removeItem(key);
      store.set(initial);
    }
  };
}
```

Usage in `CreateTaskModal.svelte`:
```svelte
<script lang="ts">
  import { draftStore } from '../lib/draft';

  const draft = draftStore('create-task-draft', {
    name: '', url: '', priority: 1, tags: [], steps: []
  });

  // Clear draft on successful submission
  async function submit() {
    await CreateTask(...$draft);
    draft.clear();
    dispatch('close');
  }
</script>

<input bind:value={$draft.name} placeholder="Task name" />
```

Apply to: `CreateTaskModal.svelte`, `BatchCreateModal.svelte`, `SchedulePanel.svelte` (new schedule form).

---

### Phase 6 — Keyboard Shortcuts (1 day, Low complexity)

#### 6.1 Global Keyboard Handler
**File:** `frontend/src/App.svelte` — add global keydown listener:

```svelte
<script lang="ts">
  import { selectedTab, selectedTaskIds } from './lib/store';

  function handleKeydown(e: KeyboardEvent) {
    // Don't fire inside inputs/textareas
    if (['INPUT','TEXTAREA','SELECT'].includes((e.target as HTMLElement).tagName)) return;

    if (e.key === 'n' && !e.metaKey && !e.ctrlKey) showCreateModal = true;   // N → new task
    if (e.key === 'Escape')                          showCreateModal = false;  // Esc → close modal
    if (e.key === 'a' && (e.metaKey || e.ctrlKey)) { e.preventDefault(); selectAll(); }
    if (e.key === 'Delete' && $selectedTaskIds.size > 0) bulkDelete();
    if (e.key === '1') selectedTab.set('tasks');
    if (e.key === '2') selectedTab.set('flows');
    if (e.key === '3') selectedTab.set('schedules');
    if (e.key === '4') selectedTab.set('proxies');
    if (e.key === '?') showShortcutsHelp = true;
  }
</script>

<svelte:window on:keydown={handleKeydown} />
```

#### 6.2 Shortcuts Help Modal
**New file:** `frontend/src/components/ShortcutsModal.svelte`

```
┌─ Keyboard Shortcuts ──────────────────────────────┐
│  N          Create new task                        │
│  Escape     Close modal / deselect all             │
│  ⌘A / Ctrl+A  Select all visible tasks            │
│  Delete     Delete selected tasks                  │
│  1–4        Switch tabs (Tasks / Flows / etc.)     │
│  ?          Show this help                         │
└───────────────────────────────────────────────────┘
```

---

### Phase 7 — Schedule Run History (1–2 days, Low complexity)

#### 7.1 Schedule History Table
**File:** `internal/database/db_schedules.go` — add query:

```go
// ListScheduleRuns returns recent tasks spawned by a schedule.
func (db *DB) ListScheduleRuns(ctx context.Context, scheduleID string, limit int) ([]models.Task, error) {
    return db.listTasksSummary(ctx,
        `SELECT `+taskSummaryColumns+` FROM tasks
         WHERE flow_id = (SELECT flow_id FROM schedules WHERE id = ?)
           AND name LIKE '[sched] %'
         ORDER BY created_at DESC LIMIT ?`,
        scheduleID, limit)
}
```

Add `ListScheduleRuns(id string, limit int)` to `app_schedules.go` and wire into `SchedulePanel.svelte`.

#### 7.2 Schedule Panel Run History
**File:** `frontend/src/components/SchedulePanel.svelte`

Expandable run history row below each schedule:

```
▶ Daily Login Check   0 9 * * *   Next: tomorrow 09:00   [Edit] [Toggle]
  └─ Recent runs:
     ✓ 2026-03-24 09:00  completed  4.2s
     ✗ 2026-03-23 09:00  failed     Element not found
     ✓ 2026-03-22 09:00  completed  3.8s
```

---

### Phase 8 — Accessibility & Responsive Layout (2–3 days, Medium complexity)

#### 8.1 Modal Accessibility
**File:** All modal components (`CreateTaskModal.svelte`, `BatchCreateModal.svelte`, etc.)

```svelte
<!-- Focus trap: on open, move focus to first focusable element -->
<script>
  import { onMount } from 'svelte';
  onMount(() => {
    const first = modal.querySelector('input,select,button,[tabindex]') as HTMLElement;
    first?.focus();
  });

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') dispatch('close');
    if (e.key === 'Tab') trapFocus(e, modal);
  }
</script>

<div role="dialog" aria-modal="true" aria-labelledby="modal-title"
     bind:this={modal} on:keydown={handleKeydown}>
  <h2 id="modal-title">Create Task</h2>
  ...
</div>
```

#### 8.2 ARIA Labels on Interactive Elements
Apply across all components:
```svelte
<!-- TaskTable.svelte -->
<button aria-label="Start task {task.name}" on:click={() => startTask(task.id)}>▶</button>
<button aria-label="Cancel task {task.name}" on:click={() => cancelTask(task.id)}>✕</button>

<!-- Header.svelte -->
<div role="status" aria-live="polite" aria-label="Queue metrics">
  Running: {$metrics.running} | Queued: {$metrics.queued}
</div>
```

#### 8.3 Responsive Breakpoints
**File:** `frontend/src/style.css` — add responsive rules:

```css
/* Collapse proxy/schedule panels to tabs on small screens */
@media (max-width: 1280px) {
  .side-panel { width: 280px; }
  .task-table  { flex: 1; min-width: 0; }
}

@media (max-width: 1024px) {
  .layout { flex-direction: column; }
  .side-panel { width: 100%; height: 240px; overflow-y: auto; }
}

/* Hide less-critical columns on narrow screens */
@media (max-width: 1100px) {
  .col-tags, .col-proxy { display: none; }
}
```

---

### Phase 9 — Error Handling & Toast Notifications (1–2 days, Low complexity)

#### 9.1 Toast Store
**File:** `frontend/src/lib/store.ts` — add toast queue:

```typescript
export interface Toast {
  id: string;
  type: 'success' | 'error' | 'warning' | 'info';
  message: string;
  durationMs?: number;
}

export const toasts = writable<Toast[]>([]);

export function showToast(type: Toast['type'], message: string, durationMs = 4000) {
  const id = crypto.randomUUID();
  toasts.update(t => [...t, { id, type, message, durationMs }]);
  setTimeout(() => toasts.update(t => t.filter(x => x.id !== id)), durationMs);
}
```

#### 9.2 Toast Component
**New file:** `frontend/src/components/ToastContainer.svelte`

```svelte
<script>
  import { toasts } from '../lib/store';
  import { fly } from 'svelte/transition';
</script>

<div class="toast-container" aria-live="assertive" role="alert">
  {#each $toasts as toast (toast.id)}
    <div class="toast toast-{toast.type}" transition:fly={{ y: 20, duration: 200 }}>
      {toast.message}
    </div>
  {/each}
</div>
```

Replace all inline `alert()` / `console.error()` calls across components with `showToast()`.

#### 9.3 Centralized Error Handler
**File:** `frontend/src/lib/store.ts`

```typescript
export async function callApi<T>(fn: () => Promise<T>, errorMsg: string): Promise<T | null> {
  try {
    return await fn();
  } catch (e) {
    showToast('error', `${errorMsg}: ${String(e)}`);
    return null;
  }
}
```

Usage: `const tasks = await callApi(() => ListTasks(), 'Failed to load tasks');`

---

## Testing Strategy

### Frontend Unit Tests (Vitest + @testing-library/svelte)
- `TaskTable.svelte`: render 5000 tasks, verify only ~50 rows in DOM (virtual scroll)
- `LogViewer.svelte`: emit 3000 log events, verify buffer capped at 2000
- `CronBuilder.svelte`: test all presets produce valid cron strings
- `draftStore`: verify persistence and clear across simulated modal open/close
- `ToastContainer.svelte`: verify toast auto-dismisses after durationMs

### Manual Testing Checklist
- [ ] 10,000 tasks: scroll to bottom without jank
- [ ] Bulk select 500 tasks, cancel — verify all cancelled
- [ ] Open CreateTaskModal, fill form, close, reopen — data persists
- [ ] Cron `*/15 * * * *` → preview "Every 15 minutes"
- [ ] Visual diff: run comparison, verify pass/fail badge correct
- [ ] Tab key navigates all modals without reaching behind-modal elements
- [ ] Keyboard shortcut `N` opens create modal from task tab

### CI additions (`.github/workflows/ci.yml`)
```yaml
- name: Frontend test
  run: npm run test -- --run --reporter=verbose
  working-directory: frontend
```
Add coverage threshold check: `--coverage --coverage-threshold-lines=70`.

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| Virtual scroll breaks `{#key}` animations | Medium | Low | Disable CSS transitions on scrolled rows; use opacity fade only |
| Draft persistence stores sensitive data (URLs, selectors) | Low | Medium | Exclude proxy credentials from draft; only persist non-sensitive fields |
| Bulk delete fires accidentally | Medium | High | Require confirmation dialog for bulk delete > 10 tasks |
| Focus trap breaks browser DevTools shortcut | Low | Low | Only trap focus within modal bounds, not globally |
| CronBuilder produces invalid expression on edge inputs | Medium | Medium | Validate with the same `scheduler.ParseCron()` logic via a backend call |
| Toast flood on bulk operation errors | Medium | Medium | Deduplicate: same message within 2s shows once with count badge |

---

## Success Criteria

- [ ] TaskTable renders 10,000 rows: < 16ms frame time during scroll (Chrome DevTools)
- [ ] DOM node count for 10,000 tasks: < 500 nodes total (virtual scroll working)
- [ ] LogViewer: live log entries appear within 100ms of Wails event emission
- [ ] CronBuilder validates all 5 preset expressions client-side with plain-English preview
- [ ] Form draft survives modal close/reopen: zero data loss on accidental dismissal
- [ ] Bulk cancel 500 tasks: completes without UI freeze (async with progress indicator)
- [ ] All modal dialogs pass keyboard navigation test (Tab, Shift+Tab, Escape)
- [ ] ARIA audit (axe-core): zero critical violations across all components
- [ ] Vitest coverage: ≥ 70% line coverage on store.ts and all new components
- [ ] Visual diff viewer shows baseline vs current side-by-side with overlay slider
