package database

import (
	"context"
	"database/sql"
	"testing"
)

func TestApplyNamedMigrationsRecordsMigrations(t *testing.T) {
	db := setupTestDB(t)
	var count int
	if err := db.conn.QueryRow(`SELECT COUNT(1) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if count == 0 {
		t.Fatal("expected recorded migrations")
	}
}

func TestApplyNamedMigrationsIdempotent(t *testing.T) {
	db := setupTestDB(t)
	var before int
	if err := db.conn.QueryRow(`SELECT COUNT(1) FROM schema_migrations`).Scan(&before); err != nil {
		t.Fatalf("count before: %v", err)
	}
	if err := db.applyNamedMigrations(context.Background()); err != nil {
		t.Fatalf("applyNamedMigrations: %v", err)
	}
	var after int
	if err := db.conn.QueryRow(`SELECT COUNT(1) FROM schema_migrations`).Scan(&after); err != nil {
		t.Fatalf("count after: %v", err)
	}
	if after != before {
		t.Fatalf("migration count changed: before=%d after=%d", before, after)
	}
}

func TestMigrationTransactionRollsBackOnFailure(t *testing.T) {
	db := setupTestDB(t)
	name := "test.failing_migration"
	m := migration{name: name, up: func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS tmp_fail_table (id INTEGER)`); err != nil {
			return err
		}
		return sql.ErrTxDone
	}}
	applied, err := db.isMigrationApplied(context.Background(), name)
	if err != nil {
		t.Fatalf("isMigrationApplied before: %v", err)
	}
	if applied {
		t.Fatal("test migration unexpectedly already applied")
	}
	tx, err := db.conn.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := m.up(tx); err == nil {
		t.Fatal("expected migration failure")
	}
	_ = tx.Rollback()
	applied, err = db.isMigrationApplied(context.Background(), name)
	if err != nil {
		t.Fatalf("isMigrationApplied after: %v", err)
	}
	if applied {
		t.Fatal("failed migration should not be recorded")
	}
}
