package database

// Error message constants to avoid duplication (S1192)
const (
	// Task-related errors
	errTaskNotFound = "task %s not found"
	errTaskID       = "task_id must be a non-empty string"

	// Schedule-related errors
	errScheduleNotFound = "schedule %s not found"

	// Proxy-related errors
	errIterateProxies         = "iterate proxies: %w"
	errUpdateProxyHealth      = "update proxy %s health: %v"
	errProxyDecrypt           = "decrypt proxy: %w"
	errProxyPassDecrypt       = "decrypt proxy password: %w"
	errProxyUsernameDecrypt   = "decrypt proxy username: %w"

	// Captcha-related errors
	errCaptchaConfigNotFound = "captcha config %s not found"
	errTestCaptchaConfig     = "test captcha config: %w"
	errSaveCaptchaConfig     = "save captcha config: %w"

	// Task update errors
	errUpdateTask      = "update task: %w"
	errScanTaskRow     = "scan task row: %w"
	errUpdateStatus    = "update status: %v"
	errCheckUpdateTask = "check update result for task %s: %w"

	// Tag-related errors
	errTag = "tag %d: %w"

	// HTTP-related errors
	errHTTPResponse = "HTTP %d: %s — %s"

	// Webhook-related errors
	errInsertTaskEvent = "insert task event %s: %w"

	// Auth-related constants
	authBearer = "Bearer "
)

// Metric-related constants
const (
	newMetricFmt = "New: %v"
)

// Flow-related constants
const (
	errFlowNotFound = "flow %s not found"
)

// Browser step-related constants
const (
	errElementNotFoundForSelector = "click_ad: element not found for selector %q"
)
