# Prompt optimization for codebase tasks

**Session ID:** ses_34b9c7b43ffeX0G6rTOzDGt3sV
**Created:** 3/3/2026, 3:49:06 PM
**Updated:** 3/3/2026, 4:03:14 PM

---

## User

# Prompt Optimization Command

Optimize, compare, or audit the following prompt: firstt see codebase then tell a new next tasks prompt...

## Your Task

1. **Analyze** the provided prompt across 8 dimensions (clarity, specificity, structure, context, output control, edge cases, chain-of-thought, reusability)
2. **Generate 3 optimized variants** using different strategies (structured, chain-of-thought, maximum precision)
3. **Score all versions** in a comparison matrix (1-10 per dimension)
4. **Recommend the best variant** with clear reasoning

## Modes

If the user specifies a mode, follow it:

- `optimize <prompt>` — Full analysis + 3 variants + comparison + recommendation
- `compare <prompt1> vs <prompt2>` — Side-by-side comparison with scoring
- `audit <prompt>` — Checklist audit with specific improvement suggestions
- `rewrite <prompt> for <model>` — Model-specific optimization
- `simplify <prompt>` — Reduce complexity while preserving effectiveness
- `template <use-case>` — Generate a reusable prompt template

If no mode is specified, default to `optimize`.

## Output Format

```
PROMPT ANALYSIS
===============
[Assessment of original]

OPTIMIZED VARIANTS
==================
[3 variants with full prompt text]

COMPARISON MATRIX
=================
[Scoring table]

RECOMMENDATION
==============
[Best variant + reasoning]
```

---

## Assistant (Pro-Prompts-Enhanced · duo-chat-opus-4-6 · 5.7s)



I'll start by exploring the codebase to understand its structure and purpose, then craft an optimized "next tasks" prompt tailored to this project.

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation"
}
```

**Output:**
```
<path>/root/electron-web-test-automation</path>
<type>directory</type>
<entries>
.claude/
.cocoindex_code/
.git/
.gitignore
.opencode/
app.go
build/
frontend/
go.mod
go.sum
internal/
main.go
node_modules/
README.md
wails.json

(15 entries)
</entries>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/package.json"
}
```

**Error:**
```
Error: File not found: /root/electron-web-test-automation/package.json
```

**Tool: git-summary**

**Input:**
```json
{
  "includeDiff": true,
  "depth": 10
}
```

**Output:**
```
{"commands":["git branch --show-current","git status --short","git log --oneline -10","git diff --cached --stat","git diff main...HEAD --stat"],"instructions":"Run these git commands to get a comprehensive summary:\n\ngit branch --show-current\ngit status --short\ngit log --oneline -10\ngit diff --cached --stat\ngit diff main...HEAD --stat"}
```

---

## Assistant (Pro-Prompts-Enhanced · duo-chat-opus-4-6 · 6.3s)

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/main.go"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/main.go</path>
<type>file</type>
<content>1: package main
2: 
3: import (
4: 	"embed"
5: 
6: 	"github.com/wailsapp/wails/v2"
7: 	"github.com/wailsapp/wails/v2/pkg/options"
8: 	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
9: )
10: 
11: //go:embed all:frontend/dist
12: var assets embed.FS
13: 
14: func main() {
15: 	app := NewApp()
16: 
17: 	err := wails.Run(&options.App{
18: 		Title:     "Web Automation Dashboard",
19: 		Width:     1400,
20: 		Height:    900,
21: 		MinWidth:  1024,
22: 		MinHeight: 768,
23: 		AssetServer: &assetserver.Options{
24: 			Assets: assets,
25: 		},
26: 		BackgroundColour: &options.RGBA{R: 17, G: 24, B: 39, A: 1},
27: 		OnStartup:        app.startup,
28: 		OnShutdown:       app.shutdown,
29: 		Bind: []interface{}{
30: 			app,
31: 		},
32: 	})
33: 
34: 	if err != nil {
35: 		println("Error:", err.Error())
36: 	}
37: }

(End of file - total 37 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/app.go"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/app.go</path>
<type>file</type>
<content>1: package main
2: 
3: import (
4: 	"context"
5: 	"encoding/csv"
6: 	"encoding/json"
7: 	"fmt"
8: 	"os"
9: 	"path/filepath"
10: 	"time"
11: 
12: 	"web-automation/internal/browser"
13: 	"web-automation/internal/database"
14: 	"web-automation/internal/models"
15: 	"web-automation/internal/proxy"
16: 	"web-automation/internal/queue"
17: 
18: 	"github.com/google/uuid"
19: 	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
20: )
21: 
22: // App struct holds the application state and dependencies.
23: type App struct {
24: 	ctx          context.Context
25: 	db           *database.DB
26: 	runner       *browser.Runner
27: 	queue        *queue.Queue
28: 	proxyManager *proxy.Manager
29: 	dataDir      string
30: }
31: 
32: // NewApp creates a new App application struct.
33: func NewApp() *App {
34: 	return &App{}
35: }
36: 
37: // startup is called when the app starts. Initializes all dependencies.
38: func (a *App) startup(ctx context.Context) {
39: 	a.ctx = ctx
40: 
41: 	// Setup data directory
42: 	home, err := os.UserHomeDir()
43: 	if err != nil {
44: 		wailsRuntime.LogFatalf(ctx, "Failed to get home directory: %v", err)
45: 		return
46: 	}
47: 	a.dataDir = filepath.Join(home, ".web-automation")
48: 	if err := os.MkdirAll(a.dataDir, 0o755); err != nil {
49: 		wailsRuntime.LogFatalf(ctx, "Failed to create data directory: %v", err)
50: 		return
51: 	}
52: 
53: 	// Initialize database
54: 	dbPath := filepath.Join(a.dataDir, "tasks.db")
55: 	db, err := database.New(dbPath)
56: 	if err != nil {
57: 		wailsRuntime.LogFatalf(ctx, "Failed to init database: %v", err)
58: 		return
59: 	}
60: 	a.db = db
61: 
62: 	// Initialize browser runner
63: 	screenshotDir := filepath.Join(a.dataDir, "screenshots")
64: 	runner, err := browser.NewRunner(screenshotDir)
65: 	if err != nil {
66: 		wailsRuntime.LogFatalf(ctx, "Failed to init browser runner: %v", err)
67: 		return
68: 	}
69: 	a.runner = runner
70: 
71: 	// Initialize proxy manager
72: 	a.proxyManager = proxy.NewManager(db, models.ProxyPoolConfig{
73: 		Strategy:            models.RotationRoundRobin,
74: 		HealthCheckInterval: 300,
75: 		MaxFailures:         3,
76: 	})
77: 	go a.proxyManager.StartHealthChecks(ctx)
78: 
79: 	// Initialize task queue
80: 	a.queue = queue.New(db, runner, 100, func(event models.TaskEvent) {
81: 		wailsRuntime.EventsEmit(ctx, "task:event", event)
82: 	})
83: 
84: 	wailsRuntime.LogInfo(ctx, "Application started successfully")
85: }
86: 
87: // shutdown is called when the app is closing.
88: func (a *App) shutdown(ctx context.Context) {
89: 	if a.queue != nil {
90: 		a.queue.Stop()
91: 	}
92: 	if a.proxyManager != nil {
93: 		a.proxyManager.Stop()
94: 	}
95: 	if a.db != nil {
96: 		a.db.Close()
97: 	}
98: }
99: 
100: // --- Task API (bound to frontend) ---
101: 
102: // CreateTask creates a new task and optionally starts it.
103: func (a *App) CreateTask(name, url string, steps []models.TaskStep, proxyConfig models.ProxyConfig, priority int, autoStart bool) (*models.Task, error) {
104: 	task := models.Task{
105: 		ID:         uuid.New().String(),
106: 		Name:       name,
107: 		URL:        url,
108: 		Steps:      steps,
109: 		Proxy:      proxyConfig,
110: 		Priority:   models.TaskPriority(priority),
111: 		Status:     models.TaskStatusPending,
112: 		MaxRetries: 3,
113: 		CreatedAt:  time.Now(),
114: 	}
115: 
116: 	if err := a.db.CreateTask(task); err != nil {
117: 		return nil, fmt.Errorf("create task: %w", err)
118: 	}
119: 
120: 	if autoStart {
121: 		if err := a.queue.Submit(a.ctx, task); err != nil {
122: 			return nil, fmt.Errorf("submit task: %w", err)
123: 		}
124: 	}
125: 
126: 	return &task, nil
127: }
128: 
129: // GetTask returns a task by ID.
130: func (a *App) GetTask(id string) (*models.Task, error) {
131: 	return a.db.GetTask(id)
132: }
133: 
134: // ListTasks returns all tasks.
135: func (a *App) ListTasks() ([]models.Task, error) {
136: 	return a.db.ListTasks()
137: }
138: 
139: // ListTasksByStatus returns tasks with a given status.
140: func (a *App) ListTasksByStatus(status string) ([]models.Task, error) {
141: 	return a.db.ListTasksByStatus(models.TaskStatus(status))
142: }
143: 
144: // StartTask submits a pending task to the queue.
145: func (a *App) StartTask(id string) error {
146: 	task, err := a.db.GetTask(id)
147: 	if err != nil {
148: 		return fmt.Errorf("get task for start: %w", err)
149: 	}
150: 	return a.queue.Submit(a.ctx, *task)
151: }
152: 
153: // StartAllPending submits all pending tasks to the queue.
154: func (a *App) StartAllPending() error {
155: 	tasks, err := a.db.ListTasksByStatus(models.TaskStatusPending)
156: 	if err != nil {
157: 		return fmt.Errorf("list pending tasks: %w", err)
158: 	}
159: 	return a.queue.SubmitBatch(a.ctx, tasks)
160: }
161: 
162: // CancelTask cancels a running task.
163: func (a *App) CancelTask(id string) error {
164: 	return a.queue.Cancel(id)
165: }
166: 
167: // DeleteTask deletes a task.
168: func (a *App) DeleteTask(id string) error {
169: 	return a.db.DeleteTask(id)
170: }
171: 
172: // GetTaskStats returns task count per status.
173: func (a *App) GetTaskStats() (map[string]int, error) {
174: 	return a.db.GetTaskStats()
175: }
176: 
177: // GetRunningCount returns how many tasks are currently running.
178: func (a *App) GetRunningCount() int {
179: 	return a.queue.RunningCount()
180: }
181: 
182: // --- Proxy API ---
183: 
184: // AddProxy adds a proxy to the pool.
185: func (a *App) AddProxy(server, protocol, username, password, geo string) (*models.Proxy, error) {
186: 	p := models.Proxy{
187: 		ID:        uuid.New().String(),
188: 		Server:    server,
189: 		Protocol:  models.ProxyProtocol(protocol),
190: 		Username:  username,
191: 		Password:  password,
192: 		Geo:       geo,
193: 		Status:    models.ProxyStatusUnknown,
194: 		CreatedAt: time.Now(),
195: 	}
196: 
197: 	if err := a.db.CreateProxy(p); err != nil {
198: 		return nil, fmt.Errorf("add proxy: %w", err)
199: 	}
200: 	return &p, nil
201: }
202: 
203: // ListProxies returns all proxies.
204: func (a *App) ListProxies() ([]models.Proxy, error) {
205: 	return a.db.ListProxies()
206: }
207: 
208: // DeleteProxy removes a proxy.
209: func (a *App) DeleteProxy(id string) error {
210: 	return a.db.DeleteProxy(id)
211: }
212: 
213: // --- Export API ---
214: 
215: // ExportResultsJSON exports all task results as JSON.
216: func (a *App) ExportResultsJSON() (string, error) {
217: 	tasks, err := a.db.ListTasksByStatus(models.TaskStatusCompleted)
218: 	if err != nil {
219: 		return "", fmt.Errorf("list completed tasks: %w", err)
220: 	}
221: 
222: 	exportPath := filepath.Join(a.dataDir, fmt.Sprintf("export_%d.json", time.Now().Unix()))
223: 	data, err := json.MarshalIndent(tasks, "", "  ")
224: 	if err != nil {
225: 		return "", fmt.Errorf("marshal tasks to JSON: %w", err)
226: 	}
227: 
228: 	if err := os.WriteFile(exportPath, data, 0o644); err != nil {
229: 		return "", fmt.Errorf("write export file: %w", err)
230: 	}
231: 	return exportPath, nil
232: }
233: 
234: // ExportResultsCSV exports task results as CSV.
235: func (a *App) ExportResultsCSV() (string, error) {
236: 	tasks, err := a.db.ListTasksByStatus(models.TaskStatusCompleted)
237: 	if err != nil {
238: 		return "", fmt.Errorf("list completed tasks: %w", err)
239: 	}
240: 
241: 	exportPath := filepath.Join(a.dataDir, fmt.Sprintf("export_%d.csv", time.Now().Unix()))
242: 	file, err := os.Create(exportPath)
243: 	if err != nil {
244: 		return "", fmt.Errorf("create export file: %w", err)
245: 	}
246: 	defer file.Close()
247: 
248: 	writer := csv.NewWriter(file)
249: 	defer writer.Flush()
250: 
251: 	if err := writer.Write([]string{"ID", "Name", "URL", "Status", "Error", "Duration", "CreatedAt", "CompletedAt"}); err != nil {
252: 		return "", fmt.Errorf("write CSV header: %w", err)
253: 	}
254: 
255: 	for _, t := range tasks {
256: 		duration := ""
257: 		if t.Result != nil {
258: 			duration = t.Result.Duration.String()
259: 		}
260: 		completedAt := ""
261: 		if t.CompletedAt != nil {
262: 			completedAt = t.CompletedAt.Format(time.RFC3339)
263: 		}
264: 		if err := writer.Write([]string{
265: 			t.ID, t.Name, t.URL, string(t.Status), t.Error,
266: 			duration, t.CreatedAt.Format(time.RFC3339), completedAt,
267: 		}); err != nil {
268: 			return "", fmt.Errorf("write CSV row: %w", err)
269: 		}
270: 	}
271: 
272: 	return exportPath, nil
273: }

(End of file - total 273 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/wails.json"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/wails.json</path>
<type>file</type>
<content>1: {
2:   "$schema": "https://wails.io/schemas/config.v2.json",
3:   "name": "web-automation",
4:   "outputfilename": "web-automation",
5:   "frontend:install": "npm install",
6:   "frontend:build": "npm run build",
7:   "frontend:dev:watcher": "npm run dev",
8:   "frontend:dev:serverUrl": "auto",
9:   "author": {
10:     "name": "",
11:     "email": ""
12:   }
13: }

(End of file - total 13 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/README.md"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/README.md</path>
<type>file</type>
<content>1: # README
2: 
3: ## About
4: 
5: This is the official Wails Svelte-TS template.
6: 
7: ## Live Development
8: 
9: To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
10: server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
11: and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
12: to this in your browser, and you can call your Go code from devtools.
13: 
14: ## Building
15: 
16: To build a redistributable, production mode package, use `wails build`.

(End of file - total 16 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/internal"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/internal</path>
<type>directory</type>
<entries>
browser/
database/
models/
proxy/
queue/

(5 entries)
</entries>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/frontend"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/frontend</path>
<type>directory</type>
<entries>
.vscode/
index.html
package-lock.json
package.json
package.json.md5
README.md
src/
svelte.config.js
tsconfig.json
tsconfig.node.json
vite.config.ts
wailsjs/

(12 entries)
</entries>
```

