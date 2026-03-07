import { writable, derived } from 'svelte/store';
import type { Task, Proxy, TaskEvent, TaskStatus, RecordedFlow, RecordedStep, WebSocketLog } from './types';

export const tasks = writable<Task[]>([]);
export const proxies = writable<Proxy[]>([]);
export const selectedTaskId = writable<string | null>(null);
export const activeTab = writable<'tasks' | 'proxies' | 'recorder'>('tasks');
export const statusFilter = writable<TaskStatus | 'all'>('all');
export const tagFilter = writable<string>('');
export const recordedFlows = writable<RecordedFlow[]>([]);
export const isRecording = writable<boolean>(false);
export const recordingSteps = writable<RecordedStep[]>([]);
export const webSocketLogs = writable<WebSocketLog[]>([]);

export const selectedTask = derived(
  [tasks, selectedTaskId],
  ([$tasks, $selectedTaskId]) => $tasks.find(t => t.id === $selectedTaskId) ?? null
);

export const filteredTasks = derived(
  [tasks, statusFilter, tagFilter],
  ([$tasks, $statusFilter, $tagFilter]) => {
    let result = $tasks;
    if ($statusFilter !== 'all') {
      result = result.filter(t => t.status === $statusFilter);
    }
    if ($tagFilter) {
      result = result.filter(t => t.tags?.includes($tagFilter));
    }
    return result;
  }
);

export const allTags = derived(tasks, ($tasks) => {
  const tagSet = new Set<string>();
  for (const t of $tasks) {
    for (const tag of t.tags ?? []) {
      tagSet.add(tag);
    }
  }
  return [...tagSet].sort();
});

export const taskStats = derived(tasks, ($tasks) => {
  const stats: Record<string, number> = {
    total: $tasks.length,
    pending: 0,
    queued: 0,
    running: 0,
    completed: 0,
    failed: 0,
    cancelled: 0,
    retrying: 0,
  };
  for (const t of $tasks) {
    stats[t.status] = (stats[t.status] || 0) + 1;
  }
  return stats;
});

// Update a single task in the store
export function updateTaskInStore(event: TaskEvent) {
  tasks.update(list => 
    list.map(t => 
      t.id === event.taskId 
        ? { ...t, status: event.status, error: event.error || t.error }
        : t
    )
  );
}
