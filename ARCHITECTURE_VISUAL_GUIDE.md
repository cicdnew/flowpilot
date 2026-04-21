# FlowPilot Execution Architecture - Visual Reference

## 1. COMPLETE TASK EXECUTION FLOW (With File References)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      USER ACTION - CREATE TASK                              │
│                    (Frontend: CreateTaskModal.svelte)                       │
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ App.CreateTask(params) [app_tasks.go:55-89]                                 │
│ ├─ Validate params (app_tasks.go:35-52)                                     │
│ │  ├─ ValidateTask() [internal/validation]                                  │
│ │  ├─ ValidateProxyConfig() [internal/validation]                           │
│ │  └─ ValidateTaskLoggingPolicy() [internal/validation]                     │
│ ├─ Create Task with UUID [google/uuid]                                      │
│ │  ├─ ID: uuid.New().String()                                               │
│ │  ├─ Status: TaskStatusPending                                             │
│ │  ├─ MaxRetries: DefaultMaxRetries (5)                                     │
│ │  └─ CreatedAt: time.Now()                                                 │
│ ├─ Store in DB [app_tasks.go:78]                                            │
│ │  └─ App.db.CreateTask(ctx, task)                                          │
│ │     └─ [internal/database/db_tasks.go:194-239]                            │
│ │        ├─ Encrypt proxy creds [internal/crypto]                           │
│ │        ├─ Marshal steps to JSON                                           │
│ │        └─ INSERT tasks table                                              │
│ └─ Auto-start? [app_tasks.go:82-86]                                         │
│    └─ If AutoStart=true: Queue.Submit()                                     │
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │
                        ┌──────────────┴──────────────┐
                        │                             │
            (AutoStart=false)              (AutoStart=true)
                        │                             │
            Return [PENDING]            Queue.Submit()
                        │                             │
                        │                             ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ MANUAL START (User clicks "Start")                                          │
│ App.StartTask(id) [app_tasks.go:128-140]                                    │
│ ├─ GetTask(id)                                                              │
│ └─ Queue.Submit(ctx, task) [internal/queue/queue.go:206-228]               │
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Queue.Submit(ctx, task) [internal/queue/queue.go:206-228]                  │
│ ├─ Validate [queue.go:175-189]                                              │
│ │  ├─ Check: not stopped                                                    │
│ │  ├─ Check: task not already running                                       │
│ │  ├─ Check: task not already pending                                       │
│ │  └─ Check: queue not full (len < maxPending)                              │
│ │                                                                           │
│ ├─ Add to priority heap [queue.go:192-204]                                  │
│ │  ├─ Create heapItem with context.WithCancel()                             │
│ │  ├─ heap.Push(&q.pq, item)                                                │
│ │  ├─ Track in heapSet for O(1) lookup                                      │
│ │  └─ Increment q.metrics.TotalSubmitted                                    │
│ │                                                                           │
│ ├─ Enqueue DB state change [queue.go:216]                                   │
│ │  └─ q.enqueueTaskStateChange(TaskStatusQueued)                            │
│ │     └─ [internal/database/db_tasks.go]                                    │
│ │        └─ Write to persistence channel (batched every 100ms)             │
│ │                                                                           │
│ ├─ Emit event [queue.go:223]                                                │
│ │  ├─ q.emitEvent(taskID, TaskStatusQueued, "")                             │
│ │  └─ [app.go:206-208] → Wails IPC: EventsEmit('task:event')               │
│ │     └─ Frontend listener [App.svelte:146-156] updates store               │
│ │                                                                           │
│ └─ Signal workers [queue.go:226]                                            │
│    └─ q.cond.Signal() ← Wakes one blocked worker                            │
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │
                                       ▼
        ┌──────────────────────────────────────────────────────┐
        │ [QUEUED STATE - IN PRIORITY HEAP]                    │
        │ Task waiting for worker thread                       │
        └──────────────────────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Queue Worker Loop [internal/queue/queue.go:620-638]                        │
│ (Fixed pool: default 200 workers, staggered 50ms warmup)                    │
│                                                                             │
│ func (q *Queue) worker(id int):                                             │
│ ├─ defer q.workerWg.Done()                                                  │
│ └─ for {                                                                    │
│    ├─ q.mu.Lock()                                                           │
│    ├─ for !q.stopped {                                                      │
│    │  ├─ item, countsAgainstProxy, autoProxy := dequeueRunnableLocked()    │
│    │  │  [queue.go:640-669]                                                 │
│    │  │  ├─ Pop from pq heap (highest priority first)                       │
│    │  │  ├─ Skip if cancelled [queue.go:677-684]                            │
│    │  │  ├─ Move to pausedPQ if batch paused [queue.go:686-693]            │
│    │  │  ├─ Check proxy concurrency limit [queue.go:701-703]               │
│    │  │  │  └─ If limit reached: defer item, schedule wake timer           │
│    │  │  ├─ Check proxy geo-availability [queue.go:705-720]                │
│    │  │  │  └─ If unavailable: defer item, wait for proxy                  │
│    │  │  └─ Mark running [queue.go:729-734]                                │
│    │  │     ├─ q.running[taskID] = cancel                                   │
│    │  │     └─ q.runningProxied++ if proxy-counted                          │
│    │  │                                                                     │
│    │  ├─ if item != nil:                                                    │
│    │  │  ├─ q.mu.Unlock()                                                   │
│    │  │  └─ q.executeTask(...) [queue.go:736-780] ← SEE NEXT BLOCK         │
│    │  │                                                                     │
│    │  └─ else: q.cond.Wait() ← BLOCK until task available                  │
│    │}                                                                       │
│    └─}                                                                      │
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Queue.executeTask() [internal/queue/queue.go:736-780]                      │
│                                                                             │
│ ├─ Check not cancelled [queue.go:738-740]                                   │
│ │  └─ if cancelled by user: return                                          │
│ │                                                                           │
│ ├─ Create task context with timeout [queue.go:742-743]                      │
│ │  └─ taskCtx, cancel := WithTimeout(ctx, task.Timeout)                     │
│ │                                                                           │
│ ├─ Handle auto-proxy [queue.go:751-754]                                     │
│ │  ├─ If task.Proxy.Server == "" && Geo != "":                              │
│ │  │  └─ proxyManager.Reserve() [internal/proxy]                            │
│ │  └─ Update task.Proxy with selected proxy                                 │
│ │                                                                           │
│ ├─ Update DB: status=RUNNING [queue.go:756]                                 │
│ │  └─ q.enqueueTaskStateChange(TaskStatusRunning)                           │
│ │                                                                           │
│ ├─ Emit RUNNING event [queue.go:760]                                        │
│ │  └─ q.emitEvent(taskID, TaskStatusRunning, "")                            │
│ │                                                                           │
│ └─ EXECUTE: result, err := q.runner.RunTask(taskCtx, task) [queue.go:762]  │
│    └─ [internal/browser/browser.go:295-373] ← SEE NEXT BLOCK              │
│                                                                             │
│ After RunTask returns:                                                      │
│ ├─ Complete proxy reservation [queue.go:763]                                │
│ ├─ if err == nil:                                                           │
│ │  ├─ q.handleSuccess(...) [queue.go:765]                                   │
│ │  │  ├─ Update DB: status=COMPLETED, success=true                          │
│ │  │  ├─ Emit COMPLETED event                                               │
│ │  │  └─ Return                                                             │
│ │  └─ return                                                                │
│ │                                                                           │
│ ├─ else if cancelled:                                                       │
│ │  ├─ q.handleTaskCancellation(...) [queue.go:768]                          │
│ │  ├─ Update DB: status=CANCELLED                                           │
│ │  └─ Emit CANCELLED event                                                  │
│ │                                                                           │
│ └─ else (error):                                                            │
│    ├─ retry := q.handleFailure(ctx, task, err, result) [queue.go:772]      │
│    │  ├─ Update DB: status=FAILED, error set                                │
│    │  ├─ Emit FAILED event                                                  │
│    │  ├─ Determine if should retry [queue.go:804-847]                       │
│    │  │  ├─ Check: task.RetryCount < task.MaxRetries                        │
│    │  │  └─ Calculate backoff: baseMs * 2^retryCount (max 5min)             │
│    │  └─ Return retryInfo{shouldRetry, task, backoff}                       │
│    │                                                                         │
│    └─ if retry.shouldRetry:                                                 │
│       ├─ q.workerWg.Add(1)                                                  │
│       ├─ go q.scheduleRetry(ctx, retry) [queue.go:777-778]                 │
│       │  ├─ Wait backoff duration                                           │
│       │  ├─ Increment task.RetryCount                                       │
│       │  └─ Queue.Submit() ← Re-enter queue                                 │
│       └─ q.workerWg.Done()                                                  │
│                                                                             │
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │
                                       ▼
        ┌──────────────────────────────────────────────────────┐
        │ Task waits for worker pool                           │
        │ (May go through multiple retries with backoff)       │
        └──────────────────────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Browser.RunTask(ctx, task) [internal/browser/browser.go:295-373]          │
