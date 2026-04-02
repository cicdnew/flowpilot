package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"flowpilot/internal/models"
)

func (db *DB) InsertTaskEvent(ctx context.Context, event models.TaskLifecycleEvent) error {
	_, err := db.conn.ExecContext(ctx, `INSERT INTO task_events (id, task_id, batch_id, from_state, to_state, error, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.TaskID, event.BatchID, event.FromState, event.ToState, event.Error, event.Timestamp)
	if err != nil {
		return fmt.Errorf("insert task event %s: %w", event.ID, err)
	}
	return nil
}

func slimTaskResult(result models.TaskResult) models.TaskResult {
	result.StepLogs = nil
	result.NetworkLogs = nil
	return result
}

func insertTaskEventTx(ctx context.Context, tx *sql.Tx, event models.TaskLifecycleEvent) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO task_events (id, task_id, batch_id, from_state, to_state, error, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.TaskID, event.BatchID, event.FromState, event.ToState, event.Error, event.Timestamp)
	if err != nil {
		return fmt.Errorf("insert task event %s: %w", event.ID, err)
	}
	return nil
}

func insertTaskEventsTx(ctx context.Context, tx *sql.Tx, events []models.TaskLifecycleEvent) error {
	if len(events) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO task_events (id, task_id, batch_id, from_state, to_state, error, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare task event insert: %w", err)
	}
	defer stmt.Close()

	for _, event := range events {
		if _, err := stmt.ExecContext(ctx, event.ID, event.TaskID, event.BatchID, event.FromState, event.ToState, event.Error, event.Timestamp); err != nil {
			return fmt.Errorf("insert task event %s: %w", event.ID, err)
		}
	}
	return nil
}

func insertStepLogsTx(ctx context.Context, tx *sql.Tx, taskID string, logs []models.StepLog) error {
	if len(logs) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO step_logs (task_id, step_index, action, selector, value, snapshot_id, error_code, error_msg, duration_ms, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare step log insert: %w", err)
	}
	defer stmt.Close()

	for _, log := range logs {
		if _, err := stmt.ExecContext(ctx, taskID, log.StepIndex, log.Action, log.Selector, log.Value, log.SnapshotID, log.ErrorCode, log.ErrorMsg, log.DurationMs, log.StartedAt); err != nil {
			return fmt.Errorf("insert step log: %w", err)
		}
	}
	return nil
}

func insertNetworkLogsTx(ctx context.Context, tx *sql.Tx, taskID string, logs []models.NetworkLog) error {
	if len(logs) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO network_logs (task_id, step_index, request_url, method, status_code, mime_type, request_headers, response_headers, request_size, response_size, duration_ms, error, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare network log insert: %w", err)
	}
	defer stmt.Close()

	for _, log := range logs {
		if _, err := stmt.ExecContext(ctx, taskID, log.StepIndex, log.RequestURL, log.Method, log.StatusCode, log.MimeType, log.RequestHeaders, log.ResponseHeaders, log.RequestSize, log.ResponseSize, log.DurationMs, log.Error, log.Timestamp); err != nil {
			return fmt.Errorf("insert network log: %w", err)
		}
	}
	return nil
}

func (db *DB) FinalizeTaskSuccess(ctx context.Context, taskID string, result models.TaskResult) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin finalize success tx: %w", err)
	}
	defer tx.Rollback()

	var fromStatus models.TaskStatus
	var batchID string
	if err := tx.QueryRowContext(ctx, `SELECT status, batch_id FROM tasks WHERE id = ?`, taskID).Scan(&fromStatus, &batchID); err != nil {
		return fmt.Errorf("task %s not found", taskID)
	}

	storedResult := slimTaskResult(result)
	resultJSON, err := json.Marshal(storedResult)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	now := time.Now()
	res, err := tx.ExecContext(ctx, `UPDATE tasks SET result = ?, status = ?, error = ?, completed_at = ? WHERE id = ?`, string(resultJSON), models.TaskStatusCompleted, "", now, taskID)
	if err != nil {
		return fmt.Errorf("update task %s success: %w", taskID, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check success update result for task %s: %w", taskID, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("task %s not found", taskID)
	}

	if err := insertStepLogsTx(ctx, tx, taskID, result.StepLogs); err != nil {
		return err
	}
	if err := insertNetworkLogsTx(ctx, tx, taskID, result.NetworkLogs); err != nil {
		return err
	}

	event := models.TaskLifecycleEvent{
		ID:        "evt_" + uuid.New().String(),
		TaskID:    taskID,
		BatchID:   batchID,
		FromState: fromStatus,
		ToState:   models.TaskStatusCompleted,
		Timestamp: now,
	}
	if err := insertTaskEventTx(ctx, tx, event); err != nil {
		return fmt.Errorf("insert task event for %s: %w", taskID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit finalize success: %w", err)
	}
	return nil
}

func (db *DB) FinalizeTaskFailure(ctx context.Context, taskID string, errMsg string, stepLogs []models.StepLog, networkLogs []models.NetworkLog) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin finalize failure tx: %w", err)
	}
	defer tx.Rollback()

	var fromStatus models.TaskStatus
	var batchID string
	if err := tx.QueryRowContext(ctx, `SELECT status, batch_id FROM tasks WHERE id = ?`, taskID).Scan(&fromStatus, &batchID); err != nil {
		return fmt.Errorf("task %s not found", taskID)
	}

	now := time.Now()
	res, err := tx.ExecContext(ctx, `UPDATE tasks SET status = ?, error = ?, completed_at = ? WHERE id = ?`, models.TaskStatusFailed, errMsg, now, taskID)
	if err != nil {
		return fmt.Errorf("update task %s failure: %w", taskID, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check failure update result for task %s: %w", taskID, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("task %s not found", taskID)
	}

	if err := insertStepLogsTx(ctx, tx, taskID, stepLogs); err != nil {
		return err
	}
	if err := insertNetworkLogsTx(ctx, tx, taskID, networkLogs); err != nil {
		return err
	}

	event := models.TaskLifecycleEvent{
		ID:        "evt_" + uuid.New().String(),
		TaskID:    taskID,
		BatchID:   batchID,
		FromState: fromStatus,
		ToState:   models.TaskStatusFailed,
		Error:     errMsg,
		Timestamp: now,
	}
	if err := insertTaskEventTx(ctx, tx, event); err != nil {
		return fmt.Errorf("insert task event for %s: %w", taskID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit finalize failure: %w", err)
	}
	return nil
}

func (db *DB) ListTaskEvents(ctx context.Context, taskID string) ([]models.TaskLifecycleEvent, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, task_id, batch_id, from_state, to_state, error, timestamp FROM task_events WHERE task_id = ? ORDER BY timestamp ASC`, taskID)
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

func (db *DB) InsertStepLogs(ctx context.Context, taskID string, logs []models.StepLog) error {
	if len(logs) == 0 {
		return nil
	}
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin step logs tx: %w", err)
	}
	defer tx.Rollback()
	if err := insertStepLogsTx(ctx, tx, taskID, logs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit step logs: %w", err)
	}
	return nil
}

