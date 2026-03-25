# Implementation Plan: Plugin & Extension Architecture

## Overview

This document outlines a phased approach to adding a plugin/extension system to FlowPilot, enabling third-party developers to extend core functionality without modifying the codebase. Currently, all step actions (60+) are hardcoded in `internal/browser/steps.go`, recorder events are fixed, and validation rules are monolithic. A plugin system would allow:

- Custom step actions registered dynamically
- Custom recorder event types
- Custom validation rules
- Custom browser hooks (pre/post execution)
- Custom UI components in the frontend

## Design Decisions

### Plugin Model: Embedded WASM + gRPC Sidecar Hybrid

After analyzing the constraints, we recommend a **hybrid approach**:

1. **WASM modules** for lightweight, stateless custom step handlers (low latency, no subprocess overhead)
2. **gRPC sidecars** for complex plugins requiring system access, persistent state, or heavy computation
3. **Go plugin (.so)** rejected: breaks Wails desktop distribution, platform-specific binaries
4. **Subprocess stdio** rejected: highest latency, complex serialization
5. **Embedded JS (goja/v8go)** rejected: security complexity, debugging difficulty

**Rationale:**
- Wails packages the Go binary as a single executable; loading dynamic .so files breaks this model
- WASM provides sandboxing out-of-the-box and works cross-platform
- gRPC sidecars handle complex use cases without embedding concerns
- Hybrid allows simple plugins to be WASM-only, advanced plugins to use gRPC

## Requirements

### Functional Requirements
1. Plugins register new `StepAction` types with handlers
2. Plugins define their own validation rules
3. Plugins hook into step lifecycle (pre-execute, post-execute, on-error)
4. Plugins can access recorder events and task context
5. Plugins can store persistent configuration
6. Plugin marketplace/registry for discovery and versioning
7. Frontend UI panel for plugin management (list, enable/disable, configure)

### Non-Functional Requirements
1. Plugins isolated via WASM sandbox or gRPC process boundary
2. Plugin crashes do not crash the main app
3. Maximum execution time limits per plugin call
4. Resource limits (memory, CPU)
5. Plugin API versioning for backward compatibility
6. Audit trail of plugin actions

## Plugin Model Options & Trade-offs

### Option A: Go Plugin (.so)
**Pros:**
- Native Go performance
- Direct access to app internals

