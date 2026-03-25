package database

import (
	"context"
	"fmt"
	"time"
)

func (db *DB) PurgeOldRecords(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays).Format("2006-01-02 15:04:05")

	tx, err := db.conn.BeginTx(ctx, nil)
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
		res, err := tx.ExecContext(ctx, pq.query, cutoff)
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