│                                                                             │
│ ├─ Setup: resolve logging policy [browser.go:297]                           │
│ │  └─ Runner.resolveLoggingPolicy(task) [browser.go:199-236]              │
│ │     ├─ Defaults: captureStepLogs=true, captureNetworkLogs=true          │
│ │     ├─ Override with default policy from Runner                           │
│ │     └─ Override with task-specific policy                                │
│ │                                                                           │
│ ├─ Create result struct [browser.go:298-302]                                │
│ │  └─ TaskResult{TaskID, ExtractedData{}, LogLimit}                        │
│ │                                                                           │
│ ├─ Panic recovery [browser.go:305-318]                                      │
│ │  └─ defer: catch chromedp panics (e.g., "close of closed channel")      │
│ │                                                                           │
│ ├─ Get/Set pools [browser.go:323-326]                                       │
│ │  └─ basePool := r.pool (shared browser pool)                              │
│ │                                                                           │
│ ├─ Resolve proxy [browser.go:328-335]                                       │
│ │  ├─ effectiveProxy := task.Proxy                                          │
│ │  ├─ If localProxyManager: map to local proxy endpoint                     │
│ │  └─ Fall back to upstream if unavailable                                  │
│ │                                                                           │
│ ├─ Acquire browser context [browser.go:337-341]                             │
│ │  └─ Runner.acquireBrowserContext(...) [browser.go:240-266]              │
│ │     ├─ Try to reuse from pool [pool.go:88-139]                            │
│ │     │  ├─ Select least-used browser (load-balanced)                       │
│ │     │  ├─ Increment inUse counter                                         │
│ │     │  └─ Return existing tab context                                     │
│ │     ├─ OR create new allocator if no available pool                       │
│ │     │  └─ Runner.createAllocator(...) [browser.go:375-414]              │
│ │     │     ├─ Copy default chromedp options                                │
│ │     │     ├─ Set headless mode (or force if configured)                   │
│ │     │     ├─ Disable GPU, background networking, etc.                     │
│ │     │     ├─ Set proxy if configured                                      │
│ │     │     └─ Return ExecAllocator context                                 │
│ │     └─ Return release() func to cleanup context                           │
│ │                                                                           │
│ ├─ Setup network logging [browser.go:343]                                   │
│ │  └─ Runner.setupNetworkLogging(...) [browser.go:268-292]                │
│ │     ├─ Create NetworkLogger [internal/logs/network.go:30-39]             │
│ │     ├─ Register CDP event listener [browser.go:275-286]                  │
│ │     │  ├─ EventRequestWillBeSent: record start time                       │
│ │     │  ├─ EventResponseReceived: store response                           │
│ │     │  ├─ EventLoadingFinished: build network log                         │
│ │     │  └─ EventLoadingFailed: cleanup tracking                            │
│ │     ├─ Enable network domain via CDP [browser.go:288]                     │
│ │     └─ Return NetworkLogger                                               │
│ │                                                                           │
│ ├─ Clear cookies [browser.go:345-347]                                       │
│ │  └─ ClearCookies(ctx) [browser.go:717-719]                              │
│ │     └─ chromedp.Run(..., network.ClearBrowserCookies())                   │
│ │                                                                           │
│ ├─ Setup proxy auth [browser.go:349-356]                                    │
│ │  └─ Runner.setupProxyAuth(...) [browser.go:677-691]                     │
│ │     ├─ Register fetch.EventAuthRequired listener                          │
│ │     ├─ Register fetch.EventRequestPaused listener                         │
│ │     └─ Enable fetch domain interceptor                                    │
│ │                                                                           │
│ ├─ EXECUTE STEPS [browser.go:358]                                           │
│ │  └─ Runner.runSteps(...) [browser.go:418-632] ← SEE NEXT BLOCK           │
│ │                                                                           │
│ ├─ Collect logs [browser.go:359-368]                                        │
│ │  ├─ result.NetworkLogs = netLogger.Logs()                                 │
│ │  ├─ result.StepLogs = stepLogger.Logs()                                   │
│ │  └─ (Screenshots already appended during execution)                       │
│ │                                                                           │
│ ├─ Set success/duration [browser.go:369-371]                                │
│ │  ├─ result.Success = true                                                 │
│ │  └─ result.Duration = time.Since(start)                                   │
│ │                                                                           │
│ └─ Return result [browser.go:372]                                           │
│    └─ return result, nil                                                    │
│                                                                             │
│ ERROR RETURN: if runSteps errors [browser.go:358-364]                       │
│ ├─ Set network logs                                                         │
│ ├─ Set duration and error message                                           │
│ └─ return result, err                                                       │
│                                                                             │
│ CLEANUP (defer): [browser.go:341]                                           │
│ └─ browserCancel() ← Releases browser context / pool tab                    │
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Browser.runSteps() [internal/browser/browser.go:418-632]                   │
│ PC-BASED CONTROL FLOW ENGINE                                                │
│                                                                             │
│ ├─ Setup step logger [browser.go:419-423]                                   │
│ │  └─ if policy.captureStepLogs:                                            │
│ │     └─ stepLogger := logs.NewStepLogger(taskID)                           │
│ │                                                                           │
│ ├─ Build label index for goto targets [browser.go:430]                      │
│ │  └─ labelIndex := map[string]int{label: pcValue, ...}                     │
│ │                                                                           │
│ ├─ Initialize stacks [browser.go:432-445]                                   │
│ │  ├─ loopStack: for nested loop tracking                                   │
│ │  └─ whileStack: for while loop tracking                                   │
│ │                                                                           │
│ ├─ MAIN LOOP: for pc := 0; pc < len(steps) [browser.go:450]                │
│ │                                                                           │
│ │  ├─ Set network logger step context [browser.go:452-454]                  │
│ │  │  └─ netLogger.SetStepIndex(pc)                                         │
│ │  │                                                                       │
│ │  ├─ Log step start [browser.go:455]                                       │
│ │  │  └─ addLog(result, "info", "step X: ACTION")                           │
│ │  │                                                                       │
│ │  ├─ CONTROL FLOW HANDLERS (no executeStep, direct pc manipulation):      │
│ │  │                                                                       │
│ │  │  ├─ ActionLoop [browser.go:458-466]                                    │
│ │  │  │  ├─ Parse maxIter from step.Value                                   │
│ │  │  │  ├─ loopStack.push({startPC: pc, maxIter, currentIter: 0})         │
│ │  │  │  └─ pc++ ← Move past loop header                                    │
│ │  │  │                                                                     │
│ │  │  ├─ ActionEndLoop [browser.go:468-480]                                 │
│ │  │  │  ├─ top := &loopStack[len-1]                                        │
│ │  │  │  ├─ top.currentIter++                                               │
│ │  │  │  ├─ if currentIter < maxIter: pc = top.startPC+1 ← Jump back       │
│ │  │  │  └─ else: loopStack.pop(), pc++ ← Exit loop                         │
│ │  │  │                                                                     │
│ │  │  ├─ ActionWhile [browser.go:482-502]                                   │
│ │  │  │  ├─ Evaluate condition [browser.go:487]                             │
│ │  │  │  │  └─ r.evaluateCondition(...) [conditional.go]                    │
│ │  │  │  ├─ if condMet: whileStack.push(), pc++                             │
│ │  │  │  └─ else: pc = findEndWhile(steps, pc)+1 ← Skip block               │
│ │  │  │                                                                     │
│ │  │  ├─ ActionEndWhile [browser.go:504-527]                                │
│ │  │  │  ├─ top := &whileStack[len-1]                                       │
│ │  │  │  ├─ top.itersDone++                                                 │
│ │  │  │  ├─ if itersDone >= maxIter: pop, pc++ ← Exit while               │
│ │  │  │  ├─ else: reevaluate condition                                      │
│ │  │  │  │  ├─ if still true: pc = top.startPC+1 ← Jump back               │
│ │  │  │  │  └─ if false: pop, pc++ ← Exit                                   │
│ │  │  │  │                                                                  │
│ │  │  ├─ ActionBreakLoop [browser.go:529-539]                               │
│ │  │  │  ├─ loopStack.pop()                                                 │
│ │  │  │  └─ pc = findEndLoop(...) + 1 ← Jump past loop                      │
│ │  │  │                                                                     │
│ │  │  ├─ ActionGoto [browser.go:541-547]                                    │
│ │  │  │  └─ pc = labelIndex[step.JumpTo] ← Unconditional jump               │
│ │  │  │                                                                     │
│ │  │  └─ ActionIfElement/IfText/IfURL [browser.go:549-565]                 │
│ │  │     ├─ Evaluate condition                                              │
│ │  │     ├─ if true && step.JumpTo: pc = labelIndex[...] ← Jump            │
│ │  │     └─ else: pc++ ← Normal progression                                 │
│ │  │                                                                       │
│ │  └─ NORMAL STEP EXECUTION:                                                │
│ │     ├─ Determine timeout [browser.go:567-570]                             │
│ │     │  └─ timeout = step.Timeout or 30s default                           │
│ │     │                                                                     │
│ │     ├─ Skip screenshots if disabled [browser.go:571-575]                  │
│ │     │  └─ if action==ActionScreenshot && !policy.captureScreenshots       │
│ │     │                                                                     │
│ │     ├─ Record start time [browser.go:577-580]                             │
│ │     │  └─ startedAt := stepLogger.StartStep(...)                          │
│ │     │                                                                     │
│ │     ├─ Execute with timeout [browser.go:581-583]                          │
│ │     │  ├─ stepCtx, cancel := WithTimeout(browserCtx, timeout)            │
│ │     │  ├─ err := r.executeStep(stepCtx, step, result)                     │
│ │     │  │  └─ [steps.go:23-132] Dispatch to action handler                │
│ │     │  └─ cancel()                                                        │
│ │     │                                                                     │
│ │     ├─ Log step completion [browser.go:585-600]                           │
│ │     │  ├─ if stepLogger:                                                  │
│ │     │  │  ├─ code := ClassifyError(err) [models.go]                       │
│ │     │  │  └─ stepLogger.EndStep({...})                                    │
│ │     │  │     ├─ Append to stepLogs[]                                      │
│ │     │  │     ├─ Record duration                                           │
│ │     │  │     └─ Record error code if failed                               │
│ │     │  │                                                                  │
│ │     ├─ Error handling [browser.go:602-612]                                │
│ │     │  ├─ if err != nil:                                                  │
│ │     │  │  ├─ result.Error = formatted error                               │
│ │     │  │  ├─ r.addLog("error", ...)                                       │
│ │     │  │  ├─ logs.Logger.Error(...) [slog]                                │
│ │     │  │  └─ return err ← EXIT runSteps on error                          │
│ │     │  │                                                                  │
│ │     ├─ Debug pause handling [browser.go:614-618]                          │
│ │     │  └─ r.debugCtrl.waitIfPaused(browserCtx)                            │
│ │     │                                                                     │
│ │     ├─ Handle variable extraction [browser.go:620-626]                    │
│ │     │  └─ if action==ActionExtract && step.VarName:                       │
│ │     │     └─ vars[step.VarName] = result.ExtractedData[...]               │
│ │     │                                                                     │
│ │     ├─ Log step success [browser.go:628]                                  │
│ │     │  └─ addLog("info", "step X completed")                              │
│ │     │                                                                     │
│ │     └─ Increment PC [browser.go:629]                                      │
│ │        └─ pc++                                                            │
│ │                                                                           │
│ └─ END OF LOOP: return nil (success) [browser.go:631]                       │
│    All steps completed without error                                        │
│                                                                             │
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │
                                       ▼
        ┌──────────────────────────────────────────────────────┐
        │ Task completes: Success or Failed                    │
        │ Result returned to Queue.executeTask()               │
        └──────────────────────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Back to Queue.executeTask() [queue.go:762-779]                             │
