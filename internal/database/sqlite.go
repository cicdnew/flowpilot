package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"flowpilot/internal/crypto"
	"flowpilot/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite connection.
type DB struct {
	conn *sql.DB
}

// New creates a new database connection and initializes the schema.
func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	conn.SetMaxOpenConns(1) // SQLite single-writer
	conn.SetMaxIdleConns(1)

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		url TEXT NOT NULL,
		steps TEXT NOT NULL DEFAULT '[]',
		batch_id TEXT DEFAULT '',
		flow_id TEXT DEFAULT '',
		headless INTEGER DEFAULT 1,
		proxy_server TEXT DEFAULT '',
		proxy_username TEXT DEFAULT '',
		proxy_password TEXT DEFAULT '',
		proxy_geo TEXT DEFAULT '',
		proxy_protocol TEXT DEFAULT '',
		priority INTEGER DEFAULT 5,
		status TEXT DEFAULT 'pending',
		retry_count INTEGER DEFAULT 0,
		max_retries INTEGER DEFAULT 3,
		error TEXT DEFAULT '',
		result TEXT DEFAULT '',
		tags TEXT DEFAULT '[]',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		started_at DATETIME,
		completed_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS recorded_flows (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		steps TEXT NOT NULL DEFAULT '[]',
		origin_url TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS dom_snapshots (
		id TEXT PRIMARY KEY,
		flow_id TEXT NOT NULL,
		step_index INTEGER NOT NULL,
		html TEXT NOT NULL,
		screenshot_path TEXT NOT NULL,
		url TEXT NOT NULL,
		captured_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS batch_groups (
		id TEXT PRIMARY KEY,
		flow_id TEXT NOT NULL,
		name TEXT NOT NULL,
		total INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS task_events (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL,
		batch_id TEXT DEFAULT '',
		from_state TEXT NOT NULL,
		to_state TEXT NOT NULL,
		error TEXT DEFAULT '',
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS step_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT NOT NULL,
		step_index INTEGER NOT NULL,
		action TEXT NOT NULL,
		selector TEXT DEFAULT '',
		value TEXT DEFAULT '',
		snapshot_id TEXT DEFAULT '',
		error_code TEXT DEFAULT '',
		error_msg TEXT DEFAULT '',
		duration_ms INTEGER DEFAULT 0,
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS network_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT NOT NULL,
		step_index INTEGER NOT NULL,
		request_url TEXT NOT NULL,
		method TEXT NOT NULL,
		status_code INTEGER DEFAULT 0,
		mime_type TEXT DEFAULT '',
		request_headers TEXT DEFAULT '',
		response_headers TEXT DEFAULT '',
		request_size INTEGER DEFAULT 0,
		response_size INTEGER DEFAULT 0,
		duration_ms INTEGER DEFAULT 0,
		error TEXT DEFAULT '',
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS websocket_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		flow_id TEXT NOT NULL,
		step_index INTEGER NOT NULL,
		request_id TEXT DEFAULT '',
		url TEXT DEFAULT '',
		event_type TEXT NOT NULL,
		direction TEXT DEFAULT '',
		opcode INTEGER DEFAULT 0,
		payload_size INTEGER DEFAULT 0,
		payload_snippet TEXT DEFAULT '',
		close_code INTEGER DEFAULT 0,
		close_reason TEXT DEFAULT '',
		error_message TEXT DEFAULT '',
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS proxies (
		id TEXT PRIMARY KEY,
		server TEXT NOT NULL,
		protocol TEXT DEFAULT 'http',
		username TEXT DEFAULT '',
		password TEXT DEFAULT '',
		geo TEXT DEFAULT '',
		status TEXT DEFAULT 'unknown',
		latency INTEGER DEFAULT 0,
		success_rate REAL DEFAULT 0.0,
		total_used INTEGER DEFAULT 0,
		last_checked DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority DESC);
	CREATE INDEX IF NOT EXISTS idx_tasks_batch_id ON tasks(batch_id);
	CREATE INDEX IF NOT EXISTS idx_tasks_flow_id ON tasks(flow_id);
	CREATE INDEX IF NOT EXISTS idx_events_task_id ON task_events(task_id);
	CREATE INDEX IF NOT EXISTS idx_events_batch_id ON task_events(batch_id);
	CREATE INDEX IF NOT EXISTS idx_step_logs_task_id ON step_logs(task_id);
	CREATE INDEX IF NOT EXISTS idx_network_logs_task_id ON network_logs(task_id);
	CREATE INDEX IF NOT EXISTS idx_websocket_logs_flow_id ON websocket_logs(flow_id);
	CREATE INDEX IF NOT EXISTS idx_proxies_status ON proxies(status);
	CREATE INDEX IF NOT EXISTS idx_proxies_geo ON proxies(geo);
	`
	_, err := db.conn.Exec(schema)
	if err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}

	// Add proxy_protocol column for existing databases (idempotent).
	// "duplicate column name" error is expected for new databases where the column
	// already exists in the CREATE TABLE; all other errors should be reported.
	_, alterErr := db.conn.Exec(`ALTER TABLE tasks ADD COLUMN proxy_protocol TEXT DEFAULT ''`)
	if alterErr != nil {
		// SQLite returns "duplicate column name" if column already exists — that's OK.
		if !strings.Contains(alterErr.Error(), "duplicate column name") {
			return fmt.Errorf("add proxy_protocol column: %w", alterErr)
		}
	}

	alterColumns := []string{
		"ALTER TABLE tasks ADD COLUMN batch_id TEXT DEFAULT ''",
		"ALTER TABLE tasks ADD COLUMN flow_id TEXT DEFAULT ''",
		"ALTER TABLE tasks ADD COLUMN headless INTEGER DEFAULT 1",
	}
	for _, stmt := range alterColumns {
		if _, err := db.conn.Exec(stmt); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("alter tasks: %w", err)
			}
		}
	}

	return nil
}

// --- Task CRUD ---

// CreateTask inserts a new task.
func (db *DB) CreateTask(task models.Task) error {
	stepsJSON, err := json.Marshal(task.Steps)
	if err != nil {
		return fmt.Errorf("marshal steps: %w", err)
	}
	tagsJSON, err := json.Marshal(task.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	encUsername, err := crypto.Encrypt(task.Proxy.Username)
	if err != nil {
		return fmt.Errorf("encrypt proxy username: %w", err)
	}
	encPassword, err := crypto.Encrypt(task.Proxy.Password)
	if err != nil {
		return fmt.Errorf("encrypt proxy password: %w", err)
	}

	headless := 1
	if !task.Headless {
		headless = 0
	}

	_, err = db.conn.Exec(`
		INSERT INTO tasks (id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol, priority, status, max_retries, tags, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.Name, task.URL, string(stepsJSON), task.BatchID, task.FlowID, headless,
		task.Proxy.Server, encUsername, encPassword, task.Proxy.Geo, task.Proxy.Protocol,
		task.Priority, task.Status, task.MaxRetries, string(tagsJSON), task.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert task %s: %w", task.ID, err)
	}
	return nil
}

// GetTask retrieves a task by ID.
func (db *DB) GetTask(id string) (*models.Task, error) {
	row := db.conn.QueryRow(`SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, error, result, tags, created_at, started_at, completed_at
		FROM tasks WHERE id = ?`, id)
	task, err := db.scanTask(row)
	if err != nil {
		return nil, fmt.Errorf("get task %s: %w", id, err)
	}
	return task, nil
}

// ListTasks returns all tasks, ordered by priority desc, created_at desc.
func (db *DB) ListTasks() ([]models.Task, error) {
	rows, err := db.conn.Query(`SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, error, result, tags, created_at, started_at, completed_at
		FROM tasks ORDER BY priority DESC, created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		task, err := db.scanTaskRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task row: %w", err)
		}
		tasks = append(tasks, *task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}
	return tasks, nil
}

// ListTasksByStatus returns tasks with a given status.
func (db *DB) ListTasksByStatus(status models.TaskStatus) ([]models.Task, error) {
	rows, err := db.conn.Query(`SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, error, result, tags, created_at, started_at, completed_at
		FROM tasks WHERE status = ? ORDER BY priority DESC, created_at DESC`, status)
	if err != nil {
		return nil, fmt.Errorf("query tasks by status %s: %w", status, err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		task, err := db.scanTaskRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task row: %w", err)
		}
		tasks = append(tasks, *task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}
	return tasks, nil
}

// UpdateTaskStatus updates the status of a task.
func (db *DB) UpdateTaskStatus(id string, status models.TaskStatus, errMsg string) error {
	now := time.Now()
	var res sql.Result
	var err error
	switch status {
	case models.TaskStatusRunning:
		res, err = db.conn.Exec(`UPDATE tasks SET status = ?, started_at = ? WHERE id = ?`, status, now, id)
	case models.TaskStatusCompleted, models.TaskStatusFailed:
		res, err = db.conn.Exec(`UPDATE tasks SET status = ?, error = ?, completed_at = ? WHERE id = ?`, status, errMsg, now, id)
	default:
		res, err = db.conn.Exec(`UPDATE tasks SET status = ?, error = ? WHERE id = ?`, status, errMsg, id)
	}
	if err != nil {
		return fmt.Errorf("update task %s status to %s: %w", id, status, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update result for task %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("task %s not found", id)
	}
	return nil
}

// UpdateTaskResult stores the result JSON for a task.
func (db *DB) UpdateTaskResult(id string, result models.TaskResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	res, err := db.conn.Exec(`UPDATE tasks SET result = ? WHERE id = ?`, string(resultJSON), id)
	if err != nil {
		return fmt.Errorf("update task %s result: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update result for task %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("task %s not found", id)
	}
	return nil
}

// IncrementRetry increases retry_count for a task.
func (db *DB) IncrementRetry(id string) error {
	res, err := db.conn.Exec(`UPDATE tasks SET retry_count = retry_count + 1, status = 'retrying' WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("increment retry for task %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check retry result for task %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("task %s not found", id)
	}
	return nil
}

// ResetRetryCount resets a task's retry_count to zero.
func (db *DB) ResetRetryCount(id string) error {
	res, err := db.conn.Exec(`UPDATE tasks SET retry_count = 0 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("reset retry count for task %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check reset retry result for task %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("task %s not found", id)
	}
	return nil
}

// UpdateTask updates editable fields of a task. Only allowed for pending/failed tasks.
// Uses an atomic UPDATE with a status guard to prevent TOCTOU races.
func (db *DB) UpdateTask(id, name, url string, steps []models.TaskStep, proxyConfig models.ProxyConfig, priority models.TaskPriority, tags []string) error {
	stepsJSON, err := json.Marshal(steps)
	if err != nil {
		return fmt.Errorf("marshal steps: %w", err)
	}
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	encUsername, err := crypto.Encrypt(proxyConfig.Username)
	if err != nil {
		return fmt.Errorf("encrypt proxy username: %w", err)
	}
	encPassword, err := crypto.Encrypt(proxyConfig.Password)
	if err != nil {
		return fmt.Errorf("encrypt proxy password: %w", err)
	}

	res, err := db.conn.Exec(`UPDATE tasks SET name = ?, url = ?, steps = ?, proxy_server = ?, proxy_username = ?, proxy_password = ?, proxy_geo = ?, proxy_protocol = ?, priority = ?, tags = ? WHERE id = ? AND status IN (?, ?)`,
		name, url, string(stepsJSON), proxyConfig.Server, encUsername, encPassword, proxyConfig.Geo, proxyConfig.Protocol, priority, string(tagsJSON), id,
		models.TaskStatusPending, models.TaskStatusFailed)
	if err != nil {
		return fmt.Errorf("update task %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update result for task %s: %w", id, err)
	}
	if n == 0 {
		// Distinguish between "not found" and "wrong status"
		task, getErr := db.GetTask(id)
		if getErr != nil {
			return fmt.Errorf("task %s not found", id)
		}
		return fmt.Errorf("cannot edit task with status %s", task.Status)
	}
	return nil
}

// DeleteTask removes a task by ID.
func (db *DB) DeleteTask(id string) error {
	res, err := db.conn.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete task %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete result for task %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("task %s not found", id)
	}
	return nil
}

// --- Proxy CRUD ---

// CreateProxy inserts a new proxy.
func (db *DB) CreateProxy(proxy models.Proxy) error {
	encUsername, err := crypto.Encrypt(proxy.Username)
	if err != nil {
		return fmt.Errorf("encrypt proxy username: %w", err)
	}
	encPassword, err := crypto.Encrypt(proxy.Password)
	if err != nil {
		return fmt.Errorf("encrypt proxy password: %w", err)
	}

	_, err = db.conn.Exec(`
		INSERT INTO proxies (id, server, protocol, username, password, geo, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		proxy.ID, proxy.Server, proxy.Protocol, encUsername, encPassword,
		proxy.Geo, proxy.Status, proxy.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert proxy %s: %w", proxy.ID, err)
	}
	return nil
}

// ListProxies returns all proxies.
func (db *DB) ListProxies() ([]models.Proxy, error) {
	rows, err := db.conn.Query(`SELECT id, server, protocol, username, password, geo, status, latency, success_rate, total_used, last_checked, created_at
		FROM proxies ORDER BY success_rate DESC, latency ASC`)
	if err != nil {
		return nil, fmt.Errorf("query proxies: %w", err)
	}
	defer rows.Close()

	var proxies []models.Proxy
	for rows.Next() {
		p, err := db.scanProxyRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan proxy row: %w", err)
		}
		proxies = append(proxies, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate proxies: %w", err)
	}
	return proxies, nil
}

// ListHealthyProxies returns proxies with healthy status.
func (db *DB) ListHealthyProxies() ([]models.Proxy, error) {
	rows, err := db.conn.Query(`SELECT id, server, protocol, username, password, geo, status, latency, success_rate, total_used, last_checked, created_at
		FROM proxies WHERE status = 'healthy' ORDER BY success_rate DESC, latency ASC`)
	if err != nil {
		return nil, fmt.Errorf("query healthy proxies: %w", err)
	}
	defer rows.Close()

	var proxies []models.Proxy
	for rows.Next() {
		p, err := db.scanProxyRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan proxy row: %w", err)
		}
		proxies = append(proxies, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate proxies: %w", err)
	}
	return proxies, nil
}

// UpdateProxyHealth updates proxy health check data.
func (db *DB) UpdateProxyHealth(id string, status models.ProxyStatus, latency int) error {
	now := time.Now()
	res, err := db.conn.Exec(`UPDATE proxies SET status = ?, latency = ?, last_checked = ? WHERE id = ?`,
		status, latency, now, id)
	if err != nil {
		return fmt.Errorf("update proxy %s health: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check health update for proxy %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("proxy %s not found", id)
	}
	return nil
}

// IncrementProxyUsage increments usage counter and updates success rate.
func (db *DB) IncrementProxyUsage(id string, success bool) error {
	var err error
	if success {
		_, err = db.conn.Exec(`UPDATE proxies SET total_used = total_used + 1,
			success_rate = (success_rate * total_used + 1.0) / (total_used + 1) WHERE id = ?`, id)
	} else {
		_, err = db.conn.Exec(`UPDATE proxies SET total_used = total_used + 1,
			success_rate = (success_rate * total_used) / (total_used + 1) WHERE id = ?`, id)
	}
	if err != nil {
		return fmt.Errorf("increment proxy %s usage: %w", id, err)
	}
	return nil
}

// DeleteProxy removes a proxy by ID.
func (db *DB) DeleteProxy(id string) error {
	res, err := db.conn.Exec(`DELETE FROM proxies WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete proxy %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete result for proxy %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("proxy %s not found", id)
	}
	return nil
}

// --- Scan helpers ---

type scanner interface {
	Scan(dest ...any) error
}

func (db *DB) scanTask(row scanner) (*models.Task, error) {
	var t models.Task
	var stepsJSON, resultJSON, tagsJSON string
	var startedAt, completedAt sql.NullTime
	var headlessInt int

	err := row.Scan(
		&t.ID, &t.Name, &t.URL, &stepsJSON, &t.BatchID, &t.FlowID, &headlessInt,
		&t.Proxy.Server, &t.Proxy.Username, &t.Proxy.Password, &t.Proxy.Geo, &t.Proxy.Protocol,
		&t.Priority, &t.Status, &t.RetryCount, &t.MaxRetries, &t.Error,
		&resultJSON, &tagsJSON, &t.CreatedAt, &startedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}

	if startedAt.Valid {
		t.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Time
	}

	t.Headless = headlessInt != 0

	if stepsJSON != "" {
		if err := json.Unmarshal([]byte(stepsJSON), &t.Steps); err != nil {
			return nil, fmt.Errorf("parse steps JSON: %w", err)
		}
	}
	if resultJSON != "" {
		var result models.TaskResult
		if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
			return nil, fmt.Errorf("parse result JSON: %w", err)
		}
		t.Result = &result
	}
	if tagsJSON != "" {
		if err := json.Unmarshal([]byte(tagsJSON), &t.Tags); err != nil {
			return nil, fmt.Errorf("parse tags JSON: %w", err)
		}
	}

	if decUser, err := crypto.Decrypt(t.Proxy.Username); err == nil {
		t.Proxy.Username = decUser
	}
	if decPass, err := crypto.Decrypt(t.Proxy.Password); err == nil {
		t.Proxy.Password = decPass
	}

	return &t, nil
}

func (db *DB) scanTaskRow(rows *sql.Rows) (*models.Task, error) {
	return db.scanTask(rows)
}

func (db *DB) scanProxyRow(rows *sql.Rows) (*models.Proxy, error) {
	var p models.Proxy
	var lastChecked sql.NullTime

	err := rows.Scan(
		&p.ID, &p.Server, &p.Protocol, &p.Username, &p.Password,
		&p.Geo, &p.Status, &p.Latency, &p.SuccessRate, &p.TotalUsed,
		&lastChecked, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if lastChecked.Valid {
		p.LastChecked = &lastChecked.Time
	}

	if decUser, err := crypto.Decrypt(p.Username); err == nil {
		p.Username = decUser
	}
	if decPass, err := crypto.Decrypt(p.Password); err == nil {
		p.Password = decPass
	}

	return &p, nil
}

// GetTaskStats returns task counts by status.
func (db *DB) GetTaskStats() (map[string]int, error) {
	rows, err := db.conn.Query(`SELECT status, COUNT(*) FROM tasks GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("query task stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan task stats: %w", err)
		}
		stats[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task stats: %w", err)
	}
	return stats, nil
}

// --- Recorded Flows ---

// CreateRecordedFlow inserts a new recorded flow.
func (db *DB) CreateRecordedFlow(flow models.RecordedFlow) error {
	stepsJSON, err := json.Marshal(flow.Steps)
	if err != nil {
		return fmt.Errorf("marshal flow steps: %w", err)
	}
	_, err = db.conn.Exec(`INSERT INTO recorded_flows (id, name, description, steps, origin_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		flow.ID, flow.Name, flow.Description, string(stepsJSON), flow.OriginURL, flow.CreatedAt, flow.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert recorded flow %s: %w", flow.ID, err)
	}
	return nil
}

// UpdateRecordedFlow updates an existing recorded flow.
func (db *DB) UpdateRecordedFlow(flow models.RecordedFlow) error {
	stepsJSON, err := json.Marshal(flow.Steps)
	if err != nil {
		return fmt.Errorf("marshal flow steps: %w", err)
	}
	res, err := db.conn.Exec(`UPDATE recorded_flows SET name = ?, description = ?, steps = ?, origin_url = ?, updated_at = ? WHERE id = ?`,
		flow.Name, flow.Description, string(stepsJSON), flow.OriginURL, flow.UpdatedAt, flow.ID)
	if err != nil {
		return fmt.Errorf("update recorded flow %s: %w", flow.ID, err)
	}
	if rows, err := res.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return fmt.Errorf("check update flow %s: %w", flow.ID, err)
		}
		return fmt.Errorf("flow %s not found", flow.ID)
	}
	return nil
}

// GetRecordedFlow returns a flow by ID.
func (db *DB) GetRecordedFlow(id string) (*models.RecordedFlow, error) {
	row := db.conn.QueryRow(`SELECT id, name, description, steps, origin_url, created_at, updated_at FROM recorded_flows WHERE id = ?`, id)
	var flow models.RecordedFlow
	var stepsJSON string
	if err := row.Scan(&flow.ID, &flow.Name, &flow.Description, &stepsJSON, &flow.OriginURL, &flow.CreatedAt, &flow.UpdatedAt); err != nil {
		return nil, fmt.Errorf("get recorded flow %s: %w", id, err)
	}
	if stepsJSON != "" {
		if err := json.Unmarshal([]byte(stepsJSON), &flow.Steps); err != nil {
			return nil, fmt.Errorf("parse flow steps: %w", err)
		}
	}
	return &flow, nil
}

// ListRecordedFlows returns all recorded flows ordered by updated_at desc.
func (db *DB) ListRecordedFlows() ([]models.RecordedFlow, error) {
	rows, err := db.conn.Query(`SELECT id, name, description, steps, origin_url, created_at, updated_at FROM recorded_flows ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list recorded flows: %w", err)
	}
	defer rows.Close()

	flows := []models.RecordedFlow{}
	for rows.Next() {
		var flow models.RecordedFlow
		var stepsJSON string
		if err := rows.Scan(&flow.ID, &flow.Name, &flow.Description, &stepsJSON, &flow.OriginURL, &flow.CreatedAt, &flow.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan recorded flow: %w", err)
		}
		if stepsJSON != "" {
			if err := json.Unmarshal([]byte(stepsJSON), &flow.Steps); err != nil {
				return nil, fmt.Errorf("parse flow steps: %w", err)
			}
		}
		flows = append(flows, flow)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recorded flows: %w", err)
	}
	return flows, nil
}

// DeleteRecordedFlow removes a recorded flow and its associated data.
func (db *DB) DeleteRecordedFlow(id string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin delete flow tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM dom_snapshots WHERE flow_id = ?`, id); err != nil {
		return fmt.Errorf("delete dom snapshots for flow %s: %w", id, err)
	}
	if _, err := tx.Exec(`DELETE FROM websocket_logs WHERE flow_id = ?`, id); err != nil {
		return fmt.Errorf("delete websocket logs for flow %s: %w", id, err)
	}

	res, err := tx.Exec(`DELETE FROM recorded_flows WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete recorded flow %s: %w", id, err)
	}
	if rows, err := res.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return fmt.Errorf("check delete flow %s: %w", id, err)
		}
		return fmt.Errorf("flow %s not found", id)
	}
	return tx.Commit()
}

// --- DOM Snapshots ---

// CreateDOMSnapshot inserts a DOM snapshot record.
func (db *DB) CreateDOMSnapshot(snapshot models.DOMSnapshot) error {
	_, err := db.conn.Exec(`INSERT INTO dom_snapshots (id, flow_id, step_index, html, screenshot_path, url, captured_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		snapshot.ID, snapshot.FlowID, snapshot.StepIndex, snapshot.HTML, snapshot.ScreenshotPath, snapshot.URL, snapshot.CapturedAt)
	if err != nil {
		return fmt.Errorf("insert dom snapshot %s: %w", snapshot.ID, err)
	}
	return nil
}

// ListDOMSnapshots returns snapshots for a flow ordered by step_index.
func (db *DB) ListDOMSnapshots(flowID string) ([]models.DOMSnapshot, error) {
	rows, err := db.conn.Query(`SELECT id, flow_id, step_index, html, screenshot_path, url, captured_at FROM dom_snapshots WHERE flow_id = ? ORDER BY step_index ASC`, flowID)
	if err != nil {
		return nil, fmt.Errorf("list dom snapshots: %w", err)
	}
	defer rows.Close()

	snapshots := []models.DOMSnapshot{}
	for rows.Next() {
		var s models.DOMSnapshot
		if err := rows.Scan(&s.ID, &s.FlowID, &s.StepIndex, &s.HTML, &s.ScreenshotPath, &s.URL, &s.CapturedAt); err != nil {
			return nil, fmt.Errorf("scan dom snapshot: %w", err)
		}
		snapshots = append(snapshots, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dom snapshots: %w", err)
	}
	return snapshots, nil
}

// --- Batch Groups ---

// CreateBatchGroup inserts a batch group record.
func (db *DB) CreateBatchGroup(group models.BatchGroup) error {
	_, err := db.conn.Exec(`INSERT INTO batch_groups (id, flow_id, name, total, created_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		group.ID, group.FlowID, group.Name, group.Total)
	if err != nil {
		return fmt.Errorf("insert batch group %s: %w", group.ID, err)
	}
	return nil
}

// GetBatchProgress returns aggregate status counts for a batch.
func (db *DB) GetBatchProgress(batchID string) (models.BatchProgress, error) {
	progress := models.BatchProgress{BatchID: batchID}
	rows, err := db.conn.Query(`SELECT status, COUNT(*) FROM tasks WHERE batch_id = ? GROUP BY status`, batchID)
	if err != nil {
		return progress, fmt.Errorf("query batch progress: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return progress, fmt.Errorf("scan batch progress: %w", err)
		}
		progress.Total += count
		switch models.TaskStatus(status) {
		case models.TaskStatusPending:
			progress.Pending = count
		case models.TaskStatusQueued:
			progress.Queued = count
		case models.TaskStatusRunning:
			progress.Running = count
		case models.TaskStatusCompleted:
			progress.Completed = count
		case models.TaskStatusFailed:
			progress.Failed = count
		case models.TaskStatusCancelled:
			progress.Cancelled = count
		}
	}
	if err := rows.Err(); err != nil {
		return progress, fmt.Errorf("iterate batch progress: %w", err)
	}
	return progress, nil
}

// ListTasksByBatch returns tasks for a batch ID.
func (db *DB) ListTasksByBatch(batchID string) ([]models.Task, error) {
	rows, err := db.conn.Query(`SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, error, result, tags, created_at, started_at, completed_at
		FROM tasks WHERE batch_id = ? ORDER BY created_at ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("query tasks by batch %s: %w", batchID, err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		task, err := db.scanTaskRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan batch task row: %w", err)
		}
		tasks = append(tasks, *task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate batch tasks: %w", err)
	}
	return tasks, nil
}

// ListTasksByBatchStatus returns tasks in a batch with a specific status.
func (db *DB) ListTasksByBatchStatus(batchID string, status models.TaskStatus) ([]models.Task, error) {
	rows, err := db.conn.Query(`SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, error, result, tags, created_at, started_at, completed_at
		FROM tasks WHERE batch_id = ? AND status = ? ORDER BY created_at ASC`, batchID, status)
	if err != nil {
		return nil, fmt.Errorf("query tasks by batch status %s: %w", batchID, err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		task, err := db.scanTaskRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan batch task row: %w", err)
		}
		tasks = append(tasks, *task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate batch tasks: %w", err)
	}
	return tasks, nil
}

// --- Task Events ---

// InsertTaskEvent records a task lifecycle event.
func (db *DB) InsertTaskEvent(event models.TaskLifecycleEvent) error {
	_, err := db.conn.Exec(`INSERT INTO task_events (id, task_id, batch_id, from_state, to_state, error, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.TaskID, event.BatchID, event.FromState, event.ToState, event.Error, event.Timestamp)
	if err != nil {
		return fmt.Errorf("insert task event %s: %w", event.ID, err)
	}
	return nil
}

// ListTaskEvents returns lifecycle events for a task.
func (db *DB) ListTaskEvents(taskID string) ([]models.TaskLifecycleEvent, error) {
	rows, err := db.conn.Query(`SELECT id, task_id, batch_id, from_state, to_state, error, timestamp FROM task_events WHERE task_id = ? ORDER BY timestamp ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task events: %w", err)
	}
	defer rows.Close()

	events := []models.TaskLifecycleEvent{}
	for rows.Next() {
		var ev models.TaskLifecycleEvent
		if err := rows.Scan(&ev.ID, &ev.TaskID, &ev.BatchID, &ev.FromState, &ev.ToState, &ev.Error, &ev.Timestamp); err != nil {
			return nil, fmt.Errorf("scan task event: %w", err)
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task events: %w", err)
	}
	return events, nil
}

// --- Step Logs ---

// InsertStepLogs inserts step logs for a task.
func (db *DB) InsertStepLogs(taskID string, logs []models.StepLog) error {
	if len(logs) == 0 {
		return nil
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin step logs tx: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO step_logs (task_id, step_index, action, selector, value, snapshot_id, error_code, error_msg, duration_ms, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare step log insert: %w", err)
	}
	defer stmt.Close()

	for _, log := range logs {
		if _, err := stmt.Exec(taskID, log.StepIndex, log.Action, log.Selector, log.Value, log.SnapshotID, log.ErrorCode, log.ErrorMsg, log.DurationMs, log.StartedAt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert step log: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit step logs: %w", err)
	}
	return nil
}

// ListStepLogs returns step logs for a task.
func (db *DB) ListStepLogs(taskID string) ([]models.StepLog, error) {
	rows, err := db.conn.Query(`SELECT task_id, step_index, action, selector, value, snapshot_id, error_code, error_msg, duration_ms, started_at
		FROM step_logs WHERE task_id = ? ORDER BY step_index ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list step logs: %w", err)
	}
	defer rows.Close()

	logs := []models.StepLog{}
	for rows.Next() {
		var log models.StepLog
		if err := rows.Scan(&log.TaskID, &log.StepIndex, &log.Action, &log.Selector, &log.Value, &log.SnapshotID, &log.ErrorCode, &log.ErrorMsg, &log.DurationMs, &log.StartedAt); err != nil {
			return nil, fmt.Errorf("scan step log: %w", err)
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate step logs: %w", err)
	}
	return logs, nil
}

// --- Network Logs ---

// InsertNetworkLogs inserts network logs for a task.
func (db *DB) InsertNetworkLogs(taskID string, logs []models.NetworkLog) error {
	if len(logs) == 0 {
		return nil
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin network logs tx: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO network_logs (task_id, step_index, request_url, method, status_code, mime_type, request_headers, response_headers, request_size, response_size, duration_ms, error, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare network log insert: %w", err)
	}
	defer stmt.Close()

	for _, log := range logs {
		if _, err := stmt.Exec(taskID, log.StepIndex, log.RequestURL, log.Method, log.StatusCode, log.MimeType, log.RequestHeaders, log.ResponseHeaders, log.RequestSize, log.ResponseSize, log.DurationMs, log.Error, log.Timestamp); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert network log: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit network logs: %w", err)
	}
	return nil
}

// ListNetworkLogs returns network logs for a task.
func (db *DB) ListNetworkLogs(taskID string) ([]models.NetworkLog, error) {
	rows, err := db.conn.Query(`SELECT task_id, step_index, request_url, method, status_code, mime_type, request_headers, response_headers, request_size, response_size, duration_ms, error, timestamp
		FROM network_logs WHERE task_id = ? ORDER BY timestamp ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list network logs: %w", err)
	}
	defer rows.Close()

	logs := []models.NetworkLog{}
	for rows.Next() {
		var log models.NetworkLog
		if err := rows.Scan(&log.TaskID, &log.StepIndex, &log.RequestURL, &log.Method, &log.StatusCode, &log.MimeType, &log.RequestHeaders, &log.ResponseHeaders, &log.RequestSize, &log.ResponseSize, &log.DurationMs, &log.Error, &log.Timestamp); err != nil {
			return nil, fmt.Errorf("scan network log: %w", err)
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate network logs: %w", err)
	}
	return logs, nil
}

// --- WebSocket Logs ---

// InsertWebSocketLogs inserts WebSocket logs for a recorded flow.
func (db *DB) InsertWebSocketLogs(flowID string, logs []models.WebSocketLog) error {
	if len(logs) == 0 {
		return nil
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin websocket logs tx: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO websocket_logs (flow_id, step_index, request_id, url, event_type, direction, opcode, payload_size, payload_snippet, close_code, close_reason, error_message, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare websocket log insert: %w", err)
	}
	defer stmt.Close()

	for _, log := range logs {
		if _, err := stmt.Exec(flowID, log.StepIndex, log.RequestID, log.URL, log.EventType, log.Direction, log.Opcode, log.PayloadSize, log.PayloadSnippet, log.CloseCode, log.CloseReason, log.ErrorMessage, log.Timestamp); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert websocket log: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit websocket logs: %w", err)
	}
	return nil
}

// ListWebSocketLogs returns WebSocket logs for a recorded flow.
func (db *DB) ListWebSocketLogs(flowID string) ([]models.WebSocketLog, error) {
	rows, err := db.conn.Query(`SELECT flow_id, step_index, request_id, url, event_type, direction, opcode, payload_size, payload_snippet, close_code, close_reason, error_message, timestamp
		FROM websocket_logs WHERE flow_id = ? ORDER BY timestamp ASC`, flowID)
	if err != nil {
		return nil, fmt.Errorf("list websocket logs: %w", err)
	}
	defer rows.Close()

	logs := []models.WebSocketLog{}
	for rows.Next() {
		var log models.WebSocketLog
		if err := rows.Scan(&log.FlowID, &log.StepIndex, &log.RequestID, &log.URL, &log.EventType, &log.Direction, &log.Opcode, &log.PayloadSize, &log.PayloadSnippet, &log.CloseCode, &log.CloseReason, &log.ErrorMessage, &log.Timestamp); err != nil {
			return nil, fmt.Errorf("scan websocket log: %w", err)
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate websocket logs: %w", err)
	}
	return logs, nil
}

// --- Paginated Queries ---

// ListTasksPaginated returns a page of tasks.
func (db *DB) ListTasksPaginated(page, pageSize int, status string, tag string) (models.PaginatedTasks, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	where := "WHERE 1=1"
	args := []any{}
	if status != "" && status != "all" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if tag != "" {
		escapedTag := strings.NewReplacer("%", "\\%", "_", "\\_").Replace(tag)
		where += " AND tags LIKE ? ESCAPE '\\'"
		args = append(args, "%\""+escapedTag+"\"%")
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM tasks " + where
	if err := db.conn.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return models.PaginatedTasks{}, fmt.Errorf("count tasks: %w", err)
	}

	totalPages := (total + pageSize - 1) / pageSize
	offset := (page - 1) * pageSize

	query := `SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, error, result, tags, created_at, started_at, completed_at
		FROM tasks ` + where + ` ORDER BY priority DESC, created_at DESC LIMIT ? OFFSET ?`
	queryArgs := make([]any, 0, len(args)+2)
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, pageSize, offset)

	rows, err := db.conn.Query(query, queryArgs...)
	if err != nil {
		return models.PaginatedTasks{}, fmt.Errorf("query paginated tasks: %w", err)
	}
	defer rows.Close()

	tasks := []models.Task{}
	for rows.Next() {
		task, err := db.scanTaskRow(rows)
		if err != nil {
			return models.PaginatedTasks{}, fmt.Errorf("scan task row: %w", err)
		}
		tasks = append(tasks, *task)
	}
	if err := rows.Err(); err != nil {
		return models.PaginatedTasks{}, fmt.Errorf("iterate paginated tasks: %w", err)
	}

	return models.PaginatedTasks{
		Tasks:      tasks,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// --- Data Retention ---

// PurgeOldRecords deletes completed/failed tasks and associated logs older than the given duration.
func (db *DB) PurgeOldRecords(retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays).Format("2006-01-02 15:04:05")

	tx, err := db.conn.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin purge tx: %w", err)
	}

	subquery := `SELECT id FROM tasks WHERE completed_at < ? AND status IN ('completed', 'failed', 'cancelled')`
	var total int64

	purgeQueries := []struct {
		label string
		query string
	}{
		{"step logs", `DELETE FROM step_logs WHERE task_id IN (` + subquery + `)`},
		{"network logs", `DELETE FROM network_logs WHERE task_id IN (` + subquery + `)`},
		{"task events", `DELETE FROM task_events WHERE task_id IN (` + subquery + `)`},
		{"tasks", `DELETE FROM tasks WHERE completed_at < ? AND status IN ('completed', 'failed', 'cancelled')`},
	}

	for _, pq := range purgeQueries {
		res, err := tx.Exec(pq.query, cutoff)
		if err != nil {
			_ = tx.Rollback()
			return total, fmt.Errorf("purge %s: %w", pq.label, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			total += n
		}
	}

	if err := tx.Commit(); err != nil {
		return total, fmt.Errorf("commit purge tx: %w", err)
	}
	return total, nil
}

// ListAuditTrail returns task lifecycle events, optionally filtered by task ID.
func (db *DB) ListAuditTrail(taskID string, limit int) ([]models.TaskLifecycleEvent, error) {
	query := `SELECT id, task_id, batch_id, from_state, to_state, error, timestamp FROM task_events`
	args := []any{}
	if taskID != "" {
		query += ` WHERE task_id = ?`
		args = append(args, taskID)
	}
	query += ` ORDER BY timestamp DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit trail: %w", err)
	}
	defer rows.Close()

	events := []models.TaskLifecycleEvent{}
	for rows.Next() {
		var ev models.TaskLifecycleEvent
		if err := rows.Scan(&ev.ID, &ev.TaskID, &ev.BatchID, &ev.FromState, &ev.ToState, &ev.Error, &ev.Timestamp); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit trail: %w", err)
	}
	return events, nil
}
