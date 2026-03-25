# Implementation Plan: Distributed Multi-Node Execution

## Overview

This plan outlines a phased approach to transform FlowPilot from a single-process Wails desktop application into a distributed, multi-node execution system. The current architecture (Sections: `app.go`, `internal/queue/queue.go`, `internal/database/sqlite.go`, `internal/browser/pool.go`, `internal/proxy/manager.go`) uses in-process state management with SQLite persistence. The distributed model will extract shared state into an external coordination layer, enable standalone worker nodes, and introduce a control plane for scheduling and monitoring.

### Current Single-Process Limitations

| Component | Current | Limitation |
|-----------|---------|-----------|
| **Queue State** | In-memory heap + 50ms SQLite flush (`internal/queue/queue.go:29-61`) | No external visibility; lost on restart |
| **Proxy Reservations** | In-memory map `activeReservations` (`internal/proxy/manager.go:59`) | No cross-process coordination |
| **Browser Pool** | Embedded Chrome processes (`internal/browser/pool.go:27-40`) | Local-only; no remote execution |
| **Task Assignment** | Single process polls and executes (`internal/queue/queue.go:96-107`) | No worker pool scaling |
| **Database** | SQLite single-writer WAL mode (`internal/database/sqlite.go:28`) | Single file; no HA or clustering |

## Target Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                    DISTRIBUTED FLOWPILOT                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────────────┐      Coordinator (etcd/Redis/Postgres)      │
│  │  Control Plane   │─────────────────────────────────────────────┤
│  │  (Master Node)   │  - Distributed locks (task assignment)      │
│  │                  │  - Shared queue state (priority heap)       │
│  │  - Schedule tasks│  - Proxy reservations (global map)          │
│  │  - Monitor nodes │  - Worker registry & health                 │
│  │  - Assign work   │  - Lease-based task ownership               │
│  │  - Collect stats │                                             │
│  └──────────────────┘                                              │
│          ▲                                                          │
│          │ gRPC heartbeat, task status, metrics                   │
│          │ every 10s (configurable)                               │
│          │                                                          │
│  ┌───────┴────────┬────────────┬────────────┬─────────────────┐  │
│  │                │            │            │                 │  │
│  ▼                ▼            ▼            ▼                 ▼  │
│ ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌──────────┐ │
│ │ Worker1 │  │ Worker2 │  │ Worker3 │  │ WorkerN │  │ Isolated │ │
│ │         │  │         │  │         │  │         │  │ Browser  │ │
│ │ - Poll  │  │ - Poll  │  │ - Poll  │  │ - Poll  │  │ Pool     │ │
│ │ - Run   │  │ - Run   │  │ - Run   │  │ - Run   │  │          │ │
│ │ - Report│  │ - Report│  │ - Report│  │ - Report│  │ (Remote  │ │
│ │         │  │         │  │         │  │         │  │  CDP)    │ │
│ └─────────┘  └─────────┘  └─────────┘  └─────────┘  └──────────┘ │
│                                                                     │
│  ┌──────────────────┐      Persistent Storage                      │
│  │  SQLite (Leader) │  or  Postgres (Multi-region HA)             │
│  │  + WAL replication   or  S3 + Event Log                        │
│  └──────────────────┘                                              │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Coordination Layer Selection

### Analysis: etcd vs Redis vs Postgres vs Custom TCP

| Criteria | etcd | Redis | Postgres | Custom TCP |
|----------|------|-------|----------|-----------|
| **Distributed Locks** | ⭐⭐⭐ Native leases | ⭐⭐ Lua scripts | ⭐⭐⭐ Advisory locks | ⭐ Manual |
| **Pub/Sub** | ⭐⭐ Watch API | ⭐⭐⭐ Native | ⭐⭐ LISTEN/NOTIFY | ⭐ Manual |
| **Persistence** | ⭐⭐⭐ Replicated | ⭐ Optional | ⭐⭐⭐ ACID | ⭐ Manual |
| **Operational Complexity** | ⭐⭐⭐ (3-5 nodes) | ⭐⭐⭐ (Simple) | ⭐⭐⭐ (Proven) | ⭐ (None) |
| **Consistency Model** | Linearizable | Eventually consistent | ACID | Custom |
| **Network Partition** | Split-brain safe | Risk of data loss | Consensus-based | Risk |
| **Recommended For** | High consistency | Session/cache | State + events | MVP |