│ Handle result and cleanup                                                   │
│                                                                             │
│ ├─ finishExecuteTask() [queue.go:782-791]                                   │
│ │  ├─ q.mu.Lock()                                                           │
│ │  ├─ delete(q.running, taskID)                                             │
│ │  ├─ if countsAgainstProxyLimit: q.runningProxied--                        │
│ │  ├─ q.mu.Unlock()                                                         │
│ │  └─ q.cond.Broadcast() ← Signal all workers                               │
│ │                                                                           │
│ └─ Return (continue to worker loop) [queue.go:637]                          │
│    Worker goes back to waiting for next task                                │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. BROWSER POOL MANAGEMENT DIAGRAM

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                     BrowserPool Architecture                                 │
│               [internal/browser/pool.go & browser.go]                        │
└──────────────────────────────────────────────────────────────────────────────┘

NewBrowserPool(cfg PoolConfig)
│
├─ cfg.Size = 5 (default), max 200
├─ cfg.MaxTabs = 10 (tabs per browser)
├─ cfg.IdleTimeout = 5 minutes
├─ cfg.AcquireTimeout = 60 seconds
│
└─ Initialize:
   ├─ browsers []*pooledBrowser = [] (grows to cfg.Size)
   ├─ cleanupLoop() goroutine (runs every 30s)
   └─ return *BrowserPool


                    ┌─ Task 1
                    │
        Browser #1  ├─ Task 2  ← inUse = 3/10
        (Chrome     │
        Process)   ├─ Task 3
                    │
                    └─ Empty slots


