package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite connections.
type DB struct {
	conn     *sql.DB // write-only connection
	readConn *sql.DB // read-only connection for concurrent queries
}

// Reader returns the read-only connection for concurrent queries.
func (db *DB) Reader() *sql.DB { return db.readConn }

// New creates a new database connection and initializes the schema.
func New(dbPath string) (*DB, error) {
	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=30000&_txlock=immediate"
	conn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	conn.SetMaxOpenConns(1) // SQLite single-writer
	conn.SetMaxIdleConns(1)

	readConn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&mode=ro")
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open read database: %w", err)
	}
	readConn.SetMaxOpenConns(4)
	readConn.SetMaxIdleConns(4)

	db := &DB{conn: conn, readConn: readConn}

	// Performance PRAGMAs (apply to both connections)
	for _, c := range []*sql.DB{conn, readConn} {
		c.Exec("PRAGMA synchronous=NORMAL")
		c.Exec("PRAGMA cache_size=-64000")
		c.Exec("PRAGMA mmap_size=268435456")
		c.Exec("PRAGMA temp_store=MEMORY")
	}

	if err := db.migrate(); err != nil {
		conn.Close()
		readConn.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}

// Close closes both database connections.
func (db *DB) Close() error {
	readErr := db.readConn.Close()
	writeErr := db.conn.Close()
	if writeErr != nil {
		return writeErr
	}
	return readErr
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
		timeout_seconds INTEGER DEFAULT 0,
		error TEXT DEFAULT '',
		result TEXT DEFAULT '',
		tags TEXT DEFAULT '[]',
		logging_policy TEXT DEFAULT '',
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

	CREATE TABLE IF NOT EXISTS proxy_routing_presets (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		random_by_country INTEGER DEFAULT 0,
		country TEXT DEFAULT '',
		fallback TEXT DEFAULT 'strict',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS schedules (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		cron_expr TEXT NOT NULL,
		flow_id TEXT NOT NULL,
		url TEXT NOT NULL,
		proxy_server TEXT DEFAULT '',
		proxy_username TEXT DEFAULT '',
		proxy_password TEXT DEFAULT '',
		proxy_geo TEXT DEFAULT '',
		proxy_protocol TEXT DEFAULT '',
		priority INTEGER DEFAULT 5,
		headless INTEGER DEFAULT 1,
		tags TEXT DEFAULT '[]',
		enabled INTEGER DEFAULT 1,
		last_run_at DATETIME,
		next_run_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
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
	CREATE TABLE IF NOT EXISTS captcha_config (
		id TEXT PRIMARY KEY,
		provider TEXT NOT NULL,
		api_key TEXT NOT NULL,
		enabled INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_schedules_enabled ON schedules(enabled);
	CREATE INDEX IF NOT EXISTS idx_schedules_next_run ON schedules(next_run_at);

	CREATE TABLE IF NOT EXISTS visual_baselines (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		task_id TEXT DEFAULT '',
		url TEXT NOT NULL,
		screenshot_path TEXT NOT NULL,
		width INTEGER DEFAULT 0,
		height INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS visual_diffs (
		id TEXT PRIMARY KEY,
		baseline_id TEXT NOT NULL,
		task_id TEXT NOT NULL,
		screenshot_path TEXT NOT NULL,
		diff_image_path TEXT NOT NULL,
		diff_percent REAL DEFAULT 0.0,
		pixel_count INTEGER DEFAULT 0,
		threshold REAL DEFAULT 5.0,
		passed INTEGER DEFAULT 0,
		width INTEGER DEFAULT 0,
		height INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_visual_baselines_url ON visual_baselines(url);
	CREATE INDEX IF NOT EXISTS idx_visual_diffs_baseline ON visual_diffs(baseline_id);
	CREATE INDEX IF NOT EXISTS idx_visual_diffs_task ON visual_diffs(task_id);

	CREATE INDEX IF NOT EXISTS idx_tasks_status_priority ON tasks(status, priority DESC, created_at ASC);
	CREATE INDEX IF NOT EXISTS idx_tasks_batch_status ON tasks(batch_id, status);
	CREATE INDEX IF NOT EXISTS idx_network_logs_task_step ON network_logs(task_id, step_index);
	CREATE INDEX IF NOT EXISTS idx_tasks_created ON tasks(created_at);
	CREATE INDEX IF NOT EXISTS idx_step_logs_task_step ON step_logs(task_id, step_index);
	`
	_, err := db.conn.Exec(schema)
	if err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}
	if _, err := db.conn.Exec(`ALTER TABLE tasks ADD COLUMN logging_policy TEXT DEFAULT ''`); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("add tasks.logging_policy column: %w", err)
	}

	return nil
}

func (db *DB) BeginTx(ctx context.Context) (*sql.Tx, error) {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return tx, nil
}
