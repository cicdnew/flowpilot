export namespace database {
	
	export class TaskUpdateParams {
	    Name: string;
	    URL: string;
	    Steps: models.TaskStep[];
	    ProxyConfig: models.ProxyConfig;
	    Priority: number;
	    Tags: string[];
	    Timeout: number;
	    LoggingPolicy?: models.TaskLoggingPolicy;
	
	    static createFrom(source: any = {}) {
	        return new TaskUpdateParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.URL = source["URL"];
	        this.Steps = this.convertValues(source["Steps"], models.TaskStep);
	        this.ProxyConfig = this.convertValues(source["ProxyConfig"], models.ProxyConfig);
	        this.Priority = source["Priority"];
	        this.Tags = source["Tags"];
	        this.Timeout = source["Timeout"];
	        this.LoggingPolicy = this.convertValues(source["LoggingPolicy"], models.TaskLoggingPolicy);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}

}

export namespace main {
	
	export class CreateTaskFromFlowParams {
	    FlowID: string;
	    Name: string;
	    URL: string;
	    ProxyConfig: models.ProxyConfig;
	    Priority: number;
	    AutoStart: boolean;
	    Tags: string[];
	
	    static createFrom(source: any = {}) {
	        return new CreateTaskFromFlowParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.FlowID = source["FlowID"];
	        this.Name = source["Name"];
	        this.URL = source["URL"];
	        this.ProxyConfig = this.convertValues(source["ProxyConfig"], models.ProxyConfig);
	        this.Priority = source["Priority"];
	        this.AutoStart = source["AutoStart"];
	        this.Tags = source["Tags"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}
	export class CreateTaskParams {
	    Name: string;
	    URL: string;
	    Steps: models.TaskStep[];
	    ProxyConfig: models.ProxyConfig;
	    Priority: number;
	    AutoStart: boolean;
	    Tags: string[];
	    Timeout: number;
	    LoggingPolicy?: models.TaskLoggingPolicy;
	
	    static createFrom(source: any = {}) {
	        return new CreateTaskParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.URL = source["URL"];
	        this.Steps = this.convertValues(source["Steps"], models.TaskStep);
	        this.ProxyConfig = this.convertValues(source["ProxyConfig"], models.ProxyConfig);
	        this.Priority = source["Priority"];
	        this.AutoStart = source["AutoStart"];
	        this.Tags = source["Tags"];
	        this.Timeout = source["Timeout"];
	        this.LoggingPolicy = this.convertValues(source["LoggingPolicy"], models.TaskLoggingPolicy);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}
	export class FlowImportResult {
	    tasks: models.Task[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new FlowImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tasks = this.convertValues(source["tasks"], models.Task);
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}
	export class ScheduleParams {
	    Name: string;
	    CronExpr: string;
	    FlowID: string;
	    URL: string;
	    ProxyConfig: models.ProxyConfig;
	    Priority: number;
	    Headless: boolean;
	    Tags: string[];
	
	    static createFrom(source: any = {}) {
	        return new ScheduleParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.CronExpr = source["CronExpr"];
	        this.FlowID = source["FlowID"];
	        this.URL = source["URL"];
	        this.ProxyConfig = this.convertValues(source["ProxyConfig"], models.ProxyConfig);
	        this.Priority = source["Priority"];
	        this.Headless = source["Headless"];
	        this.Tags = source["Tags"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}

}

export namespace models {
	
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
	    timeout?: number;
	    loggingPolicy?: TaskLoggingPolicy;
	
	    static createFrom(source: any = {}) {
	        return new AdvancedBatchInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.flowId = source["flowId"];
	        this.urls = source["urls"];
	        this.namingTemplate = source["namingTemplate"];
	        this.priority = source["priority"];
	        this.proxy = this.convertValues(source["proxy"], ProxyConfig);
	        this.tags = source["tags"];
	        this.proxyCountry = source["proxyCountry"];
	        this.proxyFallback = source["proxyFallback"];
	        this.autoStart = source["autoStart"];
	        this.headless = source["headless"];
	        this.timeout = source["timeout"];
	        this.loggingPolicy = this.convertValues(source["loggingPolicy"], TaskLoggingPolicy);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}
	export class AlertFiring {
	    id: string;
	    rule_id: string;
	    rule_name: string;
	    severity: string;
	    current_value: number;
	    threshold: number;
	    // Go type: time
	    fired_at: any;
	    // Go type: time
	    resolved_at?: any;
	    notified: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AlertFiring(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.rule_id = source["rule_id"];
	        this.rule_name = source["rule_name"];
	        this.severity = source["severity"];
	        this.current_value = source["current_value"];
	        this.threshold = source["threshold"];
	        this.fired_at = this.convertValues(source["fired_at"], null);
	        this.resolved_at = this.convertValues(source["resolved_at"], null);
	        this.notified = source["notified"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}
	export class AlertRule {
	    id: string;
	    name: string;
	    description: string;
	    metric: string;
	    condition: string;
	    threshold: number;
	    window_secs: number;
	    cooldown: number;
	    severity: string;
	    enabled: boolean;
	    webhook_url?: string;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new AlertRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.metric = source["metric"];
	        this.condition = source["condition"];
	        this.threshold = source["threshold"];
	        this.window_secs = source["window_secs"];
	        this.cooldown = source["cooldown"];
	        this.severity = source["severity"];
	        this.enabled = source["enabled"];
	        this.webhook_url = source["webhook_url"];
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}
	export class BatchGroup {
	    id: string;
	    flowId: string;
	    taskIds: string[];
	    total: number;
	    name: string;
	    // Go type: time
	    createdAt: any;
	
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
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	    timeout?: number;
	    tags?: string[];
	    loggingPolicy?: TaskLoggingPolicy;
	    headless: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BatchTaskInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.steps = this.convertValues(source["steps"], TaskStep);
	        this.proxy = this.convertValues(source["proxy"], ProxyConfig);
	        this.priority = source["priority"];
	        this.timeout = source["timeout"];
	        this.tags = source["tags"];
	        this.loggingPolicy = this.convertValues(source["loggingPolicy"], TaskLoggingPolicy);
	        this.headless = source["headless"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        this.capturedAt = this.convertValues(source["capturedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.level = source["level"];
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        this.timestamp = this.convertValues(source["timestamp"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        this.startedAt = this.convertValues(source["startedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        this.logs = this.convertValues(source["logs"], LogEntry);
	        this.stepLogs = this.convertValues(source["stepLogs"], StepLog);
	        this.networkLogs = this.convertValues(source["networkLogs"], NetworkLog);
	        this.duration = source["duration"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	    webhookUrl?: string;
	    webhookEvents?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Task(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.steps = this.convertValues(source["steps"], TaskStep);
	        this.proxy = this.convertValues(source["proxy"], ProxyConfig);
	        this.priority = source["priority"];
	        this.status = source["status"];
	        this.retryCount = source["retryCount"];
	        this.maxRetries = source["maxRetries"];
	        this.timeout = source["timeout"];
	        this.error = source["error"];
	        this.result = this.convertValues(source["result"], TaskResult);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.completedAt = this.convertValues(source["completedAt"], null);
	        this.tags = source["tags"];
	        this.batchId = source["batchId"];
	        this.flowId = source["flowId"];
	        this.headless = source["headless"];
	        this.loggingPolicy = this.convertValues(source["loggingPolicy"], TaskLoggingPolicy);
	        this.webhookUrl = source["webhookUrl"];
	        this.webhookEvents = source["webhookEvents"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        this.tasks = this.convertValues(source["tasks"], Task);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	    maxRequestsPerMinute?: number;
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
	        this.maxRequestsPerMinute = source["maxRequestsPerMinute"];
	        this.lastChecked = this.convertValues(source["lastChecked"], null);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.localEndpoint = source["localEndpoint"];
	        this.localEndpointOn = source["localEndpointOn"];
	        this.localAuthEnabled = source["localAuthEnabled"];
	        this.activeLocalUsers = source["activeLocalUsers"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}
	export class QueueMetrics {
	    running: number;
	    queued: number;
	    pending: number;
	    totalSubmitted: number;
	    totalCompleted: number;
	    totalFailed: number;
	    totalRetried: number;
	    runningProxied: number;
	    proxyConcurrencyLimit: number;
	    persistenceQueueDepth: number;
	    persistenceQueueCapacity: number;
	    persistenceBatchSize: number;
	    workerUtilizationPercent: number;
	    avgStepDurationMs: number;
	    // Go type: time
	    lastUpdated: any;
	
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
	        this.totalRetried = source["totalRetried"];
	        this.runningProxied = source["runningProxied"];
	        this.proxyConcurrencyLimit = source["proxyConcurrencyLimit"];
	        this.persistenceQueueDepth = source["persistenceQueueDepth"];
	        this.persistenceQueueCapacity = source["persistenceQueueCapacity"];
	        this.persistenceBatchSize = source["persistenceBatchSize"];
	        this.workerUtilizationPercent = source["workerUtilizationPercent"];
	        this.avgStepDurationMs = source["avgStepDurationMs"];
	        this.lastUpdated = this.convertValues(source["lastUpdated"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        this.selectorCandidates = this.convertValues(source["selectorCandidates"], SelectorCandidate);
	        this.timestamp = this.convertValues(source["timestamp"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}
	export class RecordedFlow {
	    id: string;
	    name: string;
	    description?: string;
	    steps: RecordedStep[];
	    originUrl: string;
	    timeout?: number;
	    loggingPolicy?: TaskLoggingPolicy;
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
	        this.steps = this.convertValues(source["steps"], RecordedStep);
	        this.originUrl = source["originUrl"];
	        this.timeout = source["timeout"];
	        this.loggingPolicy = this.convertValues(source["loggingPolicy"], TaskLoggingPolicy);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}
	
	export class RepeatConfig {
	    mode: string;
	    varName: string;
	    startVal: number;
	    endVal: number;
	    step: number;
	    values: string[];
	    batchSize: number;
	
	    static createFrom(source: any = {}) {
	        return new RepeatConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.varName = source["varName"];
	        this.startVal = source["startVal"];
	        this.endVal = source["endVal"];
	        this.step = source["step"];
	        this.values = source["values"];
	        this.batchSize = source["batchSize"];
	    }
	}
	export class RepeatTaskInput {
	    name: string;
	    url: string;
	    steps: TaskStep[];
	    repeat: RepeatConfig;
	    proxy: ProxyConfig;
	    priority: number;
	    autoStart: boolean;
	    tags?: string[];
	    timeout?: number;
	    loggingPolicy?: TaskLoggingPolicy;
	    headless?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RepeatTaskInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.steps = this.convertValues(source["steps"], TaskStep);
	        this.repeat = this.convertValues(source["repeat"], RepeatConfig);
	        this.proxy = this.convertValues(source["proxy"], ProxyConfig);
	        this.priority = source["priority"];
	        this.autoStart = source["autoStart"];
	        this.tags = source["tags"];
	        this.timeout = source["timeout"];
	        this.loggingPolicy = this.convertValues(source["loggingPolicy"], TaskLoggingPolicy);
	        this.headless = source["headless"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        this.proxy = this.convertValues(source["proxy"], ProxyConfig);
	        this.priority = source["priority"];
	        this.headless = source["headless"];
	        this.tags = source["tags"];
	        this.enabled = source["enabled"];
	        this.lastRunAt = this.convertValues(source["lastRunAt"], null);
	        this.nextRunAt = this.convertValues(source["nextRunAt"], null);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        this.timestamp = this.convertValues(source["timestamp"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        this.timestamp = this.convertValues(source["timestamp"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}

}

export namespace monitoring {
	
	export class Component {
	    status: string;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new Component(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class SystemMetrics {
	    uptimeSeconds: number;
	    // Go type: time
	    startTime: any;
	    totalRequests: number;
	    totalErrors: number;
	    memoryUsageMb: number;
	    goroutineCount: number;
	    databaseConnections: number;
	    requestsPerMinute: number;
	    errorsPerMinute: number;
	    tasksPerMinute: number;
	
	    static createFrom(source: any = {}) {
	        return new SystemMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uptimeSeconds = source["uptimeSeconds"];
	        this.startTime = this.convertValues(source["startTime"], null);
	        this.totalRequests = source["totalRequests"];
	        this.totalErrors = source["totalErrors"];
	        this.memoryUsageMb = source["memoryUsageMb"];
	        this.goroutineCount = source["goroutineCount"];
	        this.databaseConnections = source["databaseConnections"];
	        this.requestsPerMinute = source["requestsPerMinute"];
	        this.errorsPerMinute = source["errorsPerMinute"];
	        this.tasksPerMinute = source["tasksPerMinute"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}
	export class RepeatTaskMetrics {
	    totalRepeatedBatches: number;
	    totalRepeatedTasks: number;
	    activeRepeatedBatches: number;
	    completedRepeatedTasks: number;
	    failedRepeatedTasks: number;
	    avgTasksPerBatch: number;
	    avgBatchCompletionTimeMs: number;
	    // Go type: time
	    lastBatchCreatedAt: any;
	    lastBatchId: string;
	    counterModeBatches: number;
	    rangeModeBatches: number;
	    listModeBatches: number;
	
	    static createFrom(source: any = {}) {
	        return new RepeatTaskMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalRepeatedBatches = source["totalRepeatedBatches"];
	        this.totalRepeatedTasks = source["totalRepeatedTasks"];
	        this.activeRepeatedBatches = source["activeRepeatedBatches"];
	        this.completedRepeatedTasks = source["completedRepeatedTasks"];
	        this.failedRepeatedTasks = source["failedRepeatedTasks"];
	        this.avgTasksPerBatch = source["avgTasksPerBatch"];
	        this.avgBatchCompletionTimeMs = source["avgBatchCompletionTimeMs"];
	        this.lastBatchCreatedAt = this.convertValues(source["lastBatchCreatedAt"], null);
	        this.lastBatchId = source["lastBatchId"];
	        this.counterModeBatches = source["counterModeBatches"];
	        this.rangeModeBatches = source["rangeModeBatches"];
	        this.listModeBatches = source["listModeBatches"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}
	export class HealthStatus {
	    status: string;
	    // Go type: time
	    timestamp: any;
	    uptime: number;
	    components: Record<string, Component>;
	    repeatTaskMetrics: RepeatTaskMetrics;
	    systemMetrics: SystemMetrics;
	
	    static createFrom(source: any = {}) {
	        return new HealthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.uptime = source["uptime"];
	        this.components = this.convertValues(source["components"], Component, true);
	        this.repeatTaskMetrics = this.convertValues(source["repeatTaskMetrics"], RepeatTaskMetrics);
	        this.systemMetrics = this.convertValues(source["systemMetrics"], SystemMetrics);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}
	export class LogEntry {
	    // Go type: time
	    timestamp: any;
	    level: string;
	    message: string;
	    context?: Record<string, string>;
	    taskId?: string;
	    batchId?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.level = source["level"];
	        this.message = source["message"];
	        this.context = source["context"];
	        this.taskId = source["taskId"];
	        this.batchId = source["batchId"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}
	export class LogStats {
	    totalEntries: number;
	    byLevel: Record<string, number>;
	    // Go type: time
	    oldestEntry?: any;
	    // Go type: time
	    newestEntry?: any;
	
	    static createFrom(source: any = {}) {
	        return new LogStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalEntries = source["totalEntries"];
	        this.byLevel = source["byLevel"];
	        this.oldestEntry = this.convertValues(source["oldestEntry"], null);
	        this.newestEntry = this.convertValues(source["newestEntry"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	}
	

}