Task.RunTask() calls:
├─ BrowserPool.Acquire(ctx)
│  │
│  ├─ FAST PATH: Reuse existing browser
│  │  ├─ acquireReusableBrowserLocked()
│  │  │  ├─ Find browser with lowest inUse count (load balance)
│  │  │  ├─ if browser.inUse < browser.maxTabs:
│  │  │  │  ├─ browser.inUse++ ← Increment usage
│  │  │  │  └─ chromedp.NewContext(browser.browserCtx) ← NEW TAB
│  │  │  └─ else: continue to slow path
│  │  │
│  │  └─ Return (browserCtx, release, nil)
│  │
│  └─ SLOW PATH: Create new browser (if not at pool limit)
│     ├─ if len(browsers) + creating < poolSize:
│     │  ├─ p.creating++ ← Mark slot taken
│     │  ├─ p.mu.Unlock() ← Release lock during creation
│     │  │
│     │  ├─ createBrowser(ctx)
│     │  │  ├─ allocCtx := chromedp.NewExecAllocator(baseCtx, opts...)
│     │  │  │  └─ Launch Chrome process with:
│     │  │  │     ├─ headless mode
│     │  │  │     ├─ --disable-gpu
│     │  │  │     ├─ --disable-dev-shm-usage
│     │  │  │     ├─ --js-flags="--max-old-space-size=512"
│     │  │  │     └─ proxy if configured
│     │  │  │
│     │  │  └─ browserCtx := chromedp.NewContext(allocCtx)
│     │  │     └─ Connect to launched Chrome
│     │  │
│     │  ├─ p.mu.Lock() ← Re-acquire lock
│     │  ├─ if p.stopped: cancel allocCtx, return error
│     │  ├─ p.browsers.append(pooledBrowser{
│     │  │    allocCtx, browserCtx, inUse: 1, maxTabs: 10, lastUsed: now
│     │  │ })
│     │  ├─ p.creating-- ← Release slot
│     │  │
│     │  └─ Return newTabContext(browser, allocCtx)
│     │
│     └─ WAIT PATH: Block until slot available
│        ├─ waitForSlot(ctx, deadline)
│        ├─ Wait on:
│        │  ├─ ctx.Done() → return context cancelled
│        │  ├─ stopCh → return pool stopped
│        │  ├─ notifyCh → return, try again
│        │  └─ timer → return timeout
│        └─ Loop back to try Acquire again


Task release:
├─ release() ← Called after task completes (via defer)
│  ├─ browser.inUse-- ← Decrement usage
│  ├─ browser.lastUsed = now ← Update timestamp
│  └─ (Tab/context automatically cleaned up by chromedp)


Cleanup Loop (every 30s):
├─ for _, browser := range p.browsers:
│  ├─ if time.Since(browser.lastUsed) > idleTimeout (5min):
│  │  ├─ browser.allocCancel() ← Kill Chrome process
│  │  ├─ p.browsers.remove(browser)
│  │  └─ p.totalClosed++
│  │
│  └─ if browser.inUse == 0:
│     └─ browser eligible for cleanup in next cycle


MEMORY MANAGEMENT:
├─ Pool Size: 5 browsers × ~100MB each = ~500MB base
├─ Per Tab: ~10-50MB per tab
├─ Max Memory: 5 browsers × 10 tabs × 50MB = ~2.5GB
├─ Memory Limit: --js-flags="--max-old-space-size=512" per process
└─ Cleanup: Idle browsers > 5min released automatically


CONCURRENCY CONTROL:
├─ Pool Level: sync.Mutex guards all operations
├─ Browser Level: Each browser can serve up to maxTabs (10) concurrent tabs
├─ Global Proxy Limit: Separate from pool (default 80 concurrent proxies)
├─ Worker Pool: Fixed 200 workers (configurable) + staggered 50ms warmup
└─ Backlog: Tasks queued in priority heap until pool/proxy slot available


TAB LIFECYCLE (within Browser):
├─ chromedp.NewContext(browserCtx)
│  ├─ Creates new target (tab) via CDP
│  ├─ Returns tab-specific context
│  └─ Tracks in Browser's tab list (implicit)
│
├─ Use tab for all CDp actions
│  ├─ Navigation, clicks, scripts, etc.
│  └─ All scoped to this tab
│
└─ Cancel context / Return to pool
   ├─ chromedp automatically closes tab via CDP
   └─ Tab removed from Chrome's target list


