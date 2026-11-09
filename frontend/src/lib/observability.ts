export interface StepStat {
  action: string;
  count: number;
  errors: number;
  errorRate: number;
  avgMs: number;
  maxMs: number;
  p50Ms: number;
  p95Ms: number;
  slowCount: number;
}

export interface SlowStepEntry {
  taskId: string;
  stepIndex: number;
  action: string;
  durationMs: number;
  error?: string;
  at: string;
}

export interface ObservabilityView {
  generatedAt: string;
  health: string;
  queue: Record<string, number>;
  pool: { totalBrowsers: number; maxBrowsers: number; activeTabs: number; idleBrowsers: number };
  plugins: { loaded: number; running: number; restarts: number; errors: number };
  steps: StepStat[];
  slowSteps: SlowStepEntry[];
  recentErrors: string[];
  activeAlerts: { id: string; rule_name: string; severity: string; current_value: number; threshold: number }[];
  runtime: { goroutines: number; heapMB: number; numGC: number; uptimeSec: number };
  mcp: { enabled: boolean; url: string; auth: boolean };
}

const arr = <T>(v: unknown): T[] => (Array.isArray(v) ? (v as T[]) : []);
const num = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) ? v : 0);

export function normalizeObservability(raw: unknown): ObservabilityView {
  const src = (raw ?? {}) as Record<string, any>;
  const queue: Record<string, number> = {};
  for (const [k, v] of Object.entries(src.queue ?? {})) queue[k] = num(v);
  const plugins = arr<any>(src.plugins?.plugins);
  return {
    generatedAt: typeof src.generatedAt === 'string' ? src.generatedAt : '',
    health: typeof src.health?.status === 'string' ? src.health.status : 'unknown',
    queue,
    pool: {
      totalBrowsers: num(src.browserPool?.totalBrowsers),
      maxBrowsers: num(src.browserPool?.maxBrowsers),
      activeTabs: num(src.browserPool?.activeTabs),
      idleBrowsers: num(src.browserPool?.idleBrowsers),
    },
    plugins: {
      loaded: plugins.length,
      running: plugins.filter((p) => !p.status || p.status.running).length,
      restarts: plugins.reduce((n, p) => n + num(p.status?.restarts), 0),
      errors: arr(src.plugins?.errors).length,
    },
    steps: sortStepsBySlowest(arr<StepStat>(src.steps)),
    slowSteps: arr<SlowStepEntry>(src.slowSteps),
    recentErrors: arr<string>(src.recentErrors).filter((e) => typeof e === 'string'),
    activeAlerts: arr(src.activeAlerts),
    runtime: {
      goroutines: num(src.runtime?.goroutines),
      heapMB: num(src.runtime?.heapMB),
      numGC: num(src.runtime?.numGC),
      uptimeSec: num(src.runtime?.uptimeSec),
    },
    mcp: { enabled: !!src.mcp?.enabled, url: typeof src.mcp?.url === 'string' ? src.mcp.url : '', auth: !!src.mcp?.auth },
  };
}

export function sortStepsBySlowest(steps: StepStat[]): StepStat[] {
  return [...steps].sort((a, b) => b.p95Ms - a.p95Ms || b.count - a.count || a.action.localeCompare(b.action));
}

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)} ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(ms < 10000 ? 2 : 1)} s`;
  return `${Math.floor(ms / 60000)}m ${Math.round((ms % 60000) / 1000)}s`;
}

export function formatUptime(sec: number): string {
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = Math.floor(sec % 60);
  return h > 0 ? `${h}h ${m}m` : m > 0 ? `${m}m ${s}s` : `${s}s`;
}

export function healthTone(status: string): 'ok' | 'warn' | 'bad' {
  if (status === 'healthy') return 'ok';
  if (status === 'degraded' || status === 'warning') return 'warn';
  return 'bad';
}

export function successRate(queue: Record<string, number>): number | null {
  const done = (queue.totalCompleted ?? 0) + (queue.totalFailed ?? 0);
  return done === 0 ? null : (queue.totalCompleted ?? 0) / done;
}
