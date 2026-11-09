import { describe, expect, it } from 'vitest';
import { formatDuration, formatUptime, healthTone, normalizeObservability, successRate } from './observability';

const sample = {
  generatedAt: '2026-11-09T13:00:00Z',
  health: { status: 'healthy' },
  queue: { running: 2, queued: 5, totalCompleted: 9, totalFailed: 1, persistenceBackpressure: 3 },
  browserPool: { totalBrowsers: 1, maxBrowsers: 2, activeTabs: 1, idleBrowsers: 0 },
  plugins: { plugins: [{ id: 'a', status: { running: true, restarts: 2 } }, { id: 'b', status: { running: false, restarts: 1 } }, { id: 'c' }], errors: ['bad'] },
  steps: [
    { action: 'click', count: 10, errors: 1, errorRate: 0.1, avgMs: 40, maxMs: 90, p50Ms: 50, p95Ms: 100, slowCount: 0 },
    { action: 'navigate', count: 3, errors: 0, errorRate: 0, avgMs: 900, maxMs: 1500, p50Ms: 1000, p95Ms: 2500, slowCount: 0 },
  ],
  slowSteps: null,
  recentErrors: ['boom', 7],
  activeAlerts: null,
  runtime: { goroutines: 40, heapMB: 12.5, numGC: 3, uptimeSec: 3725 },
  mcp: { enabled: true, url: 'http://127.0.0.1:8765/mcp', auth: true },
};

describe('normalizeObservability', () => {
  it('summarises a full snapshot', () => {
    const v = normalizeObservability(sample);
    expect(v.health).toBe('healthy');
    expect(v.queue.running).toBe(2);
    expect(v.plugins).toEqual({ loaded: 3, running: 2, restarts: 3, errors: 1 });
    expect(v.steps.map((s) => s.action)).toEqual(['navigate', 'click']);
    expect(v.slowSteps).toEqual([]);
    expect(v.recentErrors).toEqual(['boom']);
    expect(v.activeAlerts).toEqual([]);
    expect(v.mcp.url).toBe('http://127.0.0.1:8765/mcp');
  });

  it('tolerates an empty or missing snapshot', () => {
    const v = normalizeObservability(undefined);
    expect(v.health).toBe('unknown');
    expect(v.steps).toEqual([]);
    expect(v.pool.maxBrowsers).toBe(0);
    expect(v.mcp.enabled).toBe(false);
  });
});

describe('formatters', () => {
  it('formats durations and uptime', () => {
    expect(formatDuration(42)).toBe('42 ms');
    expect(formatDuration(2500)).toBe('2.50 s');
    expect(formatDuration(30000)).toBe('30.0 s');
    expect(formatDuration(125000)).toBe('2m 5s');
    expect(formatUptime(3725)).toBe('1h 2m');
    expect(formatUptime(65)).toBe('1m 5s');
    expect(formatUptime(9)).toBe('9s');
  });

  it('maps health and success rate', () => {
    expect(healthTone('healthy')).toBe('ok');
    expect(healthTone('degraded')).toBe('warn');
    expect(healthTone('unhealthy')).toBe('bad');
    expect(successRate({ totalCompleted: 9, totalFailed: 1 })).toBe(0.9);
    expect(successRate({})).toBeNull();
  });
});
