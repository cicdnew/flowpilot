export type TaskStatus = 'pending' | 'queued' | 'running' | 'completed' | 'failed' | 'cancelled' | 'retrying';

export interface TaskStep {
  action: string;
  selector?: string;
  value?: string;
  timeout?: number;
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

export interface ProxyConfig {
  server: string;
  protocol?: string;
  username?: string;
  password?: string;
  geo?: string;
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
