import { writable, derived } from 'svelte/store';
import type { Task, Proxy, TaskEvent, TaskStatus, RecordedFlow, RecordedStep, WebSocketLog, Schedule, CaptchaConfig, VisualBaseline } from './types';

export const tasks = writable<Task[]>([]);
export const proxies = writable<Proxy[]>([]);
export const selectedTaskId = writable<string | null>(null);
export const activeTab = writable<'tasks' | 'proxies' | 'recorder' | 'schedules' | 'settings' | 'visual'>('tasks');
export const statusFilter = writable<TaskStatus | 'all'>('all');
export const tagFilter = writable<string>('');
export const recordedFlows = writable<RecordedFlow[]>([]);
export const isRecording = writable<boolean>(false);
export const recordingSteps = writable<RecordedStep[]>([]);
export const webSocketLogs = writable<WebSocketLog[]>([]);
export const schedules = writable<Schedule[]>([]);
export const captchaConfig = writable<CaptchaConfig | null>(null);
export const visualBaselines = writable<VisualBaseline[]>([]);

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

export function updateTaskInStore(event: TaskEvent) {
  tasks.update(list => {
    const index = list.findIndex(t => t.id === event.taskId);
    if (index === -1) {
      return list;
    }

    const current = list[index];
    const nextError = event.error || current.error;
    if (current.status === event.status && current.error === nextError) {
      return list;
    }

    const updated = [...list];
    updated[index] = { ...current, status: event.status, error: nextError };
    return updated;
  });
}

export function replaceTaskInStore(updated: Task) {
  tasks.update(list => {
    const index = list.findIndex(t => t.id === updated.id);
    if (index === -1) {
      return list;
    }

    const next = [...list];
    next[index] = updated;
    return next;
  });
}

export function removeTaskFromStore(taskId: string) {
  tasks.update(list => {
    const index = list.findIndex(t => t.id === taskId);
    if (index === -1) {
      return list;
    }

    return list.filter(t => t.id !== taskId);
  });
}
