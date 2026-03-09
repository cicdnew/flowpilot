package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"flowpilot/internal/crypto"
	"flowpilot/internal/models"
)

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
		&t.Priority, &t.Status, &t.RetryCount, &t.MaxRetries, &t.Timeout, &t.Error,
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

func (db *DB) CreateTask(ctx context.Context, task models.Task) error {
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

	_, err = db.conn.ExecContext(ctx, `
		INSERT INTO tasks (id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol, priority, status, max_retries, timeout_seconds, tags, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.Name, task.URL, string(stepsJSON), task.BatchID, task.FlowID, headless,
		task.Proxy.Server, encUsername, encPassword, task.Proxy.Geo, task.Proxy.Protocol,
		task.Priority, task.Status, task.MaxRetries, task.Timeout, string(tagsJSON), task.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert task %s: %w", task.ID, err)
	}
	return nil
}

func (db *DB) CreateTaskTx(ctx context.Context, tx *sql.Tx, task models.Task) error {
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

	_, err = tx.ExecContext(ctx, `
		INSERT INTO tasks (id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol, priority, status, max_retries, timeout_seconds, tags, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.Name, task.URL, string(stepsJSON), task.BatchID, task.FlowID, headless,
		task.Proxy.Server, encUsername, encPassword, task.Proxy.Geo, task.Proxy.Protocol,
		task.Priority, task.Status, task.MaxRetries, task.Timeout, string(tagsJSON), task.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert task %s: %w", task.ID, err)
	}
	return nil
}

func (db *DB) GetTask(ctx context.Context, id string) (*models.Task, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, timeout_seconds, error, result, tags, created_at, started_at, completed_at
		FROM tasks WHERE id = ?`, id)
	task, err := db.scanTask(row)
	if err != nil {
		return nil, fmt.Errorf("get task %s: %w", id, err)
	}
	return task, nil
}

func (db *DB) ListTasks(ctx context.Context) ([]models.Task, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, timeout_seconds, error, result, tags, created_at, started_at, completed_at
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

func (db *DB) ListTasksByStatus(ctx context.Context, status models.TaskStatus) ([]models.Task, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, timeout_seconds, error, result, tags, created_at, started_at, completed_at
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

func (db *DB) UpdateTaskStatus(ctx context.Context, id string, status models.TaskStatus, errMsg string) error {
	var fromStatus models.TaskStatus
	var batchID string
	if err := db.conn.QueryRowContext(ctx, `SELECT status, batch_id FROM tasks WHERE id = ?`, id).Scan(&fromStatus, &batchID); err != nil {
		return fmt.Errorf("task %s not found", id)
	}

	now := time.Now()
	var res sql.Result
	var err error
	switch status {
	case models.TaskStatusRunning:
		res, err = db.conn.ExecContext(ctx, `UPDATE tasks SET status = ?, started_at = ? WHERE id = ?`, status, now, id)
	case models.TaskStatusCompleted, models.TaskStatusFailed:
		res, err = db.conn.ExecContext(ctx, `UPDATE tasks SET status = ?, error = ?, completed_at = ? WHERE id = ?`, status, errMsg, now, id)
	default:
		res, err = db.conn.ExecContext(ctx, `UPDATE tasks SET status = ?, error = ? WHERE id = ?`, status, errMsg, id)
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

	event := models.TaskLifecycleEvent{
		ID:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		TaskID:    id,
		BatchID:   batchID,
		FromState: fromStatus,
		ToState:   status,
		Error:     errMsg,
		Timestamp: now,
	}
	if insertErr := db.InsertTaskEvent(ctx, event); insertErr != nil {
		return fmt.Errorf("insert task event for %s: %w", id, insertErr)
	}

	return nil
}

func (db *DB) UpdateTaskResult(ctx context.Context, id string, result models.TaskResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	res, err := db.conn.ExecContext(ctx, `UPDATE tasks SET result = ? WHERE id = ?`, string(resultJSON), id)
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

func (db *DB) IncrementRetry(ctx context.Context, id string) error {
	res, err := db.conn.ExecContext(ctx, `UPDATE tasks SET retry_count = retry_count + 1, status = 'retrying' WHERE id = ?`, id)
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

func (db *DB) ResetRetryCount(ctx context.Context, id string) error {
	res, err := db.conn.ExecContext(ctx, `UPDATE tasks SET retry_count = 0 WHERE id = ?`, id)
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

func (db *DB) UpdateTask(ctx context.Context, id, name, url string, steps []models.TaskStep, proxyConfig models.ProxyConfig, priority models.TaskPriority, tags []string, timeout int) error {
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

	res, err := db.conn.ExecContext(ctx, `UPDATE tasks SET name = ?, url = ?, steps = ?, proxy_server = ?, proxy_username = ?, proxy_password = ?, proxy_geo = ?, proxy_protocol = ?, priority = ?, tags = ?, timeout_seconds = ? WHERE id = ? AND status IN (?, ?)`,
		name, url, string(stepsJSON), proxyConfig.Server, encUsername, encPassword, proxyConfig.Geo, proxyConfig.Protocol, priority, string(tagsJSON), timeout, id,
		models.TaskStatusPending, models.TaskStatusFailed)
	if err != nil {
		return fmt.Errorf("update task %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update result for task %s: %w", id, err)
	}
	if n == 0 {
		task, getErr := db.GetTask(ctx, id)
		if getErr != nil {
			return fmt.Errorf("task %s not found", id)
		}
		return fmt.Errorf("cannot edit task with status %s", task.Status)
	}
	return nil
}

func (db *DB) DeleteTask(ctx context.Context, id string) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete task tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM step_logs WHERE task_id = ?`, id); err != nil {
		return fmt.Errorf("delete step logs for task %s: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM network_logs WHERE task_id = ?`, id); err != nil {
		return fmt.Errorf("delete network logs for task %s: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM task_events WHERE task_id = ?`, id); err != nil {
		return fmt.Errorf("delete task events for task %s: %w", id, err)
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
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
	return tx.Commit()
}

func (db *DB) GetTaskStats(ctx context.Context) (map[string]int, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT status, COUNT(*) FROM tasks GROUP BY status`)
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

func (db *DB) ListTasksPaginated(ctx context.Context, page, pageSize int, status string, tag string) (models.PaginatedTasks, error) {
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
	if err := db.conn.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return models.PaginatedTasks{}, fmt.Errorf("count tasks: %w", err)
	}

	totalPages := (total + pageSize - 1) / pageSize
	offset := (page - 1) * pageSize

	query := `SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, timeout_seconds, error, result, tags, created_at, started_at, completed_at
		FROM tasks ` + where + ` ORDER BY priority DESC, created_at DESC LIMIT ? OFFSET ?`
	queryArgs := make([]any, 0, len(args)+2)
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, pageSize, offset)

	rows, err := db.conn.QueryContext(ctx, query, queryArgs...)
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