### Recommendation: **Postgres (Primary) + etcd (Optional Upgrade)**

**Rationale:**
- FlowPilot already uses SQLite; Postgres is a natural evolution
- Single source of truth for all state (tasks, proxies, workers, leases)
- LISTEN/NOTIFY for real-time event propagation
- Advisory locks for task assignment ("SELECT ... FOR UPDATE")
- No new infrastructure; existing Postgres deployments work
- Easy to add etcd later for high-consistency distributed locks if needed

**Phase 1 decision:** Use **Postgres with LISTEN/NOTIFY** for coordination. Fallback to **file-based leases** for MVP.

---

## Architecture Changes by Phase


## Architecture Changes by Phase

### Phase 0: Coordination Layer Selection & Setup

**Deliverable:** Postgres cluster (local or cloud) + coordinator interface abstraction.

**Changes:**

1. **New Interface:** `internal/coordinator/coordinator.go`
   ```go
   package coordinator

   import "context"

   // Coordinator abstracts distributed state management.
   type Coordinator interface {
       // Lease management for task ownership
       AcquireLease(ctx context.Context, key string, ttl time.Duration) (LeaseID string, err error)
       ReleaseLease(ctx context.Context, leaseID string) error
       RenewLease(ctx context.Context, leaseID string, ttl time.Duration) error

       // Shared queue operations
       EnqueueTask(ctx context.Context, task *models.Task) error
       DequeueTask(ctx context.Context, nodeID string, maxPriority int) (*models.Task, error)
       UpdateTaskStatus(ctx context.Context, taskID, status string, leaseID string) error

       // Worker registry
       RegisterWorker(ctx context.Context, nodeID, addr string, capacity int) error
       DeregisterWorker(ctx context.Context, nodeID string) error
       GetWorkers(ctx context.Context) ([]WorkerInfo, error)

       // Proxy coordination
       ReserveProxy(ctx context.Context, proxyID, workerID string) (ReservationID string, err error)
       ReleaseProxy(ctx context.Context, reservationID string, success bool) error

       // Watch for changes
       Watch(ctx context.Context, pattern string) (<-chan Event, error)

       // Cleanup
       Close() error
   }

   type LeaseID string
   type ReservationID string

   type WorkerInfo struct {
       NodeID   string
       Address  string
       Capacity int
       LastSeen time.Time
   }

   type Event struct {
       Type  string // "task_enqueued", "worker_joined", "lease_expired"
       Key   string
       Value []byte
   }
   ```

2. **Postgres Implementation:** `internal/coordinator/postgres.go`
   - Single `coordinators` table tracking active nodes with heartbeat timestamps
   - `task_leases` table for distributed locks (task ownership)
   - `proxy_reservations` table replaces in-process map
   - Use `LISTEN/NOTIFY` for pub/sub
   - Heartbeat goroutine renews leases every 5s (TTL=10s)

3. **Fallback:** `internal/coordinator/file.go` (MVP, single-machine or network FS)
   - Lock files for leases
   - JSON files for state
   - 30s polling fallback if LISTEN unavailable

4. **Database Schema Changes** (`internal/database/sqlite.go` → PostgreSQL migration)
   - Add columns: `owner_node_id` (task owner), `lease_expires_at` (timestamp)
   - Maintain SQLite for local fallback cache

---

### Phase 1: Extract Shared State from In-Process Queue

