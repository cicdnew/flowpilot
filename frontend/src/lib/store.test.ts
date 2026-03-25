import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { tasks, proxies, selectedTaskId, statusFilter, tagFilter, activeTab, selectedTask, filteredTasks, taskStats, allTags, updateTaskInStore } from './store';
import type { Task, TaskEvent } from './types';

function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    id: 'task-1',
    name: 'Test Task',
    url: 'https://example.com',
    steps: [{ action: 'navigate', value: 'https://example.com' }],
    proxy: { server: '' },
    priority: 5,
    status: 'pending',
    retryCount: 0,
    maxRetries: 3,
    createdAt: '2024-01-01T00:00:00Z',
    ...overrides,
  };
}

beforeEach(() => {
  tasks.set([]);
  proxies.set([]);
  selectedTaskId.set(null);
  statusFilter.set('all');
  tagFilter.set('');
  activeTab.set('tasks' as 'tasks' | 'proxies');
});

describe('filteredTasks', () => {
  it('returns all tasks when filter is all', () => {
    tasks.set([makeTask({ id: '1' }), makeTask({ id: '2', status: 'running' })]);
    statusFilter.set('all');
    expect(get(filteredTasks)).toHaveLength(2);
  });

  it('filters by pending status', () => {
    tasks.set([
      makeTask({ id: '1', status: 'pending' }),
      makeTask({ id: '2', status: 'running' }),
      makeTask({ id: '3', status: 'pending' }),
    ]);
    statusFilter.set('pending');
    const result = get(filteredTasks);
    expect(result).toHaveLength(2);
    expect(result.every(t => t.status === 'pending')).toBe(true);
  });

  it('filters by completed status', () => {
    tasks.set([
      makeTask({ id: '1', status: 'completed' }),
      makeTask({ id: '2', status: 'failed' }),
    ]);
    statusFilter.set('completed');
    expect(get(filteredTasks)).toHaveLength(1);
    expect(get(filteredTasks)[0].id).toBe('1');
  });

  it('returns empty when no tasks match filter', () => {
    tasks.set([makeTask({ id: '1', status: 'pending' })]);
    statusFilter.set('running');
    expect(get(filteredTasks)).toHaveLength(0);
  });
});

describe('taskStats', () => {
  it('counts tasks by status', () => {
    tasks.set([
      makeTask({ id: '1', status: 'pending' }),
      makeTask({ id: '2', status: 'pending' }),
      makeTask({ id: '3', status: 'running' }),
      makeTask({ id: '4', status: 'completed' }),
      makeTask({ id: '5', status: 'failed' }),
    ]);
    const stats = get(taskStats);
    expect(stats.total).toBe(5);
    expect(stats.pending).toBe(2);
    expect(stats.running).toBe(1);
    expect(stats.completed).toBe(1);
    expect(stats.failed).toBe(1);
  });

  it('returns all zeros for empty task list', () => {
    tasks.set([]);
    const stats = get(taskStats);
    expect(stats.total).toBe(0);
    expect(stats.pending).toBe(0);
    expect(stats.running).toBe(0);
  });
});

describe('selectedTask', () => {
  it('returns null when no task selected', () => {
    tasks.set([makeTask({ id: '1' })]);
    selectedTaskId.set(null);
    expect(get(selectedTask)).toBeNull();
  });

  it('returns the selected task', () => {
    tasks.set([makeTask({ id: '1', name: 'First' }), makeTask({ id: '2', name: 'Second' })]);
    selectedTaskId.set('2');
    expect(get(selectedTask)?.name).toBe('Second');
  });

  it('returns null when selected id does not match', () => {
    tasks.set([makeTask({ id: '1' })]);
    selectedTaskId.set('nonexistent');
    expect(get(selectedTask)).toBeNull();
  });
});

describe('updateTaskInStore', () => {
  it('updates task status from event', () => {
    tasks.set([
      makeTask({ id: '1', status: 'pending' }),
      makeTask({ id: '2', status: 'pending' }),
    ]);

    const event: TaskEvent = { taskId: '1', status: 'running' };
    updateTaskInStore(event);

    const list = get(tasks);
    expect(list[0].status).toBe('running');
    expect(list[1].status).toBe('pending');
  });

  it('updates task error from event', () => {
    tasks.set([makeTask({ id: '1', status: 'running' })]);

    const event: TaskEvent = { taskId: '1', status: 'failed', error: 'timeout' };
    updateTaskInStore(event);

    const list = get(tasks);
    expect(list[0].status).toBe('failed');
    expect(list[0].error).toBe('timeout');
  });

  it('preserves existing error when event has no error', () => {
    tasks.set([makeTask({ id: '1', status: 'failed', error: 'old error' })]);

    const event: TaskEvent = { taskId: '1', status: 'retrying' };
    updateTaskInStore(event);

    expect(get(tasks)[0].error).toBe('old error');
  });

  it('does not modify other tasks', () => {
    tasks.set([
      makeTask({ id: '1', status: 'pending', name: 'First' }),
      makeTask({ id: '2', status: 'pending', name: 'Second' }),
    ]);

    updateTaskInStore({ taskId: '1', status: 'running' });

    expect(get(tasks)[1].status).toBe('pending');
    expect(get(tasks)[1].name).toBe('Second');
  });
});

