package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"
)

func (a *App) startMetricsServer(ctx context.Context) {
	a.configMu.Lock()
	addr := a.config.MetricsAddr
	a.configMu.Unlock()
	if addr == "" {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(a.GetEnhancedPrometheusMetrics()))
	})

	mux.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		metrics := a.GetQueueMetrics()
		if err := json.NewEncoder(w).Encode(metrics); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/api/errors/recent", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		limit := 10
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
				limit = parsed
			}
		}

		var errors []string
		if a.monitor != nil {
			errors = a.monitor.GetRecentErrors(limit)
		}

		if errors == nil {
			errors = []string{}
		}

		if err := json.NewEncoder(w).Encode(errors); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logWarningf(ctx, "metrics server listen failed on %s: %v", addr, err)
		return
	}

	a.metricsMu.Lock()
	a.metricsListener = ln
	a.metricsServer = &http.Server{Handler: mux}
	a.metricsMu.Unlock()

	go func() {
		if err := a.metricsServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			logWarningf(ctx, "metrics server stopped with error: %v", err)
		}
	}()
	logInfof(ctx, "metrics server listening on %s", ln.Addr().String())
}

func (a *App) stopMetricsServer(ctx context.Context) {
	a.metricsMu.Lock()
	server := a.metricsServer
	listener := a.metricsListener
	a.metricsServer = nil
	a.metricsListener = nil
	a.metricsMu.Unlock()
	if server != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logWarningf(ctx, "metrics server shutdown failed: %v", err)
		}
	}
	if listener != nil {
		_ = listener.Close()
	}
}

func (a *App) metricsAddress() string {
	a.metricsMu.Lock()
	defer a.metricsMu.Unlock()
	if a.metricsListener == nil {
		return ""
	}
	return a.metricsListener.Addr().String()
}

func defaultMetricsAddr() string {
	return "127.0.0.1:9464"
}

func normalizeMetricsAddr(addr string) string {
	if addr == "" {
		return defaultMetricsAddr()
	}
	return addr
}
