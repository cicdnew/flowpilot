package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"flowpilot/internal/crypto"
	"flowpilot/internal/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func requireChromeForMCP(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("end-to-end MCP test")
	}
	for _, c := range []string{"google-chrome", "chromium", "chromium-browser", "google-chrome-stable"} {
		if _, err := exec.LookPath(c); err == nil {
			return
		}
	}
	t.Skip("chrome not installed")
}

func testSite(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head><title>MCP Test %s</title></head><body><h1 id="greeting">Hello from %s</h1></body></html>`, r.URL.Path, r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func startHeadlessApp(t *testing.T) *App {
	t.Helper()
	crypto.ResetForTest()
	t.Cleanup(crypto.ResetForTest)
	a := NewApp()
	a.headless = true
	a.dataDirOverride = t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	a.startup(ctx)
	if a.initErr != nil {
		cancel()
		t.Fatalf("headless startup: %v", a.initErr)
	}
	t.Cleanup(func() {
		a.cleanup()
		cancel()
	})
	return a
}

func connectInMemory(t *testing.T, a *App) *mcp.ClientSession {
	t.Helper()
	server := newMCPServer(a, "test")
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "1"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func callTool[T any](t *testing.T, cs *mcp.ClientSession, name string, args any) T {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s returned tool error: %s", name, res.Content[0].(*mcp.TextContent).Text)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var wrapper struct {
		Result T `json:"result"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatalf("%s: decode %s: %v", name, raw, err)
	}
	return wrapper.Result
}

func callToolError(t *testing.T, cs *mcp.ClientSession, name string, args any) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return err.Error()
	}
	if !res.IsError {
		t.Fatalf("%s: expected a tool error", name)
	}
	return res.Content[0].(*mcp.TextContent).Text
}

type waitResult struct {
	Finished bool        `json:"finished"`
	Task     models.Task `json:"task"`
}

func extractSteps(url string) []map[string]any {
	return []map[string]any{
		{"action": "navigate", "value": url},
		{"action": "extract", "selector": "#greeting", "value": "greeting"},
		{"action": "get_title", "value": "title"},
	}
}

