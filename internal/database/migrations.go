package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type migration struct {
	name   string
	table  string
	column string
	up     func(*sql.Tx) error
}

func (db *DB) applyNamedMigrations(ctx context.Context) error {
	if _, err := db.conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	migrations := []migration{
		{
			name:   "tasks.logging_policy",
			table:  "tasks",
			column: "logging_policy",
			up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`ALTER TABLE tasks ADD COLUMN logging_policy TEXT DEFAULT ''`)
				return err
			},
		},
		{
			name:   "recorded_flows.timeout",
			table:  "recorded_flows",
			column: "timeout",
			up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`ALTER TABLE recorded_flows ADD COLUMN timeout INTEGER DEFAULT 0`)
				return err
			},
		},
		{
			name:   "recorded_flows.logging_policy",
			table:  "recorded_flows",
			column: "logging_policy",
			up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`ALTER TABLE recorded_flows ADD COLUMN logging_policy TEXT DEFAULT ''`)
				return err
			},
		},
		{
			name:   "tasks.webhook_url",
			table:  "tasks",
			column: "webhook_url",
			up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`ALTER TABLE tasks ADD COLUMN webhook_url TEXT DEFAULT ''`)
				return err
			},
		},
		{
			name:   "tasks.webhook_events",
			table:  "tasks",
			column: "webhook_events",
			up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`ALTER TABLE tasks ADD COLUMN webhook_events TEXT DEFAULT '[]'`)
				return err
			},
		},
		{
			name:   "proxies.max_requests_per_minute",
			table:  "proxies",
			column: "max_requests_per_minute",
			up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`ALTER TABLE proxies ADD COLUMN max_requests_per_minute INTEGER DEFAULT 0`)
				return err
			},
		},
	}

	for _, m := range migrations {
		applied, err := db.isMigrationApplied(ctx, m.name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if m.table != "" && m.column != "" {
			exists, err := db.columnExists(ctx, m.table, m.column)
			if err != nil {
				return err
			}
			if exists {
				if err := db.recordMigration(ctx, m.name); err != nil {
					return err
				}
				continue
			}
		}
		tx, err := db.conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", m.name, err)
		}
		if err := m.up(tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(name) VALUES (?)`, m.name); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", m.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", m.name, err)
		}
	}
	return nil
}

func (db *DB) isMigrationApplied(ctx context.Context, name string) (bool, error) {
	var count int
	if err := db.conn.QueryRowContext(ctx, `SELECT COUNT(1) FROM schema_migrations WHERE name = ?`, name).Scan(&count); err != nil {
		return false, fmt.Errorf("check migration %s: %w", name, err)
	}
	return count > 0, nil
}

func (db *DB) recordMigration(ctx context.Context, name string) error {
	if _, err := db.conn.ExecContext(ctx, `INSERT INTO schema_migrations(name) VALUES (?)`, name); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	return nil
}

func (db *DB) columnExists(ctx context.Context, table, column string) (bool, error) {
	rows, err := db.conn.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info("%s")`, strings.ReplaceAll(table, `"`, `""`)))
	if err != nil {
		return false, fmt.Errorf("query table info for %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return false, fmt.Errorf("scan table info for %s: %w", table, err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate table info for %s: %w", table, err)
	}
	return false, nil
}