MULTI-TAB SUPPORT:
├─ Each browser can manage 10 independent tabs
├─ ActionTabSwitch changes active tab reference
├─ Network logs tagged per step (which tab making request)
├─ Context switching via chromedp.Targets() + CDP SetDiscoverTargets
│
└─ Limitations:
   ├─ No cross-tab coordination (each tab independent)
   ├─ Cookies shared at browser level (unless isolated contexts used)
   └─ Network logs merged into single array (not per-tab separated)
```

---

## 3. STEP EXECUTION DISPATCH DIAGRAM

```
┌──────────────────────────────────────────────────────────────────────────────┐
│          Browser Step Executor Dispatch                                      │
│    [internal/browser/steps.go: executeStep() architecture]                   │
└──────────────────────────────────────────────────────────────────────────────┘

executeStep(ctx, step, result) [steps.go:23-79]
│
├─ SIMPLE ACTIONS (Switch/Case, no result needed):
│  │
│  ├─ ActionNavigate → execNavigate(ctx, step)
│  │  └─ r.exec.RunResponse(ctx, chromedp.Navigate(step.Value))
│  │
│  ├─ ActionClick → execClick(ctx, step)
│  │  └─ chromedp.WaitVisible(step.Selector), Click(...)
│  │
│  ├─ ActionType → execType(ctx, step)
│  │  └─ WaitVisible, Clear, SendKeys(step.Value)
│  │
│  ├─ ActionWait → execWait(ctx, step)
│  │  ├─ if step.Selector: WaitVisible(...)
│  │  └─ else: Sleep(step.Value milliseconds)
│  │
│  ├─ ActionScroll → execScroll(ctx, step)
│  │  └─ (implementation for scroll to x,y)
│  │
│  ├─ ActionSelect → execSelect(ctx, step)
│  │  └─ Select dropdown option
│  │
│  ├─ ActionEval → execEval(ctx, step)
│  │  ├─ validateEvalScript(step.Value) ← Check for dangerous patterns
│  │  ├─ chromedp.Evaluate(script)
│  │  └─ Return result in result.ExtractedData
│  │
│  ├─ ActionTabSwitch → execTabSwitch(ctx, step)
│  │  └─ chromedp.Targets(), find target by step.Value
│  │
│  ├─ ActionDoubleClick → execDoubleClick(ctx, step)
│  │  └─ chromedp.MouseAction(doubleClick)
│  │
│  ├─ ActionFileUpload → execFileUpload(ctx, step)
│  │  ├─ Validate path within sandbox
│  │  ├─ chromedp.SendKeys(selector, "file://path")
│  │  └─ (simulate file selection)
│  │
│  ├─ ActionNavigateBack → execNavigateBack(ctx)
│  │  └─ page.GoBack()
│  │
│  ├─ ActionNavigateForward → execNavigateForward(ctx)
│  │  └─ page.GoForward()
│  │
│  ├─ ActionReload → execReload(ctx)
│  │  └─ page.Reload()
│  │
│  ├─ ActionScrollIntoView → execScrollIntoView(ctx, step)
│  │  └─ Evaluate JS: element.scrollIntoView()
│  │
│  ├─ ActionSubmitForm → execSubmitForm(ctx, step)
│  │  └─ Find form, call submit()
│  │
│  ├─ ActionWaitNotPresent → execWaitNotPresent(ctx, step)
│  │  └─ Poll until selector no longer exists
│  │
│  ├─ ActionWaitEnabled → execWaitEnabled(ctx, step)
│  │  └─ Poll until element not disabled
│  │
│  ├─ ActionWaitFunction → execWaitFunction(ctx, step)
│  │  └─ Poll custom JS condition
│  │
│  ├─ ActionEmulateDevice → execEmulateDevice(ctx, step)
│  │  └─ chromedp.SetUserAgent(), setDeviceSize()
│  │
│  ├─ ActionHover → execHover(ctx, step)
│  │  └─ chromedp.MouseAction(move)
│  │
│  ├─ ActionDragDrop → execDragDrop(ctx, step)
│  │  └─ MouseAction(down, move, up)
│  │
│  ├─ ActionContextClick → execContextClick(ctx, step)
│  │  └─ chromedp.MouseAction(rightClick)
│  │
│  ├─ ActionRandomMouse → execRandomMouse(ctx, step)
│  │  └─ Move mouse to random position (anti-bot)
│  │
│  ├─ ActionHumanTyping → execHumanTyping(ctx, step)
│  │  └─ SendKeys with random delays (anti-bot)
│  │
│  └─ ActionScreenshot → execScreenshot(ctx, result)
│     ├─ chromedp.FullScreenshot(&buf)
│     ├─ Sanitize filename
│     ├─ Write to disk with 0o600 perms
│     └─ Append path to result.Screenshots[]
│
│
└─ COMPLEX ACTIONS (Require result, use handler map):
   │ [steps.go:85-132 getStepHandlerWithResult]
   │
   └─ executeStepWithResult(ctx, step, result)
      │
      ├─ handler := getStepHandlerWithResult(step.Action)
      │  └─ Returns: stepHandlerWithResult func type
      │
      └─ handler(ctx, step, result) [calls specific handler]:
         │
         ├─ ActionExtract → execExtract(ctx, step, result)
         │  ├─ if step.Attribute: getAttribute(step.Selector)
         │  ├─ else: innerText(step.Selector)
         │  ├─ result.ExtractedData[step.Selector] = value
         │  └─ if step.VarName: vars[step.VarName] = value
         │
         ├─ ActionSolveCaptcha → execSolveCaptcha(ctx, step, result)
         │  ├─ Detect captcha type (reCAPTCHA, hCaptcha, etc.)
         │  ├─ Call r.captchaSolver.Solve(...)
         │  └─ Fill in solution
         │
         ├─ ActionGetTitle → execGetTitle(ctx, step, result)
         │  ├─ chromedp.Title(&title)
         │  └─ result.ExtractedData[step.Selector] = title
         │
         ├─ ActionGetAttributes → execGetAttributes(ctx, step, result)
         │  └─ Get all attributes of element
         │
         ├─ ActionClickAd → execClickAd(ctx, step, result)
         │  ├─ Find ad element (by attribute or text)
         │  ├─ chromedp.Click()
         │  └─ (anti-ad-blocker logic)
         │
         ├─ ActionWhile → execWhile(ctx, step, result)
         │  └─ [Handled in PC-based loop, not direct execution]
         │
         ├─ ActionEndWhile → execEndWhile(ctx, step, result)
         │  └─ [Handled in PC-based loop]
         │
         ├─ ActionIfExists → execIfExists(ctx, step, result)
         │  └─ [Handled in PC-based loop]
         │
         ├─ ActionIfNotExists → execIfNotExists(ctx, step, result)
         │  └─ [Handled in PC-based loop]
         │
         ├─ ActionIfVisible → execIfVisible(ctx, step, result)
         │  └─ [Handled in PC-based loop]
         │
         ├─ ActionIfEnabled → execIfEnabled(ctx, step, result)
         │  └─ [Handled in PC-based loop]
         │
         ├─ ActionVariableSet → execVariableSet(ctx, step, result)
         │  ├─ vars[step.VarName] = step.Value
         │  └─ Supports string templates: "Hello {{other_var}}"
         │
         ├─ ActionVariableMath → execVariableMath(ctx, step, result)
         │  ├─ Parse: "var_x = var_y + 5"
         │  └─ Evaluate math expression
         │
         ├─ ActionVariableString → execVariableString(ctx, step, result)
         │  ├─ Parse: "var_x = uppercase(var_y)"
         │  └─ Evaluate string operations
         │
         ├─ ActionHighlight → execHighlight(ctx, step, result)
         │  ├─ Inject CSS to highlight element
         │  └─ Add to screenshot
         │
         ├─ ActionGetCookies → execGetCookies(ctx, step, result)
         │  ├─ network.GetCookies()
         │  └─ result.ExtractedData[...] = JSON cookies
         │
         ├─ ActionSetCookie → execSetCookie(ctx, step, result)
         │  └─ network.SetCookie(cookie)
         │
         ├─ ActionDeleteCookies → execDeleteCookies(ctx, step, result)
         │  └─ network.DeleteCookies()
         │
         ├─ ActionGetStorage → execGetStorage(ctx, step, result)
         │  ├─ dom.GetBoxModel() + evaluate JS
         │  └─ result.ExtractedData = localStorage/sessionStorage
         │
         ├─ ActionSetStorage → execSetStorage(ctx, step, result)
         │  └─ runtime.Evaluate(JS: localStorage.setItem(...))
         │
         ├─ ActionDeleteStorage → execDeleteStorage(ctx, step, result)
         │  └─ runtime.Evaluate(JS: localStorage.removeItem(...))
         │
         ├─ ActionDownload → execDownload(ctx, step, result)
         │  ├─ Set download handler
         │  ├─ Click link
         │  └─ Wait for file
         │
         ├─ ActionSelectRandom → execSelectRandom(ctx, step, result)
         │  ├─ Find all options
         │  ├─ Pick random
         │  └─ Select
         │
         ├─ ActionDebugPause → execDebugPause(ctx, step, result)
         │  └─ r.debugCtrl.pause() ← Pause execution
         │
         ├─ ActionDebugResume → execDebugResume(ctx, step, result)
         │  └─ r.debugCtrl.resume() ← Resume execution
         │
         ├─ ActionDebugStep → execDebugStep(ctx, step, result)
         │  └─ r.debugCtrl.step() ← Single-step execution
         │
         ├─ ActionAntiBot → execAntiBot(ctx, step, result)
         │  ├─ Random mouse movements
         │  ├─ Random waits
         │  └─ Inject webdriver=false
         │
         ├─ ActionGetSession → execGetSession(ctx, step, result)
         │  └─ Return session ID / cookies
         │
         ├─ ActionSetSession → execSetSession(ctx, step, result)
         │  └─ Set cookies from session
         │
         ├─ ActionLoadSession → execLoadSession(ctx, step, result)
         │  └─ Load from file
         │
         ├─ ActionSaveSession → execSaveSession(ctx, step, result)
         │  └─ Save to file
         │
         ├─ ActionCacheGet → execCacheGet(ctx, step, result)
         │  └─ result.ExtractedData = from in-memory cache
         │
         ├─ ActionCacheSet → execCacheSet(ctx, step, result)
         │  └─ in-memory cache[key] = value
         │
         └─ ActionCacheClear → execCacheClear(ctx, step, result)
            └─ Clear in-memory cache


