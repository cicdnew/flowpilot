package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	libsql "github.com/tursodatabase/libsql-go"
)

// DB wraps the SQLite connections.
type DB struct {
	conn     *sql.DB // write-only connection
	readConn *sql.DB // read-only connection for concurrent queries
}

// Reader returns the read-only connection for concurrent queries.
func (db *DB) Reader() *sql.DB { return db.readConn }

// Conn returns the write connection. Used in tests to insert raw rows.
func (db *DB) Conn() *sql.DB { return db.conn }

// New creates a new SQLite database connection and initializes the schema.
func New(dbPath string) (*DB, error) {
	return NewWithConfig(DatabaseConfig{URL: dbPath})
}

// NewWithConfig creates a database connection from the provided config.
func NewWithConfig(config DatabaseConfig) (*DB, error) {
	if strings.TrimSpace(config.URL) == "" {
		return nil, fmt.Errorf("open database: empty database URL")
	}
	if DetectType(config.URL) == DatabaseTurso {
		return newTursoDB(config)
	}
	return newSQLiteDB(config.URL)
}

func newSQLiteDB(dbPath string) (*DB, error) {
	dsn := sqliteLocalDSN(dbPath)
	conn, err := sql.Open("libsql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Use a single pool for both reads and writes. libsql's local driver
	// serialises writes internally; a shared pool avoids "database is locked"
	// errors that occur when two separate *sql.DB handles contend on the file.
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	db := &DB{conn: conn, readConn: conn}
	applySQLitePragmas(conn)
	if err := db.migrate(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	return db, nil
}

func sqliteLocalDSN(dbPath string) string {
	trimmed := strings.TrimSpace(dbPath)
	if trimmed == "" || trimmed == ":memory:" || strings.HasPrefix(trimmed, "file:") {
		return trimmed
	}
	if strings.Contains(trimmed, "://") {
		return trimmed
	}
	return "file:" + trimmed
}

func newTursoDB(config DatabaseConfig) (*DB, error) {
	var (
		conn *sql.DB
		err  error
	)

	if strings.TrimSpace(config.LocalPath) != "" {
		opts := make([]libsql.Option, 0, 1)
		if strings.TrimSpace(config.AuthToken) != "" {
			opts = append(opts, libsql.WithAuthToken(config.AuthToken))
		}
		connector, err := libsql.NewEmbeddedReplicaConnector(config.LocalPath, config.URL, opts...)
		if err != nil {
			return nil, fmt.Errorf("open turso embedded replica: %w", err)
		}
		conn = sql.OpenDB(connector)
	} else {
		dsn := config.URL
		if strings.TrimSpace(config.AuthToken) != "" {
			dsn = appendQueryParam(dsn, "authToken", config.AuthToken)
		}
		conn, err = sql.Open("libsql", dsn)
		if err != nil {
			return nil, fmt.Errorf("open turso database: %w", err)
		}
	}

	conn.SetMaxOpenConns(4)
	conn.SetMaxIdleConns(4)
	db := &DB{conn: conn, readConn: conn}
	if err := db.migrate(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	return db, nil
}

func applySQLitePragmas(conns ...*sql.DB) {
	for _, c := range conns {
		if c == nil {
			continue
		}
		_, _ = c.Exec("PRAGMA busy_timeout=5000")
		_, _ = c.Exec("PRAGMA journal_mode=WAL")
		_, _ = c.Exec("PRAGMA synchronous=NORMAL")
		_, _ = c.Exec("PRAGMA cache_size=-64000")
		_, _ = c.Exec("PRAGMA mmap_size=268435456")
		_, _ = c.Exec("PRAGMA temp_store=MEMORY")
	}
}

func appendQueryParam(raw, key, value string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	if strings.HasPrefix(raw, "file:") || strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err == nil {
			query := parsed.Query()
			query.Set(key, value)
			parsed.RawQuery = query.Encode()
			return parsed.String()
		}
	}
	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}
	return raw + sep + url.QueryEscape(key) + "=" + url.QueryEscape(value)
}

// Close closes both database connections.
func (db *DB) Close() error {
	if db == nil {
		return nil
	}
	if db.readConn == nil || db.readConn == db.conn {
		if db.conn == nil {
			return nil
		}
		return db.conn.Close()
	}
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
		status TEXT DEFAULT 'pending' CHECK(status IN ('pending','queued','running','retrying','completed','failed','cancelled')),
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
		status TEXT DEFAULT 'unknown' CHECK(status IN ('unknown','healthy','unhealthy')),
		latency INTEGER DEFAULT 0,
		success_rate REAL DEFAULT 0.0,
		total_used INTEGER DEFAULT 0,
		max_requests_per_minute INTEGER DEFAULT 0,
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
	CREATE INDEX IF NOT EXISTS idx_flows_updated ON recorded_flows(updated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_tasks_completed_status ON tasks(completed_at, status);
	`
	for _, stmt := range strings.Split(schema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.conn.Exec(stmt); err != nil {
			return fmt.Errorf("exec schema statement: %w", err)
		}
	}
	return db.applyNamedMigrations(context.Background())
}

func (db *DB) BeginTx(ctx context.Context) (*sql.Tx, error) {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return tx, nil
}
