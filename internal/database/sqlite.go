package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"web-automation/internal/crypto"
	"web-automation/internal/models"

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
		proxy_server TEXT DEFAULT '',
		proxy_username TEXT DEFAULT '',
		proxy_password TEXT DEFAULT '',
		proxy_geo TEXT DEFAULT '',
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
	CREATE INDEX IF NOT EXISTS idx_proxies_status ON proxies(status);
	CREATE INDEX IF NOT EXISTS idx_proxies_geo ON proxies(geo);
	`
	_, err := db.conn.Exec(schema)
	if err != nil {
		return fmt.Errorf("exec schema: %w", err)
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

	_, err = db.conn.Exec(`
		INSERT INTO tasks (id, name, url, steps, proxy_server, proxy_username, proxy_password, proxy_geo, priority, status, max_retries, tags, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.Name, task.URL, string(stepsJSON),
		task.Proxy.Server, encUsername, encPassword, task.Proxy.Geo,
		task.Priority, task.Status, task.MaxRetries, string(tagsJSON), task.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert task %s: %w", task.ID, err)
	}
	return nil
}

// GetTask retrieves a task by ID.
func (db *DB) GetTask(id string) (*models.Task, error) {
	row := db.conn.QueryRow(`SELECT id, name, url, steps, proxy_server, proxy_username, proxy_password, proxy_geo,
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
	rows, err := db.conn.Query(`SELECT id, name, url, steps, proxy_server, proxy_username, proxy_password, proxy_geo,
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
	rows, err := db.conn.Query(`SELECT id, name, url, steps, proxy_server, proxy_username, proxy_password, proxy_geo,
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
	var err error
	switch status {
	case models.TaskStatusRunning:
		_, err = db.conn.Exec(`UPDATE tasks SET status = ?, started_at = ? WHERE id = ?`, status, now, id)
	case models.TaskStatusCompleted, models.TaskStatusFailed:
		_, err = db.conn.Exec(`UPDATE tasks SET status = ?, error = ?, completed_at = ? WHERE id = ?`, status, errMsg, now, id)
	default:
		_, err = db.conn.Exec(`UPDATE tasks SET status = ?, error = ? WHERE id = ?`, status, errMsg, id)
	}
	if err != nil {
		return fmt.Errorf("update task %s status to %s: %w", id, status, err)
	}
	return nil
}

// UpdateTaskResult stores the result JSON for a task.
func (db *DB) UpdateTaskResult(id string, result models.TaskResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	_, err = db.conn.Exec(`UPDATE tasks SET result = ? WHERE id = ?`, string(resultJSON), id)
	if err != nil {
		return fmt.Errorf("update task %s result: %w", id, err)
	}
	return nil
}

// IncrementRetry increases retry_count for a task.
func (db *DB) IncrementRetry(id string) error {
	_, err := db.conn.Exec(`UPDATE tasks SET retry_count = retry_count + 1, status = 'retrying' WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("increment retry for task %s: %w", id, err)
	}
	return nil
}

// UpdateTask updates editable fields of a task. Only allowed for pending/failed tasks.
func (db *DB) UpdateTask(id, name, url string, steps []models.TaskStep, proxyConfig models.ProxyConfig, priority models.TaskPriority) error {
	task, err := db.GetTask(id)
	if err != nil {
		return fmt.Errorf("get task for update: %w", err)
	}
	if task.Status != models.TaskStatusPending && task.Status != models.TaskStatusFailed {
		return fmt.Errorf("cannot edit task with status %s", task.Status)
	}

	stepsJSON, err := json.Marshal(steps)
	if err != nil {
		return fmt.Errorf("marshal steps: %w", err)
	}

	encUsername, err := crypto.Encrypt(proxyConfig.Username)
	if err != nil {
		return fmt.Errorf("encrypt proxy username: %w", err)
	}
	encPassword, err := crypto.Encrypt(proxyConfig.Password)
	if err != nil {
		return fmt.Errorf("encrypt proxy password: %w", err)
	}

	_, err = db.conn.Exec(`UPDATE tasks SET name = ?, url = ?, steps = ?, proxy_server = ?, proxy_username = ?, proxy_password = ?, proxy_geo = ?, priority = ? WHERE id = ?`,
		name, url, string(stepsJSON), proxyConfig.Server, encUsername, encPassword, proxyConfig.Geo, priority, id)
	if err != nil {
		return fmt.Errorf("update task %s: %w", id, err)
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
	_, err := db.conn.Exec(`UPDATE proxies SET status = ?, latency = ?, last_checked = ? WHERE id = ?`,
		status, latency, now, id)
	if err != nil {
		return fmt.Errorf("update proxy %s health: %w", id, err)
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
	Scan(dest ...interface{}) error
}

func (db *DB) scanTask(row scanner) (*models.Task, error) {
	var t models.Task
	var stepsJSON, resultJSON, tagsJSON string
	var startedAt, completedAt sql.NullTime

	err := row.Scan(
		&t.ID, &t.Name, &t.URL, &stepsJSON,
		&t.Proxy.Server, &t.Proxy.Username, &t.Proxy.Password, &t.Proxy.Geo,
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
