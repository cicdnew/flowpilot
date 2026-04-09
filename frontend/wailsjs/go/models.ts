export namespace models {

	export function convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice && a.map) {
	        return (a as any[]).map(elem => convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}

	
	export class ProxyConfig {
	    server: string;
	    protocol?: string;
	    username?: string;
	    password?: string;
	    geo?: string;
	    fallback?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server = source["server"];
	        this.protocol = source["protocol"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.geo = source["geo"];
	        this.fallback = source["fallback"];
	    }
	}
	export class AdvancedBatchInput {
	    flowId: string;
	    urls: string[];
	    namingTemplate: string;
	    priority: number;
	    proxy: ProxyConfig;
	    tags?: string[];
	    proxyCountry?: string;
	    proxyFallback?: string;
	    autoStart: boolean;
	    headless?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AdvancedBatchInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.flowId = source["flowId"];
	        this.urls = source["urls"];
	        this.namingTemplate = source["namingTemplate"];
	        this.priority = source["priority"];
	        this.proxy = convertValues(source["proxy"], ProxyConfig);
	        this.tags = source["tags"];
	        this.proxyCountry = source["proxyCountry"];
	        this.proxyFallback = source["proxyFallback"];
	        this.autoStart = source["autoStart"];
	        this.headless = source["headless"];
	    }
	
	}
	export class BatchGroup {
	    id: string;
	    flowId: string;
	    taskIds: string[];
	    total: number;
	    name: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new BatchGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.flowId = source["flowId"];
	        this.taskIds = source["taskIds"];
	        this.total = source["total"];
	        this.name = source["name"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class BatchProgress {
	    batchId: string;
	    total: number;
	    pending: number;
	    queued: number;
	    running: number;
	    completed: number;
	    failed: number;
	    cancelled: number;
	
	    static createFrom(source: any = {}) {
	        return new BatchProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.batchId = source["batchId"];
	        this.total = source["total"];
	        this.pending = source["pending"];
	        this.queued = source["queued"];
	        this.running = source["running"];
	        this.completed = source["completed"];
	        this.failed = source["failed"];
	        this.cancelled = source["cancelled"];
	    }
	}
	export class TaskStep {
	    action: string;
	    selector?: string;
	    value?: string;
	    timeout?: number;
	    condition?: string;
	    label?: string;
	    jumpTo?: string;
	    varName?: string;
	    operator?: string;
	    maxLoops?: number;
	    target?: string;
	    source?: string;
	    keys?: string;
	    duration?: number;
	    domain?: string;
	    name?: string;
	    path?: string;
	    data?: string;
	    strategy?: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.selector = source["selector"];
	        this.value = source["value"];
	        this.timeout = source["timeout"];
	        this.condition = source["condition"];
	        this.label = source["label"];
	        this.jumpTo = source["jumpTo"];
	        this.varName = source["varName"];
	        this.operator = source["operator"];
	        this.maxLoops = source["maxLoops"];
	        this.target = source["target"];
	        this.source = source["source"];
	        this.keys = source["keys"];
	        this.duration = source["duration"];
	        this.domain = source["domain"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.data = source["data"];
	        this.strategy = source["strategy"];
	    }
	}
	export class BatchTaskInput {
	    name: string;
	    url: string;
	    steps: TaskStep[];
	    proxy: ProxyConfig;
	    priority: number;
	
	    static createFrom(source: any = {}) {
	        return new BatchTaskInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.steps = convertValues(source["steps"], TaskStep);
	        this.proxy = convertValues(source["proxy"], ProxyConfig);
	        this.priority = source["priority"];
	    }
	
	}
	export class CaptchaConfig {
	    id: string;
	    provider: string;
	    apiKey: string;
	    enabled: boolean;
	    balance?: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new CaptchaConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.provider = source["provider"];
	        this.apiKey = source["apiKey"];
	        this.enabled = source["enabled"];
	        this.balance = source["balance"];
	        this.createdAt = convertValues(source["createdAt"], null);
	        this.updatedAt = convertValues(source["updatedAt"], null);
	    }
	
	}
	export class DOMSnapshot {
	    id: string;
	    flowId: string;
	    stepIndex: number;
	    html: string;
	    screenshotPath: string;
	    url: string;
	    // Go type: time
	    capturedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new DOMSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.flowId = source["flowId"];
	        this.stepIndex = source["stepIndex"];
	        this.html = source["html"];
	        this.screenshotPath = source["screenshotPath"];
	        this.url = source["url"];
	        this.capturedAt = convertValues(source["capturedAt"], null);
	    }
	
	}
	export class DiffRequest {
	    baselineId: string;
	    taskId: string;
	    threshold: number;
	
	    static createFrom(source: any = {}) {
	        return new DiffRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baselineId = source["baselineId"];
	        this.taskId = source["taskId"];
	        this.threshold = source["threshold"];
	    }
	}
	export class LocalProxyGatewayStats {
	    activeEndpoints: number;
	    endpointCreations: number;
	    endpointReuses: number;
	    authFailures: number;
	    upstreamFailures: number;
	    lastError?: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalProxyGatewayStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.activeEndpoints = source["activeEndpoints"];
	        this.endpointCreations = source["endpointCreations"];
	        this.endpointReuses = source["endpointReuses"];
	        this.authFailures = source["authFailures"];
	        this.upstreamFailures = source["upstreamFailures"];
	        this.lastError = source["lastError"];
	    }
	}
	export class LogEntry {
	    // Go type: time
	    timestamp: any;
	    level: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = convertValues(source["timestamp"], null);
	        this.level = source["level"];
	        this.message = source["message"];
	    }
	
	}
	export class NetworkLog {
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
	    // Go type: time
	    timestamp: any;
	
	    static createFrom(source: any = {}) {
	        return new NetworkLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.taskId = source["taskId"];
	        this.stepIndex = source["stepIndex"];
	        this.requestUrl = source["requestUrl"];
	        this.method = source["method"];
	        this.statusCode = source["statusCode"];
	        this.mimeType = source["mimeType"];
	        this.requestHeaders = source["requestHeaders"];
	        this.responseHeaders = source["responseHeaders"];
	        this.requestSize = source["requestSize"];
	        this.responseSize = source["responseSize"];
	        this.durationMs = source["durationMs"];
	        this.error = source["error"];
	        this.timestamp = convertValues(source["timestamp"], null);
	    }
	
	}
	export class TaskLoggingPolicy {
	    captureStepLogs?: boolean;
	    captureNetworkLogs?: boolean;
	    captureScreenshots?: boolean;
	    maxExecutionLogs?: number;
	
	    static createFrom(source: any = {}) {
	        return new TaskLoggingPolicy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.captureStepLogs = source["captureStepLogs"];
	        this.captureNetworkLogs = source["captureNetworkLogs"];
	        this.captureScreenshots = source["captureScreenshots"];
	        this.maxExecutionLogs = source["maxExecutionLogs"];
	    }
	}
	export class StepLog {
	    taskId: string;
	    stepIndex: number;
	    action: string;
	    selector?: string;
	    value?: string;
	    snapshotId?: string;
	    errorCode?: string;
	    errorMsg?: string;
	    durationMs: number;
	    // Go type: time
	    startedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new StepLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.taskId = source["taskId"];
	        this.stepIndex = source["stepIndex"];
	        this.action = source["action"];
	        this.selector = source["selector"];
	        this.value = source["value"];
	        this.snapshotId = source["snapshotId"];
	        this.errorCode = source["errorCode"];
	        this.errorMsg = source["errorMsg"];
	        this.durationMs = source["durationMs"];
	        this.startedAt = convertValues(source["startedAt"], null);
	    }
	
	}
	export class TaskResult {
	    taskId: string;
	    success: boolean;
	    extractedData?: Record<string, string>;
	    screenshots?: string[];
	    logs: LogEntry[];
	    stepLogs?: StepLog[];
	    networkLogs?: NetworkLog[];
	    duration: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.taskId = source["taskId"];
	        this.success = source["success"];
	        this.extractedData = source["extractedData"];
	        this.screenshots = source["screenshots"];
	        this.logs = convertValues(source["logs"], LogEntry);
	        this.stepLogs = convertValues(source["stepLogs"], StepLog);
	        this.networkLogs = convertValues(source["networkLogs"], NetworkLog);
	        this.duration = source["duration"];
	        this.error = source["error"];
	    }
	
	}
	export class Task {
	    id: string;
	    name: string;
	    url: string;
	    steps: TaskStep[];
	    proxy: ProxyConfig;
	    priority: number;
	    status: string;
	    retryCount: number;
	    maxRetries: number;
	    timeout?: number;
	    error?: string;
	    result?: TaskResult;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    startedAt?: any;
	    // Go type: time
	    completedAt?: any;
	    tags?: string[];
	    batchId?: string;
	    flowId?: string;
	    headless: boolean;
	    loggingPolicy?: TaskLoggingPolicy;
	
	    static createFrom(source: any = {}) {
	        return new Task(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.steps = convertValues(source["steps"], TaskStep);
	        this.proxy = convertValues(source["proxy"], ProxyConfig);
	        this.priority = source["priority"];
	        this.status = source["status"];
	        this.retryCount = source["retryCount"];
	        this.maxRetries = source["maxRetries"];
	        this.timeout = source["timeout"];
	        this.error = source["error"];
	        this.result = convertValues(source["result"], TaskResult);
	        this.createdAt = convertValues(source["createdAt"], null);
	        this.startedAt = convertValues(source["startedAt"], null);
	        this.completedAt = convertValues(source["completedAt"], null);
	        this.tags = source["tags"];
	        this.batchId = source["batchId"];
	        this.flowId = source["flowId"];
	        this.headless = source["headless"];
	        this.loggingPolicy = convertValues(source["loggingPolicy"], TaskLoggingPolicy);
	    }
	
	}
	export class PaginatedTasks {
	    tasks: Task[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new PaginatedTasks(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tasks = convertValues(source["tasks"], Task);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
	}
	export class Proxy {
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
	    // Go type: time
	    lastChecked?: any;
	    // Go type: time
	    createdAt: any;
	    localEndpoint?: string;
	    localEndpointOn?: boolean;
	    localAuthEnabled?: boolean;
	    activeLocalUsers?: number;
	
	    static createFrom(source: any = {}) {
	        return new Proxy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.server = source["server"];
	        this.protocol = source["protocol"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.geo = source["geo"];
	        this.status = source["status"];
	        this.latency = source["latency"];
	        this.successRate = source["successRate"];
	        this.totalUsed = source["totalUsed"];
	        this.lastChecked = convertValues(source["lastChecked"], null);
	        this.createdAt = convertValues(source["createdAt"], null);
	        this.localEndpoint = source["localEndpoint"];
	        this.localEndpointOn = source["localEndpointOn"];
	        this.localAuthEnabled = source["localAuthEnabled"];
	        this.activeLocalUsers = source["activeLocalUsers"];
	    }
	
	}
	
	export class ProxyCountryStats {
	    country: string;
	    total: number;
	    healthy: number;
	    activeReservations: number;
	    totalUsed: number;
	    fallbackAssignments: number;
	    activeLocalEndpoints: number;
	
	    static createFrom(source: any = {}) {
	        return new ProxyCountryStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.country = source["country"];
	        this.total = source["total"];
	        this.healthy = source["healthy"];
	        this.activeReservations = source["activeReservations"];
	        this.totalUsed = source["totalUsed"];
	        this.fallbackAssignments = source["fallbackAssignments"];
	        this.activeLocalEndpoints = source["activeLocalEndpoints"];
	    }
	}
	export class ProxyRoutingPreset {
	    id: string;
	    name: string;
	    randomByCountry: boolean;
	    country?: string;
	    fallback?: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new ProxyRoutingPreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.randomByCountry = source["randomByCountry"];
	        this.country = source["country"];
	        this.fallback = source["fallback"];
	        this.createdAt = convertValues(source["createdAt"], null);
	    }
	
	}
	export class QueueMetrics {
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
	
	    static createFrom(source: any = {}) {
	        return new QueueMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.queued = source["queued"];
	        this.pending = source["pending"];
	        this.totalSubmitted = source["totalSubmitted"];
	        this.totalCompleted = source["totalCompleted"];
	        this.totalFailed = source["totalFailed"];
	        this.runningProxied = source["runningProxied"];
	        this.proxyConcurrencyLimit = source["proxyConcurrencyLimit"];
	        this.persistenceQueueDepth = source["persistenceQueueDepth"];
	        this.persistenceQueueCapacity = source["persistenceQueueCapacity"];
	        this.persistenceBatchSize = source["persistenceBatchSize"];
	    }
	}
	export class SelectorCandidate {
	    selector: string;
	    strategy: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new SelectorCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.selector = source["selector"];
	        this.strategy = source["strategy"];
	        this.score = source["score"];
	    }
	}
	export class RecordedStep {
	    index: number;
	    action: string;
	    selector?: string;
	    value?: string;
	    timeout?: number;
	    snapshotId?: string;
	    selectorCandidates?: SelectorCandidate[];
	    // Go type: time
	    timestamp: any;
	
	    static createFrom(source: any = {}) {
	        return new RecordedStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.action = source["action"];
	        this.selector = source["selector"];
	        this.value = source["value"];
	        this.timeout = source["timeout"];
	        this.snapshotId = source["snapshotId"];
	        this.selectorCandidates = convertValues(source["selectorCandidates"], SelectorCandidate);
	        this.timestamp = convertValues(source["timestamp"], null);
	    }
	
	}
	export class RecordedFlow {
	    id: string;
	    name: string;
	    description?: string;
	    steps: RecordedStep[];
	    originUrl: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new RecordedFlow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.steps = convertValues(source["steps"], RecordedStep);
	        this.originUrl = source["originUrl"];
	        this.createdAt = convertValues(source["createdAt"], null);
	        this.updatedAt = convertValues(source["updatedAt"], null);
	    }
	
	}
	
	export class Schedule {
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
	    // Go type: time
	    lastRunAt?: any;
	    // Go type: time
	    nextRunAt?: any;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Schedule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.cronExpr = source["cronExpr"];
	        this.flowId = source["flowId"];
	        this.url = source["url"];
	        this.proxy = convertValues(source["proxy"], ProxyConfig);
	        this.priority = source["priority"];
	        this.headless = source["headless"];
	        this.tags = source["tags"];
	        this.enabled = source["enabled"];
	        this.lastRunAt = convertValues(source["lastRunAt"], null);
	        this.nextRunAt = convertValues(source["nextRunAt"], null);
	        this.createdAt = convertValues(source["createdAt"], null);
	        this.updatedAt = convertValues(source["updatedAt"], null);
	    }
	
	}
	
	
	
	export class TaskLifecycleEvent {
	    id: string;
	    taskId: string;
	    batchId?: string;
	    fromState: string;
	    toState: string;
	    error?: string;
	    // Go type: time
	    timestamp: any;
	
	    static createFrom(source: any = {}) {
	        return new TaskLifecycleEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.taskId = source["taskId"];
	        this.batchId = source["batchId"];
	        this.fromState = source["fromState"];
	        this.toState = source["toState"];
	        this.error = source["error"];
	        this.timestamp = convertValues(source["timestamp"], null);
	    }
	
	}
	
	
	
	export class VisualBaseline {
	    id: string;
	    name: string;
	    taskId?: string;
	    url: string;
	    screenshotPath: string;
	    width: number;
	    height: number;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new VisualBaseline(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.taskId = source["taskId"];
	        this.url = source["url"];
	        this.screenshotPath = source["screenshotPath"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.createdAt = convertValues(source["createdAt"], null);
	    }
	
	}
	export class VisualDiff {
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
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new VisualDiff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.baselineId = source["baselineId"];
	        this.taskId = source["taskId"];
	        this.screenshotPath = source["screenshotPath"];
	        this.diffImagePath = source["diffImagePath"];
	        this.diffPercent = source["diffPercent"];
	        this.pixelCount = source["pixelCount"];
	        this.threshold = source["threshold"];
	        this.passed = source["passed"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.createdAt = convertValues(source["createdAt"], null);
	    }
	
	}
	export class WebSocketLog {
	    flowId: string;
	    stepIndex: number;
	    requestId: string;
	    url: string;
	    eventType: string;
	    direction?: string;
	    opcode?: number;
	    payloadSize: number;
	    payloadSnippet?: string;
	    closeCode?: number;
	    closeReason?: string;
	    errorMessage?: string;
	    // Go type: time
	    timestamp: any;
	
	    static createFrom(source: any = {}) {
	        return new WebSocketLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.flowId = source["flowId"];
	        this.stepIndex = source["stepIndex"];
	        this.requestId = source["requestId"];
	        this.url = source["url"];
	        this.eventType = source["eventType"];
	        this.direction = source["direction"];
	        this.opcode = source["opcode"];
	        this.payloadSize = source["payloadSize"];
	        this.payloadSnippet = source["payloadSnippet"];
	        this.closeCode = source["closeCode"];
	        this.closeReason = source["closeReason"];
	        this.errorMessage = source["errorMessage"];
	        this.timestamp = convertValues(source["timestamp"], null);
	    }
	
	}

}