**Current Queue State** (in RAM, lost on restart):
- Priority heap: `pq` (`internal/queue/queue.go:49`)
- Running tasks: `running` map (line 53)
- Paused batches: `paused` map (line 55)
- Proxy concurrency: `runningProxied` counter (line 56)

**Target:** Move to Coordinator.

**Changes:**

1. **New Structure:** `internal/queue/coordinator_queue.go`
   ```go
   // CoordinatedQueue wraps the local Queue with coordinator integration
   type CoordinatedQueue struct {
       localQueue    *Queue        // still exists for local execution
       coordinator   Coordinator
       nodeID        string
       leaseCache    map[string]LeaseID
       syncInterval  time.Duration
   }

   // Submit now: (1) enqueue to coordinator, (2) local queue for immediate execution
   func (cq *CoordinatedQueue) Submit(ctx context.Context, task *models.Task) error {
       if err := cq.coordinator.EnqueueTask(ctx, task); err != nil {
           return err
       }
       return cq.localQueue.Submit(ctx, task) // local fast-path
   }

   // Cancel: (1) release lease if owned, (2) update coordinator
   func (cq *CoordinatedQueue) Cancel(ctx context.Context, taskID string) error {
       if leaseID, ok := cq.leaseCache[taskID]; ok {
           cq.coordinator.ReleaseLease(ctx, leaseID)
           delete(cq.leaseCache, taskID)
       }
       return cq.localQueue.Cancel(ctx, taskID)
   }
   ```

2. **Proxy Reservation Coordinator:** `internal/proxy/coordinator_proxy.go`
   ```go
   // CoordinatedManager wraps Manager with global reservation tracking
   type CoordinatedManager struct {
       local       *Manager
       coordinator Coordinator
       nodeID      string
   }

   func (cm *CoordinatedManager) Reserve(ctx context.Context, cfg ProxyFilter) (*Reservation, error) {
       // Get healthy proxy list from DB
       proxies, _ := cm.local.GetHealthyProxies(ctx, cfg)
       // Reserve in coordinator (increments global counter)
       resID, _ := cm.coordinator.ReserveProxy(ctx, proxies[0].ID, cm.nodeID)
       // Return wrapped reservation
       return &Reservation{
           proxy:  proxies[0],
           resID:  resID,
           coord:  cm.coordinator,
       }, nil
   }
   ```

3. **Database Changes:**
   - Add `owner_node_id TEXT` to `tasks` table
   - Add `lease_id TEXT` to track which node owns the task
   - Add `last_heartbeat DATETIME` to track worker liveness

---

### Phase 2: Worker Nodes (Standalone Binary)

**New Binary:** `cmd/worker/main.go`

**Worker responsibilities:**
1. Register with control plane every 10s
2. Poll coordinator for assigned tasks (or listen for NOTIFY)
3. Execute tasks using local browser pool
4. Report status/results back to DB
5. De-register on shutdown

**Architecture:**

```
┌─────────────────────┐
│  flowpilot-worker   │
├─────────────────────┤
│ Config:             │
│  - coordinator_url  │
│  - node_id          │
│  - capacity (4)     │
│  - data_dir         │
│                     │
│ Runtime:            │
│  - Local queue (4)  │
│  - Browser pool (4) │
│  - Heartbeat loop   │
│  - Task poller      │
│  - Status reporter  │
└─────────────────────┘
```

**New Code:**

