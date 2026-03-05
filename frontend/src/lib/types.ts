export type TaskStatus = 'pending' | 'queued' | 'running' | 'completed' | 'failed' | 'cancelled' | 'retrying';

export interface TaskStep {
  action: string;
  selector?: string;
  value?: string;
  timeout?: number;
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

export interface TaskResult {
  taskId: string;
  success: boolean;
  extractedData?: Record<string, string>;
  screenshots?: string[];
  logs: LogEntry[];
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

export interface TaskEvent {
  taskId: string;
  status: TaskStatus;
  error?: string;
  log?: LogEntry;
}