func (db *DB) ListStepLogs(ctx context.Context, taskID string) ([]models.StepLog, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT task_id, step_index, action, selector, value, snapshot_id, error_code, error_msg, duration_ms, started_at
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

func (db *DB) InsertNetworkLogs(ctx context.Context, taskID string, logs []models.NetworkLog) error {
	if len(logs) == 0 {
		return nil
	}
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin network logs tx: %w", err)
	}
	defer tx.Rollback()
	if err := insertNetworkLogsTx(ctx, tx, taskID, logs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit network logs: %w", err)
	}
	return nil
}

func (db *DB) ListNetworkLogs(ctx context.Context, taskID string) ([]models.NetworkLog, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT task_id, step_index, request_url, method, status_code, mime_type, request_headers, response_headers, request_size, response_size, duration_ms, error, timestamp
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

func (db *DB) InsertWebSocketLogs(ctx context.Context, flowID string, logs []models.WebSocketLog) error {
	if len(logs) == 0 {
		return nil
	}
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin websocket logs tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO websocket_logs (flow_id, step_index, request_id, url, event_type, direction, opcode, payload_size, payload_snippet, close_code, close_reason, error_message, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare websocket log insert: %w", err)
	}
	defer stmt.Close()

	for _, log := range logs {
		if _, err := stmt.ExecContext(ctx, flowID, log.StepIndex, log.RequestID, log.URL, log.EventType, log.Direction, log.Opcode, log.PayloadSize, log.PayloadSnippet, log.CloseCode, log.CloseReason, log.ErrorMessage, log.Timestamp); err != nil {
			return fmt.Errorf("insert websocket log: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit websocket logs: %w", err)
	}
	return nil
}

func (db *DB) ListWebSocketLogs(ctx context.Context, flowID string) ([]models.WebSocketLog, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT flow_id, step_index, request_id, url, event_type, direction, opcode, payload_size, payload_snippet, close_code, close_reason, error_message, timestamp
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

func (db *DB) ListAuditTrail(ctx context.Context, taskID string, limit int) ([]models.TaskLifecycleEvent, error) {
	if limit < 0 {
		limit = 0
	}
	query := `SELECT id, task_id, batch_id, from_state, to_state, error, timestamp FROM task_events`
	args := []any{}
	if taskID != "" {
		query += ` WHERE task_id = ?`
		args = append(args, taskID)
	}
	query += ` ORDER BY timestamp DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := db.readConn.QueryContext(ctx, query, args...)
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