`cmd/worker/main.go`:
```go
package main

import (
    "context"
    "flag"
    "log"
    "os"
    "os/signal"

    "flowpilot/internal/agent"
    "flowpilot/internal/coordinator"
    "flowpilot/internal/database"
    "flowpilot/internal/queue"
)

func main() {
    coordURL := flag.String("coordinator", "localhost:5432", "Postgres coordinator URL")
    nodeID := flag.String("id", os.Getenv("WORKER_ID"), "Worker node ID")
    capacity := flag.Int("capacity", 4, "Task concurrency")
    flag.Parse()

    // Connect to coordinator
    coord, err := coordinator.NewPostgres(*coordURL)
    if err != nil {
        log.Fatalf("coordinator init: %v", err)
    }
    defer coord.Close()

    // Register worker
    if err := coord.RegisterWorker(context.Background(), *nodeID, "localhost:9090", *capacity); err != nil {
        log.Fatalf("register worker: %v", err)
    }

    // Create agent with coordinator integration
    cfg := agent.Config{
        DataDir:        "/tmp/flowpilot-worker-" + *nodeID,
        MaxConcurrency: *capacity,
        PollInterval:   5 * time.Second, // faster than desktop
    }
    agnt, err := agent.New(cfg)
    if err != nil {
        log.Fatalf("agent init: %v", err)
    }

    // Wrap queue with coordinator
    coordQueue := &queue.CoordinatedQueue{
        localQueue:   agnt.queue,
        coordinator:  coord,
        nodeID:       *nodeID,
    }

    ctx, cancel := context.WithCancel(context.Background())
    go agnt.Run(ctx)
    go heartbeat(ctx, coord, *nodeID)

    // Graceful shutdown
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt)
    <-sigCh
    cancel()
    coord.DeregisterWorker(context.Background(), *nodeID)
}

func heartbeat(ctx context.Context, coord coordinator.Coordinator, nodeID string) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            coord.RegisterWorker(ctx, nodeID, "localhost:9090", 4) // re-register = heartbeat
        case <-ctx.Done():
            return
        }
    }
}
```

**Database Changes:**
- Add `workers` table (node_id, address, capacity, last_heartbeat, status)
- Add task indices: `(owner_node_id, status, priority)` for fast lookup

---

### Phase 3: Control Plane (Master Node)

**Responsibilities:**
1. Monitor worker health (prune dead nodes if no heartbeat > 30s)
2. Task assignment logic (find least-loaded worker matching constraints)
3. Lease management (renew on task progress, expire on timeout)
4. Audit trail & compliance logging
5. Metrics aggregation (throughput, latency, error rates)

**New Binary:** `cmd/master/main.go`

**Control Plane Loop (every 10s):**

```
1. Prune dead workers: DELETE FROM workers WHERE last_heartbeat < now() - 30s
2. Check for stale leases: SELECT * FROM task_leases WHERE expires_at < now()
   → For each expired: reassign to available worker OR mark task as failed
3. Assignment: SELECT COUNT(*) FROM tasks WHERE owner_node_id = $worker_id
   → Find worker with capacity < limit
   → Assign pending task via UPDATE tasks SET owner_node_id = $worker_id
   → Create lease record
4. Emit metrics: gauge(tasks_pending), gauge(workers_active), counter(tasks_completed)
```

**Code Structure:**

`internal/control_plane/scheduler.go`:
```go
package control_plane

type Scheduler struct {
    coordinator Coordinator
    db          *database.DB
    interval    time.Duration
}

func (s *Scheduler) Run(ctx context.Context) error {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            s.reconcile(ctx)
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func (s *Scheduler) reconcile(ctx context.Context) {
    // 1. Prune dead workers
    s.db.Exec(`DELETE FROM workers WHERE last_heartbeat < NOW() - '30 seconds'::interval`)

    // 2. Detect stale leases and reassign
    stale, _ := s.db.ListExpiredLeases(ctx)
    for _, lease := range stale {
        s.db.UpdateTaskStatus(ctx, lease.TaskID, models.TaskStatusPending, "")
        s.coordinator.ReleaseLease(ctx, lease.ID)
    }

    // 3. Assign tasks to workers
    workers, _ := s.coordinator.GetWorkers(ctx)
    pending, _ := s.db.ListTasksByStatus(ctx, models.TaskStatusPending)

    for _, task := range pending {
        worker := s.findBestWorker(ctx, workers, task)
        if worker != nil {
            leaseID, _ := s.coordinator.AcquireLease(ctx, "task-"+task.ID, 30*time.Second)
            s.db.UpdateTaskStatus(ctx, task.ID, models.TaskStatusQueued, leaseID)
            // Worker will see the assignment and start execution
        }
    }
}

func (s *Scheduler) findBestWorker(ctx context.Context, workers []WorkerInfo, task *models.Task) *WorkerInfo {
    // First-fit: find worker with lowest load
    for _, w := range workers {
        count, _ := s.db.CountTasksByNodeAndStatus(ctx, w.NodeID, models.TaskStatusRunning)
        if count < w.Capacity {
            return &w
        }
    }
    return nil
}
```