ERROR HANDLING (for all actions):
├─ Wrapped in context timeout (per-step, default 30s)
├─ Error returned: "selector not found", "timeout", "network error", etc.
├─ Classified [models.ClassifyError]:
│  ├─ "context deadline" → ErrorCodeTimeout
│  ├─ "not found" → ErrorCodeSelectorNotFound
│  ├─ "network" → ErrorCodeNetworkError
│  ├─ "eval blocked" → ErrorCodeEvalNotAllowed
│  └─ Others → ErrorCodeUnknown
│
└─ Logged with context:
   ├─ step_index, action, selector, error message
   ├─ Traceback captured in logs
   └─ Step duration recorded even on failure
```

---

## 4. MONITORING DATA FLOW DIAGRAM

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                  Monitoring & Logging Data Pipeline                          │
└──────────────────────────────────────────────────────────────────────────────┘

TASK LOGS:
  ├─ Source: runner.addLog(result, level, message) [browser.go:693-714]
  ├─ Captured in: TaskResult.Logs []LogEntry
  ├─ Size: Limited to LogLimit (default 1000 entries)
  ├─ Storage: In-memory in TaskResult, saved to DB
  ├─ Output Channels:
  │  ├─ slog [logs.Logger.*]
  │  ├─ TaskResult struct (returned to Queue)
  │  └─ task_logs table (if DB implementation added)
  └─ Example:
     └─ {Timestamp: "2026-04-21T12:34:56Z", Level: "info", Message: "step 1: click"}


STEP LOGS:
  ├─ Source: StepLogger.EndStep() [internal/logs/logger.go:37-54]
  ├─ Captured in: TaskResult.StepLogs []StepLog
  ├─ Structure:
  │  ├─ TaskID, StepIndex, Action, Selector, Value
  │  ├─ DurationMs (calculated from StartedAt + End time)
  │  ├─ ErrorMsg, ErrorCode (if failed)
  │  └─ SnapshotID (future: DOM snapshot reference)
  ├─ Storage: In-memory in TaskResult
  ├─ Database: step_logs table
  ├─ Query: App.GetTask(id) → result.StepLogs[]
  └─ Use Cases:
     ├─ Analyze slow steps (p95, p99 percentile)
     ├─ Error attribution (which steps fail most)
     └─ Performance trends


NETWORK LOGS:
  ├─ Source: CDP Event Listeners [browser.go:275-286]
  │  ├─ EventRequestWillBeSent → record start time
  │  ├─ EventResponseReceived → store response metadata
  │  ├─ EventLoadingFinished → build NetworkLog entry
  │  └─ EventLoadingFailed → cleanup tracking
  │
  ├─ Aggregation: NetworkLogger.Aggregate() [logs/network.go]
  │  └─ Total requests, failed count, avg response time, largest response
  │
  ├─ Storage:
  │  ├─ In-memory: NetworkLogger.logs []NetworkLog (max 10,000)
  │  ├─ Database: network_logs table
  │  └─ Dropped entries tracked: NetworkLogger.dropped counter
  │
  ├─ Structure:
  │  ├─ TaskID, StepIndex (which step made this request)
  │  ├─ RequestURL, Method, StatusCode
  │  ├─ DurationMs (RTT from request start to response end)
  │  ├─ ResponseSize (EncodedDataLength from CDP)
  │  ├─ RequestHeaders, ResponseHeaders (JSON)
  │  └─ Timestamp
  │
  └─ Use Cases:
     ├─ Network performance analysis (slow endpoints)
     ├─ Failed request debugging (4xx, 5xx)
     ├─ Step-level network breakdown
     └─ Bandwidth tracking


QUEUE METRICS:
  ├─ Source: Queue operations [internal/queue/queue.go]
  │  ├─ On Submit: q.metrics.TotalSubmitted++
  │  ├─ On Success: q.metrics.TotalCompleted++
  │  ├─ On Failure: q.metrics.TotalFailed++
  │  ├─ On Retry: q.metrics.TotalRetried++
  │  └─ Current state: q.pq.Len(), q.running map, q.runningProxied
  │
  ├─ Exposure: App.GetQueueMetrics() [app.go]
  ├─ Structure: QueueMetrics{
  │  ├─ TotalSubmitted, TotalCompleted, TotalFailed, TotalRetried
  │  ├─ PendingCount (current tasks in heap)
  │  ├─ WorkersActive (currently running tasks)
  │  ├─ WorkerPoolSize (configured, e.g., 200)
  │  ├─ ProxyConcurrentUsed (tasks using proxy slots)
  │  └─ MaxPendingObserved (high watermark)
  ├─ Update Frequency: Real-time on state changes
  └─ Use Cases:
     ├─ Queue depth monitoring (detect backlog)
     ├─ Worker utilization (% busy)
     ├─ Success/failure rate trending


BROWSER POOL METRICS:
  ├─ Source: BrowserPool operations [internal/browser/pool.go]
  │  ├─ On Acquire: record acquisition time
  │  ├─ On Create: increment TotalCreated
  │  ├─ On Close: increment TotalClosed
  │  └─ On Cleanup: record idle browser removal
  │
  ├─ Exposure: Pool.Stats() [pool.go]
  ├─ Structure: PoolStats{
  │  ├─ TotalCreated, TotalClosed (lifetime counters)
  │  ├─ CurrentBrowsers, IdleBrowsers (current state)
  │  ├─ MaxTabsObserved (peak usage per browser)
  │  └─ AcquireTimeP95Ms (95th percentile acquisition latency)
  ├─ Tracking:
  │  └─ acquireTimes []int64 (rolling window of last 1000)
  └─ Use Cases:
     ├─ Pool health (browsers, tabs, idle ratio)
     ├─ Acquisition latency (detect contention)
     └─ Memory pressure (infer from browser count)


TASK EVENTS (Audit Trail):
  ├─ Source: Queue.emitEvent() [internal/queue/queue.go]
  ├─ Trigger: Every task status transition
  │  ├─ pending → queued (on Submit)
  │  ├─ queued → running (on worker dequeue)
  │  ├─ running → completed (on success)
  │  ├─ running → failed (on error)
  │  ├─ running → retrying (on scheduled retry)
  │  └─ Any state → cancelled (on user cancel)
  │
  ├─ Storage: task_events table
  │  ├─ id (UUID), task_id, from_state, to_state
  │  ├─ event_data (JSON: reason, metadata)
  │  └─ created_at
  │
  ├─ Query: App.ListAuditTrail(taskID) [app.go]
  ├─ Retention: 90 days (App.PurgeOldRecords runs daily)
  └─ Use Cases:
     ├─ Task lifecycle replay
     ├─ Event correlation
     └─ Debugging task behavior


EXTRACTION & VARIABLES:
  ├─ Source: execExtract() [steps.go]
  │  ├─ querySelector(step.Selector)
  │  ├─ getAttribute() or innerText()
  │  └─ result.ExtractedData[key] = value
  │
  ├─ Storage: TaskResult.ExtractedData map[string]string
  ├─ Persistence: Stored in task.result JSON (DB)
  └─ Use Cases:
     ├─ Data extraction from websites
     ├─ Dynamic variable population
     └─ Batch processing (template substitution)


SCREENSHOTS:
  ├─ Source: execScreenshot() [steps.go:203-219]
  │  ├─ chromedp.FullScreenshot(&buf)
  │  ├─ Write to disk: screenshotDir/{sanitized_id}_{timestamp}.png
  │  └─ Append path to result.Screenshots[]
  │
  ├─ Storage:
  │  ├─ Disk: ~/.flowpilot/screenshots/ (perms 0o600)
  │  └─ Reference in DB task.screenshots (JSON array of paths)
  │
  ├─ Retention: Until task deleted
  └─ Use Cases:
     ├─ Visual debugging
     ├─ Proof of execution
     └─ UI testing verification


FULL DATA FLOW (Task Creation → Completion):
  │
  1. App.CreateTask(params) creates Task record
     └─ task.id = UUID
     └─ task.status = pending
     └─ stored in DB
  │
  2. App.StartTask(id) / Auto-start
     └─ Queue.Submit(task)
     └─ emit('task:event', {from: pending, to: queued})
     └─ task.status updated in DB
  │
  3. Queue.worker dequeues
     └─ emit('task:event', {from: queued, to: running})
     └─ task.status updated in DB
  │
  4. Browser.RunTask() executes steps
     ├─ For each step:
     │  ├─ runner.addLog() → result.Logs[]
     │  ├─ stepLogger.EndStep() → result.StepLogs[]
     │  └─ Network requests trigger CDP listeners → result.NetworkLogs[]
     │
     ├─ If screenshot step: result.Screenshots.append(path)
     ├─ If extract step: result.ExtractedData[key] = value
     └─ If error: result.Error, continue trying retries
  │
  5. Browser.RunTask() returns result
     ├─ result.Success = true/false
     ├─ result.Duration = elapsed time
     └─ result contains all logs/network/extracted data
  │
  6. Queue handles completion
     ├─ If success:
     │  ├─ Update DB: task.status = completed, task.result = result JSON
     │  ├─ Store StepLogs to step_logs table
     │  ├─ Store NetworkLogs to network_logs table
     │  └─ emit('task:event', {from: running, to: completed})
     │
     ├─ Else if error:
     │  ├─ Determine retry
     │  ├─ If retry: Schedule after backoff, go to step 2
     │  ├─ Else: Mark as failed
     │  ├─ Update DB: task.status = failed, task.error, task.result
     │  └─ emit('task:event', {from: running, to: failed})
     │
     └─ Update queue metrics
  │
  7. Frontend receives 'task:event'
     ├─ Update task store
     ├─ Show status to user
     └─ Optionally fetch full task details (includes logs/network)


API ENDPOINTS FOR MONITORING:
  ├─ GET /api/app/task/{id}
  │  └─ Returns Task + TaskResult (all logs, network, screenshots)
  │
  ├─ GET /api/app/tasks (with pagination)
  │  └─ Returns Task list (without result details)
  │
  ├─ GET /api/app/audit-trail?task_id=...
  │  └─ Returns task_events[] (status transitions)
  │
  ├─ GET /api/queue/metrics (NEW)
  │  └─ Returns QueueMetrics{pending, running, completed, etc.}
  │
  ├─ GET /api/browser-pool/stats (NEW)
  │  └─ Returns PoolStats{current, idle, acquire_time_p95}
  │
  └─ Wails IPC: EventsOn('task:event') (real-time)
     └─ Emitted on each status change

```