**Tool: bash**

**Input:**
```json
{
  "command": "git branch --show-current && echo \"---STATUS---\" && git status --short && echo \"---LOG---\" && git log --oneline -10",
  "description": "Get git branch, status, and log"
}
```

**Output:**
```
main
---STATUS---
 M .opencode/opencode.json
---LOG---
061596a chore: update opencode tools to new plugin API and add build artifacts
428a5f9 feat: implement Go + Wails + chromedp web automation desktop app

```

---

## Assistant (Pro-Prompts-Enhanced · duo-chat-opus-4-6 · 12.5s)

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/internal/models"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/internal/models</path>
<type>directory</type>
<entries>
proxy.go
task.go

(2 entries)
</entries>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/internal/browser"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/internal/browser</path>
<type>directory</type>
<entries>
browser_test.go
browser.go

(2 entries)
</entries>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/internal/database"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/internal/database</path>
<type>directory</type>
<entries>
sqlite_test.go
sqlite.go

(2 entries)
</entries>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/internal/queue"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/internal/queue</path>
<type>directory</type>
<entries>
queue_test.go
queue.go

(2 entries)
</entries>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/internal/proxy"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/internal/proxy</path>
<type>directory</type>
<entries>
manager_test.go
manager.go

(2 entries)
</entries>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/frontend/src"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/frontend/src</path>
<type>directory</type>
<entries>
App.svelte
assets/
components/
lib/
main.ts
style.css
vite-env.d.ts

(7 entries)
</entries>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/frontend/package.json"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/frontend/package.json</path>
<type>file</type>
<content>1: {
2:   "name": "frontend",
3:   "private": true,
4:   "version": "0.0.0",
5:   "type": "module",
6:   "scripts": {
7:     "dev": "vite",
8:     "build": "vite build",
9:     "preview": "vite preview",
10:     "check": "svelte-check --tsconfig ./tsconfig.json"
11:   },
12:   "devDependencies": {
13:     "@sveltejs/vite-plugin-svelte": "^1.0.1",
14:     "@tsconfig/svelte": "^3.0.0",
15:     "svelte": "^3.49.0",
16:     "svelte-check": "^2.8.0",
17:     "svelte-preprocess": "^4.10.7",
18:     "tslib": "^2.4.0",
19:     "typescript": "^4.6.4",
20:     "vite": "^3.0.7"
21:   }
22: }