---

### Phase 4: Remote Browser Support (CDP over WebSocket)

**Current:** Each worker runs embedded Chrome locally (`internal/browser/pool.go`).

**Goal:** Support remote Chrome/Playwright servers (e.g., browserless.io, Playwright Grid, BrowserStack).

**New Interface:** `internal/browser/remote.go`
```go
package browser

// RemoteBrowserProvider abstracts remote execution
type RemoteBrowserProvider interface {
    // Launch returns a context connected to remote Chrome via CDP/WS
    Launch(ctx context.Context) (context.Context, error)
    Close(ctx context.Context) error
    Health(ctx context.Context) error
}

// BrowserlessProvider implements RemoteBrowserProvider for browserless.io
type BrowserlessProvider struct {
    endpoint string // ws://browserless.io:3000
    apiKey   string
}

func (b *BrowserlessProvider) Launch(ctx context.Context) (context.Context, error) {
    // POST /session/launch?token=apiKey → returns WS URL
    // Return chromedp.NewContext(context.Background(), chromedp.WithDialerURL(wsURL))
}
```

**Worker Config (Phase 4):**
```yaml
browser:
  type: "remote"  # or "local"
  provider: "browserless"
  endpoint: "https://api.browserless.io"
  api_key: "${BROWSERLESS_API_KEY}"
```

**No changes to executor logic** — chromedp abstracts both local and remote execution.

---

### Phase 5: Fault Tolerance

**Challenges:**

| Scenario | Current | Distributed Fix |
|----------|---------|-----------------|
| Worker crashes mid-task | Task stuck in `running` | Lease TTL expires → reassign |
| Network partition (worker ↔ coordinator) | No recovery | Lease expires, task reassigned |
| Coordinator unavailable | N/A | Worker falls back to local queue |
| Task timeout (infinite loop) | Manual kill | Timeout + lease expiry → reassign |
| Duplicate execution | Impossible (single worker) | At-least-once: idempotent retry ID |

**Implementations:**

1. **Lease-based Task Ownership** (Phase 1+)
   - Task has `owner_node_id` + `lease_id` + `lease_expires_at`
   - Worker holds lease while executing; renews every 5s
   - If lease expires, control plane reassigns
   - Prevents duplicate execution (SELECT ... FOR UPDATE)

2. **Idempotent Task Execution**
   - Add `retry_token TEXT` (UUID) to tasks table
   - Worker includes `retry_token` in status updates
   - DB constraint: `UNIQUE(task_id, retry_token)` prevents duplicate log entries
   - Code in `internal/queue/queue.go:persistenceWorker()` already writes atomically

3. **Graceful Worker Shutdown** (Phase 2)
   - Worker catches SIGTERM → calls `coord.ReleaseLease()` on all tasks
   - Coordinator reassigns tasks within 1-2 seconds
   - New worker picks up task mid-execution (browser state is transient)

4. **Split-Brain Prevention**
   - Use Postgres advisory locks for critical operations
   - "Task assignment" uses `SELECT ... FOR UPDATE` on tasks table
   - Serializable isolation level for consistency

---

### Phase 6: Horizontal Scaling API

