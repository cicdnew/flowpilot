package database

import (
	"context"
	"database/sql"
	"fmt"

	"flowpilot/internal/models"
)

func (db *DB) CreateBatchGroup(ctx context.Context, group models.BatchGroup) error {
	_, err := db.conn.ExecContext(ctx, `INSERT INTO batch_groups (id, flow_id, name, total, created_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		group.ID, group.FlowID, group.Name, group.Total)
	if err != nil {
		return fmt.Errorf("insert batch group %s: %w", group.ID, err)
	}
	return nil
}

func (db *DB) CreateBatchGroupTx(ctx context.Context, tx *sql.Tx, group models.BatchGroup) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO batch_groups (id, flow_id, name, total, created_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		group.ID, group.FlowID, group.Name, group.Total)
	if err != nil {
		return fmt.Errorf("insert batch group %s: %w", group.ID, err)
	}
	return nil
}

func (db *DB) ListBatchGroups(ctx context.Context) ([]models.BatchGroup, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, flow_id, name, total, created_at FROM batch_groups ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query batch groups: %w", err)
	}
	defer rows.Close()

	var groups []models.BatchGroup
	for rows.Next() {
		var g models.BatchGroup
		if err := rows.Scan(&g.ID, &g.FlowID, &g.Name, &g.Total, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan batch group: %w", err)
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate batch groups: %w", err)
	}
	return groups, nil
}

func (db *DB) GetBatchProgress(ctx context.Context, batchID string) (models.BatchProgress, error) {
	progress := models.BatchProgress{BatchID: batchID}
	rows, err := db.readConn.QueryContext(ctx, `SELECT status, COUNT(*) FROM tasks WHERE batch_id = ? GROUP BY status`, batchID)
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

func (db *DB) ListTasksByBatch(ctx context.Context, batchID string) ([]models.Task, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, timeout_seconds, error, result, tags, logging_policy, created_at, started_at, completed_at, webhook_url, webhook_events
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

func (db *DB) ListTasksByBatchStatus(ctx context.Context, batchID string, status models.TaskStatus) ([]models.Task, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, timeout_seconds, error, result, tags, logging_policy, created_at, started_at, completed_at, webhook_url, webhook_events
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
