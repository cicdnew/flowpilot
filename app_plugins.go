package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"flowpilot/internal/logs"
	"flowpilot/internal/plugin"
)

const pluginShutdownTimeout = 10 * time.Second

type PluginStatus struct {
	Dir     string        `json:"dir"`
	Plugins []plugin.Info `json:"plugins"`
	Errors  []string      `json:"errors"`
}

func (a *App) pluginsDir() string {
	return filepath.Join(a.dataDir, "plugins")
}

func (a *App) pluginRegistry() *plugin.Registry {
	if a.plugins != nil {
		return a.plugins
	}
	return plugin.Default()
}

func (a *App) initPlugins(ctx context.Context) {
	if err := os.MkdirAll(a.pluginsDir(), 0o700); err != nil {
		a.setPluginErrors([]string{fmt.Sprintf("create plugins dir: %v", err)})
		return
	}
	a.loadPlugins(ctx)
}

func (a *App) loadPlugins(ctx context.Context) {
	ids, err := plugin.LoadDir(ctx, a.pluginRegistry(), a.pluginsDir())
	var messages []string
	if err != nil {
		messages = splitJoinedErrors(err)
		for _, msg := range messages {
			logs.Logger.Warn("plugin load failed", "error", msg)
		}
	}
	if len(ids) > 0 {
		logs.Logger.Info("plugins loaded", "plugins", strings.Join(ids, ","), "dir", a.pluginsDir())
	}
	a.setPluginErrors(messages)
}

func splitJoinedErrors(err error) []string {
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		out := make([]string, 0, len(joined.Unwrap()))
		for _, e := range joined.Unwrap() {
			out = append(out, e.Error())
		}
		return out
	}
	return []string{err.Error()}
}

func (a *App) setPluginErrors(messages []string) {
	a.pluginMu.Lock()
	a.pluginErrors = messages
	a.pluginMu.Unlock()
}

func (a *App) ListPlugins() PluginStatus {
	infos := a.pluginRegistry().List()
	for i := range infos {
		infos[i].Config = nil
	}
	a.pluginMu.Lock()
	errs := append([]string{}, a.pluginErrors...)
	a.pluginMu.Unlock()
	return PluginStatus{Dir: a.pluginsDir(), Plugins: infos, Errors: errs}
}

func (a *App) ReloadPlugins() (PluginStatus, error) {
	if err := a.ready(); err != nil {
		return PluginStatus{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), pluginShutdownTimeout)
	defer cancel()
	a.pluginReloadMu.Lock()
	defer a.pluginReloadMu.Unlock()
	if err := a.pluginRegistry().Close(ctx); err != nil {
		logs.Logger.Warn("plugin shutdown during reload failed", "error", err)
	}
	a.loadPlugins(context.Background())
	return a.ListPlugins(), nil
}

func (a *App) shutdownPlugins() {
	ctx, cancel := context.WithTimeout(context.Background(), pluginShutdownTimeout)
	defer cancel()
	if err := a.pluginRegistry().Close(ctx); err != nil {
		logs.Logger.Warn("plugin shutdown failed", "error", err)
	}
}