**New Endpoints (on control plane):**

```go
// POST /workers/scale
// Request: {"count": 10, "capacity": 4, "region": "us-east-1"}
// Spawns 10 worker containers (K8s, Docker, etc.)
POST /workers/scale → Trigger orchestration (K8s deployment, CloudFormation, etc.)

// GET /workers
// Response: list of active workers with current load
GET /workers → [{id: "worker-1", capacity: 4, running: 2, last_seen: "2025-01-15T10:00:00Z"}]

// DELETE /workers/{id}
// Gracefully drain tasks before shutting down
DELETE /workers/{id} → Cordon node, wait for in-flight tasks

// GET /metrics
// Dashboard data: throughput, latency, error rate per worker
GET /metrics → {throughput_tasks_per_min: 150, avg_latency_ms: 5000, error_rate: 0.02}
```

**Integration Points:**
- Kubernetes: worker pod templates + HPA (Horizontal Pod Autoscaler)
- Docker Swarm: service scaling
- Cloud: Lambda (for short bursts), EC2 (for sustained)
- Manual: API calls for ops

---

## Data Consistency Model

### Guarantees

| Operation | Consistency | Mechanism |
|-----------|-------------|-----------|
| **Task Enqueue** | At-least-once | Idempotent by task ID |
| **Task Execution** | At-most-once per lease | Lease prevents duplicate ownership |
| **Status Update** | Atomic | Single DB write with version check |
| **Proxy Reservation** | Linearizable | Postgres advisory lock |
| **Worker Registration** | Eventual | Heartbeat gossip |

### Conflict Resolution

**Scenario: Two workers claim same task**
- DB has `owner_node_id` column with unique constraint per task
- First UPDATE wins: `UPDATE tasks SET owner_node_id=$1 WHERE id=$2 AND owner_node_id IS NULL`
- Loser retries next task

**Scenario: Task status race (running → completed vs running → failed)**
- Use optimistic locking: `UPDATE tasks SET status=$1, version=version+1 WHERE id=$2 AND version=$3`
- Retry on conflict

**Scenario: Coordinator down, workers have stale lease cache**
- Workers fall back to local execution (enqueue, run, report status)
- On coordinator recovery, status updates are idempotent (same task ID, same result)

---

## Fault Tolerance

### Worker Crash Recovery

```
Worker crashes while running task T1

T=0: Worker1 acquires lease(T1, TTL=30s)
T=15: Task running, worker renews lease
T=25: CRASH (no renewal)
T=30: Lease expires; control plane detects
T=31: Control plane reassigns T1 to Worker2
T=32: Worker2 polls, sees T1 assigned to it, executes
```

**Implementation:**
- `internal/coordinator/postgres.go`: goroutine monitors lease expiry
- `cmd/master/main.go` Scheduler: periodic task reassignment
- Each worker renews lease every 5s while executing (middleware in `internal/queue/queue.go:executeTask()`)

### Network Partition Recovery

**Scenario: Worker ↔ Coordinator network cut for 60s**

1. Worker **cannot** reach coordinator
   - Local queue continues executing buffered tasks
   - Status updates queued in memory (ring buffer)
   - On reconnect, replay buffered updates to DB

2. Coordinator **cannot** reach worker
   - Heartbeat timeout triggers lease expiry (30s)
   - Task reassigned to healthy worker
   - Reassigned worker may start task again (idempotent retry token)

3. After partition heals
   - Worker sends buffered updates; DB deduplicates by retry token
   - Coordinator sees task completed; cancels duplicate assignment

### Split-Brain Prevention

**Postgres advisory locks** (not needed for MVP, but documented for Phase 5+):