---

## 5. Error Handling & Retry Flow

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    Error Handling & Retry Strategy                           │
│                    [internal/queue/queue.go:804-847]                         │
└──────────────────────────────────────────────────────────────────────────────┘

Task Failure → Queue.handleFailure()
│
├─ Log error: logs.Logger.Error("task failed", task_id, error)
│
├─ Update DB: TaskStatusFailed
│
├─ Classify error [models.ClassifyError(err)]
│  ├─ Determine error type:
│  │  ├─ "context deadline exceeded" → ErrorCodeTimeout
│  │  ├─ "not found" → ErrorCodeSelectorNotFound
│  │  ├─ "network error" → ErrorCodeNetworkError
│  │  ├─ "eval blocked" → ErrorCodeEvalNotAllowed
│  │  └─ Others → ErrorCodeUnknown
│  │
│  └─ Store in result.Error for debugging
│
├─ Determine retry eligibility:
│  │
│  ├─ Check 1: task.RetryCount < task.MaxRetries (default 5)
│  │  └─ If false: Don't retry, mark final failure
│  │
│  ├─ Check 2: Is error retryable?
│  │  └─ Some errors (validation, permissions) are non-retryable
│  │
│  └─ Check 3: Backoff time
│     └─ backoff = baseMs * 2^retryCount (exponential)
│     └─ baseMs = 500 (configurable via SetRetryBackoffBaseMs)
│     └─ Max backoff: 5 minutes (cap exponential growth)
│     │
│     └─ Example progression:
│        Retry 0 (initial): No delay
│        Retry 1: 500ms * 2^1 = 1s
│        Retry 2: 500ms * 2^2 = 2s
│        Retry 3: 500ms * 2^3 = 4s
│        Retry 4: 500ms * 2^4 = 8s
│        Retry 5: 500ms * 2^5 = 16s (max 5min)
│
├─ Return retryInfo {shouldRetry, task, backoff}
│
├─ If shouldRetry:
│  │
│  ├─ Log scheduled retry: logs.Logger.Info("task_scheduled_retry")
│  │
│  ├─ Create goroutine:
│  │  ├─ Schedule: time.Sleep(backoff)
│  │  ├─ Increment: task.RetryCount++
│  │  ├─ Re-submit: Queue.Submit(task) ← Back to QUEUED state
│  │  └─ Task re-enters execution pipeline
│  │
│  ├─ Update DB:
│  │  └─ TaskStatusRetrying + retry_count + next_retry_at
│  │
│  └─ Emit event: "task_retrying"
│
│
└─ If !shouldRetry:
   │
   ├─ Mark permanent failure
   │
   ├─ Update DB: status = FAILED, error persisted
   │
   ├─ Emit event: "task_failed"
   │
   ├─ If configured, dispatch webhook:
   │  └─ POST task.webhook_url with {task, result, error}
   │
   └─ Alert (if threshold exceeded)
      └─ X consecutive failures in Y seconds


