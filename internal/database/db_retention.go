package database

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (db *DB) PurgeOldRecords(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays).Format("2006-01-02 15:04:05")

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin purge tx: %w", err)
	}
	defer tx.Rollback()

	var expiredIDs []string
	rows, err := tx.QueryContext(ctx, `SELECT id FROM tasks WHERE completed_at < ? AND status IN ('completed', 'failed', 'cancelled')`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("query expired tasks: %w", err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan expired task id: %w", err)
		}
		expiredIDs = append(expiredIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate expired tasks: %w", err)
	}

	if len(expiredIDs) == 0 {
		return 0, nil
	}

	placeholders := make([]string, len(expiredIDs))
	args := make([]any, len(expiredIDs))
	for i, id := range expiredIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	inClause := "(" + strings.Join(placeholders, ",") + ")"

	var total int64
	purgeQueries := []struct {
		label string
		query string
	}{
		{"step logs", `DELETE FROM step_logs WHERE task_id IN ` + inClause},
		{"network logs", `DELETE FROM network_logs WHERE task_id IN ` + inClause},
		{"task events", `DELETE FROM task_events WHERE task_id IN ` + inClause},
		{"tasks", `DELETE FROM tasks WHERE id IN ` + inClause},
	}

	for _, pq := range purgeQueries {
		res, err := tx.ExecContext(ctx, pq.query, args...)
		if err != nil {
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