```go
// Control plane exclusive access to assignment decision
SELECT pg_advisory_xact_lock(hashFunctionConsistentHash('task-assignment'))

// Within transaction:
SELECT COUNT(*) FROM workers WHERE status='healthy'
SELECT * FROM tasks WHERE status='pending' ORDER BY priority LIMIT 10

// Atomically update assignments
UPDATE tasks SET owner_node_id=$1, status='queued' WHERE id=$2

COMMIT // Release lock
```

---

## Testing Strategy

### Unit Tests (no changes)
- `internal/queue/queue_test.go` — existing
- `internal/proxy/manager_test.go` — existing

### Integration Tests (new)

`internal/coordinator/coordinator_test.go`:
```go
func TestLeaseRenewal(t *testing.T) {
    coord := setupTestCoordinator(t)
    defer coord.Close()

    leaseID, _ := coord.AcquireLease(context.Background(), "test-key", 2*time.Second)
    time.Sleep(1 * time.Second)
    coord.RenewLease(context.Background(), leaseID, 2*time.Second)
    time.Sleep(2 * time.Second)
    // Lease should still be valid
}

func TestTaskReassignmentOnLeaseExpiry(t *testing.T) {
    // Enqueue task, acquire lease, let it expire
    // Verify task is reassigned to second worker
}
```

`internal/control_plane/scheduler_test.go`:
```go
func TestSchedulerAssignment(t *testing.T) {
    // Setup: 3 workers, 5 pending tasks
    // Run scheduler.reconcile()
    // Verify tasks assigned to workers with available capacity
}

func TestSchedulerDeadWorkerDetection(t *testing.T) {
    // Create worker with stale heartbeat
    // Run scheduler.reconcile()
    // Verify worker marked as inactive
}
```

### End-to-End Tests (new)

`cmd/worker/integration_test.go`:
```bash
# Start: coordinator (Postgres), control plane, worker1, worker2
# Submit 10 tasks
# Verify all complete
# Kill worker1 mid-task
# Verify task reassigned to worker2 and completes
# Metrics should show: 10 completed, 0 failed
```

### Chaos Testing (Phase 5+)
- Randomly kill workers mid-task
- Pause coordinator for 30s
- Corrupt lease records
- Verify no task loss, no duplicate execution

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|-----------|
| **Coordinator SPOF** | All workers blocked | Multi-region Postgres (HA), fallback to local mode |
| **Lease storms** (many expiries at once) | Control plane overload | Staggered TTLs, exponential backoff in reassignment |
| **Task duplication** | Same URL clicked twice | Idempotent retry token; DB constraint UNIQUE(task_id, retry_token) |
| **Slow DB queries** | Scheduler bottleneck | Index on (owner_node_id, status, priority), connection pooling |
| **Worker data dir issues** | Task execution fails | Pre-flight check in Phase 2; fail-fast if directory unavailable |
| **Proxy reservation conflicts** | Multiple workers same proxy | Postgres lock, global counter in `proxy_reservations` table |
| **Old workers orphaned** | Stale data, lease conflicts | Unique node ID (UUID), 24h lease max, cleanup on startup |

---

## Implementation Sequence (Recommended)

```
Phase 0 (Week 1)  → Coordinator abstraction + Postgres driver
                     Fallback file-based coordination
                     
Phase 1 (Week 2)  → Extract queue & proxy state to coordinator
                     Unit tests for CoordinatedQueue
                     
Phase 2 (Week 3)  → Build flowpilot-worker binary
                     Worker registration & polling
                     Local execution unchanged
                     
Phase 3 (Week 4)  → Control plane scheduler
                     Worker health monitoring
                     Task assignment logic
                     
Phase 4 (Week 5)  → Remote browser provider abstraction
                     browserless.io / Playwright Grid support
                     
Phase 5 (Week 6)  → Fault tolerance (leases, retries, idempotency)
                     Chaos testing
                     
Phase 6 (Week 7)  → Horizontal scaling API
                     Kubernetes integration
                     Dashboard metrics
```

---

## Success Criteria