RETRY SUCCESS TRACKING:
├─ If retried task succeeds:
│  ├─ Log: "retry_succeeded" with retry_count
│  ├─ Update DB: retry_count preserved (for analytics)
│  ├─ Increment: queue.metrics.TotalRetried++
│  └─ Set status: COMPLETED
│
└─ If retried task fails again:
   ├─ Repeat handleFailure logic
   ├─ Re-evaluate: (task.RetryCount+1 < task.MaxRetries)?
   └─ Retry again or mark final failure


SPECIAL CASE: Proxy-Related Errors
├─ If error caused by proxy (network timeout, 407, etc.):
│  │
│  ├─ Mark proxy as problematic
│  │  └─ proxy.Manager.RecordFailure(proxy_id)
│  │
│  ├─ For auto-proxy tasks:
│  │  └─ proxyManager may select different proxy for retry
│  │
│  └─ Track: "retries_by_proxy" metrics
│     └─ Identify problematic proxies
│
└─ Fallback strategy:
   ├─ If specific proxy fails consistently:
   │  ├─ Try without proxy (if fallback=permissive)
   │  └─ Fail (if fallback=strict)
   │
   └─ Log: which proxy worked/failed


CANCELLATION:
├─ User calls: App.CancelTask(taskID)
│  │
│  ├─ Set: queue.cancelled[taskID] = true
│  │
│  ├─ If queued: Remove from heap immediately
│  │
│  ├─ If running: Call taskCtx.Cancel()
│  │  └─ Propagates to all chromedp operations
│  │  └─ Browser.RunTask returns context.Err()
│  │
│  ├─ Update DB: status = CANCELLED
│  │
│  ├─ Emit event: "task_cancelled"
│  │
│  └─ Do not retry
│


TIMEOUT:
├─ Each task has timeout: task.Timeout (in seconds, configurable)
│  │
│  ├─ Applied at browser.RunTask level:
│  │  └─ taskCtx, cancel := WithTimeout(parent, task.Timeout)
│  │
│  ├─ If timeout expires:
│  │  ├─ taskCtx.Err() = context.DeadlineExceeded
│  │  ├─ Browser.RunTask returns error
│  │  ├─ Queue.handleFailure determines if retryable
│  │  └─ Typically YES (transient network assumed)
│  │
│  └─ Each step also has timeout (default 30s):
│     └─ stepCtx, cancel := WithTimeout(browserCtx, step.Timeout)
│     └─ Individual step failure doesn't stop task
│     └─ Step error flows to handleFailure
│


MEMORY SAFETY ON ERROR:
├─ Defer cleanup:
│  ├─ defer browserCancel() [browser.go:341]
│  └─ defer poolRelease() [pool.go:104-132]
│
├─ Context lifecycle:
│  ├─ taskCtx → browserCtx → stepCtx
│  └─ Cancellation propagates down
│
├─ Resource cleanup:
│  ├─ On error: all contexts cancelled
│  ├─ All chromedp operations interrupted
│  └─ Browser context returned to pool
│
└─ No resource leaks:
   ├─ Every goroutine has corresponding defer
   ├─ Every context has cancel function
   └─ All cleanup happens even on panic (panic recovery)
```

---

End of Visual Reference Document
