package monitoring

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"
)

// StructuredLogger provides enhanced logging with context and levels.
type StructuredLogger struct {
	logger     *slog.Logger
	mu         sync.RWMutex
	logEntries []LogEntry
	maxEntries int
}

// LogEntry represents a single log entry.
type LogEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Context   map[string]string `json:"context,omitempty"`
	TaskID    string            `json:"taskId,omitempty"`
	BatchID   string            `json:"batchId,omitempty"`
	Error     string            `json:"error,omitempty"`
}

// LogLevel defines logging levels.
type LogLevel string

const (
	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
)

// NewStructuredLogger creates a new structured logger.
func NewStructuredLogger(level LogLevel, maxEntries int) *StructuredLogger {
	var slogLevel slog.Level
	
	switch level {
	case LogLevelDebug:
		slogLevel = slog.LevelDebug
	case LogLevelWarning:
		slogLevel = slog.LevelWarn
	case LogLevelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slogLevel,
	})
	
	return &StructuredLogger{
		logger:     slog.New(handler),
		logEntries: make([]LogEntry, 0, maxEntries),
		maxEntries: maxEntries,
	}
}

// Debug logs a debug message.
func (l *StructuredLogger) Debug(ctx context.Context, message string, attrs ...slog.Attr) {
	l.log(ctx, LogLevelDebug, message, attrs...)
}

// Info logs an info message.
func (l *StructuredLogger) Info(ctx context.Context, message string, attrs ...slog.Attr) {
	l.log(ctx, LogLevelInfo, message, attrs...)
}

// Warning logs a warning message.
func (l *StructuredLogger) Warning(ctx context.Context, message string, attrs ...slog.Attr) {
	l.log(ctx, LogLevelWarning, message, attrs...)
}

// Error logs an error message.
func (l *StructuredLogger) Error(ctx context.Context, message string, err error, attrs ...slog.Attr) {
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	l.log(ctx, LogLevelError, message, attrs...)
}

// log is the internal logging method.
func (l *StructuredLogger) log(ctx context.Context, level LogLevel, message string, attrs ...slog.Attr) {
	// Log to slog
	switch level {
	case LogLevelDebug:
		l.logger.LogAttrs(ctx, slog.LevelDebug, message, attrs...)
	case LogLevelInfo:
		l.logger.LogAttrs(ctx, slog.LevelInfo, message, attrs...)
	case LogLevelWarning:
		l.logger.LogAttrs(ctx, slog.LevelWarn, message, attrs...)
	case LogLevelError:
		l.logger.LogAttrs(ctx, slog.LevelError, message, attrs...)
	}
	
	// Store in memory for retrieval
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     string(level),
		Message:   message,
		Context:   make(map[string]string),
	}
	
	// Extract attributes
	for _, attr := range attrs {
		key := attr.Key
		value := attr.Value.String()
		
		switch key {
		case "taskId":
			entry.TaskID = value
		case "batchId":
			entry.BatchID = value
		case "error":
			entry.Error = value
		default:
			entry.Context[key] = value
		}
	}
	
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Add entry and maintain max size
	l.logEntries = append(l.logEntries, entry)
	if len(l.logEntries) > l.maxEntries {
		// Remove oldest entries
		l.logEntries = l.logEntries[len(l.logEntries)-l.maxEntries:]
	}
}

// GetRecentLogs returns recent log entries.
func (l *StructuredLogger) GetRecentLogs(limit int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	if limit <= 0 || limit > len(l.logEntries) {
		limit = len(l.logEntries)
	}
	
	// Return most recent entries
	start := len(l.logEntries) - limit
	if start < 0 {
		start = 0
	}
	
	result := make([]LogEntry, limit)
	copy(result, l.logEntries[start:])
	return result
}

