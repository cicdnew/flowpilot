export type TaskStatus = 'pending' | 'queued' | 'running' | 'completed' | 'failed' | 'cancelled' | 'retrying';

export interface TaskStep {
  action: string;
  selector?: string;
  value?: string;
  timeout?: number;
  condition?: string;
  label?: string;
  jumpTo?: string;
  varName?: string;
}

export interface SelectorCandidate {
  selector: string;
  strategy: string;
  score: number;
}

export interface RecordedStep {
  index: number;
  action: string;
  selector?: string;
  value?: string;
  timeout?: number;
  snapshotId?: string;
  selectorCandidates?: SelectorCandidate[];
  timestamp: string;
}

export interface RecordedFlow {
  id: string;
  name: string;
  description?: string;
  steps: RecordedStep[];
  originUrl: string;
  createdAt: string;
  updatedAt: string;
}

export interface BatchGroup {
  id: string;
  flowId: string;
  name: string;
  total: number;
  taskIds?: string[];
  createdAt: string;
}

export interface BatchProgress {
  batchId: string;
  total: number;
  pending: number;
  queued: number;
  running: number;
  completed: number;
  failed: number;
  cancelled: number;
}

export type ProxyRoutingFallback = 'strict' | 'any_healthy' | 'direct';

export interface ProxyConfig {
  server: string;
  protocol?: string;
  username?: string;
  password?: string;
  geo?: string;
  fallback?: ProxyRoutingFallback | string;
}

export interface LogEntry {
  timestamp: string;
  level: string;
  message: string;
}

export interface NetworkLog {
  taskId: string;
  stepIndex: number;
  requestUrl: string;
  method: string;
  statusCode: number;
  mimeType?: string;
  requestHeaders?: string;
  responseHeaders?: string;
  requestSize: number;
  responseSize: number;
  durationMs: number;
  error?: string;
  timestamp: string;
}

export interface TaskResult {
  taskId: string;
  success: boolean;
  extractedData?: Record<string, string>;
  screenshots?: string[];
  logs: LogEntry[];
  stepLogs?: StepLog[];
  networkLogs?: NetworkLog[];
  duration: number;
  error?: string;
}

export interface TaskLoggingPolicy {
  captureStepLogs?: boolean;
  captureNetworkLogs?: boolean;
  captureScreenshots?: boolean;
  maxExecutionLogs?: number;
}

export interface Task {
  id: string;
  name: string;
  url: string;
  steps: TaskStep[];
  proxy: ProxyConfig;
  priority: number;
  status: TaskStatus;
  retryCount: number;
  maxRetries: number;
  timeout?: number;
  error?: string;
  result?: TaskResult;
  createdAt: string;
  startedAt?: string;
  completedAt?: string;
  tags?: string[];
  batchId?: string;
  flowId?: string;
  headless?: boolean;
  loggingPolicy?: TaskLoggingPolicy;
}

export interface PaginatedTasks {
  tasks: Task[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
}

export interface Proxy {
  id: string;
  server: string;
  protocol: string;
  username?: string;
  password?: string;
  geo?: string;
  status: string;
  latency: number;
  successRate: number;
  totalUsed: number;
  lastChecked?: string;
  createdAt: string;
  localEndpoint?: string;
  localEndpointOn?: boolean;
  localAuthEnabled?: boolean;
  activeLocalUsers?: number;
}

export interface ProxyCountryStats {
  country: string;
  total: number;
  healthy: number;
  activeReservations: number;
  totalUsed: number;
  fallbackAssignments: number;
  activeLocalEndpoints: number;
}

export interface ProxyRoutingPreset {
  id: string;
  name: string;
  randomByCountry: boolean;
  country?: string;
  fallback?: ProxyRoutingFallback | string;
  createdAt: string;
}

export interface LocalProxyGatewayStats {
  activeEndpoints: number;
  endpointCreations: number;
  endpointReuses: number;
  authFailures: number;
  upstreamFailures: number;
  lastError?: string;
}

export type WebSocketEventType = 'created' | 'handshake' | 'frame_sent' | 'frame_received' | 'closed' | 'error';

export interface WebSocketLog {
  flowId: string;
  stepIndex: number;
  requestId: string;
  url: string;
  eventType: WebSocketEventType;
  direction?: string;
  opcode?: number;
  payloadSize: number;
  payloadSnippet?: string;
  closeCode?: number;
  closeReason?: string;
  errorMessage?: string;
  timestamp: string;
}

export interface TaskEvent {
  taskId: string;
  status: TaskStatus;
  error?: string;
  log?: LogEntry;
}

export interface DOMSnapshot {
  id: string;
  flowId: string;
  stepIndex: number;
  html: string;
  screenshotPath: string;
  url: string;
  capturedAt: string;
}

export interface TaskLifecycleEvent {
  id: string;
  taskId: string;
  batchId: string;
  fromState: string;
  toState: string;
  error: string;
  timestamp: string;
}

export interface QueueMetrics {
  running: number;
  queued: number;
  pending: number;
  totalSubmitted: number;
  totalCompleted: number;
  totalFailed: number;
  runningProxied: number;
  proxyConcurrencyLimit: number;
  persistenceQueueDepth: number;
  persistenceQueueCapacity: number;
  persistenceBatchSize: number;
}

export interface StepLog {
  taskId: string;
  stepIndex: number;
  action: string;
  selector?: string;
  value?: string;
  snapshotId?: string;
  errorCode?: string;
  errorMsg?: string;
  durationMs: number;
  startedAt: string;
}

export interface Schedule {
  id: string;
  name: string;
  cronExpr: string;
  flowId: string;
  url: string;
  proxy: ProxyConfig;
  priority: number;
  headless: boolean;
  tags?: string[];
  enabled: boolean;
  lastRunAt?: string;
  nextRunAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface CaptchaConfig {
  id: string;
  provider: string;
  apiKey: string;
  enabled: boolean;
  balance?: number;
  createdAt: string;
  updatedAt: string;
}

export interface VisualBaseline {
  id: string;
  name: string;
  taskId?: string;
  url: string;
  screenshotPath: string;
  width: number;
  height: number;
  createdAt: string;
}

export interface VisualDiff {
  id: string;
  baselineId: string;
  taskId: string;
  screenshotPath: string;
  diffImagePath: string;
  diffPercent: number;
  pixelCount: number;
  threshold: number;
  passed: boolean;
  width: number;
  height: number;
  createdAt: string;
}

export interface DiffRequest {
  baselineId: string;
  taskId: string;
  threshold: number;
}