**Cons:**
- ❌ Breaks Wails desktop distribution (can't package dynamic .so with GUI app)
- Platform-specific compilation required
- No sandboxing; plugin crash = app crash
- **REJECTED**

### Option B: WASM (WebAssembly)
**Pros:**
- ✅ True sandboxing via runtime isolation
- ✅ Cross-platform (compile once)
- ✅ Safe: can't break out of sandbox
- ✅ Fast startup
- ✅ Works offline
- Low latency for simple operations

**Cons:**
- Limited to CPU-bound work (no direct file I/O, network, system calls)
- Larger binary footprint per plugin
- Learning curve for WASM toolchain

**Best for:** Custom step handlers, selector generators, data transformers

### Option C: gRPC Sidecar Process
**Pros:**
- ✅ Full system access (file I/O, network, external tools)
- ✅ Language-agnostic (write plugins in Python, Node.js, Rust, etc.)
- ✅ Persistent state management
- ✅ Crash isolation (sidecar death ≠ app death)

**Cons:**
- Higher latency (IPC overhead)
- Process management complexity
- Requires plugin process lifecycle management
- More resource overhead

**Best for:** Heavy computation, ML inference, database access, external service integration

### Option D: Subprocess stdio
**Pros:**
- Simple to implement
- Language-agnostic

**Cons:**
- ❌ Highest latency (JSON/serialization overhead)
- ❌ Complex error handling
- Process explosion risk
- **REJECTED**

### Option E: Embedded JS (goja/v8go)
**Pros:**
- Easy for web developers

**Cons:**
- ❌ No true sandboxing without complex filtering
- ❌ Debugging difficult
- Additional dependency footprint
- **REJECTED**

## Architecture Changes

### Phase 0: Plugin Model & Infrastructure

#### New Directories & Files

```
internal/plugin/
  ├── plugin.go           # Core plugin interfaces & registry
  ├── wasm.go             # WASM runtime management
  ├── grpc_broker.go      # gRPC sidecar lifecycle
  ├── config.go           # Plugin configuration storage
  ├── sandbox.go          # Resource limits & isolation
  └── plugin_test.go

internal/models/
  ├── plugin.go (new)     # Plugin metadata types
```

#### Database Schema Extension

Add to `internal/database/sqlite.go` migration:

```sql
CREATE TABLE plugins (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  version TEXT NOT NULL,
  enabled BOOLEAN DEFAULT 1,
  type TEXT NOT NULL, -- 'wasm' or 'grpc'
  path TEXT,  -- path to .wasm or config
  config_json TEXT,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);

CREATE TABLE plugin_actions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  plugin_id TEXT NOT NULL,
  action_name TEXT NOT NULL UNIQUE,
  handler_type TEXT, -- 'wasm_func' or 'grpc_method'
  metadata_json TEXT,
  FOREIGN KEY(plugin_id) REFERENCES plugins(id)
);

CREATE TABLE plugin_hooks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  plugin_id TEXT NOT NULL,
  hook_name TEXT NOT NULL, -- 'pre_execute', 'post_execute', 'on_error'
  handler_type TEXT,
  FOREIGN KEY(plugin_id) REFERENCES plugins(id)
);

CREATE TABLE plugin_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  plugin_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  payload_json TEXT,
  created_at TIMESTAMP,
  FOREIGN KEY(plugin_id) REFERENCES plugins(id)
);
```

### Phase 1: Custom Step Actions

**File changes:**

1. **`internal/models/plugin.go` (new)**
   - Define `Plugin` struct, `PluginType` enum (WASM, gRPC)
   - Define `PluginAction` registration struct
   - Define `StepHandler` interface

2. **`internal/plugin/plugin.go` (new)**
   - `Registry` struct with methods: `Register()`, `Lookup()`, `List()`, `Unload()`
   - `StepHandler` interface (for both WASM & gRPC)
   - Integration point: call `Registry.Lookup()` in `internal/browser/steps.go` before hardcoded switch

3. **`internal/browser/steps.go` (modify)**
   - In `executeStep()`: add registry lookup before switch statement
   - Pattern:
     ```go
     func (r *Runner) executeStep(ctx context.Context, step models.TaskStep, result *models.TaskResult) error {
         // Check plugin registry first
         if handler := r.pluginRegistry.Lookup(step.Action); handler != nil {
             return handler.Execute(ctx, step, result)
         }
         // Fallback to hardcoded actions
         switch step.Action { ... }
     }
     ```

4. **`internal/validation/validate.go` (modify)**
   - Hook plugin-registered validators before hardcoded switch
   - Allow plugins to extend `selectorRequiredActions`, `valueRequiredActions` maps

### Phase 2: Plugin Lifecycle

**Files:**

1. **`internal/plugin/plugin.go` (extend)**
   - Add `Load()`, `Init()`, `Configure()`, `Unload()` lifecycle hooks
   - Plugin manifest format (JSON):
     ```json
     {
       "id": "my-plugin",
       "name": "My Custom Plugin",
       "version": "1.0.0",
       "type": "wasm|grpc",
       "entrypoint": "path/to/plugin.wasm or grpc://localhost:5000",
       "actions": [
         {
           "name": "custom_action",
           "selector_required": true,
           "value_required": false
         }
       ],
       "hooks": ["pre_execute", "post_execute"],
       "config_schema": { ... }
     }
     ```

2. **`internal/plugin/wasm.go` (new)**
   - Load .wasm files via `wasmtime` or `wasmer` crate (via CGO)
   - Instantiate with memory sandbox
   - Export host functions: `log`, `read_config`, `store_data`
   - Call plugin exports: `on_load`, `on_execute_step`, `on_execute_step_done`

3. **`internal/plugin/grpc_broker.go` (new)**
   - Spawn gRPC plugin process
   - Handle lifecycle: start, healthcheck, shutdown
   - Retry logic for unhealthy sidecars

### Phase 3: Plugin SDK

**Go Plugin SDK** (new package `sdk/go/plugin`):

```go
package plugin

import "context"

// StepContext provides access to browser automation context
type StepContext struct {
    TaskID        string
    FlowID        string
    Variables     map[string]string
    ExtractedData map[string]string
    Step          TaskStep // the step being executed
}

// StepHandler executes a custom action
type StepHandler interface {
    Execute(ctx context.Context, sc *StepContext) (map[string]interface{}, error)
    Name() string
}

// Hook lifecycle hooks
type Hook interface {
    OnBeforeExecute(ctx context.Context, sc *StepContext) error
    OnAfterExecute(ctx context.Context, sc *StepContext, result interface{}) error
    OnError(ctx context.Context, sc *StepContext, err error) error
}

// Plugin is the main plugin interface
type Plugin interface {
    Init(config map[string]interface{}) error
    Name() string
    Version() string
    Actions() []StepHandler
    Hooks() []Hook
    Shutdown() error
}
```

**TypeScript Plugin SDK** (new `sdk/typescript/plugin.ts`):

```typescript
export interface StepContext {
    taskId: string;
    flowId: string;
    variables: Record<string, string>;
    extractedData: Record<string, string>;
    step: TaskStep;
}

export interface StepHandler {
    name: string;
    execute(ctx: StepContext): Promise<Record<string, any>>;
}

export interface Hook {
    onBeforeExecute?(ctx: StepContext): Promise<void>;
    onAfterExecute?(ctx: StepContext, result: any): Promise<void>;
    onError?(ctx: StepContext, error: Error): Promise<void>;
}

export interface PluginManifest {
    id: string;
    name: string;
    version: string;
    type: 'wasm' | 'grpc';
    actions: Array<{
        name: string;
        selectorRequired?: boolean;
        valueRequired?: boolean;
    }>;
    hooks?: Array<'pre_execute' | 'post_execute' | 'on_error'>;
}
```

**WASM Plugin Example** (AssemblyScript):

```typescript
// plugins/custom-click/index.ts
import { StepContext } from '@flowpilot/plugin-sdk';

export function on_execute_step(ctxPtr: usize): i32 {
    // WASM memory layout: deserialize StepContext from buffer
    let selector = readString(ctxPtr);
    // Custom click logic
    return 0; // success
}

export function on_load(): i32 {
    return 0;
}
```

### Phase 4: Event Hooks & Middleware

**File changes:**

1. **`internal/plugin/hooks.go` (new)**
   - Hook registry with lifecycle: `PreExecute`, `PostExecute`, `OnError`, `OnRecorderEvent`
   - Execution chain: plugins run sequentially, can veto execution or modify context

2. **`internal/browser/executor.go` (modify)**
   - Before step execution: call `r.pluginRegistry.ExecuteHooks("pre_execute", ...)`
   - After step execution: call `r.pluginRegistry.ExecuteHooks("post_execute", ...)`
   - On error: call `r.pluginRegistry.ExecuteHooks("on_error", ...)`

3. **`internal/recorder/recorder.go` (modify)**
   - On event recorded: emit to plugin hooks
   - Allow plugins to suppress or modify recorded events

## Implementation Steps (Phased)

### Phase 0: Plugin Infrastructure (Week 1-2)
- [ ] Design plugin registry and interfaces (`internal/plugin/plugin.go`)
- [ ] Extend database schema with plugin tables
- [ ] Create plugin models (`internal/models/plugin.go`)
- [ ] Implement basic WASM runtime wrapper (`internal/plugin/wasm.go`)
- [ ] Tests for plugin loading and registration

### Phase 1: Custom Step Actions (Week 2-3)
- [ ] Integrate plugin registry lookup in `internal/browser/steps.go`
- [ ] Implement step handler execution for WASM
- [ ] Add plugin-aware validation in `internal/validation/validate.go`
- [ ] Create CLI/API endpoint to load plugin manifest
- [ ] Tests: custom step execution, validation bypass

### Phase 2: Plugin Lifecycle (Week 3-4)
- [ ] Implement plugin lifecycle: Load, Init, Configure, Unload
- [ ] Add gRPC broker (`internal/plugin/grpc_broker.go`)
- [ ] Plugin configuration storage and retrieval
- [ ] Plugin health monitoring
- [ ] Tests: plugin lifecycle, graceful shutdown

### Phase 3: SDK & Examples (Week 4-5)
- [ ] Create Go SDK package
- [ ] Create TypeScript SDK
- [ ] Write 2-3 example plugins (WASM + gRPC)
- [ ] Documentation and developer guide

### Phase 4: Hooks & Advanced Features (Week 5-6)
- [ ] Implement pre/post/error hooks
- [ ] Integrate hooks into browser executor
- [ ] Recorder event plugin hooks
- [ ] Tests: hook execution order, veto logic

### Phase 5: Plugin Registry & Marketplace (Week 6-7)
- [ ] Create plugin registry API endpoints
- [ ] Plugin listing, search, version management
- [ ] Plugin dependency resolution
- [ ] Plugin download and auto-install
- [ ] Signature verification for security

### Phase 6: Sandbox & Security (Week 7-8)
- [ ] WASM memory limits and timeout enforcement
- [ ] gRPC resource quotas
- [ ] Permission model (which APIs can plugin access?)
- [ ] Audit logging of plugin actions
- [ ] Security scanning before install

### Phase 7: Frontend UI (Week 8-9)
- [ ] New "Plugins" tab in UI
- [ ] Plugin list view with enable/disable
- [ ] Plugin configuration panel
- [ ] Plugin logs viewer
- [ ] Plugin marketplace browser

## Plugin SDK

### Go SDK Structure

**Package:** `flowpilot/sdk/go/plugin`

```go
// plugin.go
package plugin

import "context"

type TaskStep struct {
    Action   string                 `json:"action"`
    Selector string                 `json:"selector"`
    Value    string                 `json:"value"`
    // ... other fields
}

type StepContext struct {
    TaskID        string
    FlowID        string
    Variables     map[string]string
    ExtractedData map[string]string
    Step          TaskStep
    Logger        Logger
}

type Logger interface {
    Info(msg string)
    Error(msg string)
    Debug(msg string)
}

type StepHandler interface {
    Execute(ctx context.Context, sc *StepContext) (interface{}, error)
    Name() string
    Description() string
    Selector() bool // requires selector?
    Value() bool    // requires value?
}

type HookHandler interface {
    Name() string
}

type PreExecuteHook interface {
    HookHandler
    OnBeforeExecute(ctx context.Context, sc *StepContext) error
}

type PostExecuteHook interface {
    HookHandler
    OnAfterExecute(ctx context.Context, sc *StepContext, result interface{}) error
}

type ErrorHook interface {
    HookHandler
    OnError(ctx context.Context, sc *StepContext, err error) error
}

type Plugin interface {
    Init(config map[string]interface{}) error
    Name() string
    Version() string
    StepHandlers() []StepHandler
    Hooks() []HookHandler
    Shutdown() error
}
```

### TypeScript/WASM SDK

**Package:** `@flowpilot/plugin-sdk` (npm)

See Phase 3 section above for TypeScript definitions.

## Security & Sandboxing

### WASM Sandbox Model

1. **Memory Isolation:** WASM runs in isolated linear memory (64KB-4GB)
2. **No Direct Host Access:** Can only call exported host functions
3. **Timeout Enforcement:** Each plugin call wrapped in `context.WithTimeout(ctx, 5*time.Second)`
4. **Memory Limits:** Configure max memory per WASM instance (default 256MB)

**Implementation:**

```go
// internal/plugin/sandbox.go
type WASMSandbox struct {
    store     *wasmtime.Store
    instance  *wasmtime.Instance
    timeout   time.Duration
    maxMemory int64
}

func (s *WASMSandbox) Execute(ctx context.Context, fn string, args ...interface{}) (interface{}, error) {
    ctxWithTimeout, cancel := context.WithTimeout(ctx, s.timeout)
    defer cancel()
    
    // Execute with cancellation
    result := make(chan interface{}, 1)
    errors := make(chan error, 1)
    
    go func() {
        res, err := s.instance.GetExport(fn).Func().Call(args...)
        if err != nil {
            errors <- err
        } else {
            result <- res
        }
    }()
    
    select {
    case <-ctxWithTimeout.Done():
        return nil, fmt.Errorf("plugin execution timeout: %w", ctxWithTimeout.Err())
    case err := <-errors:
        return nil, err
    case res := <-result:
        return res, nil
    }
}
```

### gRPC Sidecar Sandbox Model

1. **Process Boundary:** Plugin runs in separate process
2. **Resource Quotas:** cgroups (Linux) or process limits (Windows)
3. **Health Monitoring:** Periodically ping plugin, restart if unresponsive
4. **API Versioning:** gRPC service versioned; old plugins rejected
5. **Capability Grants:** Plugin whitelist of accessible APIs

**Implementation:**

```go
// internal/plugin/grpc_broker.go
type gRPCPlugin struct {
    cmd         *exec.Cmd
    conn        *grpc.ClientConn
    healthy     bool
    lastHealthy time.Time
    timeout     time.Duration
}

func (p *gRPCPlugin) Start(ctx context.Context) error {
    p.cmd = exec.CommandContext(ctx, p.execPath)
    // Set resource limits via syscall.SysProcAttr
    p.cmd.SysProcAttr = &syscall.SysProcAttr{
        // cgroups config on Linux
    }
    return p.cmd.Start()
}

func (p *gRPCPlugin) HealthCheck(ctx context.Context) error {
    // gRPC health.v1.Health check
    client := grpc_health_v1.NewHealthClient(p.conn)
    res, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
    if err != nil {
        p.healthy = false
        return err
    }
    p.healthy = res.Status == grpc_health_v1.HealthCheckResponse_SERVING
    p.lastHealthy = time.Now()
    return nil
}
```

### Permission Model

Plugins request capabilities in manifest:

```json
{
  "capabilities": [
    "browser.execute_step",
    "browser.read_variables",
    "database.read",
    "filesystem.read:/var/flowpilot/data"
  ]
}
```

At install time, user approves requested permissions. Plugin can only call APIs listed.

### Audit Logging

```go
// internal/plugin/audit.go
func (r *Registry) LogPluginAction(pluginID, action string, context map[string]interface{}) {
    // Insert into plugin_events table
    // Includes: timestamp, plugin_id, action, context, result
}
```

## Testing Strategy

### Unit Tests

1. **Plugin Registry** (`internal/plugin/plugin_test.go`)
   - Test Register/Lookup/Unload
   - Test duplicate registration rejection
   - Test hook execution order

2. **WASM Sandbox** (`internal/plugin/wasm_test.go`)
   - Test WASM module loading
   - Test function execution
   - Test timeout enforcement
   - Test memory limits

3. **gRPC Broker** (`internal/plugin/grpc_broker_test.go`)
   - Test sidecar startup/shutdown
   - Test health check
   - Test reconnection on failure
   - Test method dispatch

### Integration Tests

1. **Custom Step Execution** (`internal/browser/steps_test.go`)
   - Load WASM plugin with custom action
   - Execute task with custom action
   - Verify result

2. **Hook Execution** (`internal/browser/executor_test.go`)
   - Test pre/post/error hooks in sequence
   - Test hook veto (returning error stops execution)

3. **End-to-End** (`app_test.go`)
   - Record flow with custom recorder event
   - Execute task with plugin step action
   - Verify audit trail

### Example Test

```go
// internal/plugin/wasm_test.go
func TestWASMSandboxExecution(t *testing.T) {
    sandbox, err := NewWASMSandbox("testdata/sample.wasm", 256*1024*1024, 5*time.Second)
    require.NoError(t, err)
    defer sandbox.Close()
    
    result, err := sandbox.Execute(context.Background(), "add", 2, 3)
    require.NoError(t, err)
    assert.Equal(t, int32(5), result)
}

func TestWASMTimeoutEnforcement(t *testing.T) {
    sandbox, err := NewWASMSandbox("testdata/infinite_loop.wasm", 256*1024*1024, 100*time.Millisecond)
    require.NoError(t, err)
    defer sandbox.Close()
    
    _, err = sandbox.Execute(context.Background(), "infinite")
    assert.ErrorIs(t, err, context.DeadlineExceeded)
}
```

## Risks & Mitigations

| Risk | Severity | Mitigation |
|------|----------|-----------|
| Malicious plugin crashes app | High | WASM/gRPC process boundary, health monitoring, restart logic |
| Plugin memory exhaustion | High | WASM memory limits, gRPC cgroup limits |
| Plugin breaks task execution | High | Hook veto logic, error handling, audit trail |
| Plugin data leakage | Medium | WASM sandbox, permission model, audit logging |
| Plugin version conflicts | Medium | Semantic versioning, API version negotiation, deprecation warnings |
| Plugin dependency hell | Medium | Lockfile for dependencies, version pinning |
| Slow plugin slows app | Medium | Per-plugin timeout, async hook execution option |
| Plugin market spam/malware | Medium | Code review, signature verification, community rating |

## Success Criteria

### Phase 0 Complete
- [ ] Plugin registry API fully tested
- [ ] Database schema migration runs cleanly
- [ ] Plugin models compile and serialize correctly

### Phase 1 Complete
- [ ] Can load a simple WASM plugin
- [ ] Custom step action executes in task flow
- [ ] Validation accepts plugin-registered actions
- [ ] No core functionality regression

### Phase 2 Complete
- [ ] Plugin lifecycle callbacks (init, shutdown) work
- [ ] gRPC sidecar starts/stops cleanly
- [ ] Plugin configuration persists and loads

### Phase 3 Complete
- [ ] Example WASM plugin (custom selector generator) works
- [ ] Example gRPC plugin (AI step generator) works
- [ ] SDK documentation complete with tutorials

### Phase 4 Complete
- [ ] Hooks execute in correct order
- [ ] Hooks can veto step execution
- [ ] Recorder event plugins capture custom events

### Phase 5 Complete
- [ ] Plugin registry endpoints functional
- [ ] Can list/search/install plugins via UI
- [ ] Plugin versioning and updates work

### Phase 6 Complete
- [ ] WASM memory/timeout limits enforced
- [ ] gRPC resource limits enforced
- [ ] Audit trail shows all plugin actions
- [ ] Permission model prevents unauthorized API calls

### Phase 7 Complete
- [ ] Frontend "Plugins" panel fully functional
- [ ] Plugin configuration UI works
- [ ] Plugin logs viewer accessible
- [ ] User can enable/disable plugins without restart

## File Reference Summary

**Core Plugin System:**
- `internal/plugin/plugin.go` — Plugin registry, interfaces
- `internal/plugin/wasm.go` — WASM runtime
- `internal/plugin/grpc_broker.go` — gRPC sidecar management
- `internal/plugin/hooks.go` — Hook registry and execution
- `internal/plugin/sandbox.go` — Resource limits and isolation
- `internal/models/plugin.go` — Plugin metadata types

**Integration Points:**
- `internal/browser/steps.go` — Plugin action dispatch (line ~18)
- `internal/browser/executor.go` — Hook injection points
- `internal/validation/validate.go` — Plugin validator registration
- `internal/recorder/recorder.go` — Plugin event hooks
- `internal/database/sqlite.go` — Schema migrations

**Frontend:**
- `frontend/src/components/PluginPanel.svelte` (new)
- `frontend/src/lib/types.ts` — Add Plugin, PluginAction types
- `app.go` — New Wails API methods for plugin management

**SDKs & Examples:**
- `sdk/go/plugin/` — Go plugin SDK
- `sdk/typescript/plugin.ts` — TypeScript SDK
- `examples/plugin-custom-click/` — WASM example
- `examples/plugin-ml-agent/` — gRPC example

## Next Steps

1. Review this plan with the team
2. Validate gRPC sidecar vs. WASM trade-offs
3. Proof-of-concept: implement Phase 0 + Phase 1 MVP
4. Gather community feedback on plugin API design
5. Begin Phase 2 implementation