// GetLogsByLevel returns logs filtered by level.
func (l *StructuredLogger) GetLogsByLevel(level LogLevel, limit int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	filtered := make([]LogEntry, 0)
	
	// Filter by level (reverse order to get most recent first)
	for i := len(l.logEntries) - 1; i >= 0 && len(filtered) < limit; i-- {
		if l.logEntries[i].Level == string(level) {
			filtered = append(filtered, l.logEntries[i])
		}
	}
	
	return filtered
}

// GetLogsByTaskID returns logs for a specific task.
func (l *StructuredLogger) GetLogsByTaskID(taskID string, limit int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	filtered := make([]LogEntry, 0)
	
	for i := len(l.logEntries) - 1; i >= 0 && len(filtered) < limit; i-- {
		if l.logEntries[i].TaskID == taskID {
			filtered = append(filtered, l.logEntries[i])
		}
	}
	
	return filtered
}

// GetLogsByBatchID returns logs for a specific batch.
func (l *StructuredLogger) GetLogsByBatchID(batchID string, limit int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	filtered := make([]LogEntry, 0)
	
	for i := len(l.logEntries) - 1; i >= 0 && len(filtered) < limit; i-- {
		if l.logEntries[i].BatchID == batchID {
			filtered = append(filtered, l.logEntries[i])
		}
	}
	
	return filtered
}

// Clear clears all log entries from memory.
func (l *StructuredLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logEntries = make([]LogEntry, 0, l.maxEntries)
}

// LogStats returns statistics about stored logs.
type LogStats struct {
	TotalEntries int            `json:"totalEntries"`
	ByLevel      map[string]int `json:"byLevel"`
	OldestEntry  time.Time      `json:"oldestEntry,omitempty"`
	NewestEntry  time.Time      `json:"newestEntry,omitempty"`
}

// GetLogStats returns statistics about logs.
func (l *StructuredLogger) GetLogStats() LogStats {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	stats := LogStats{
		TotalEntries: len(l.logEntries),
		ByLevel:      make(map[string]int),
	}
	
	if len(l.logEntries) > 0 {
		stats.OldestEntry = l.logEntries[0].Timestamp
		stats.NewestEntry = l.logEntries[len(l.logEntries)-1].Timestamp
		
		for _, entry := range l.logEntries {
			stats.ByLevel[entry.Level]++
		}
	}
	
	return stats
}

// WithTaskID returns a logger with task ID context.
func (l *StructuredLogger) WithTaskID(taskID string) *ContextLogger {
	return &ContextLogger{
		logger: l,
		attrs:  []slog.Attr{slog.String("taskId", taskID)},
	}
}

// WithBatchID returns a logger with batch ID context.
func (l *StructuredLogger) WithBatchID(batchID string) *ContextLogger {
	return &ContextLogger{
		logger: l,
		attrs:  []slog.Attr{slog.String("batchId", batchID)},
	}
}

// ContextLogger wraps a logger with contextual attributes.
type ContextLogger struct {
	logger *StructuredLogger
	attrs  []slog.Attr
}

// Debug logs a debug message with context.
func (c *ContextLogger) Debug(ctx context.Context, message string, attrs ...slog.Attr) {
	c.logger.Debug(ctx, message, append(c.attrs, attrs...)...)
}

// Info logs an info message with context.
func (c *ContextLogger) Info(ctx context.Context, message string, attrs ...slog.Attr) {
	c.logger.Info(ctx, message, append(c.attrs, attrs...)...)
}

// Warning logs a warning message with context.
func (c *ContextLogger) Warning(ctx context.Context, message string, attrs ...slog.Attr) {
	c.logger.Warning(ctx, message, append(c.attrs, attrs...)...)
}

// Error logs an error message with context.
func (c *ContextLogger) Error(ctx context.Context, message string, err error, attrs ...slog.Attr) {
	c.logger.Error(ctx, message, err, append(c.attrs, attrs...)...)
}

// With adds additional context attributes.
func (c *ContextLogger) With(attrs ...slog.Attr) *ContextLogger {
	return &ContextLogger{
		logger: c.logger,
		attrs:  append(c.attrs, attrs...),
	}
}
