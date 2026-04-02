package logs

import (
	"log/slog"
	"os"
	"strings"
)

// Logger is the package-level structured logger. It defaults to info level
// with a JSON handler writing to stderr.
var Logger *slog.Logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// Init configures the package-level Logger with a JSON handler at the given
// level. Valid levels are "debug", "info", "warn", and "error". An unrecognised
// level string falls back to info.
func Init(level string) {
	var l slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		l = slog.LevelDebug
	case "warn", "warning":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	Logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: l,
	}))
}