func TestMCPFullControlEndToEnd(t *testing.T) {
	requireChromeForMCP(t)
	site := testSite(t)
	a := startHeadlessApp(t)
	cs := connectInMemory(t, a)
	ctx := context.Background()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"run_task", "create_task", "start_task", "cancel_task", "wait_for_task", "list_tasks", "create_batch",
		"get_batch_progress", "get_observability", "get_prometheus_metrics", "get_logs", "list_plugins", "reload_plugins", "create_alert_rule"} {
		if !names[want] {
			t.Errorf("tool %s missing", want)
		}
	}
	if len(tools.Tools) < 50 {
		t.Errorf("expected at least 50 tools, got %d", len(tools.Tools))
	}

	actions := callTool[[]string](t, cs, "list_step_actions", map[string]any{})
	if len(actions) < 50 || actions[0] == "" {
		t.Fatalf("step catalog: %v", actions)
	}

	t.Run("run_task headless", func(t *testing.T) {
		got := callTool[waitResult](t, cs, "run_task", map[string]any{
			"name": "mcp headless", "url": site.URL + "/headless", "steps": extractSteps(site.URL + "/headless"), "waitTimeoutSec": 90,
		})
		if !got.Finished || got.Task.Status != models.TaskStatusCompleted || !got.Task.Headless {
			t.Fatalf("headless run: finished=%v status=%s headless=%v err=%s", got.Finished, got.Task.Status, got.Task.Headless, got.Task.Error)
		}
		if got.Task.Result.ExtractedData["greeting"] != "Hello from /headless" || got.Task.Result.ExtractedData["title"] != "MCP Test /headless" {
			t.Fatalf("extracted: %v", got.Task.Result.ExtractedData)
		}
	})

	t.Run("run_task headed", func(t *testing.T) {
		if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			t.Skip("no display for a headed browser")
		}
		got := callTool[waitResult](t, cs, "run_task", map[string]any{
			"name": "mcp headed", "url": site.URL + "/headed", "steps": extractSteps(site.URL + "/headed"), "headless": false, "waitTimeoutSec": 90,
		})
		if got.Task.Status != models.TaskStatusCompleted || got.Task.Headless {
			t.Fatalf("headed run: status=%s headless=%v err=%s", got.Task.Status, got.Task.Headless, got.Task.Error)
		}
		if got.Task.Result.ExtractedData["greeting"] != "Hello from /headed" {
			t.Fatalf("extracted: %v", got.Task.Result.ExtractedData)
		}
	})

	t.Run("create, start, wait, inspect", func(t *testing.T) {
		task := callTool[models.Task](t, cs, "create_task", map[string]any{
			"name": "mcp two-step", "url": site.URL + "/manual", "steps": extractSteps(site.URL + "/manual"), "tags": []string{"mcp"},
		})
		if task.Status != models.TaskStatusPending {
			t.Fatalf("created task status: %s", task.Status)
		}
		callTool[string](t, cs, "start_task", map[string]any{"id": task.ID})
		got := callTool[waitResult](t, cs, "wait_for_task", map[string]any{"id": task.ID, "waitTimeoutSec": 90})
		if got.Task.Status != models.TaskStatusCompleted {
			t.Fatalf("status: %s (%s)", got.Task.Status, got.Task.Error)
		}
		fetched := callTool[models.Task](t, cs, "get_task", map[string]any{"id": task.ID})
		if fetched.Result == nil || fetched.Result.ExtractedData["greeting"] != "Hello from /manual" {
			t.Fatalf("get_task result: %+v", fetched.Result)
		}
		page := callTool[models.PaginatedTasks](t, cs, "list_tasks", map[string]any{"tag": "mcp"})
		if len(page.Tasks) != 1 || page.Tasks[0].ID != task.ID {
			t.Fatalf("list_tasks by tag: %+v", page)
		}
		events := callTool[[]models.TaskLifecycleEvent](t, cs, "get_task_events", map[string]any{"id": task.ID})
		if len(events) == 0 {
			t.Fatal("no lifecycle events recorded")
		}
	})

	t.Run("cancel a running task", func(t *testing.T) {
		task := callTool[models.Task](t, cs, "create_task", map[string]any{
			"name": "mcp long", "url": site.URL + "/long", "autoStart": true,
			"steps": []map[string]any{{"action": "navigate", "value": site.URL + "/long"}, {"action": "wait", "value": "60000"}},
		})
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			if callTool[models.Task](t, cs, "get_task", map[string]any{"id": task.ID}).Status == models.TaskStatusRunning {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		callTool[string](t, cs, "cancel_task", map[string]any{"id": task.ID})
		got := callTool[waitResult](t, cs, "wait_for_task", map[string]any{"id": task.ID, "waitTimeoutSec": 30})
		if got.Task.Status != models.TaskStatusCancelled {
			t.Fatalf("expected cancelled, got %s", got.Task.Status)
		}
	})

	t.Run("batch", func(t *testing.T) {
		var specs []map[string]any
		for i := range 3 {
			path := fmt.Sprintf("/batch-%d", i)
			specs = append(specs, map[string]any{"name": "batch " + path, "url": site.URL + path, "steps": extractSteps(site.URL + path)})
		}
		created := callTool[[]models.Task](t, cs, "create_batch", map[string]any{"tasks": specs, "autoStart": true})
		if len(created) != 3 || created[0].BatchID == "" {
			t.Fatalf("create_batch: %+v", created)
		}
		for _, task := range created {
			got := callTool[waitResult](t, cs, "wait_for_task", map[string]any{"id": task.ID, "waitTimeoutSec": 120})
			if got.Task.Status != models.TaskStatusCompleted {
				t.Fatalf("batch task %s: %s (%s)", task.Name, got.Task.Status, got.Task.Error)
			}
		}
		progress := callTool[models.BatchProgress](t, cs, "get_batch_progress", map[string]any{"id": created[0].BatchID})
		if progress.Completed != 3 {
			t.Fatalf("batch progress: %+v", progress)
		}
	})

	t.Run("observability", func(t *testing.T) {
		snap := callTool[ObservabilitySnapshot](t, cs, "get_observability", map[string]any{})
		byAction := map[string]int64{}
		for _, s := range snap.Steps {
			byAction[s.Action] = s.Count
		}
		if byAction["navigate"] < 6 || byAction["extract"] < 6 {
			t.Fatalf("step metrics not recorded: %+v", snap.Steps)
		}
		if snap.Queue.TotalCompleted < 5 || snap.Queue.TotalCancelled < 1 {
			t.Fatalf("queue metrics: %+v", snap.Queue)
		}
		prom := callTool[string](t, cs, "get_prometheus_metrics", map[string]any{})
		for _, want := range []string{`flowpilot_step_duration_ms_count{action="navigate"}`, "flowpilot_queue_running", "flowpilot_plugins_loaded"} {
			if !strings.Contains(prom, want) {
				t.Errorf("prometheus output missing %s", want)
			}
		}
		stats := callTool[map[string]int](t, cs, "get_task_stats", map[string]any{})
		if stats["completed"] < 5 || stats["cancelled"] < 1 {
			t.Fatalf("task stats: %v", stats)
		}
		res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "flowpilot://observability"})
		if err != nil || !strings.Contains(res.Contents[0].Text, `"queue"`) {
			t.Fatalf("observability resource: %v", err)
		}
		callTool[PluginStatus](t, cs, "list_plugins", map[string]any{})
		callTool[PluginStatus](t, cs, "reload_plugins", map[string]any{})
	})

	t.Run("errors are reported as tool errors", func(t *testing.T) {
		if msg := callToolError(t, cs, "create_task", map[string]any{"name": "bad", "url": site.URL, "steps": []map[string]any{{"action": "teleport"}}}); !strings.Contains(msg, "teleport") {
			t.Fatalf("invalid action error: %s", msg)
		}
		if msg := callToolError(t, cs, "get_task", map[string]any{"id": "does-not-exist"}); msg == "" {
			t.Fatal("missing task should be an error")
		}
	})
}