describe('allTags', () => {
  it('returns empty array when no tasks', () => {
    tasks.set([]);
    expect(get(allTags)).toEqual([]);
  });

  it('collects unique tags from all tasks sorted', () => {
    tasks.set([
      makeTask({ id: '1', tags: ['beta', 'alpha'] }),
      makeTask({ id: '2', tags: ['gamma', 'alpha'] }),
      makeTask({ id: '3' }),
    ]);
    expect(get(allTags)).toEqual(['alpha', 'beta', 'gamma']);
  });

  it('handles tasks with no tags field', () => {
    tasks.set([makeTask({ id: '1' }), makeTask({ id: '2', tags: ['only'] })]);
    expect(get(allTags)).toEqual(['only']);
  });
});

describe('filteredTasks with tagFilter', () => {
  it('filters tasks by tag', () => {
    tasks.set([
      makeTask({ id: '1', tags: ['scraping', 'daily'] }),
      makeTask({ id: '2', tags: ['daily'] }),
      makeTask({ id: '3', tags: ['scraping'] }),
    ]);
    tagFilter.set('scraping');
    const result = get(filteredTasks);
    expect(result).toHaveLength(2);
    expect(result.map(t => t.id).sort()).toEqual(['1', '3']);
  });

  it('returns all when tagFilter is empty', () => {
    tasks.set([
      makeTask({ id: '1', tags: ['a'] }),
      makeTask({ id: '2' }),
    ]);
    tagFilter.set('');
    expect(get(filteredTasks)).toHaveLength(2);
  });

  it('combines status and tag filters', () => {
    tasks.set([
      makeTask({ id: '1', status: 'pending', tags: ['prod'] }),
      makeTask({ id: '2', status: 'running', tags: ['prod'] }),
      makeTask({ id: '3', status: 'pending', tags: ['dev'] }),
    ]);
    statusFilter.set('pending');
    tagFilter.set('prod');
    const result = get(filteredTasks);
    expect(result).toHaveLength(1);
    expect(result[0].id).toBe('1');
  });

  it('returns empty when tag matches no tasks', () => {
    tasks.set([makeTask({ id: '1', tags: ['a'] })]);
    tagFilter.set('nonexistent');
    expect(get(filteredTasks)).toHaveLength(0);
  });
});

describe('proxies store', () => {
  it('starts empty', () => {
    expect(get(proxies)).toEqual([]);
  });

  it('can store and retrieve proxies', () => {
    proxies.set([
      { id: 'p1', server: 'proxy1:8080', protocol: 'http', status: 'healthy', latency: 50, successRate: 0.95, totalUsed: 10, createdAt: '2024-01-01T00:00:00Z' },
      { id: 'p2', server: 'proxy2:8080', protocol: 'socks5', status: 'unknown', latency: 0, successRate: 0, totalUsed: 0, createdAt: '2024-01-01T00:00:00Z' },
    ]);
    expect(get(proxies)).toHaveLength(2);
    expect(get(proxies)[0].server).toBe('proxy1:8080');
  });
});

describe('activeTab store', () => {
  it('defaults to tasks', () => {
    expect(get(activeTab)).toBe('tasks');
  });

  it('can be set to proxies', () => {
    activeTab.set('proxies');
    expect(get(activeTab)).toBe('proxies');
  });
});

describe('selectedTaskId store', () => {
  it('defaults to null', () => {
    expect(get(selectedTaskId)).toBeNull();
  });

  it('can track a selected task', () => {
    selectedTaskId.set('abc-123');
    expect(get(selectedTaskId)).toBe('abc-123');
  });
});

describe('statusFilter store', () => {
  it('defaults to all', () => {
    expect(get(statusFilter)).toBe('all');
  });

  it('can be set to a specific status', () => {
    statusFilter.set('failed');
    expect(get(statusFilter)).toBe('failed');
  });
});

describe('updateTaskInStore edge cases', () => {
  it('does nothing when event targets nonexistent task', () => {
    tasks.set([makeTask({ id: '1', status: 'pending' })]);
    updateTaskInStore({ taskId: 'nonexistent', status: 'running' });
    expect(get(tasks)[0].status).toBe('pending');
  });

  it('handles empty task list', () => {
    tasks.set([]);
    updateTaskInStore({ taskId: '1', status: 'running' });
    expect(get(tasks)).toHaveLength(0);
  });
});

describe('filteredTasks ordering', () => {
  it('preserves original task order', () => {
    tasks.set([
      makeTask({ id: 'a', name: 'Alpha' }),
      makeTask({ id: 'b', name: 'Beta' }),
      makeTask({ id: 'c', name: 'Charlie' }),
    ]);
    const result = get(filteredTasks);
    expect(result.map(t => t.id)).toEqual(['a', 'b', 'c']);
  });
});

describe('taskStats edge cases', () => {
  it('handles retrying status', () => {
    tasks.set([
      makeTask({ id: '1', status: 'retrying' }),
      makeTask({ id: '2', status: 'retrying' }),
    ]);
    const stats = get(taskStats);
    expect(stats.retrying).toBe(2);
    expect(stats.total).toBe(2);
  });

  it('handles all statuses at once', () => {
    tasks.set([
      makeTask({ id: '1', status: 'pending' }),
      makeTask({ id: '2', status: 'queued' }),
      makeTask({ id: '3', status: 'running' }),
      makeTask({ id: '4', status: 'completed' }),
      makeTask({ id: '5', status: 'failed' }),
      makeTask({ id: '6', status: 'cancelled' }),
      makeTask({ id: '7', status: 'retrying' }),
    ]);
    const stats = get(taskStats);
    expect(stats.total).toBe(7);
    expect(stats.pending).toBe(1);
    expect(stats.queued).toBe(1);
    expect(stats.running).toBe(1);
    expect(stats.completed).toBe(1);
    expect(stats.failed).toBe(1);
    expect(stats.cancelled).toBe(1);
    expect(stats.retrying).toBe(1);
  });
});
