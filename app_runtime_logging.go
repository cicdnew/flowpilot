package main

import (
	"context"
	"fmt"
	"sync/atomic"

	"flowpilot/internal/logs"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func logInfof(ctx context.Context, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logs.Logger.Info(msg)
	safeWailsLog(ctx, func() { wailsRuntime.LogInfo(ctx, msg) })
}

func logWarningf(ctx context.Context, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logs.Logger.Warn(msg)
	safeWailsLog(ctx, func() { wailsRuntime.LogWarning(ctx, msg) })
}

func logErrorf(ctx context.Context, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logs.Logger.Error(msg)
	safeWailsLog(ctx, func() { wailsRuntime.LogError(ctx, msg) })
}

var wailsRuntimeLoggingEnabled atomic.Bool

func safeWailsLog(ctx context.Context, logFn func()) {
	if ctx == nil || !wailsRuntimeLoggingEnabled.Load() {
		return
	}
	defer func() {
		_ = recover()
	}()
	logFn()
}