### Functional
- [ ] Single control plane + 5 worker nodes execute 1000 tasks without data loss
- [ ] Worker crash mid-task → task reassigned and completes within 60s
- [ ] Network partition (60s) → workers continue locally, sync on reconnect
- [ ] Task duplication rate < 0.1% (measured over 10K tasks)
- [ ] Proxy concurrency limits enforced across all nodes (no more than N simultaneously)

### Performance
- [ ] Task assignment latency < 500ms (control plane decision)
- [ ] Worker heartbeat overhead < 5% CPU
- [ ] Scaling from 1 worker → 10 workers (9x throughput increase within 90%)

### Operational
- [ ] Add worker without downtime (drain old, spawn new)
- [ ] Coordinator failover < 5min (manual, documented)
- [ ] Audit trail complete (task_events table all transitions)
- [ ] Metrics exported (Prometheus format): tasks_queued, tasks_completed, error_rate

---

## API Changes Summary

### New Interfaces

| File | Type | Purpose |
|------|------|---------|
| `internal/coordinator/coordinator.go` | Interface | Abstract distributed state |
| `internal/coordinator/postgres.go` | Struct | Postgres implementation |
| `internal/coordinator/file.go` | Struct | File-based fallback |
| `internal/queue/coordinator_queue.go` | Struct | Wraps Queue with coordination |
| `internal/proxy/coordinator_proxy.go` | Struct | Wraps Manager with coordination |
| `internal/browser/remote.go` | Interface | Remote Chrome provider |
| `internal/control_plane/scheduler.go` | Struct | Master node scheduling |
| `cmd/worker/main.go` | Binary | Worker process |
| `cmd/master/main.go` | Binary | Control plane process |

### Database Schema Additions

```sql
-- Postgres only (Phase 1)
ALTER TABLE tasks ADD COLUMN owner_node_id TEXT;
ALTER TABLE tasks ADD COLUMN lease_id TEXT;
ALTER TABLE tasks ADD COLUMN lease_expires_at TIMESTAMP;
ALTER TABLE tasks ADD COLUMN retry_token TEXT;
ALTER TABLE tasks ADD UNIQUE(id, retry_token);

CREATE TABLE coordinators (
    node_id TEXT PRIMARY KEY,
    address TEXT,
    capacity INT,
    last_heartbeat TIMESTAMP,
    status TEXT
);

CREATE TABLE task_leases (
    id TEXT PRIMARY KEY,
    task_id TEXT REFERENCES tasks(id),
    owner_node_id TEXT REFERENCES coordinators(node_id),
    acquired_at TIMESTAMP,
    expires_at TIMESTAMP
);

CREATE TABLE proxy_reservations (
    id TEXT PRIMARY KEY,
    proxy_id TEXT REFERENCES proxies(id),
    owner_node_id TEXT REFERENCES coordinators(node_id),
    reserved_at TIMESTAMP,
    released_at TIMESTAMP
);

CREATE INDEX idx_tasks_owner_status_priority ON tasks(owner_node_id, status, priority);
CREATE INDEX idx_task_leases_expires_at ON task_leases(expires_at);
```

### Wails Frontend (No Changes)
- Control plane scheduler can be run headless
- Worker nodes are CLI only (no Wails required)
- Optional: dashboard UI to monitor workers/metrics (future)

---

## References to Existing Code

- Queue state extraction: `internal/queue/queue.go:29-61` (pq, running, paused, heapSet)
- Proxy reservation tracking: `internal/proxy/manager.go:59` (activeReservations)
- Browser pool local-only: `internal/browser/pool.go:27-40`, `pool.go:98-110` (Acquire)
- Agent polling: `internal/agent/agent.go:125-147` (Run loop)
- Persistence: `internal/queue/queue.go:persistenceWorker()` (flush every 50ms)
- DB schema: `internal/database/sqlite.go:68-290` (migrate function)
- Task status enum: `internal/models/task.go:4-15`

