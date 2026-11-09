package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var appVersion = "dev"

const (
	mcpAddrEnv  = "FLOWPILOT_MCP_ADDR"
	mcpTokenEnv = "FLOWPILOT_MCP_TOKEN"
	mcpPath     = "/mcp"
)

type mcpHTTPServer struct {
	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	auth     bool
}

func mcpAuthMiddleware(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="flowpilot-mcp"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func startMCPHTTP(server *mcp.Server, addr, token string) (*mcpHTTPServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("mcp listen on %s: %w", addr, err)
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	mux := http.NewServeMux()
	mux.Handle(mcpPath, mcpAuthMiddleware(token, handler))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	s := &mcpHTTPServer{server: &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}, listener: ln, auth: token != ""}
	go func() {
		if err := s.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "mcp http server stopped: %v\n", err)
		}
	}()
	return s, nil
}

func (s *mcpHTTPServer) Addr() string {
	if s == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *mcpHTTPServer) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.server.Shutdown(ctx)
}

func (a *App) startEmbeddedMCP() {
	addr := strings.TrimSpace(os.Getenv(mcpAddrEnv))
	if addr == "" || a.headless {
		return
	}
	srv, err := startMCPHTTP(newMCPServer(a, appVersion), addr, os.Getenv(mcpTokenEnv))
	if err != nil {
		logWarningf(a.ctx, "embedded MCP server failed: %v", err)
		return
	}
	a.mcpHTTP = srv
	logInfof(a.ctx, "MCP server listening on http://%s%s", srv.Addr(), mcpPath)
}

func runMCPCommand(args []string) int {
	fs := flag.NewFlagSet("flowpilot mcp", flag.ContinueOnError)
	transport := fs.String("transport", "stdio", "stdio or http")
	addr := fs.String("addr", "127.0.0.1:8765", "listen address for --transport=http")
	dataDir := fs.String("data-dir", os.Getenv("FLOWPILOT_DATA_DIR"), "data directory (default ~/.flowpilot)")
	forceHeadless := fs.Bool("force-headless", false, "run every task headless even if headless=false is requested")
	token := fs.String("token", os.Getenv(mcpTokenEnv), "require this bearer token for --transport=http (or send it with --connect)")
	tokenFile := fs.String("token-file", "", "read the bearer token from this file")
	connect := fs.String("connect", "", "bridge stdio to an already running FlowPilot MCP endpoint, e.g. http://127.0.0.1:8765/mcp")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *transport != "stdio" && *transport != "http" {
		fmt.Fprintf(os.Stderr, "unknown transport %q (use stdio or http)\n", *transport)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	resolvedToken, err := readToken(*token, *tokenFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if *connect != "" {
		return runMCPBridge(ctx, *connect, resolvedToken)
	}

	a := NewApp()
	a.headless = true
	a.dataDirOverride = *dataDir
	a.startup(ctx)
	if a.initErr != nil {
		fmt.Fprintf(os.Stderr, "flowpilot mcp: startup failed: %v\n", a.initErr)
		return 1
	}
	defer a.cleanup()
	if *forceHeadless {
		a.runner.SetForceHeadless(true)
	}
	server := newMCPServer(a, appVersion)

	if *transport == "stdio" {
		if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, "flowpilot mcp: %v\n", err)
			return 1
		}
		return 0
	}
	srv, err := startMCPHTTP(server, *addr, resolvedToken)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer srv.Close()
	fmt.Fprintf(os.Stderr, "flowpilot mcp listening on http://%s%s\n", srv.Addr(), mcpPath)
	<-ctx.Done()
	return 0
}