(End of file - total 22 lines)
</content>
```

---

## Assistant (Pro-Prompts-Enhanced · duo-chat-opus-4-6 · 5.8s)

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/internal/models/task.go"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/internal/models/task.go</path>
<type>file</type>
<content>1: package models
2: 
3: import "time"
4: 
5: // TaskStatus represents the current state of a task.
6: type TaskStatus string
7: 
8: const (
9: 	TaskStatusPending   TaskStatus = "pending"
10: 	TaskStatusQueued    TaskStatus = "queued"
11: 	TaskStatusRunning   TaskStatus = "running"
12: 	TaskStatusCompleted TaskStatus = "completed"
13: 	TaskStatusFailed    TaskStatus = "failed"
14: 	TaskStatusCancelled TaskStatus = "cancelled"
15: 	TaskStatusRetrying  TaskStatus = "retrying"
16: )
17: 
18: // TaskPriority controls queue ordering.
19: type TaskPriority int
20: 
21: const (
22: 	PriorityLow    TaskPriority = 1
23: 	PriorityNormal TaskPriority = 5
24: 	PriorityHigh   TaskPriority = 10
25: )
26: 
27: // StepAction defines what a task step does.
28: type StepAction string
29: 
30: const (
31: 	ActionNavigate   StepAction = "navigate"
32: 	ActionClick      StepAction = "click"
33: 	ActionType       StepAction = "type"
34: 	ActionWait       StepAction = "wait"
35: 	ActionScreenshot StepAction = "screenshot"
36: 	ActionExtract    StepAction = "extract"
37: 	ActionScroll     StepAction = "scroll"
38: 	ActionSelect     StepAction = "select"
39: 	ActionEval       StepAction = "eval"
40: )
41: 
42: // TaskStep represents a single browser action within a task.
43: type TaskStep struct {
44: 	Action   StepAction `json:"action"`
45: 	Selector string     `json:"selector,omitempty"`
46: 	Value    string     `json:"value,omitempty"`
47: 	Timeout  int        `json:"timeout,omitempty"` // milliseconds
48: }
49: 
50: // ProxyConfig holds proxy connection details for a task.
51: type ProxyConfig struct {
52: 	Server   string `json:"server"`
53: 	Username string `json:"username,omitempty"`
54: 	Password string `json:"password,omitempty"`
55: 	Geo      string `json:"geo,omitempty"` // country code
56: }
57: 
58: // Task represents a single automated browser task.
59: type Task struct {
60: 	ID          string       `json:"id"`
61: 	Name        string       `json:"name"`
62: 	URL         string       `json:"url"`
63: 	Steps       []TaskStep   `json:"steps"`
64: 	Proxy       ProxyConfig  `json:"proxy"`
65: 	Priority    TaskPriority `json:"priority"`
66: 	Status      TaskStatus   `json:"status"`
67: 	RetryCount  int          `json:"retryCount"`
68: 	MaxRetries  int          `json:"maxRetries"`
69: 	Error       string       `json:"error,omitempty"`
70: 	Result      *TaskResult  `json:"result,omitempty"`
71: 	CreatedAt   time.Time    `json:"createdAt"`
72: 	StartedAt   *time.Time   `json:"startedAt,omitempty"`
73: 	CompletedAt *time.Time   `json:"completedAt,omitempty"`
74: 	Tags        []string     `json:"tags,omitempty"`
75: }
76: 
77: // TaskResult holds the output of a completed task.
78: type TaskResult struct {
79: 	TaskID        string            `json:"taskId"`
80: 	Success       bool              `json:"success"`
81: 	ExtractedData map[string]string `json:"extractedData,omitempty"`
82: 	Screenshots   []string          `json:"screenshots,omitempty"` // file paths
83: 	Logs          []LogEntry        `json:"logs"`
84: 	Duration      time.Duration     `json:"duration"`
85: 	Error         string            `json:"error,omitempty"`
86: }
87: 
88: // LogEntry is a single log message from task execution.
89: type LogEntry struct {
90: 	Timestamp time.Time `json:"timestamp"`
91: 	Level     string    `json:"level"` // info, warn, error
92: 	Message   string    `json:"message"`
93: }
94: 
95: // TaskEvent is emitted to the frontend via Wails events.
96: type TaskEvent struct {
97: 	TaskID string     `json:"taskId"`
98: 	Status TaskStatus `json:"status"`
99: 	Error  string     `json:"error,omitempty"`
100: 	Log    *LogEntry  `json:"log,omitempty"`
101: }
102: 
103: // BatchConfig is used to create multiple tasks at once.
104: type BatchConfig struct {
105: 	Tasks       []Task `json:"tasks"`
106: 	Concurrency int    `json:"concurrency"` // max concurrent, default 100
107: }

(End of file - total 107 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/internal/models/proxy.go"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/internal/models/proxy.go</path>
<type>file</type>
<content>1: package models
2: 
3: import "time"
4: 
5: // ProxyStatus indicates the health of a proxy.
6: type ProxyStatus string
7: 
8: const (
9: 	ProxyStatusHealthy   ProxyStatus = "healthy"
10: 	ProxyStatusUnhealthy ProxyStatus = "unhealthy"
11: 	ProxyStatusUnknown   ProxyStatus = "unknown"
12: )
13: 
14: // ProxyProtocol is the supported proxy type.
15: type ProxyProtocol string
16: 
17: const (
18: 	ProxyHTTP   ProxyProtocol = "http"
19: 	ProxyHTTPS  ProxyProtocol = "https"
20: 	ProxySOCKS5 ProxyProtocol = "socks5"
21: )
22: 
23: // Proxy represents a proxy server in the pool.
24: type Proxy struct {
25: 	ID          string        `json:"id"`
26: 	Server      string        `json:"server"` // host:port
27: 	Protocol    ProxyProtocol `json:"protocol"`
28: 	Username    string        `json:"username,omitempty"`
29: 	Password    string        `json:"password,omitempty"`
30: 	Geo         string        `json:"geo,omitempty"` // country code
31: 	Status      ProxyStatus   `json:"status"`
32: 	Latency     int           `json:"latency"` // ms, last measured
33: 	SuccessRate float64       `json:"successRate"`
34: 	TotalUsed   int           `json:"totalUsed"`
35: 	LastChecked *time.Time    `json:"lastChecked,omitempty"`
36: 	CreatedAt   time.Time     `json:"createdAt"`
37: }
38: 
39: // RotationStrategy controls how proxies are selected.
40: type RotationStrategy string
41: 
42: const (
43: 	RotationRoundRobin    RotationStrategy = "round_robin"
44: 	RotationRandom        RotationStrategy = "random"
45: 	RotationLeastUsed     RotationStrategy = "least_used"
46: 	RotationLowestLatency RotationStrategy = "lowest_latency"
47: )
48: 
49: // ProxyPoolConfig configures the proxy pool behavior.
50: type ProxyPoolConfig struct {
51: 	Strategy            RotationStrategy `json:"strategy"`
52: 	HealthCheckInterval int              `json:"healthCheckInterval"` // seconds
53: 	MaxFailures         int              `json:"maxFailures"`
54: 	HealthCheckURL      string           `json:"healthCheckUrl"`
55: }

(End of file - total 55 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/internal/browser/browser.go"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/internal/browser/browser.go</path>
<type>file</type>
<content>1: package browser
2: 
3: import (
4: 	"context"
5: 	"fmt"
6: 	"os"
7: 	"path/filepath"
8: 	"strconv"
9: 	"time"
10: 
11: 	"web-automation/internal/models"
12: 
13: 	"github.com/chromedp/cdproto/fetch"
14: 	"github.com/chromedp/cdproto/network"
15: 	"github.com/chromedp/chromedp"
16: )
17: 
18: const defaultTimeout = 30 * time.Second
19: 
20: // Runner executes browser automation tasks using chromedp.
21: type Runner struct {
22: 	screenshotDir string
23: }
24: 
25: // NewRunner creates a new browser runner.
26: func NewRunner(screenshotDir string) (*Runner, error) {
27: 	if err := os.MkdirAll(screenshotDir, 0o755); err != nil {
28: 		return nil, fmt.Errorf("create screenshot dir: %w", err)
29: 	}
30: 	return &Runner{screenshotDir: screenshotDir}, nil
31: }
32: 
33: // RunTask executes a single task with its own browser context and proxy.
34: func (r *Runner) RunTask(ctx context.Context, task models.Task) (*models.TaskResult, error) {
35: 	start := time.Now()
36: 	result := &models.TaskResult{
37: 		TaskID:        task.ID,
38: 		ExtractedData: make(map[string]string),
39: 	}
40: 
41: 	allocCtx, allocCancel := r.createAllocator(ctx, task.Proxy)
42: 	defer allocCancel()
43: 
44: 	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
45: 	defer browserCancel()
46: 
47: 	if task.Proxy.Username != "" {
48: 		r.setupProxyAuth(browserCtx, task.Proxy)
49: 	}
50: 
51: 	if err := r.runSteps(browserCtx, task.Steps, result); err != nil {
52: 		result.Duration = time.Since(start)
53: 		return result, err
54: 	}
55: 
56: 	result.Success = true
57: 	result.Duration = time.Since(start)
58: 	r.addLog(result, "info", fmt.Sprintf("task completed in %s", result.Duration))
59: 	return result, nil
60: }
61: 
62: // createAllocator builds a chromedp allocator with safe option copying and optional proxy.
63: func (r *Runner) createAllocator(ctx context.Context, proxyConfig models.ProxyConfig) (context.Context, context.CancelFunc) {
64: 	// Copy default options to avoid mutating the shared slice.
65: 	opts := make([]chromedp.ExecAllocatorOption, len(chromedp.DefaultExecAllocatorOptions))
66: 	copy(opts, chromedp.DefaultExecAllocatorOptions[:])
67: 
68: 	opts = append(opts,
69: 		chromedp.Flag("headless", true),
70: 		chromedp.Flag("disable-gpu", true),
71: 		chromedp.Flag("no-sandbox", true),
72: 		chromedp.Flag("disable-dev-shm-usage", true),
73: 	)
74: 
75: 	if proxyConfig.Server != "" {
76: 		opts = append(opts, chromedp.ProxyServer(proxyConfig.Server))
77: 	}
78: 
79: 	return chromedp.NewExecAllocator(ctx, opts...)
80: }
81: 
82: // runSteps iterates through each task step and executes it.
83: func (r *Runner) runSteps(browserCtx context.Context, steps []models.TaskStep, result *models.TaskResult) error {
84: 	for i, step := range steps {
85: 		r.addLog(result, "info", fmt.Sprintf("step %d: %s", i+1, step.Action))
86: 
87: 		timeout := defaultTimeout
88: 		if step.Timeout > 0 {
89: 			timeout = time.Duration(step.Timeout) * time.Millisecond
90: 		}
91: 
92: 		stepCtx, stepCancel := context.WithTimeout(browserCtx, timeout)
93: 		err := r.executeStep(stepCtx, step, result)
94: 		stepCancel()
95: 
96: 		if err != nil {
97: 			r.addLog(result, "error", fmt.Sprintf("step %d failed: %v", i+1, err))
98: 			result.Error = fmt.Sprintf("step %d (%s) failed: %v", i+1, step.Action, err)
99: 			return err
100: 		}
101: 
102: 		r.addLog(result, "info", fmt.Sprintf("step %d completed", i+1))
103: 	}
104: 	return nil
105: }
106: 
107: func (r *Runner) setupProxyAuth(ctx context.Context, proxy models.ProxyConfig) {
108: 	chromedp.ListenTarget(ctx, func(ev interface{}) {
109: 		switch e := ev.(type) {
110: 		case *fetch.EventAuthRequired:
111: 			go func() {
112: 				execCtx := chromedp.FromContext(ctx)
113: 				if execCtx == nil || execCtx.Target == nil {
114: 					return
115: 				}
116: 				_ = fetch.ContinueWithAuth(e.RequestID, &fetch.AuthChallengeResponse{
117: 					Response: fetch.AuthChallengeResponseResponseProvideCredentials,
118: 					Username: proxy.Username,
119: 					Password: proxy.Password,
120: 				}).Do(context.WithValue(ctx, "cdp", execCtx.Target))
121: 			}()
122: 		case *fetch.EventRequestPaused:
123: 			go func() {
124: 				execCtx := chromedp.FromContext(ctx)
125: 				if execCtx == nil || execCtx.Target == nil {
126: 					return
127: 				}
128: 				_ = fetch.ContinueRequest(e.RequestID).Do(context.WithValue(ctx, "cdp", execCtx.Target))
129: 			}()
130: 		}
131: 	})
132: }
133: 
134: // executeStep dispatches to the appropriate action handler.
135: func (r *Runner) executeStep(ctx context.Context, step models.TaskStep, result *models.TaskResult) error {
136: 	switch step.Action {
137: 	case models.ActionNavigate:
138: 		return r.execNavigate(ctx, step)
139: 	case models.ActionClick:
140: 		return r.execClick(ctx, step)
141: 	case models.ActionType:
142: 		return r.execType(ctx, step)
143: 	case models.ActionWait:
144: 		return r.execWait(ctx, step)
145: 	case models.ActionScreenshot:
146: 		return r.execScreenshot(ctx, result)
147: 	case models.ActionExtract:
148: 		return r.execExtract(ctx, step, result)
149: 	case models.ActionScroll:
150: 		return r.execScroll(ctx, step)
151: 	case models.ActionSelect:
152: 		return r.execSelect(ctx, step)
153: 	case models.ActionEval:
154: 		return r.execEval(ctx, step)
155: 	default:
156: 		return fmt.Errorf("unknown action: %s", step.Action)
157: 	}
158: }
159: 
160: func (r *Runner) execNavigate(ctx context.Context, step models.TaskStep) error {
161: 	return chromedp.Run(ctx, chromedp.Navigate(step.Value))
162: }
163: 
164: func (r *Runner) execClick(ctx context.Context, step models.TaskStep) error {
165: 	return chromedp.Run(ctx,
166: 		chromedp.WaitVisible(step.Selector, chromedp.ByQuery),
167: 		chromedp.Click(step.Selector, chromedp.ByQuery),
168: 	)
169: }
170: 
171: func (r *Runner) execType(ctx context.Context, step models.TaskStep) error {
172: 	return chromedp.Run(ctx,
173: 		chromedp.WaitVisible(step.Selector, chromedp.ByQuery),
174: 		chromedp.Clear(step.Selector, chromedp.ByQuery),
175: 		chromedp.SendKeys(step.Selector, step.Value, chromedp.ByQuery),
176: 	)
177: }
178: 
179: func (r *Runner) execWait(ctx context.Context, step models.TaskStep) error {
180: 	if step.Selector != "" {
181: 		return chromedp.Run(ctx,
182: 			chromedp.WaitVisible(step.Selector, chromedp.ByQuery),
183: 		)
184: 	}
185: 	dur, err := time.ParseDuration(step.Value + "ms")
186: 	if err != nil {
187: 		dur = 1 * time.Second
188: 	}
189: 	// Respect context cancellation during wait.
190: 	select {
191: 	case <-time.After(dur):
192: 		return nil
193: 	case <-ctx.Done():
194: 		return ctx.Err()
195: 	}
196: }
197: 
198: func (r *Runner) execScreenshot(ctx context.Context, result *models.TaskResult) error {
199: 	var buf []byte
200: 	if err := chromedp.Run(ctx, chromedp.FullScreenshot(&buf, 90)); err != nil {
201: 		return fmt.Errorf("capture screenshot: %w", err)
202: 	}
203: 	filename := fmt.Sprintf("%s_%d.png", result.TaskID, time.Now().UnixMilli())
204: 	path := filepath.Join(r.screenshotDir, filename)
205: 	if err := os.WriteFile(path, buf, 0o644); err != nil {
206: 		return fmt.Errorf("save screenshot: %w", err)
207: 	}
208: 	result.Screenshots = append(result.Screenshots, path)
209: 	return nil
210: }
211: 
212: func (r *Runner) execExtract(ctx context.Context, step models.TaskStep, result *models.TaskResult) error {
213: 	var text string
214: 	if err := chromedp.Run(ctx,
215: 		chromedp.WaitVisible(step.Selector, chromedp.ByQuery),
216: 		chromedp.Text(step.Selector, &text, chromedp.ByQuery),
217: 	); err != nil {
218: 		return fmt.Errorf("extract text: %w", err)
219: 	}
220: 	key := step.Value
221: 	if key == "" {
222: 		key = step.Selector
223: 	}
224: 	result.ExtractedData[key] = text
225: 	return nil
226: }
227: 
228: func (r *Runner) execScroll(ctx context.Context, step models.TaskStep) error {
229: 	// Validate the scroll value is a number to prevent JS injection.
230: 	if _, err := strconv.Atoi(step.Value); err != nil {
231: 		return fmt.Errorf("invalid scroll value %q: must be an integer", step.Value)
232: 	}
233: 	return chromedp.Run(ctx,
234: 		chromedp.Evaluate(`window.scrollBy(0, `+step.Value+`)`, nil),
235: 	)
236: }
237: 
238: func (r *Runner) execSelect(ctx context.Context, step models.TaskStep) error {
239: 	return chromedp.Run(ctx,
240: 		chromedp.WaitVisible(step.Selector, chromedp.ByQuery),
241: 		chromedp.SetValue(step.Selector, step.Value, chromedp.ByQuery),
242: 	)
243: }
244: 
245: func (r *Runner) execEval(ctx context.Context, step models.TaskStep) error {
246: 	var res interface{}
247: 	return chromedp.Run(ctx,
248: 		chromedp.Evaluate(step.Value, &res),
249: 	)
250: }
251: 
252: func (r *Runner) addLog(result *models.TaskResult, level, message string) {
253: 	result.Logs = append(result.Logs, models.LogEntry{
254: 		Timestamp: time.Now(),
255: 		Level:     level,
256: 		Message:   message,
257: 	})
258: }
259: 
260: // ClearCookies clears cookies in a browser context.
261: func ClearCookies(ctx context.Context) error {
262: 	return chromedp.Run(ctx, network.ClearBrowserCookies())
263: }

(End of file - total 263 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/internal/queue/queue.go"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/internal/queue/queue.go</path>
<type>file</type>
<content>1: package queue
2: 
3: import (
4: 	"context"
5: 	"fmt"
6: 	"math"
7: 	"sync"
8: 	"time"
9: 
10: 	"web-automation/internal/browser"
11: 	"web-automation/internal/database"
12: 	"web-automation/internal/models"
13: 
14: 	"golang.org/x/sync/semaphore"
15: )
16: 
17: // EventCallback is called when a task event occurs.
18: type EventCallback func(event models.TaskEvent)
19: 
20: // Queue manages task execution with concurrency control.
21: type Queue struct {
22: 	db             *database.DB
23: 	runner         *browser.Runner
24: 	sem            *semaphore.Weighted
25: 	maxConcurrency int64
26: 	onEvent        EventCallback
27: 
28: 	mu      sync.Mutex
29: 	running map[string]context.CancelFunc // taskID -> cancel
30: 	stopped bool
31: 	stopOnce sync.Once
32: 	stopCh  chan struct{}
33: }
34: 
35: // New creates a new task queue.
36: func New(db *database.DB, runner *browser.Runner, maxConcurrency int, onEvent EventCallback) *Queue {
37: 	return &Queue{
38: 		db:             db,
39: 		runner:         runner,
40: 		sem:            semaphore.NewWeighted(int64(maxConcurrency)),
41: 		maxConcurrency: int64(maxConcurrency),
42: 		onEvent:        onEvent,
43: 		running:        make(map[string]context.CancelFunc),
44: 		stopCh:         make(chan struct{}),
45: 	}
46: }
47: 
48: // Submit adds a task to the queue and starts executing it.
49: func (q *Queue) Submit(ctx context.Context, task models.Task) error {
50: 	q.mu.Lock()
51: 	if q.stopped {
52: 		q.mu.Unlock()
53: 		return fmt.Errorf("queue is stopped")
54: 	}
55: 	q.mu.Unlock()
56: 
57: 	if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusQueued, ""); err != nil {
58: 		return fmt.Errorf("update task status to queued: %w", err)
59: 	}
60: 	q.emitEvent(task.ID, models.TaskStatusQueued, "")
61: 
62: 	go q.executeTask(ctx, task)
63: 	return nil
64: }
65: 
66: // SubmitBatch submits multiple tasks.
67: func (q *Queue) SubmitBatch(ctx context.Context, tasks []models.Task) error {
68: 	for _, task := range tasks {
69: 		if err := q.Submit(ctx, task); err != nil {
70: 			return fmt.Errorf("submit task %s: %w", task.ID, err)
71: 		}
72: 	}
73: 	return nil
74: }
75: 
76: // Cancel cancels a running task.
77: func (q *Queue) Cancel(taskID string) error {
78: 	q.mu.Lock()
79: 	cancel, ok := q.running[taskID]
80: 	q.mu.Unlock()
81: 
82: 	if ok {
83: 		cancel()
84: 	}
85: 
86: 	if err := q.db.UpdateTaskStatus(taskID, models.TaskStatusCancelled, "cancelled by user"); err != nil {
87: 		return fmt.Errorf("update task status to cancelled: %w", err)
88: 	}
89: 	return nil
90: }
91: 
92: // Stop stops all running tasks and prevents new submissions.
93: func (q *Queue) Stop() {
94: 	q.stopOnce.Do(func() {
95: 		q.mu.Lock()
96: 		q.stopped = true
97: 		for id, cancel := range q.running {
98: 			cancel()
99: 			delete(q.running, id)
100: 		}
101: 		q.mu.Unlock()
102: 		close(q.stopCh)
103: 	})
104: }
105: 
106: // RunningCount returns how many tasks are currently running.
107: func (q *Queue) RunningCount() int {
108: 	q.mu.Lock()
109: 	defer q.mu.Unlock()
110: 	return len(q.running)
111: }
112: 
113: func (q *Queue) executeTask(ctx context.Context, task models.Task) {
114: 	if err := q.sem.Acquire(ctx, 1); err != nil {
115: 		q.emitEvent(task.ID, models.TaskStatusFailed, "failed to acquire queue slot")
116: 		return
117: 	}
118: 	defer q.sem.Release(1)
119: 
120: 	taskCtx, cancel := context.WithCancel(ctx)
121: 	defer cancel()
122: 
123: 	q.mu.Lock()
124: 	q.running[task.ID] = cancel
125: 	q.mu.Unlock()
126: 
127: 	defer func() {
128: 		q.mu.Lock()
129: 		delete(q.running, task.ID)
130: 		q.mu.Unlock()
131: 	}()
132: 
133: 	if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusRunning, ""); err != nil {
134: 		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
135: 		return
136: 	}
137: 	q.emitEvent(task.ID, models.TaskStatusRunning, "")
138: 
139: 	result, err := q.runner.RunTask(taskCtx, task)
140: 	if err != nil {
141: 		q.handleFailure(ctx, taskCtx, task, err)
142: 		return
143: 	}
144: 
145: 	q.handleSuccess(task, result)
146: }
147: 
148: // handleFailure processes a task execution error, retrying if allowed.
149: func (q *Queue) handleFailure(parentCtx, taskCtx context.Context, task models.Task, execErr error) {
150: 	if task.RetryCount < task.MaxRetries {
151: 		if err := q.db.IncrementRetry(task.ID); err != nil {
152: 			q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("increment retry: %v", err))
153: 			return
154: 		}
155: 		q.emitEvent(task.ID, models.TaskStatusRetrying, execErr.Error())
156: 
157: 		backoff := time.Duration(math.Pow(2, float64(task.RetryCount))) * time.Second
158: 
159: 		// Wait for backoff respecting context cancellation.
160: 		select {
161: 		case <-time.After(backoff):
162: 		case <-taskCtx.Done():
163: 			if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusCancelled, "cancelled during retry backoff"); err != nil {
164: 				q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
165: 			}
166: 			return
167: 		}
168: 
169: 		// Retry: create a copy with incremented count and re-submit.
170: 		retryTask := task
171: 		retryTask.RetryCount++
172: 		retryTask.Status = models.TaskStatusPending
173: 		// Deep copy steps to avoid shared slice mutation.
174: 		retryTask.Steps = make([]models.TaskStep, len(task.Steps))
175: 		copy(retryTask.Steps, task.Steps)
176: 
177: 		go q.executeTask(parentCtx, retryTask)
178: 		return
179: 	}
180: 
181: 	// Max retries exceeded.
182: 	if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusFailed, execErr.Error()); err != nil {
183: 		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
184: 		return
185: 	}
186: 	q.emitEvent(task.ID, models.TaskStatusFailed, execErr.Error())
187: }
188: 
189: // handleSuccess records a successful task result.
190: func (q *Queue) handleSuccess(task models.Task, result *models.TaskResult) {
191: 	if err := q.db.UpdateTaskResult(task.ID, *result); err != nil {
192: 		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("save result: %v", err))
193: 		return
194: 	}
195: 	if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusCompleted, ""); err != nil {
196: 		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
197: 		return
198: 	}
199: 	q.emitEvent(task.ID, models.TaskStatusCompleted, "")
200: }
201: 
202: func (q *Queue) emitEvent(taskID string, status models.TaskStatus, errMsg string) {
203: 	if q.onEvent != nil {
204: 		q.onEvent(models.TaskEvent{
205: 			TaskID: taskID,
206: 			Status: status,
207: 			Error:  errMsg,
208: 		})
209: 	}
210: }

(End of file - total 210 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/internal/proxy/manager.go"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/internal/proxy/manager.go</path>
<type>file</type>
<content>1: package proxy
2: 
3: import (
4: 	"context"
5: 	"fmt"
6: 	"math/rand"
7: 	"net/http"
8: 	"net/url"
9: 	"sync"
10: 	"time"
11: 
12: 	"web-automation/internal/database"
13: 	"web-automation/internal/models"
14: )
15: 
16: // Manager handles proxy pool selection, rotation, and health checks.
17: type Manager struct {
18: 	db     *database.DB
19: 	config models.ProxyPoolConfig
20: 
21: 	mu       sync.Mutex
22: 	rrIndex  int // round-robin index
23: 	stopOnce sync.Once
24: 	stopCh   chan struct{}
25: }
26: 
27: // NewManager creates a new proxy manager.
28: func NewManager(db *database.DB, config models.ProxyPoolConfig) *Manager {
29: 	if config.HealthCheckURL == "" {
30: 		config.HealthCheckURL = "https://httpbin.org/ip"
31: 	}
32: 	if config.HealthCheckInterval == 0 {
33: 		config.HealthCheckInterval = 300 // 5 minutes
34: 	}
35: 	if config.MaxFailures == 0 {
36: 		config.MaxFailures = 3
37: 	}
38: 
39: 	return &Manager{
40: 		db:     db,
41: 		config: config,
42: 		stopCh: make(chan struct{}),
43: 	}
44: }
45: 
46: // StartHealthChecks begins periodic proxy health checks.
47: func (m *Manager) StartHealthChecks(ctx context.Context) {
48: 	ticker := time.NewTicker(time.Duration(m.config.HealthCheckInterval) * time.Second)
49: 	defer ticker.Stop()
50: 
51: 	// Run initial check.
52: 	m.checkAll(ctx)
53: 
54: 	for {
55: 		select {
56: 		case <-ticker.C:
57: 			m.checkAll(ctx)
58: 		case <-m.stopCh:
59: 			return
60: 		case <-ctx.Done():
61: 			return
62: 		}
63: 	}
64: }
65: 
66: // Stop halts the health check loop. Safe to call multiple times.
67: func (m *Manager) Stop() {
68: 	m.stopOnce.Do(func() {
69: 		close(m.stopCh)
70: 	})
71: }
72: 
73: // SelectProxy picks a proxy based on the configured rotation strategy.
74: // If geo is specified, only proxies in that geo are considered.
75: func (m *Manager) SelectProxy(geo string) (*models.Proxy, error) {
76: 	proxies, err := m.db.ListHealthyProxies()
77: 	if err != nil {
78: 		return nil, fmt.Errorf("list healthy proxies: %w", err)
79: 	}
80: 
81: 	if geo != "" {
82: 		filtered := make([]models.Proxy, 0, len(proxies))
83: 		for _, p := range proxies {
84: 			if p.Geo == geo {
85: 				filtered = append(filtered, p)
86: 			}
87: 		}
88: 		proxies = filtered
89: 	}
90: 
91: 	if len(proxies) == 0 {
92: 		return nil, fmt.Errorf("no healthy proxies available (geo=%s)", geo)
93: 	}
94: 
95: 	var selected models.Proxy
96: 	switch m.config.Strategy {
97: 	case models.RotationRoundRobin:
98: 		selected = m.selectRoundRobin(proxies)
99: 	case models.RotationRandom:
100: 		selected = m.selectRandom(proxies)
101: 	case models.RotationLeastUsed:
102: 		selected = m.selectLeastUsed(proxies)
103: 	case models.RotationLowestLatency:
104: 		selected = m.selectLowestLatency(proxies)
105: 	default:
106: 		selected = m.selectRoundRobin(proxies)
107: 	}
108: 	return &selected, nil
109: }
110: 
111: // RecordUsage records whether a proxy was used successfully.
112: func (m *Manager) RecordUsage(proxyID string, success bool) error {
113: 	return m.db.IncrementProxyUsage(proxyID, success)
114: }
115: 
116: func (m *Manager) selectRoundRobin(proxies []models.Proxy) models.Proxy {
117: 	m.mu.Lock()
118: 	defer m.mu.Unlock()
119: 
120: 	idx := m.rrIndex % len(proxies)
121: 	m.rrIndex++
122: 	return proxies[idx]
123: }
124: 
125: func (m *Manager) selectRandom(proxies []models.Proxy) models.Proxy {
126: 	idx := rand.Intn(len(proxies))
127: 	return proxies[idx]
128: }
129: 
130: func (m *Manager) selectLeastUsed(proxies []models.Proxy) models.Proxy {
131: 	best := proxies[0]
132: 	for i := 1; i < len(proxies); i++ {
133: 		if proxies[i].TotalUsed < best.TotalUsed {
134: 			best = proxies[i]
135: 		}
136: 	}
137: 	return best
138: }
139: 
140: func (m *Manager) selectLowestLatency(proxies []models.Proxy) models.Proxy {
141: 	best := proxies[0]
142: 	for i := 1; i < len(proxies); i++ {
143: 		// Only consider proxies with measured latency (> 0).
144: 		// Replace best if it has no latency data, or if this proxy is faster.
145: 		if proxies[i].Latency > 0 && (best.Latency == 0 || proxies[i].Latency < best.Latency) {
146: 			best = proxies[i]
147: 		}
148: 	}
149: 	return best
150: }
151: 
152: func (m *Manager) checkAll(ctx context.Context) {
153: 	proxies, err := m.db.ListProxies()
154: 	if err != nil {
155: 		return
156: 	}
157: 
158: 	var wg sync.WaitGroup
159: 	for _, p := range proxies {
160: 		wg.Add(1)
161: 		go func(px models.Proxy) {
162: 			defer wg.Done()
163: 			m.checkProxy(ctx, px)
164: 		}(p)
165: 	}
166: 	wg.Wait()
167: }
168: 
169: func (m *Manager) checkProxy(ctx context.Context, proxy models.Proxy) {
170: 	proxyURL, err := url.Parse(fmt.Sprintf("%s://%s", proxy.Protocol, proxy.Server))
171: 	if err != nil {
172: 		_ = m.db.UpdateProxyHealth(proxy.ID, models.ProxyStatusUnhealthy, 0)
173: 		return
174: 	}
175: 
176: 	if proxy.Username != "" {
177: 		proxyURL.User = url.UserPassword(proxy.Username, proxy.Password)
178: 	}
179: 
180: 	client := &http.Client{
181: 		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
182: 		Timeout:   10 * time.Second,
183: 	}
184: 
185: 	start := time.Now()
186: 	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.config.HealthCheckURL, nil)
187: 	if err != nil {
188: 		_ = m.db.UpdateProxyHealth(proxy.ID, models.ProxyStatusUnhealthy, 0)
189: 		return
190: 	}
191: 
192: 	resp, err := client.Do(req)
193: 	latency := int(time.Since(start).Milliseconds())
194: 
195: 	if err != nil {
196: 		_ = m.db.UpdateProxyHealth(proxy.ID, models.ProxyStatusUnhealthy, latency)
197: 		return
198: 	}
199: 	defer resp.Body.Close()
200: 
201: 	if resp.StatusCode >= 400 {
202: 		_ = m.db.UpdateProxyHealth(proxy.ID, models.ProxyStatusUnhealthy, latency)
203: 		return
204: 	}
205: 
206: 	_ = m.db.UpdateProxyHealth(proxy.ID, models.ProxyStatusHealthy, latency)
207: }

(End of file - total 207 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/internal/database/sqlite.go"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/internal/database/sqlite.go</path>
<type>file</type>
<content>1: package database
2: 
3: import (
4: 	"database/sql"
5: 	"encoding/json"
6: 	"fmt"
7: 	"time"
8: 
9: 	"web-automation/internal/models"
10: 
11: 	_ "github.com/mattn/go-sqlite3"
12: )
13: 
14: // DB wraps the SQLite connection.
15: type DB struct {
16: 	conn *sql.DB
17: }
18: 
19: // New creates a new database connection and initializes the schema.
20: func New(dbPath string) (*DB, error) {
21: 	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
22: 	if err != nil {
23: 		return nil, fmt.Errorf("open database: %w", err)
24: 	}
25: 
26: 	conn.SetMaxOpenConns(1) // SQLite single-writer
27: 	conn.SetMaxIdleConns(1)
28: 
29: 	db := &DB{conn: conn}
30: 	if err := db.migrate(); err != nil {
31: 		conn.Close()
32: 		return nil, fmt.Errorf("run migrations: %w", err)
33: 	}
34: 
35: 	return db, nil
36: }
37: 
38: // Close closes the database connection.
39: func (db *DB) Close() error {
40: 	return db.conn.Close()
41: }
42: 
43: func (db *DB) migrate() error {
44: 	schema := `
45: 	CREATE TABLE IF NOT EXISTS tasks (
46: 		id TEXT PRIMARY KEY,
47: 		name TEXT NOT NULL,
48: 		url TEXT NOT NULL,
49: 		steps TEXT NOT NULL DEFAULT '[]',
50: 		proxy_server TEXT DEFAULT '',
51: 		proxy_username TEXT DEFAULT '',
52: 		proxy_password TEXT DEFAULT '',
53: 		proxy_geo TEXT DEFAULT '',
54: 		priority INTEGER DEFAULT 5,
55: 		status TEXT DEFAULT 'pending',
56: 		retry_count INTEGER DEFAULT 0,
57: 		max_retries INTEGER DEFAULT 3,
58: 		error TEXT DEFAULT '',
59: 		result TEXT DEFAULT '',
60: 		tags TEXT DEFAULT '[]',
61: 		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
62: 		started_at DATETIME,
63: 		completed_at DATETIME
64: 	);
65: 
66: 	CREATE TABLE IF NOT EXISTS proxies (
67: 		id TEXT PRIMARY KEY,
68: 		server TEXT NOT NULL,
69: 		protocol TEXT DEFAULT 'http',
70: 		username TEXT DEFAULT '',
71: 		password TEXT DEFAULT '',
72: 		geo TEXT DEFAULT '',
73: 		status TEXT DEFAULT 'unknown',
74: 		latency INTEGER DEFAULT 0,
75: 		success_rate REAL DEFAULT 0.0,
76: 		total_used INTEGER DEFAULT 0,
77: 		last_checked DATETIME,
78: 		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
79: 	);
80: 
81: 	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
82: 	CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority DESC);
83: 	CREATE INDEX IF NOT EXISTS idx_proxies_status ON proxies(status);
84: 	CREATE INDEX IF NOT EXISTS idx_proxies_geo ON proxies(geo);
85: 	`
86: 	_, err := db.conn.Exec(schema)
87: 	if err != nil {
88: 		return fmt.Errorf("exec schema: %w", err)
89: 	}
90: 	return nil
91: }
92: 
93: // --- Task CRUD ---
94: 
95: // CreateTask inserts a new task.
96: func (db *DB) CreateTask(task models.Task) error {
97: 	stepsJSON, err := json.Marshal(task.Steps)
98: 	if err != nil {
99: 		return fmt.Errorf("marshal steps: %w", err)
100: 	}
101: 	tagsJSON, err := json.Marshal(task.Tags)
102: 	if err != nil {
103: 		return fmt.Errorf("marshal tags: %w", err)
104: 	}
105: 
106: 	_, err = db.conn.Exec(`
107: 		INSERT INTO tasks (id, name, url, steps, proxy_server, proxy_username, proxy_password, proxy_geo, priority, status, max_retries, tags, created_at)
108: 		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
109: 		task.ID, task.Name, task.URL, string(stepsJSON),
110: 		task.Proxy.Server, task.Proxy.Username, task.Proxy.Password, task.Proxy.Geo,
111: 		task.Priority, task.Status, task.MaxRetries, string(tagsJSON), task.CreatedAt,
112: 	)
113: 	if err != nil {
114: 		return fmt.Errorf("insert task %s: %w", task.ID, err)
115: 	}
116: 	return nil
117: }
118: 
119: // GetTask retrieves a task by ID.
120: func (db *DB) GetTask(id string) (*models.Task, error) {
121: 	row := db.conn.QueryRow(`SELECT id, name, url, steps, proxy_server, proxy_username, proxy_password, proxy_geo,
122: 		priority, status, retry_count, max_retries, error, result, tags, created_at, started_at, completed_at
123: 		FROM tasks WHERE id = ?`, id)
124: 	task, err := db.scanTask(row)
125: 	if err != nil {
126: 		return nil, fmt.Errorf("get task %s: %w", id, err)
127: 	}
128: 	return task, nil
129: }
130: 
131: // ListTasks returns all tasks, ordered by priority desc, created_at desc.
132: func (db *DB) ListTasks() ([]models.Task, error) {
133: 	rows, err := db.conn.Query(`SELECT id, name, url, steps, proxy_server, proxy_username, proxy_password, proxy_geo,
134: 		priority, status, retry_count, max_retries, error, result, tags, created_at, started_at, completed_at
135: 		FROM tasks ORDER BY priority DESC, created_at DESC`)
136: 	if err != nil {
137: 		return nil, fmt.Errorf("query tasks: %w", err)
138: 	}
139: 	defer rows.Close()
140: 
141: 	var tasks []models.Task
142: 	for rows.Next() {
143: 		task, err := db.scanTaskRow(rows)
144: 		if err != nil {
145: 			return nil, fmt.Errorf("scan task row: %w", err)
146: 		}
147: 		tasks = append(tasks, *task)
148: 	}
149: 	if err := rows.Err(); err != nil {
150: 		return nil, fmt.Errorf("iterate tasks: %w", err)
151: 	}
152: 	return tasks, nil
153: }
154: 
155: // ListTasksByStatus returns tasks with a given status.
156: func (db *DB) ListTasksByStatus(status models.TaskStatus) ([]models.Task, error) {
157: 	rows, err := db.conn.Query(`SELECT id, name, url, steps, proxy_server, proxy_username, proxy_password, proxy_geo,
158: 		priority, status, retry_count, max_retries, error, result, tags, created_at, started_at, completed_at
159: 		FROM tasks WHERE status = ? ORDER BY priority DESC, created_at DESC`, status)
160: 	if err != nil {
161: 		return nil, fmt.Errorf("query tasks by status %s: %w", status, err)
162: 	}
163: 	defer rows.Close()
164: 
165: 	var tasks []models.Task
166: 	for rows.Next() {
167: 		task, err := db.scanTaskRow(rows)
168: 		if err != nil {
169: 			return nil, fmt.Errorf("scan task row: %w", err)
170: 		}
171: 		tasks = append(tasks, *task)
172: 	}
173: 	if err := rows.Err(); err != nil {
174: 		return nil, fmt.Errorf("iterate tasks: %w", err)
175: 	}
176: 	return tasks, nil
177: }
178: 
179: // UpdateTaskStatus updates the status of a task.
180: func (db *DB) UpdateTaskStatus(id string, status models.TaskStatus, errMsg string) error {
181: 	now := time.Now()
182: 	var err error
183: 	switch status {
184: 	case models.TaskStatusRunning:
185: 		_, err = db.conn.Exec(`UPDATE tasks SET status = ?, started_at = ? WHERE id = ?`, status, now, id)
186: 	case models.TaskStatusCompleted, models.TaskStatusFailed:
187: 		_, err = db.conn.Exec(`UPDATE tasks SET status = ?, error = ?, completed_at = ? WHERE id = ?`, status, errMsg, now, id)
188: 	default:
189: 		_, err = db.conn.Exec(`UPDATE tasks SET status = ?, error = ? WHERE id = ?`, status, errMsg, id)
190: 	}
191: 	if err != nil {
192: 		return fmt.Errorf("update task %s status to %s: %w", id, status, err)
193: 	}
194: 	return nil
195: }
196: 
197: // UpdateTaskResult stores the result JSON for a task.
198: func (db *DB) UpdateTaskResult(id string, result models.TaskResult) error {
199: 	resultJSON, err := json.Marshal(result)
200: 	if err != nil {
201: 		return fmt.Errorf("marshal result: %w", err)
202: 	}
203: 	_, err = db.conn.Exec(`UPDATE tasks SET result = ? WHERE id = ?`, string(resultJSON), id)
204: 	if err != nil {
205: 		return fmt.Errorf("update task %s result: %w", id, err)
206: 	}
207: 	return nil
208: }
209: 
210: // IncrementRetry increases retry_count for a task.
211: func (db *DB) IncrementRetry(id string) error {
212: 	_, err := db.conn.Exec(`UPDATE tasks SET retry_count = retry_count + 1, status = 'retrying' WHERE id = ?`, id)
213: 	if err != nil {
214: 		return fmt.Errorf("increment retry for task %s: %w", id, err)
215: 	}
216: 	return nil
217: }
218: 
219: // DeleteTask removes a task by ID.
220: func (db *DB) DeleteTask(id string) error {
221: 	_, err := db.conn.Exec(`DELETE FROM tasks WHERE id = ?`, id)
222: 	if err != nil {
223: 		return fmt.Errorf("delete task %s: %w", id, err)
224: 	}
225: 	return nil
226: }
227: 
228: // --- Proxy CRUD ---
229: 
230: // CreateProxy inserts a new proxy.
231: func (db *DB) CreateProxy(proxy models.Proxy) error {
232: 	_, err := db.conn.Exec(`
233: 		INSERT INTO proxies (id, server, protocol, username, password, geo, status, created_at)
234: 		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
235: 		proxy.ID, proxy.Server, proxy.Protocol, proxy.Username, proxy.Password,
236: 		proxy.Geo, proxy.Status, proxy.CreatedAt,
237: 	)
238: 	if err != nil {
239: 		return fmt.Errorf("insert proxy %s: %w", proxy.ID, err)
240: 	}
241: 	return nil
242: }
243: 
244: // ListProxies returns all proxies.
245: func (db *DB) ListProxies() ([]models.Proxy, error) {
246: 	rows, err := db.conn.Query(`SELECT id, server, protocol, username, password, geo, status, latency, success_rate, total_used, last_checked, created_at
247: 		FROM proxies ORDER BY success_rate DESC, latency ASC`)
248: 	if err != nil {
249: 		return nil, fmt.Errorf("query proxies: %w", err)
250: 	}
251: 	defer rows.Close()
252: 
253: 	var proxies []models.Proxy
254: 	for rows.Next() {
255: 		p, err := db.scanProxyRow(rows)
256: 		if err != nil {
257: 			return nil, fmt.Errorf("scan proxy row: %w", err)
258: 		}
259: 		proxies = append(proxies, *p)
260: 	}
261: 	if err := rows.Err(); err != nil {
262: 		return nil, fmt.Errorf("iterate proxies: %w", err)
263: 	}
264: 	return proxies, nil
265: }
266: 
267: // ListHealthyProxies returns proxies with healthy status.
268: func (db *DB) ListHealthyProxies() ([]models.Proxy, error) {
269: 	rows, err := db.conn.Query(`SELECT id, server, protocol, username, password, geo, status, latency, success_rate, total_used, last_checked, created_at
270: 		FROM proxies WHERE status = 'healthy' ORDER BY success_rate DESC, latency ASC`)
271: 	if err != nil {
272: 		return nil, fmt.Errorf("query healthy proxies: %w", err)
273: 	}
274: 	defer rows.Close()
275: 
276: 	var proxies []models.Proxy
277: 	for rows.Next() {
278: 		p, err := db.scanProxyRow(rows)
279: 		if err != nil {
280: 			return nil, fmt.Errorf("scan proxy row: %w", err)
281: 		}
282: 		proxies = append(proxies, *p)
283: 	}
284: 	if err := rows.Err(); err != nil {
285: 		return nil, fmt.Errorf("iterate proxies: %w", err)
286: 	}
287: 	return proxies, nil
288: }
289: 
290: // UpdateProxyHealth updates proxy health check data.
291: func (db *DB) UpdateProxyHealth(id string, status models.ProxyStatus, latency int) error {
292: 	now := time.Now()
293: 	_, err := db.conn.Exec(`UPDATE proxies SET status = ?, latency = ?, last_checked = ? WHERE id = ?`,
294: 		status, latency, now, id)
295: 	if err != nil {
296: 		return fmt.Errorf("update proxy %s health: %w", id, err)
297: 	}
298: 	return nil
299: }
300: 
301: // IncrementProxyUsage increments usage counter and updates success rate.
302: func (db *DB) IncrementProxyUsage(id string, success bool) error {
303: 	var err error
304: 	if success {
305: 		_, err = db.conn.Exec(`UPDATE proxies SET total_used = total_used + 1,
306: 			success_rate = (success_rate * total_used + 1.0) / (total_used + 1) WHERE id = ?`, id)
307: 	} else {
308: 		_, err = db.conn.Exec(`UPDATE proxies SET total_used = total_used + 1,
309: 			success_rate = (success_rate * total_used) / (total_used + 1) WHERE id = ?`, id)
310: 	}
311: 	if err != nil {
312: 		return fmt.Errorf("increment proxy %s usage: %w", id, err)
313: 	}
314: 	return nil
315: }
316: 
317: // DeleteProxy removes a proxy by ID.
318: func (db *DB) DeleteProxy(id string) error {
319: 	_, err := db.conn.Exec(`DELETE FROM proxies WHERE id = ?`, id)
320: 	if err != nil {
321: 		return fmt.Errorf("delete proxy %s: %w", id, err)
322: 	}
323: 	return nil
324: }
325: 
326: // --- Scan helpers ---
327: 
328: type scanner interface {
329: 	Scan(dest ...interface{}) error
330: }
331: 
332: func (db *DB) scanTask(row scanner) (*models.Task, error) {
333: 	var t models.Task
334: 	var stepsJSON, resultJSON, tagsJSON string
335: 	var startedAt, completedAt sql.NullTime
336: 
337: 	err := row.Scan(
338: 		&t.ID, &t.Name, &t.URL, &stepsJSON,
339: 		&t.Proxy.Server, &t.Proxy.Username, &t.Proxy.Password, &t.Proxy.Geo,
340: 		&t.Priority, &t.Status, &t.RetryCount, &t.MaxRetries, &t.Error,
341: 		&resultJSON, &tagsJSON, &t.CreatedAt, &startedAt, &completedAt,
342: 	)
343: 	if err != nil {
344: 		return nil, err
345: 	}
346: 
347: 	if startedAt.Valid {
348: 		t.StartedAt = &startedAt.Time
349: 	}
350: 	if completedAt.Valid {
351: 		t.CompletedAt = &completedAt.Time
352: 	}
353: 
354: 	if stepsJSON != "" {
355: 		if err := json.Unmarshal([]byte(stepsJSON), &t.Steps); err != nil {
356: 			return nil, fmt.Errorf("parse steps JSON: %w", err)
357: 		}
358: 	}
359: 	if resultJSON != "" {
360: 		var result models.TaskResult
361: 		if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
362: 			return nil, fmt.Errorf("parse result JSON: %w", err)
363: 		}
364: 		t.Result = &result
365: 	}
366: 	if tagsJSON != "" {
367: 		if err := json.Unmarshal([]byte(tagsJSON), &t.Tags); err != nil {
368: 			return nil, fmt.Errorf("parse tags JSON: %w", err)
369: 		}
370: 	}
371: 
372: 	return &t, nil
373: }
374: 
375: func (db *DB) scanTaskRow(rows *sql.Rows) (*models.Task, error) {
376: 	return db.scanTask(rows)
377: }
378: 
379: func (db *DB) scanProxyRow(rows *sql.Rows) (*models.Proxy, error) {
380: 	var p models.Proxy
381: 	var lastChecked sql.NullTime
382: 
383: 	err := rows.Scan(
384: 		&p.ID, &p.Server, &p.Protocol, &p.Username, &p.Password,
385: 		&p.Geo, &p.Status, &p.Latency, &p.SuccessRate, &p.TotalUsed,
386: 		&lastChecked, &p.CreatedAt,
387: 	)
388: 	if err != nil {
389: 		return nil, err
390: 	}
391: 	if lastChecked.Valid {
392: 		p.LastChecked = &lastChecked.Time
393: 	}
394: 	return &p, nil
395: }
396: 
397: // GetTaskStats returns task counts by status.
398: func (db *DB) GetTaskStats() (map[string]int, error) {
399: 	rows, err := db.conn.Query(`SELECT status, COUNT(*) FROM tasks GROUP BY status`)
400: 	if err != nil {
401: 		return nil, fmt.Errorf("query task stats: %w", err)
402: 	}
403: 	defer rows.Close()
404: 
405: 	stats := make(map[string]int)
406: 	for rows.Next() {
407: 		var status string
408: 		var count int
409: 		if err := rows.Scan(&status, &count); err != nil {
410: 			return nil, fmt.Errorf("scan task stats: %w", err)
411: 		}
412: 		stats[status] = count
413: 	}
414: 	if err := rows.Err(); err != nil {
415: 		return nil, fmt.Errorf("iterate task stats: %w", err)
416: 	}
417: 	return stats, nil
418: }

(End of file - total 418 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/frontend/src/App.svelte"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/frontend/src/App.svelte</path>
<type>file</type>
<content>1: <script lang="ts">
2:   import { onMount, onDestroy } from 'svelte';
3:   import Header from './components/Header.svelte';
4:   import TaskToolbar from './components/TaskToolbar.svelte';
5:   import TaskTable from './components/TaskTable.svelte';
6:   import TaskDetail from './components/TaskDetail.svelte';
7:   import CreateTaskModal from './components/CreateTaskModal.svelte';
8:   import ProxyPanel from './components/ProxyPanel.svelte';
9:   import { tasks, activeTab, updateTaskInStore } from './lib/store';
10:   import { ListTasks } from '../wailsjs/go/main/App';
11:   import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';
12: 
13:   let showCreateModal = false;
14: 
15:   async function refreshTasks() {
16:     try {
17:       const list = await ListTasks();
18:       tasks.set(list || []);
19:     } catch (err) {
20:       console.error('Failed to load tasks:', err);
21:     }
22:   }
23: 
24:   onMount(() => {
25:     refreshTasks();
26: 
27:     // Listen for real-time task events from Go backend
28:     EventsOn('task:event', (event: any) => {
29:       updateTaskInStore(event);
30:     });
31:   });
32: 
33:   onDestroy(() => {
34:     EventsOff('task:event');
35:   });
36: 
37:   // Auto-refresh every 5 seconds
38:   const interval = setInterval(refreshTasks, 5000);
39:   onDestroy(() => clearInterval(interval));
40: </script>
41: 
42: <div class="app-layout">
43:   <Header />
44: 
45:   <nav class="tabs">
46:     <button
47:       class="tab"
48:       class:active={$activeTab === 'tasks'}
49:       on:click={() => $activeTab = 'tasks'}
50:     >
51:       Tasks
52:     </button>
53:     <button
54:       class="tab"
55:       class:active={$activeTab === 'proxies'}
56:       on:click={() => $activeTab = 'proxies'}
57:     >
58:       Proxies
59:     </button>
60:   </nav>
61: 
62:   {#if $activeTab === 'tasks'}
63:     <TaskToolbar on:create={() => showCreateModal = true} />
64:     <div class="main-content">
65:       <TaskTable on:refresh={refreshTasks} />
66:       <TaskDetail />
67:     </div>
68:   {:else if $activeTab === 'proxies'}
69:     <ProxyPanel />
70:   {/if}
71: </div>
72: 
73: {#if showCreateModal}
74:   <CreateTaskModal
75:     on:close={() => showCreateModal = false}
76:     on:created={refreshTasks}
77:   />
78: {/if}
79: 
80: <style>
81:   .app-layout {
82:     display: flex;
83:     flex-direction: column;
84:     height: 100vh;
85:   }
86: 
87:   .tabs {
88:     display: flex;
89:     gap: 0;
90:     background: var(--bg-secondary);
91:     border-bottom: 1px solid var(--border);
92:     padding: 0 20px;
93:     flex-shrink: 0;
94:   }
95:   .tab {
96:     padding: 10px 20px;
97:     background: none;
98:     color: var(--text-muted);
99:     font-size: 13px;
100:     font-weight: 500;
101:     border-radius: 0;
102:     border-bottom: 2px solid transparent;
103:     transition: all 0.15s ease;
104:   }
105:   .tab:hover {
106:     color: var(--text-primary);
107:   }
108:   .tab.active {
109:     color: var(--accent);
110:     border-bottom-color: var(--accent);
111:   }
112: 
113:   .main-content {
114:     display: flex;
115:     flex: 1;
116:     overflow: hidden;
117:   }
118: </style>

(End of file - total 118 lines)
</content>
```

---

## Assistant (Pro-Prompts-Enhanced · duo-chat-opus-4-6 · 11.9s)

Now let me also check the frontend components and existing tests to get a complete picture:

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/frontend/src/components"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/frontend/src/components</path>
<type>directory</type>
<entries>
CreateTaskModal.svelte
Header.svelte
ProxyPanel.svelte
TaskDetail.svelte
TaskTable.svelte
TaskToolbar.svelte

(6 entries)
</entries>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/frontend/src/lib"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/frontend/src/lib</path>
<type>directory</type>
<entries>
store.ts
types.ts

(2 entries)
</entries>
```

**Tool: glob**

**Input:**
```json
{
  "pattern": "**/*_test.go"
}
```

**Output:**
```
/root/electron-web-test-automation/internal/database/sqlite_test.go
/root/electron-web-test-automation/internal/proxy/manager_test.go
/root/electron-web-test-automation/internal/browser/browser_test.go
/root/electron-web-test-automation/internal/queue/queue_test.go
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/go.mod"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/go.mod</path>
<type>file</type>
<content>1: module web-automation
2: 
3: go 1.24.0
4: 
5: toolchain go1.24.13
6: 
7: require (
8: 	github.com/chromedp/cdproto v0.0.0-20250803210736-d308e07a266d
9: 	github.com/chromedp/chromedp v0.14.2
10: 	github.com/google/uuid v1.6.0
11: 	github.com/mattn/go-sqlite3 v1.14.34
12: 	github.com/wailsapp/wails/v2 v2.11.0
13: 	golang.org/x/sync v0.19.0
14: )
15: 
16: require (
17: 	github.com/bep/debounce v1.2.1 // indirect
18: 	github.com/chromedp/sysutil v1.1.0 // indirect
19: 	github.com/go-json-experiment/json v0.0.0-20250725192818-e39067aee2d2 // indirect
20: 	github.com/go-ole/go-ole v1.3.0 // indirect
21: 	github.com/gobwas/httphead v0.1.0 // indirect
22: 	github.com/gobwas/pool v0.2.1 // indirect
23: 	github.com/gobwas/ws v1.4.0 // indirect
24: 	github.com/godbus/dbus/v5 v5.1.0 // indirect
25: 	github.com/gorilla/websocket v1.5.3 // indirect
26: 	github.com/jchv/go-winloader v0.0.0-20210711035445-715c2860da7e // indirect
27: 	github.com/labstack/echo/v4 v4.13.3 // indirect
28: 	github.com/labstack/gommon v0.4.2 // indirect
29: 	github.com/leaanthony/go-ansi-parser v1.6.1 // indirect
30: 	github.com/leaanthony/gosod v1.0.4 // indirect
31: 	github.com/leaanthony/slicer v1.6.0 // indirect
32: 	github.com/leaanthony/u v1.1.1 // indirect
33: 	github.com/mattn/go-colorable v0.1.13 // indirect
34: 	github.com/mattn/go-isatty v0.0.20 // indirect
35: 	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
36: 	github.com/pkg/errors v0.9.1 // indirect
37: 	github.com/rivo/uniseg v0.4.7 // indirect
38: 	github.com/samber/lo v1.49.1 // indirect
39: 	github.com/tkrajina/go-reflector v0.5.8 // indirect
40: 	github.com/valyala/bytebufferpool v1.0.0 // indirect
41: 	github.com/valyala/fasttemplate v1.2.2 // indirect
42: 	github.com/wailsapp/go-webview2 v1.0.22 // indirect
43: 	github.com/wailsapp/mimetype v1.4.1 // indirect
44: 	golang.org/x/crypto v0.33.0 // indirect
45: 	golang.org/x/net v0.35.0 // indirect
46: 	golang.org/x/sys v0.34.0 // indirect
47: 	golang.org/x/text v0.22.0 // indirect
48: )
49: 
50: // replace github.com/wailsapp/wails/v2 v2.11.0 => /root/go/pkg/mod

(End of file - total 50 lines)
</content>
```

---

## Assistant (Pro-Prompts-Enhanced · duo-chat-opus-4-6 · 7.6s)

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/frontend/src/lib/store.ts"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/frontend/src/lib/store.ts</path>
<type>file</type>
<content>1: import { writable, derived } from 'svelte/store';
2: import type { Task, Proxy, TaskEvent, TaskStatus } from './types';
3: 
4: export const tasks = writable<Task[]>([]);
5: export const proxies = writable<Proxy[]>([]);
6: export const selectedTaskId = writable<string | null>(null);
7: export const activeTab = writable<'tasks' | 'proxies' | 'logs'>('tasks');
8: export const statusFilter = writable<TaskStatus | 'all'>('all');
9: 
10: export const selectedTask = derived(
11:   [tasks, selectedTaskId],
12:   ([$tasks, $selectedTaskId]) => $tasks.find(t => t.id === $selectedTaskId) ?? null
13: );
14: 
15: export const filteredTasks = derived(
16:   [tasks, statusFilter],
17:   ([$tasks, $statusFilter]) => {
18:     if ($statusFilter === 'all') return $tasks;
19:     return $tasks.filter(t => t.status === $statusFilter);
20:   }
21: );
22: 
23: export const taskStats = derived(tasks, ($tasks) => {
24:   const stats: Record<string, number> = {
25:     total: $tasks.length,
26:     pending: 0,
27:     queued: 0,
28:     running: 0,
29:     completed: 0,
30:     failed: 0,
31:     cancelled: 0,
32:     retrying: 0,
33:   };
34:   for (const t of $tasks) {
35:     stats[t.status] = (stats[t.status] || 0) + 1;
36:   }
37:   return stats;
38: });
39: 
40: // Update a single task in the store
41: export function updateTaskInStore(event: TaskEvent) {
42:   tasks.update(list => 
43:     list.map(t => 
44:       t.id === event.taskId 
45:         ? { ...t, status: event.status, error: event.error || t.error }
46:         : t
47:     )
48:   );
49: }

(End of file - total 49 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/frontend/src/lib/types.ts"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/frontend/src/lib/types.ts</path>
<type>file</type>
<content>1: export type TaskStatus = 'pending' | 'queued' | 'running' | 'completed' | 'failed' | 'cancelled' | 'retrying';
2: 
3: export interface TaskStep {
4:   action: string;
5:   selector?: string;
6:   value?: string;
7:   timeout?: number;
8: }
9: 
10: export interface ProxyConfig {
11:   server: string;
12:   username?: string;
13:   password?: string;
14:   geo?: string;
15: }
16: 
17: export interface LogEntry {
18:   timestamp: string;
19:   level: string;
20:   message: string;
21: }
22: 
23: export interface TaskResult {
24:   taskId: string;
25:   success: boolean;
26:   extractedData?: Record<string, string>;
27:   screenshots?: string[];
28:   logs: LogEntry[];
29:   duration: number;
30:   error?: string;
31: }
32: 
33: export interface Task {
34:   id: string;
35:   name: string;
36:   url: string;
37:   steps: TaskStep[];
38:   proxy: ProxyConfig;
39:   priority: number;
40:   status: TaskStatus;
41:   retryCount: number;
42:   maxRetries: number;
43:   error?: string;
44:   result?: TaskResult;
45:   createdAt: string;
46:   startedAt?: string;
47:   completedAt?: string;
48:   tags?: string[];
49: }
50: 
51: export interface Proxy {
52:   id: string;
53:   server: string;
54:   protocol: string;
55:   username?: string;
56:   password?: string;
57:   geo?: string;
58:   status: string;
59:   latency: number;
60:   successRate: number;
61:   totalUsed: number;
62:   lastChecked?: string;
63:   createdAt: string;
64: }
65: 
66: export interface TaskEvent {
67:   taskId: string;
68:   status: TaskStatus;
69:   error?: string;
70:   log?: LogEntry;
71: }

(End of file - total 71 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/internal/browser/browser_test.go"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/internal/browser/browser_test.go</path>
<type>file</type>
<content>1: package browser
2: 
3: import (
4: 	"context"
5: 	"os"
6: 	"path/filepath"
7: 	"testing"
8: 	"time"
9: 
10: 	"web-automation/internal/models"
11: )
12: 
13: func TestNewRunner(t *testing.T) {
14: 	dir := t.TempDir()
15: 	screenshotDir := filepath.Join(dir, "screenshots")
16: 
17: 	runner, err := NewRunner(screenshotDir)
18: 	if err != nil {
19: 		t.Fatalf("NewRunner: %v", err)
20: 	}
21: 
22: 	if runner.screenshotDir != screenshotDir {
23: 		t.Errorf("screenshotDir: got %q, want %q", runner.screenshotDir, screenshotDir)
24: 	}
25: 
26: 	// Verify directory was created
27: 	info, err := os.Stat(screenshotDir)
28: 	if err != nil {
29: 		t.Fatalf("screenshot dir not created: %v", err)
30: 	}
31: 	if !info.IsDir() {
32: 		t.Error("screenshot path is not a directory")
33: 	}
34: }
35: 
36: func TestNewRunnerExistingDir(t *testing.T) {
37: 	dir := t.TempDir()
38: 	screenshotDir := filepath.Join(dir, "screenshots")
39: 	os.MkdirAll(screenshotDir, 0o755)
40: 
41: 	runner, err := NewRunner(screenshotDir)
42: 	if err != nil {
43: 		t.Fatalf("NewRunner with existing dir: %v", err)
44: 	}
45: 	if runner == nil {
46: 		t.Fatal("runner is nil")
47: 	}
48: }
49: 
50: func TestNewRunnerInvalidPath(t *testing.T) {
51: 	// Create a file where we'd want a directory
52: 	dir := t.TempDir()
53: 	filePath := filepath.Join(dir, "not-a-dir")
54: 	os.WriteFile(filePath, []byte("data"), 0o644)
55: 
56: 	_, err := NewRunner(filepath.Join(filePath, "screenshots"))
57: 	if err == nil {
58: 		t.Error("expected error for invalid path, got nil")
59: 	}
60: }
61: 
62: func TestAddLog(t *testing.T) {
63: 	runner := &Runner{screenshotDir: t.TempDir()}
64: 	result := &models.TaskResult{
65: 		TaskID: "test-log",
66: 	}
67: 
68: 	runner.addLog(result, "info", "test message 1")
69: 	runner.addLog(result, "error", "test error")
70: 	runner.addLog(result, "info", "test message 2")
71: 
72: 	if len(result.Logs) != 3 {
73: 		t.Fatalf("log count: got %d, want 3", len(result.Logs))
74: 	}
75: 
76: 	if result.Logs[0].Level != "info" {
77: 		t.Errorf("log[0].Level: got %q, want %q", result.Logs[0].Level, "info")
78: 	}
79: 	if result.Logs[0].Message != "test message 1" {
80: 		t.Errorf("log[0].Message: got %q, want %q", result.Logs[0].Message, "test message 1")
81: 	}
82: 	if result.Logs[1].Level != "error" {
83: 		t.Errorf("log[1].Level: got %q, want %q", result.Logs[1].Level, "error")
84: 	}
85: 
86: 	// Timestamps should be ordered
87: 	for i := 1; i < len(result.Logs); i++ {
88: 		if result.Logs[i].Timestamp.Before(result.Logs[i-1].Timestamp) {
89: 			t.Error("log timestamps are not ordered")
90: 		}
91: 	}
92: }
93: 
94: func TestExecuteStepUnknownAction(t *testing.T) {
95: 	runner := &Runner{screenshotDir: t.TempDir()}
96: 	result := &models.TaskResult{
97: 		TaskID:        "test-unknown",
98: 		ExtractedData: make(map[string]string),
99: 	}
100: 
101: 	step := models.TaskStep{
102: 		Action: "nonexistent_action",
103: 	}
104: 
105: 	err := runner.executeStep(context.Background(), step, result)
106: 	if err == nil {
107: 		t.Fatal("expected error for unknown action")
108: 	}
109: 	if err.Error() != "unknown action: nonexistent_action" {
110: 		t.Errorf("error message: got %q", err.Error())
111: 	}
112: }
113: 
114: func TestExecScrollValidation(t *testing.T) {
115: 	runner := &Runner{screenshotDir: t.TempDir()}
116: 
117: 	tests := []struct {
118: 		name    string
119: 		value   string
120: 		wantErr bool
121: 	}{
122: 		{"valid positive", "500", false},
123: 		{"valid negative", "-200", false},
124: 		{"valid zero", "0", false},
125: 		{"invalid text", "abc", true},
126: 		{"invalid float", "1.5", true},
127: 		{"invalid empty", "", true},
128: 		{"invalid js injection", "0); alert('xss", true},
129: 		{"invalid semicolon", "100; document.cookie", true},
130: 	}
131: 
132: 	for _, tc := range tests {
133: 		t.Run(tc.name, func(t *testing.T) {
134: 			step := models.TaskStep{
135: 				Action: models.ActionScroll,
136: 				Value:  tc.value,
137: 			}
138: 
139: 			// Note: execScroll will validate the value, but will fail on chromedp.Run
140: 			// in a test environment without a browser. We test that validation catches
141: 			// injection attempts before reaching chromedp.
142: 			err := runner.execScroll(context.Background(), step)
143: 			if tc.wantErr {
144: 				if err == nil {
145: 					t.Error("expected error, got nil")
146: 				}
147: 			}
148: 			// For valid values, we expect a chromedp error (no browser),
149: 			// NOT a validation error
150: 			if !tc.wantErr && err != nil {
151: 				// The error should be from chromedp, not from our validation
152: 				if err.Error() == "invalid scroll value \""+tc.value+"\": must be an integer" {
153: 					t.Errorf("got unexpected validation error for valid value %q", tc.value)
154: 				}
155: 			}
156: 		})
157: 	}
158: }
159: 
160: func TestExecWaitContextCancellation(t *testing.T) {
161: 	runner := &Runner{screenshotDir: t.TempDir()}
162: 
163: 	ctx, cancel := context.WithCancel(context.Background())
164: 
165: 	step := models.TaskStep{
166: 		Action: models.ActionWait,
167: 		Value:  "10000", // 10 seconds
168: 	}
169: 
170: 	done := make(chan error, 1)
171: 	go func() {
172: 		done <- runner.execWait(ctx, step)
173: 	}()
174: 
175: 	// Cancel after a short delay
176: 	time.Sleep(50 * time.Millisecond)
177: 	cancel()
178: 
179: 	select {
180: 	case err := <-done:
181: 		if err == nil {
182: 			t.Error("expected context.Canceled error")
183: 		}
184: 	case <-time.After(2 * time.Second):
185: 		t.Fatal("execWait did not respect context cancellation")
186: 	}
187: }
188: 
189: func TestExecWaitWithDuration(t *testing.T) {
190: 	runner := &Runner{screenshotDir: t.TempDir()}
191: 
192: 	step := models.TaskStep{
193: 		Action: models.ActionWait,
194: 		Value:  "100", // 100ms
195: 	}
196: 
197: 	start := time.Now()
198: 	err := runner.execWait(context.Background(), step)
199: 	elapsed := time.Since(start)
200: 
201: 	if err != nil {
202: 		t.Fatalf("execWait: %v", err)
203: 	}
204: 
205: 	if elapsed < 90*time.Millisecond {
206: 		t.Errorf("wait too short: %v", elapsed)
207: 	}
208: 	if elapsed > 500*time.Millisecond {
209: 		t.Errorf("wait too long: %v", elapsed)
210: 	}
211: }
212: 
213: func TestExecWaitWithSelector(t *testing.T) {
214: 	runner := &Runner{screenshotDir: t.TempDir()}
215: 
216: 	step := models.TaskStep{
217: 		Action:   models.ActionWait,
218: 		Selector: "#element",
219: 	}
220: 
221: 	// With a selector, it tries to use chromedp which will fail without a browser
222: 	err := runner.execWait(context.Background(), step)
223: 	if err == nil {
224: 		t.Error("expected error when running chromedp without browser")
225: 	}
226: }
227: 
228: func TestExecWaitInvalidDuration(t *testing.T) {
229: 	runner := &Runner{screenshotDir: t.TempDir()}
230: 
231: 	step := models.TaskStep{
232: 		Action: models.ActionWait,
233: 		Value:  "not-a-number",
234: 	}
235: 
236: 	// Should default to 1 second and not error
237: 	start := time.Now()
238: 	err := runner.execWait(context.Background(), step)
239: 	elapsed := time.Since(start)
240: 
241: 	if err != nil {
242: 		t.Fatalf("execWait with invalid duration: %v", err)
243: 	}
244: 	// Should default to 1 second
245: 	if elapsed < 900*time.Millisecond {
246: 		t.Errorf("should default to 1s, got %v", elapsed)
247: 	}
248: }
249: 
250: func TestRunStepsEmptySteps(t *testing.T) {
251: 	runner := &Runner{screenshotDir: t.TempDir()}
252: 	result := &models.TaskResult{
253: 		TaskID:        "empty-steps",
254: 		ExtractedData: make(map[string]string),
255: 	}
256: 
257: 	err := runner.runSteps(context.Background(), nil, result)
258: 	if err != nil {
259: 		t.Fatalf("runSteps with nil steps: %v", err)
260: 	}
261: 
262: 	err = runner.runSteps(context.Background(), []models.TaskStep{}, result)
263: 	if err != nil {
264: 		t.Fatalf("runSteps with empty steps: %v", err)
265: 	}
266: }
267: 
268: func TestRunStepsStopsOnError(t *testing.T) {
269: 	runner := &Runner{screenshotDir: t.TempDir()}
270: 	result := &models.TaskResult{
271: 		TaskID:        "stop-on-error",
272: 		ExtractedData: make(map[string]string),
273: 	}
274: 
275: 	steps := []models.TaskStep{
276: 		{Action: "invalid_action_1"},
277: 		{Action: "invalid_action_2"},
278: 	}
279: 
280: 	err := runner.runSteps(context.Background(), steps, result)
281: 	if err == nil {
282: 		t.Fatal("expected error from invalid steps")
283: 	}
284: 
285: 	// Should have logged the attempt and failure of step 1
286: 	// but NOT attempted step 2
287: 	infoCount := 0
288: 	errorCount := 0
289: 	for _, log := range result.Logs {
290: 		if log.Level == "info" {
291: 			infoCount++
292: 		}
293: 		if log.Level == "error" {
294: 			errorCount++
295: 		}
296: 	}
297: 
298: 	// 1 info (start of step 1) + 1 error (failure of step 1)
299: 	if infoCount != 1 {
300: 		t.Errorf("expected 1 info log, got %d", infoCount)
301: 	}
302: 	if errorCount != 1 {
303: 		t.Errorf("expected 1 error log, got %d", errorCount)
304: 	}
305: }
306: 
307: func TestCreateAllocatorWithoutProxy(t *testing.T) {
308: 	runner := &Runner{screenshotDir: t.TempDir()}
309: 
310: 	ctx := context.Background()
311: 	allocCtx, allocCancel := runner.createAllocator(ctx, models.ProxyConfig{})
312: 	defer allocCancel()
313: 
314: 	if allocCtx == nil {
315: 		t.Fatal("allocator context is nil")
316: 	}
317: }
318: 
319: func TestCreateAllocatorWithProxy(t *testing.T) {
320: 	runner := &Runner{screenshotDir: t.TempDir()}
321: 
322: 	proxy := models.ProxyConfig{
323: 		Server:   "proxy.example.com:8080",
324: 		Username: "user",
325: 		Password: "pass",
326: 	}
327: 
328: 	ctx := context.Background()
329: 	allocCtx, allocCancel := runner.createAllocator(ctx, proxy)
330: 	defer allocCancel()
331: 
332: 	if allocCtx == nil {
333: 		t.Fatal("allocator context is nil")
334: 	}
335: }
336: 
337: func TestCreateAllocatorDoesNotMutateDefaults(t *testing.T) {
338: 	runner := &Runner{screenshotDir: t.TempDir()}
339: 
340: 	// Record the original default options length
341: 	originalLen := len(chromedpDefaultOpts())
342: 
343: 	ctx := context.Background()
344: 	for i := 0; i < 10; i++ {
345: 		_, cancel := runner.createAllocator(ctx, models.ProxyConfig{
346: 			Server: "proxy.example.com:8080",
347: 		})
348: 		cancel()
349: 	}
350: 
351: 	// Default options should not have grown
352: 	if len(chromedpDefaultOpts()) != originalLen {
353: 		t.Errorf("DefaultExecAllocatorOptions mutated: original len %d, now %d",
354: 			originalLen, len(chromedpDefaultOpts()))
355: 	}
356: }
357: 
358: // chromedpDefaultOpts returns the current default options for testing.
359: func chromedpDefaultOpts() []func(*chromedpAllocator) {
360: 	// This is a proxy to check the shared state isn't mutated.
361: 	// We can't directly import chromedp types, so we check the slice length.
362: 	return nil // placeholder - the real check is in createAllocator's copy logic
363: }
364: 
365: // chromedpAllocator is a placeholder type for testing.
366: type chromedpAllocator struct{}
367: 
368: func TestExecuteStepDispatchesCorrectly(t *testing.T) {
369: 	runner := &Runner{screenshotDir: t.TempDir()}
370: 	result := &models.TaskResult{
371: 		TaskID:        "dispatch-test",
372: 		ExtractedData: make(map[string]string),
373: 	}
374: 
375: 	// All valid actions should not return "unknown action" error.
376: 	// They will fail with chromedp errors (no browser), but that's expected.
377: 	validActions := []models.StepAction{
378: 		models.ActionNavigate,
379: 		models.ActionClick,
380: 		models.ActionType,
381: 		models.ActionScreenshot,
382: 		models.ActionExtract,
383: 		models.ActionSelect,
384: 		models.ActionEval,
385: 	}
386: 
387: 	for _, action := range validActions {
388: 		step := models.TaskStep{
389: 			Action:   action,
390: 			Selector: "#test",
391: 			Value:    "test-value",
392: 		}
393: 		err := runner.executeStep(context.Background(), step, result)
394: 		if err != nil && err.Error() == "unknown action: "+string(action) {
395: 			t.Errorf("action %s was not dispatched correctly", action)
396: 		}
397: 	}
398: }

(End of file - total 398 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/root/electron-web-test-automation/internal/queue/queue_test.go"
}
```

**Output:**
```
<path>/root/electron-web-test-automation/internal/queue/queue_test.go</path>
<type>file</type>
<content>1: package queue
2: 
3: import (
4: 	"context"
5: 	"errors"
6: 	"fmt"
7: 	"path/filepath"
8: 	"sync"
9: 	"sync/atomic"
10: 	"testing"
11: 	"time"
12: 
13: 	"web-automation/internal/browser"
14: 	"web-automation/internal/database"
15: 	"web-automation/internal/models"
16: )
17: 
18: func setupTestQueue(t *testing.T, maxConcurrency int, events *[]models.TaskEvent, mu *sync.Mutex) (*Queue, *database.DB) {
19: 	t.Helper()
20: 	dir := t.TempDir()
21: 	db, err := database.New(filepath.Join(dir, "test.db"))
22: 	if err != nil {
23: 		t.Fatalf("failed to create test db: %v", err)
24: 	}
25: 	t.Cleanup(func() { db.Close() })
26: 
27: 	runner, err := browser.NewRunner(filepath.Join(dir, "screenshots"))
28: 	if err != nil {
29: 		t.Fatalf("failed to create runner: %v", err)
30: 	}
31: 
32: 	var onEvent EventCallback
33: 	if events != nil {
34: 		onEvent = func(event models.TaskEvent) {
35: 			mu.Lock()
36: 			*events = append(*events, event)
37: 			mu.Unlock()
38: 		}
39: 	}
40: 
41: 	q := New(db, runner, maxConcurrency, onEvent)
42: 	return q, db
43: }
44: 
45: func makeTestTask(id string) models.Task {
46: 	return models.Task{
47: 		ID:   id,
48: 		Name: "Test Task " + id,
49: 		URL:  "https://example.com",
50: 		Steps: []models.TaskStep{
51: 			{Action: models.ActionNavigate, Value: "https://example.com"},
52: 		},
53: 		Priority:   models.PriorityNormal,
54: 		Status:     models.TaskStatusPending,
55: 		MaxRetries: 2,
56: 		CreatedAt:  time.Now(),
57: 	}
58: }
59: 
60: func TestNewQueue(t *testing.T) {
61: 	dir := t.TempDir()
62: 	db, err := database.New(filepath.Join(dir, "test.db"))
63: 	if err != nil {
64: 		t.Fatalf("create db: %v", err)
65: 	}
66: 	defer db.Close()
67: 
68: 	runner, err := browser.NewRunner(filepath.Join(dir, "screenshots"))
69: 	if err != nil {
70: 		t.Fatalf("create runner: %v", err)
71: 	}
72: 
73: 	q := New(db, runner, 50, nil)
74: 	if q.maxConcurrency != 50 {
75: 		t.Errorf("maxConcurrency: got %d, want 50", q.maxConcurrency)
76: 	}
77: 	if q.RunningCount() != 0 {
78: 		t.Errorf("initial RunningCount: got %d, want 0", q.RunningCount())
79: 	}
80: }
81: 
82: func TestSubmitUpdatesStatusToQueued(t *testing.T) {
83: 	var events []models.TaskEvent
84: 	var mu sync.Mutex
85: 	q, db := setupTestQueue(t, 10, &events, &mu)
86: 	defer q.Stop()
87: 
88: 	task := makeTestTask("submit-1")
89: 	if err := db.CreateTask(task); err != nil {
90: 		t.Fatalf("CreateTask: %v", err)
91: 	}
92: 
93: 	ctx := context.Background()
94: 	if err := q.Submit(ctx, task); err != nil {
95: 		t.Fatalf("Submit: %v", err)
96: 	}
97: 
98: 	// Give the goroutine time to start
99: 	time.Sleep(200 * time.Millisecond)
100: 
101: 	// Check that the task was marked as queued
102: 	got, err := db.GetTask("submit-1")
103: 	if err != nil {
104: 		t.Fatalf("GetTask: %v", err)
105: 	}
106: 	// It might be queued, running, or even failed (since chromedp won't actually work in test)
107: 	if got.Status == models.TaskStatusPending {
108: 		t.Error("task should no longer be pending after submit")
109: 	}
110: 
111: 	mu.Lock()
112: 	foundQueued := false
113: 	for _, e := range events {
114: 		if e.TaskID == "submit-1" && e.Status == models.TaskStatusQueued {
115: 			foundQueued = true
116: 		}
117: 	}
118: 	mu.Unlock()
119: 
120: 	if !foundQueued {
121: 		t.Error("expected queued event to be emitted")
122: 	}
123: }
124: 
125: func TestSubmitOnStoppedQueue(t *testing.T) {
126: 	q, _ := setupTestQueue(t, 10, nil, nil)
127: 	q.Stop()
128: 
129: 	task := makeTestTask("stopped-1")
130: 	err := q.Submit(context.Background(), task)
131: 	if err == nil {
132: 		t.Fatal("expected error when submitting to stopped queue")
133: 	}
134: }
135: 
136: func TestCancel(t *testing.T) {
137: 	q, db := setupTestQueue(t, 10, nil, nil)
138: 	defer q.Stop()
139: 
140: 	task := makeTestTask("cancel-1")
141: 	if err := db.CreateTask(task); err != nil {
142: 		t.Fatalf("CreateTask: %v", err)
143: 	}
144: 
145: 	ctx := context.Background()
146: 	if err := q.Submit(ctx, task); err != nil {
147: 		t.Fatalf("Submit: %v", err)
148: 	}
149: 
150: 	// Cancel the task
151: 	time.Sleep(100 * time.Millisecond)
152: 	if err := q.Cancel("cancel-1"); err != nil {
153: 		t.Fatalf("Cancel: %v", err)
154: 	}
155: 
156: 	got, err := db.GetTask("cancel-1")
157: 	if err != nil {
158: 		t.Fatalf("GetTask: %v", err)
159: 	}
160: 	if got.Status != models.TaskStatusCancelled {
161: 		t.Errorf("status after cancel: got %q, want %q", got.Status, models.TaskStatusCancelled)
162: 	}
163: }
164: 
165: func TestStopCancelsAllRunning(t *testing.T) {
166: 	q, _ := setupTestQueue(t, 10, nil, nil)
167: 
168: 	// Manually register some fake running tasks
169: 	q.mu.Lock()
170: 	cancel1Done := make(chan struct{})
171: 	cancel2Done := make(chan struct{})
172: 	q.running["fake-1"] = func() { close(cancel1Done) }
173: 	q.running["fake-2"] = func() { close(cancel2Done) }
174: 	q.mu.Unlock()
175: 
176: 	q.Stop()
177: 
178: 	// Verify all cancels were called
179: 	select {
180: 	case <-cancel1Done:
181: 	case <-time.After(time.Second):
182: 		t.Error("cancel for fake-1 was not called")
183: 	}
184: 	select {
185: 	case <-cancel2Done:
186: 	case <-time.After(time.Second):
187: 		t.Error("cancel for fake-2 was not called")
188: 	}
189: 
190: 	// Verify running map is empty
191: 	if q.RunningCount() != 0 {
192: 		t.Errorf("RunningCount after Stop: got %d, want 0", q.RunningCount())
193: 	}
194: }
195: 
196: func TestStopIdempotent(t *testing.T) {
197: 	q, _ := setupTestQueue(t, 10, nil, nil)
198: 
199: 	// Should not panic when called multiple times
200: 	q.Stop()
201: 	q.Stop()
202: 	q.Stop()
203: }
204: 
205: func TestSubmitBatch(t *testing.T) {
206: 	q, db := setupTestQueue(t, 10, nil, nil)
207: 	defer q.Stop()
208: 
209: 	tasks := make([]models.Task, 5)
210: 	for i := range tasks {
211: 		tasks[i] = makeTestTask(fmt.Sprintf("batch-%d", i))
212: 		if err := db.CreateTask(tasks[i]); err != nil {
213: 			t.Fatalf("CreateTask %d: %v", i, err)
214: 		}
215: 	}
216: 
217: 	ctx := context.Background()
218: 	if err := q.SubmitBatch(ctx, tasks); err != nil {
219: 		t.Fatalf("SubmitBatch: %v", err)
220: 	}
221: 
222: 	// Wait for tasks to be processed
223: 	time.Sleep(500 * time.Millisecond)
224: 
225: 	// Verify all tasks were submitted (status changed from pending)
226: 	for _, task := range tasks {
227: 		got, err := db.GetTask(task.ID)
228: 		if err != nil {
229: 			t.Fatalf("GetTask %s: %v", task.ID, err)
230: 		}
231: 		if got.Status == models.TaskStatusPending {
232: 			t.Errorf("task %s should not be pending after batch submit", task.ID)
233: 		}
234: 	}
235: }
236: 
237: func TestRunningCount(t *testing.T) {
238: 	q, _ := setupTestQueue(t, 10, nil, nil)
239: 	defer q.Stop()
240: 
241: 	if q.RunningCount() != 0 {
242: 		t.Errorf("initial RunningCount: got %d, want 0", q.RunningCount())
243: 	}
244: 
245: 	// Add some fake entries
246: 	q.mu.Lock()
247: 	q.running["a"] = func() {}
248: 	q.running["b"] = func() {}
249: 	q.mu.Unlock()
250: 
251: 	if q.RunningCount() != 2 {
252: 		t.Errorf("RunningCount: got %d, want 2", q.RunningCount())
253: 	}
254: }
255: 
256: func TestConcurrencyLimit(t *testing.T) {
257: 	var maxConcurrent int64
258: 	var current int64
259: 
260: 	dir := t.TempDir()
261: 	db, err := database.New(filepath.Join(dir, "test.db"))
262: 	if err != nil {
263: 		t.Fatalf("create db: %v", err)
264: 	}
265: 	defer db.Close()
266: 
267: 	runner, err := browser.NewRunner(filepath.Join(dir, "screenshots"))
268: 	if err != nil {
269: 		t.Fatalf("create runner: %v", err)
270: 	}
271: 
272: 	maxConc := 3
273: 	q := New(db, runner, maxConc, nil)
274: 	defer q.Stop()
275: 
276: 	// We can't easily test actual execution concurrency without mocking the runner,
277: 	// but we can verify the queue was created with the right concurrency
278: 	if q.maxConcurrency != int64(maxConc) {
279: 		t.Errorf("maxConcurrency: got %d, want %d", q.maxConcurrency, maxConc)
280: 	}
281: 
282: 	// Test that atomic tracking would work
283: 	for i := 0; i < 10; i++ {
284: 		val := atomic.AddInt64(&current, 1)
285: 		if val > atomic.LoadInt64(&maxConcurrent) {
286: 			atomic.StoreInt64(&maxConcurrent, val)
287: 		}
288: 		if val > int64(maxConc) {
289: 			// This tests our tracking logic, not the queue's semaphore
290: 			// Real semaphore testing would require mocking
291: 		}
292: 		atomic.AddInt64(&current, -1)
293: 	}
294: }
295: 
296: func TestEmitEvent(t *testing.T) {
297: 	var events []models.TaskEvent
298: 	var mu sync.Mutex
299: 
300: 	q, _ := setupTestQueue(t, 10, &events, &mu)
301: 	defer q.Stop()
302: 
303: 	q.emitEvent("test-id", models.TaskStatusRunning, "")
304: 	q.emitEvent("test-id", models.TaskStatusFailed, "some error")
305: 
306: 	mu.Lock()
307: 	defer mu.Unlock()
308: 
309: 	if len(events) != 2 {
310: 		t.Fatalf("event count: got %d, want 2", len(events))
311: 	}
312: 	if events[0].Status != models.TaskStatusRunning {
313: 		t.Errorf("event 0 status: got %q, want %q", events[0].Status, models.TaskStatusRunning)
314: 	}
315: 	if events[1].Error != "some error" {
316: 		t.Errorf("event 1 error: got %q, want %q", events[1].Error, "some error")
317: 	}
318: }
319: 
320: func TestEmitEventNilCallback(t *testing.T) {
321: 	q, _ := setupTestQueue(t, 10, nil, nil)
322: 	defer q.Stop()
323: 
324: 	// Should not panic with nil callback
325: 	q.emitEvent("test-id", models.TaskStatusRunning, "")
326: }
327: 
328: func TestSubmitContextCancelled(t *testing.T) {
329: 	q, db := setupTestQueue(t, 10, nil, nil)
330: 	defer q.Stop()
331: 
332: 	task := makeTestTask("ctx-cancel-1")
333: 	if err := db.CreateTask(task); err != nil {
334: 		t.Fatalf("CreateTask: %v", err)
335: 	}
336: 
337: 	ctx, cancel := context.WithCancel(context.Background())
338: 	cancel() // Cancel immediately
339: 
340: 	// Submit should succeed (it just marks as queued and spawns goroutine)
341: 	// but the goroutine should handle the cancelled context
342: 	err := q.Submit(ctx, task)
343: 	if err != nil {
344: 		// The db update might fail if context is already cancelled in some implementations
345: 		// but in SQLite it should be fine
346: 		if !errors.Is(err, context.Canceled) {
347: 			t.Fatalf("unexpected error: %v", err)
348: 		}
349: 	}
350: 
351: 	time.Sleep(200 * time.Millisecond)
352: }

(End of file - total 352 lines)
</content>
```

---

## Assistant (Pro-Prompts-Enhanced · duo-chat-opus-4-6 · 15.9s)

Now I have a thorough understanding of the entire codebase. Let me now craft the optimized prompt.

**Tool: sequential-thinking_sequentialthinking**

**Input:**
```json
{
  "thought": "Let me analyze what the user wants: they asked me to \"see the codebase then tell a new next tasks prompt.\" This means they want me to:\n1. Understand the codebase (done)\n2. Generate an optimized prompt that an AI can use to identify and suggest \"next tasks\" for this project\n\nThe codebase is a Go + Wails + Svelte desktop app for web automation using chromedp. Key components:\n- Backend: Go with Wails framework, SQLite DB, chromedp browser automation, task queue with concurrency, proxy management\n- Frontend: Svelte + TypeScript with task management UI, proxy panel\n- Tests: Go unit tests exist for browser, queue, database, proxy\n\nWhat's missing / what are logical next tasks:\n1. No frontend tests (Svelte)\n2. No E2E tests\n3. README is generic Wails template - no project-specific docs\n4. No CI/CD pipeline\n5. No task scheduling/cron support\n6. No task templates/presets feature\n7. The eval action is potentially dangerous (arbitrary JS execution)\n8. No authentication/authorization\n9. No pagination on ListTasks\n10. No logging infrastructure beyond task logs\n11. No settings/configuration UI\n12. Frontend components could use more features (search, sort, bulk operations)\n13. No error boundary/toast notification system\n14. Tags feature exists in model but may not be fully wired up in UI\n15. No import functionality (only export)\n16. Proxy pool integration with tasks not fully wired (task has ProxyConfig but proxy selection from pool not automated)\n\nNow I need to create an optimized \"next tasks\" prompt that an LLM can use to analyze this codebase and suggest prioritized next development tasks.",
  "nextThoughtNeeded": false,
  "thoughtNumber": 1,
  "totalThoughts": 1
}
```

**Output:**
```
{
  "thoughtNumber": 1,
  "totalThoughts": 1,
  "nextThoughtNeeded": false,
  "branches": [],
  "thoughtHistoryLength": 1
}
```

---

## Assistant (Pro-Prompts-Enhanced · duo-chat-opus-4-6 · 65.2s)

---

# PROMPT ANALYSIS
===============

**Original Prompt:** `"firstt see codebase then tell a new next tasks prompt..."`

**Assessment:** This is an informal, highly abbreviated request with zero structure. It conveys intent — "explore this codebase and generate a prompt for suggesting next development tasks" — but provides no constraints on output format, audience, depth, prioritization criteria, or what "tasks" means.

**Key Issues:**
1. No role assignment — who should the LLM be when analyzing?
2. No specificity on what kind of "tasks" (features, bugs, testing, security, refactoring?)
3. No output format — how should tasks be structured?
4. No prioritization framework — what matters most?
5. No codebase context injection — how does the LLM know what exists?
6. No constraints on scope or count
7. No chain-of-thought guidance for the analysis
8. No reusability — can't be used for another project

---

# OPTIMIZED VARIANTS
==================

## Variant A — Structured Enhancement

```
You are a senior software architect conducting a project health assessment.

<context>
You are analyzing a desktop web automation application built with:
- Backend: Go 1.24, Wails v2 (desktop framework), chromedp (headless Chrome automation), SQLite
- Frontend: Svelte 3 + TypeScript, Vite
- Architecture: Task queue with concurrency control (semaphore), proxy pool with rotation strategies, real-time events via Wails EventsEmit
- Current state: 2 commits on main branch, core features implemented (task CRUD, browser automation with 9 step actions, proxy management with health checks, JSON/CSV export)
- Existing tests: Go unit tests for browser, queue, database, proxy packages. No frontend tests. No E2E tests.
</context>

<task>
Analyze this codebase and produce a prioritized list of the next development tasks. Evaluate across these categories:
1. **Security** — vulnerabilities, input validation gaps, secrets management
2. **Testing** — coverage gaps, missing test types (unit, integration, E2E)
3. **Features** — high-value features that are partially built or logically next
4. **Code Quality** — refactoring, error handling, documentation
5. **DevOps** — CI/CD, build pipeline, deployment
6. **UX** — frontend gaps, missing user-facing functionality
</task>

<format>
For each task, provide:
- **Title**: One-line description
- **Category**: Security | Testing | Feature | Quality | DevOps | UX
- **Priority**: P0 (Critical) | P1 (High) | P2 (Medium) | P3 (Low)
- **Effort**: S (hours) | M (1-3 days) | L (1-2 weeks) | XL (2+ weeks)
- **Rationale**: 1-2 sentences explaining WHY this task matters
- **Acceptance Criteria**: Bullet list of what "done" looks like

Group tasks by priority tier. Maximum 15 tasks.
</format>

<rules>
- Prioritize security issues above all else
- Only suggest tasks that are grounded in actual codebase gaps (no generic advice)
- Do NOT suggest rewriting existing working code unless there's a security risk
- Consider the project's maturity level (early-stage MVP)
</rules>
```

---

## Variant B — Chain-of-Thought / Reasoning

```
You are a tech lead performing a sprint planning review for a Go + Wails + Svelte desktop web automation tool.

Think step by step through this analysis:

**Step 1 — Inventory What Exists**
List every implemented feature, every file, and its purpose. Map the data flow from frontend → Wails binding → Go backend → browser/database.

**Step 2 — Identify Gaps by Layer**
For each architectural layer (models, database, browser engine, queue, proxy manager, frontend store, frontend components), answer:
- What is fully implemented?
- What is partially implemented (model exists but not wired to UI)?
- What is missing entirely?

**Step 3 — Security Audit**
Check for:
- Arbitrary code execution risks (eval action)
- Input validation on all user-facing Go methods
- Proxy credential storage security
- SQL injection vectors
- XSS in the Svelte frontend

**Step 4 — Test Coverage Assessment**
Review existing test files. For each package:
- What percentage of functions have tests?
- What critical paths are untested?
- What test types are missing (integration, E2E)?

**Step 5 — Prioritize and Propose**
Based on Steps 1-4, generate a prioritized task list. For each task explain:
- What gap it addresses (reference specific files/functions)
- Why it should be done before other tasks
- Estimated complexity (T-shirt size: S/M/L/XL)

**Output Format:**

## Analysis Summary
[2-3 paragraph overview of project health]

## Priority Tiers

### P0 — Must Do Before Next Release
[Tasks with file references and rationale]

### P1 — Should Do This Sprint  
[Tasks with file references and rationale]

### P2 — Nice to Have
[Tasks with file references and rationale]

Show your reasoning at each step before presenting the final task list.
```

---

## Variant C — Maximum Precision

```
<system>
You are a principal engineer performing a comprehensive codebase audit for sprint planning. You are methodical, security-conscious, and ruthlessly practical. You never suggest generic improvements — every recommendation must reference a specific file, function, or architectural gap.
</system>

<project_context>
Repository: web-automation (Go 1.24 + Wails v2 + Svelte 3 + TypeScript)
Purpose: Desktop app for automated browser task execution with proxy rotation
Commit count: 2 (early MVP stage)
Tech stack:
  - Go backend: main.go, app.go (Wails bindings), internal/{browser,database,queue,proxy,models}
  - Frontend: Svelte components (Header, TaskToolbar, TaskTable, TaskDetail, CreateTaskModal, ProxyPanel)
  - State: Svelte stores (store.ts), types (types.ts)
  - DB: SQLite with WAL mode, single-writer
  - Browser: chromedp with 9 step actions (navigate, click, type, wait, screenshot, extract, scroll, select, eval)
  - Queue: Semaphore-based concurrency (max 100), exponential backoff retry
  - Proxy: Round-robin/random/least-used/lowest-latency rotation, periodic health checks
  
Existing test files: browser_test.go, sqlite_test.go, queue_test.go, manager_test.go
Frontend tests: NONE
E2E tests: NONE
CI/CD: NONE
</project_context>

<task>
Generate exactly 12 prioritized next development tasks.
</task>

<constraints>
- MUST reference specific files, functions, or code locations for every task
- MUST NOT suggest tasks for features that already work correctly
- MUST prioritize: Security > Testing > Feature Completeness > UX > DevOps
- MUST estimate effort as hours (e.g., "~4h", "~16h", "~40h")
- MUST include at least 2 security tasks, 3 testing tasks, and 2 feature tasks
- MUST NOT exceed 12 tasks total
- If a model/type exists but isn't wired to UI, flag it specifically
</constraints>

<output_schema>
Return a JSON array with this exact structure:

```json
[
  {
    "rank": 1,
    "title": "string — imperative verb + specific target",
    "category": "security | testing | feature | quality | devops | ux",
    "priority": "P0 | P1 | P2",
    "effort_hours": "number",
    "files_affected": ["list of specific file paths"],
    "rationale": "string — why this matters, referencing specific code",
    "acceptance_criteria": ["list of measurable done conditions"],
    "depends_on": ["list of task ranks this depends on, or empty"]
  }
]
```
</output_schema>

<verification>
After generating the list:
1. Verify every file reference exists in the project
2. Verify no two tasks overlap in scope
3. Verify dependency ordering is logically sound
4. Verify security tasks appear in top 4 ranks
5. Confirm total estimated hours and flag if > 200h
</verification>
```

---

# COMPARISON MATRIX
=================

```
PROMPT COMPARISON MATRIX
========================

| Dimension        | Original | Variant A | Variant B | Variant C |
|-----------------|----------|-----------|-----------|-----------|
| Clarity          |    2     |     8     |     8     |     9     |
| Specificity      |    1     |     8     |     7     |    10     |
| Structure        |    1     |     9     |     8     |    10     |
| Context          |    1     |     8     |     6     |    10     |
| Output Control   |    1     |     9     |     7     |    10     |
| Edge Cases       |    0     |     6     |     5     |     9     |
| Chain-of-Thought |    0     |     3     |    10     |     5     |
| Reusability      |    1     |     7     |     8     |     6     |
|-----------------|----------|-----------|-----------|-----------|
| TOTAL            |   7/80   |   58/80   |   59/80   |   69/80   |
```

---

# RECOMMENDATION
==============

**Best: Variant C — Maximum Precision**

**Why it wins:**
- **Highest specificity (10/10)**: Requires exact file references, preventing generic "improve error handling" fluff
- **Strongest output control (10/10)**: JSON schema means structured, parseable, actionable output
- **Best context injection (10/10)**: Provides full project inventory so the LLM doesn't have to guess
- **Self-verification step**: Catches hallucinated file paths and logical inconsistencies
- **Hard constraints**: "exactly 12 tasks" with category minimums prevents both under- and over-delivery
- **Dependency tracking**: `depends_on` field creates a natural execution order

**When to use the alternatives:**

| Variant | Best For |
|---------|----------|
| **A (Structured)** | Quick sprint planning where you want readable markdown output — good for sharing with non-technical stakeholders |
| **B (Chain-of-Thought)** | Deep architectural reviews where the *reasoning process* matters as much as the output — great for onboarding a new team member to understand the codebase |
| **C (Maximum Precision)** | Automated pipeline integration, task tracker import (JSON), or when you need machine-actionable output with zero ambiguity |

---

# APPLIED TECHNIQUES
==================

| Technique | Where Applied | Why |
|-----------|---------------|-----|
| **Role Priming** | All 3 variants ("senior architect", "tech lead", "principal engineer") | Activates domain expertise framing |
| **Context Windowing** | Variant C's `<project_context>` block | Eliminates guessing about tech stack |
| **Structured Output** | Variant A (markdown template), C (JSON schema) | Ensures consistent, parseable results |
| **Chain-of-Thought** | Variant B's 5-step analysis | Forces systematic reasoning before conclusions |
| **Negative Prompting** | All variants ("do NOT suggest rewriting working code") | Prevents generic filler recommendations |
| **Constraint Specification** | Variant C ("exactly 12", "at least 2 security") | Precise scope control |
| **Self-Verification** | Variant C's verification block | Catches hallucinations and logic errors |
| **Few-Shot Schema** | Variant C's JSON example | Eliminates format ambiguity |
| **Priority Anchoring** | All variants ("security above all else") | Aligns output with business reality |
| **Specificity Forcing** | Variant C ("MUST reference specific files") | Grounds every recommendation in reality |

---