func TestMCPHTTPTransportRequiresToken(t *testing.T) {
	requireChromeForMCP(t)
	a := startHeadlessApp(t)
	srv, err := startMCPHTTP(newMCPServer(a, "test"), "127.0.0.1:0", "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	endpoint := "http://" + srv.Addr() + mcpPath
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "http-e2e", Version: "1"}, nil)

	if _, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint}, nil); err == nil {
		t.Fatal("connecting without the bearer token must fail")
	}
	authed := &http.Client{Transport: bearerTransport{token: "s3cret"}}
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint, HTTPClient: authed}, nil)
	if err != nil {
		t.Fatalf("connect with token: %v", err)
	}
	defer cs.Close()
	health := callTool[map[string]any](t, cs, "get_system_health", map[string]any{})
	if health["status"] == "" {
		t.Fatalf("health over HTTP: %v", health)
	}
}

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func TestMCPStdioBinary(t *testing.T) {
	requireChromeForMCP(t)
	bin := filepath.Join(t.TempDir(), "flowpilot")
	build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("production binary does not build in this environment: %v\n%s", err, out)
	}
	site := testSite(t)
	cmd := exec.Command(bin, "mcp", "--data-dir", t.TempDir())
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "stdio-e2e", Version: "1"}, nil).Connect(context.Background(), &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect over stdio: %v", err)
	}
	defer cs.Close()
	if init := cs.InitializeResult(); init.ServerInfo.Name != mcpServerName || !strings.Contains(init.Instructions, "headless") {
		t.Fatalf("initialize: %+v", init)
	}
	got := callTool[waitResult](t, cs, "run_task", map[string]any{
		"name": "stdio", "url": site.URL + "/stdio", "steps": extractSteps(site.URL + "/stdio"), "waitTimeoutSec": 90,
	})
	if got.Task.Status != models.TaskStatusCompleted || got.Task.Result.ExtractedData["greeting"] != "Hello from /stdio" {
		t.Fatalf("stdio run_task: %s %v (%s)", got.Task.Status, got.Task.Result, got.Task.Error)
	}
}

func TestEmbeddedMCPInGUIModeUsesEnvAddress(t *testing.T) {
	t.Setenv(mcpAddrEnv, "127.0.0.1:0")
	t.Setenv(mcpTokenEnv, "")
	a := &App{ctx: context.Background(), config: DefaultAppConfig()}
	a.startEmbeddedMCP()
	if a.mcpHTTP == nil {
		t.Fatal("embedded MCP server not started in GUI mode")
	}
	defer a.mcpHTTP.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "gui", Version: "1"}, nil).Connect(context.Background(),
		&mcp.StreamableClientTransport{Endpoint: "http://" + a.mcpHTTP.Addr() + mcpPath}, nil)
	if err != nil {
		t.Fatalf("connect to embedded MCP: %v", err)
	}
	defer cs.Close()
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) < 50 {
		t.Fatalf("embedded MCP tools: %d %v", len(tools.Tools), err)
	}

	headless := &App{ctx: context.Background(), headless: true}
	headless.startEmbeddedMCP()
	if headless.mcpHTTP != nil {
		t.Fatal("headless mode must not start a second embedded MCP server")
	}
}

func TestMCPBridgeForwardsToRunningInstance(t *testing.T) {
	requireChromeForMCP(t)
	a := startHeadlessApp(t)
	srv, err := startMCPHTTP(newMCPServer(a, "test"), "127.0.0.1:0", "bridge-token")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	endpoint := "http://" + srv.Addr() + mcpPath
	ctx := context.Background()

	if _, err := connectRemoteMCP(ctx, endpoint, "wrong"); err == nil {
		t.Fatal("bridge must fail with a wrong token")
	}
	remote, err := connectRemoteMCP(ctx, endpoint, "bridge-token")
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	bridge, err := newBridgeServer(ctx, remote)
	if err != nil {
		t.Fatal(err)
	}
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := bridge.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "agent", Version: "1"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	if cs.InitializeResult().ServerInfo.Name != mcpServerName {
		t.Fatalf("bridge should present the FlowPilot identity: %+v", cs.InitializeResult().ServerInfo)
	}
	tools, err := cs.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) < 50 {
		t.Fatalf("bridged tools: %d %v", len(tools.Tools), err)
	}
	task := callTool[models.Task](t, cs, "create_task", map[string]any{
		"name": "via bridge", "url": "https://example.com", "steps": []map[string]any{{"action": "navigate", "value": "https://example.com"}},
	})
	if got, err := a.GetTask(task.ID); err != nil || got.Name != "via bridge" {
		t.Fatalf("task created through the bridge is not in the running instance: %v %v", got, err)
	}
	if msg := callToolError(t, cs, "get_task", map[string]any{"id": "missing"}); msg == "" {
		t.Fatal("remote tool errors must propagate through the bridge")
	}
	if res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "flowpilot://metrics"}); err != nil || !strings.Contains(res.Contents[0].Text, "flowpilot_") {
		t.Fatalf("bridged resource: %v", err)
	}
}
