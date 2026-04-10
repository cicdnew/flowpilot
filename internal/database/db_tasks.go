package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"flowpilot/internal/crypto"
	"flowpilot/internal/models"
)

// unmarshalIfNonEmpty unmarshals jsonStr into dest only if jsonStr is non-empty.
func unmarshalIfNonEmpty(jsonStr string, dest any) error {
	if jsonStr == "" {
		return nil
	}
	return json.Unmarshal([]byte(jsonStr), dest)
}

const (
	errMarshalSteps         = "marshal steps: %w"
	errMarshalTags          = "marshal tags: %w"
	errMarshalLoggingPolicy = "marshal logging policy: %w"
	errMarshalWebhookEvents = "marshal webhook events: %w"
	errEncryptProxyUsername = "encrypt proxy username: %w"
	errEncryptProxyPassword = "encrypt proxy password: %w"
)

// TaskUpdateParams holds parameters for updating a task.
type TaskUpdateParams struct {
	Name          string
	URL           string
	Steps         []models.TaskStep
	ProxyConfig   models.ProxyConfig
	Priority      models.TaskPriority
	Tags          []string
	Timeout       int
	LoggingPolicy *models.TaskLoggingPolicy
}

type scanner interface {
	Scan(dest ...any) error
}

type TaskStateChange struct {
	TaskID         string
	Status         models.TaskStatus
	Error          string
	IncrementRetry bool
}

const errScanTaskRowFmt = errScanTaskRow

// parseTaskJSON unmarshals task-related JSON fields (S3776 - reduce complexity)
func (db *DB) parseTaskJSON(t *models.Task, stepsJSON, resultJSON, tagsJSON, loggingPolicyJSON, webhookEventsJSON string) error {
	if err := unmarshalIfNonEmpty(stepsJSON, &t.Steps); err != nil {
		return fmt.Errorf("parse steps JSON: %w", err)
	}
	if resultJSON != "" {
		var result models.TaskResult
		if err := unmarshalIfNonEmpty(resultJSON, &result); err != nil {
			return fmt.Errorf("parse result JSON: %w", err)
		}
		t.Result = &result
	}
	if err := unmarshalIfNonEmpty(tagsJSON, &t.Tags); err != nil {
		return fmt.Errorf("parse tags JSON: %w", err)
	}
	if loggingPolicyJSON != "" {
		var policy models.TaskLoggingPolicy
		if err := unmarshalIfNonEmpty(loggingPolicyJSON, &policy); err != nil {
			return fmt.Errorf("parse logging policy JSON: %w", err)
		}
		t.LoggingPolicy = &policy
	}
	if webhookEventsJSON != "" && webhookEventsJSON != "[]" {
		if err := unmarshalIfNonEmpty(webhookEventsJSON, &t.WebhookEvents); err != nil {
			return fmt.Errorf("parse webhook events JSON: %w", err)
		}
	}
	return nil
}

// decryptProxyCredentials decrypts proxy username and password (S3776 - reduce complexity)
func (db *DB) decryptProxyCredentials(t *models.Task) error {
	if t.Proxy.Username != "" {
		decUser, err := crypto.Decrypt(t.Proxy.Username)
		if err != nil {
			return fmt.Errorf("decrypt proxy username for task %s: %w", t.ID, err)
		}
		t.Proxy.Username = decUser
	}
	if t.Proxy.Password != "" {
		decPass, err := crypto.Decrypt(t.Proxy.Password)
		if err != nil {
			return fmt.Errorf("decrypt proxy password for task %s: %w", t.ID, err)
		}
		t.Proxy.Password = decPass
	}
	return nil
}

func (db *DB) scanTask(row scanner) (*models.Task, error) {
	var t models.Task
	var stepsJSON, resultJSON, tagsJSON, loggingPolicyJSON, webhookEventsJSON string
	var startedAt, completedAt sql.NullTime
	var headlessInt int

	err := row.Scan(
		&t.ID, &t.Name, &t.URL, &stepsJSON, &t.BatchID, &t.FlowID, &headlessInt,
		&t.Proxy.Server, &t.Proxy.Username, &t.Proxy.Password, &t.Proxy.Geo, &t.Proxy.Protocol,
		&t.Priority, &t.Status, &t.RetryCount, &t.MaxRetries, &t.Timeout, &t.Error,
		&resultJSON, &tagsJSON, &loggingPolicyJSON, &t.CreatedAt, &startedAt, &completedAt,
		&t.WebhookURL, &webhookEventsJSON,
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

	if err := db.parseTaskJSON(&t, stepsJSON, resultJSON, tagsJSON, loggingPolicyJSON, webhookEventsJSON); err != nil {
		return nil, err
	}

	if err := db.decryptProxyCredentials(&t); err != nil {
		return nil, err
	}

	return &t, nil
}

func (db *DB) scanTaskRow(rows *sql.Rows) (*models.Task, error) {
	return db.scanTask(rows)
}

func (db *DB) scanTaskSummary(row scanner) (*models.Task, error) {
	var t models.Task
	var tagsJSON, loggingPolicyJSON string
	var startedAt, completedAt sql.NullTime
	var headlessInt int

	err := row.Scan(
		&t.ID, &t.Name, &t.URL, &t.BatchID, &t.FlowID, &headlessInt,
		&t.Proxy.Server, &t.Proxy.Geo, &t.Proxy.Protocol,
		&t.Priority, &t.Status, &t.RetryCount, &t.MaxRetries, &t.Timeout, &t.Error,
		&tagsJSON, &loggingPolicyJSON, &t.CreatedAt, &startedAt, &completedAt,
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

	if tagsJSON != "" {
		if err := json.Unmarshal([]byte(tagsJSON), &t.Tags); err != nil {
			return nil, fmt.Errorf("parse tags JSON: %w", err)
		}
	}
	if loggingPolicyJSON != "" {
		var policy models.TaskLoggingPolicy
		if err := json.Unmarshal([]byte(loggingPolicyJSON), &policy); err != nil {
			return nil, fmt.Errorf("parse logging policy JSON: %w", err)
		}
		t.LoggingPolicy = &policy
	}

	return &t, nil
}

func (db *DB) scanTaskSummaryRow(rows *sql.Rows) (*models.Task, error) {
	return db.scanTaskSummary(rows)
}

func (db *DB) CreateTask(ctx context.Context, task models.Task) error {
	stepsJSON, err := json.Marshal(task.Steps)
	if err != nil {
		return fmt.Errorf(errMarshalSteps, err)
	}
	tagsJSON, err := json.Marshal(task.Tags)
	if err != nil {
		return fmt.Errorf(errMarshalTags, err)
	}
	loggingPolicyJSON, err := json.Marshal(task.LoggingPolicy)
	if err != nil {
		return fmt.Errorf(errMarshalLoggingPolicy, err)
	}

	encUsername, err := crypto.Encrypt(task.Proxy.Username)
	if err != nil {
		return fmt.Errorf(errEncryptProxyUsername, err)
	}
	encPassword, err := crypto.Encrypt(task.Proxy.Password)
	if err != nil {
		return fmt.Errorf(errEncryptProxyPassword, err)
	}

	headless := 1
	if !task.Headless {
		headless = 0
	}

	webhookEventsJSON, err := json.Marshal(task.WebhookEvents)
	if err != nil {
		return fmt.Errorf(errMarshalWebhookEvents, err)
	}

	_, err = db.conn.ExecContext(ctx, `
		INSERT INTO tasks (id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol, priority, status, max_retries, timeout_seconds, tags, logging_policy, webhook_url, webhook_events, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.Name, task.URL, string(stepsJSON), task.BatchID, task.FlowID, headless,
		task.Proxy.Server, encUsername, encPassword, task.Proxy.Geo, task.Proxy.Protocol,
		task.Priority, task.Status, task.MaxRetries, task.Timeout, string(tagsJSON), string(loggingPolicyJSON),
		task.WebhookURL, string(webhookEventsJSON), task.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert task %s: %w", task.ID, err)
	}
	return nil
}

func (db *DB) CreateTaskTx(ctx context.Context, tx *sql.Tx, task models.Task) error {
	stepsJSON, err := json.Marshal(task.Steps)
	if err != nil {
		return fmt.Errorf(errMarshalSteps, err)
	}
	tagsJSON, err := json.Marshal(task.Tags)
	if err != nil {
		return fmt.Errorf(errMarshalTags, err)
	}
	loggingPolicyJSON, err := json.Marshal(task.LoggingPolicy)
	if err != nil {
		return fmt.Errorf(errMarshalLoggingPolicy, err)
	}
	webhookEventsJSON, err := json.Marshal(task.WebhookEvents)
	if err != nil {
		return fmt.Errorf(errMarshalWebhookEvents, err)
	}

	encUsername, err := crypto.Encrypt(task.Proxy.Username)
	if err != nil {
		return fmt.Errorf(errEncryptProxyUsername, err)
	}
	encPassword, err := crypto.Encrypt(task.Proxy.Password)
	if err != nil {
		return fmt.Errorf(errEncryptProxyPassword, err)
	}

	headless := 1
	if !task.Headless {
		headless = 0
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO tasks (id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol, priority, status, max_retries, timeout_seconds, tags, logging_policy, webhook_url, webhook_events, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.Name, task.URL, string(stepsJSON), task.BatchID, task.FlowID, headless,
		task.Proxy.Server, encUsername, encPassword, task.Proxy.Geo, task.Proxy.Protocol,
		task.Priority, task.Status, task.MaxRetries, task.Timeout, string(tagsJSON), string(loggingPolicyJSON),
		task.WebhookURL, string(webhookEventsJSON), task.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert task %s: %w", task.ID, err)
	}
	return nil
}

func (db *DB) GetTask(ctx context.Context, id string) (*models.Task, error) {
	row := db.readConn.QueryRowContext(ctx, `SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, timeout_seconds, error, result, tags, logging_policy, created_at, started_at, completed_at, webhook_url, webhook_events
		FROM tasks WHERE id = ?`, id)
	task, err := db.scanTask(row)
	if err != nil {
		return nil, fmt.Errorf("get task %s: %w", id, err)
	}
	return task, nil
}

func (db *DB) ListTasks(ctx context.Context) ([]models.Task, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, timeout_seconds, error, result, tags, logging_policy, created_at, started_at, completed_at, webhook_url, webhook_events
		FROM tasks ORDER BY priority DESC, created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		task, err := db.scanTaskRow(rows)
		if err != nil {
			return nil, fmt.Errorf(errScanTaskRowFmt, err)
		}
		tasks = append(tasks, *task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}
	return tasks, nil
}

func (db *DB) ListTasksByStatus(ctx context.Context, status models.TaskStatus) ([]models.Task, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, timeout_seconds, error, result, tags, logging_policy, created_at, started_at, completed_at, webhook_url, webhook_events
		FROM tasks WHERE status = ? ORDER BY priority DESC, created_at DESC`, status)
	if err != nil {
		return nil, fmt.Errorf("query tasks by status %s: %w", status, err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		task, err := db.scanTaskRow(rows)
		if err != nil {
			return nil, fmt.Errorf(errScanTaskRowFmt, err)
		}
		tasks = append(tasks, *task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}
	return tasks, nil
}

func (db *DB) UpdateTaskStatus(ctx context.Context, id string, status models.TaskStatus, errMsg string) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update task status tx: %w", err)
	}
	defer tx.Rollback()

	var fromStatus models.TaskStatus
	var batchID string
	if err := tx.QueryRowContext(ctx, `SELECT status, batch_id FROM tasks WHERE id = ?`, id).Scan(&fromStatus, &batchID); err != nil {
		return fmt.Errorf(errTaskNotFound, id)
	}

	now := time.Now()
	var res sql.Result
	switch status {
	case models.TaskStatusRunning:
		res, err = tx.ExecContext(ctx, `UPDATE tasks SET status = ?, started_at = ? WHERE id = ?`, status, now, id)
	case models.TaskStatusCompleted, models.TaskStatusFailed:
		res, err = tx.ExecContext(ctx, `UPDATE tasks SET status = ?, error = ?, completed_at = ? WHERE id = ?`, status, errMsg, now, id)
	default:
		res, err = tx.ExecContext(ctx, `UPDATE tasks SET status = ?, error = ? WHERE id = ?`, status, errMsg, id)
	}
	if err != nil {
		return fmt.Errorf("update task %s status to %s: %w", id, status, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf(errCheckUpdateTask, id, err)
	}
	if n == 0 {
		return fmt.Errorf(errTaskNotFound, id)
	}

	event := models.TaskLifecycleEvent{
		ID:        "evt_" + uuid.New().String(),
		TaskID:    id,
		BatchID:   batchID,
		FromState: fromStatus,
		ToState:   status,
		Error:     errMsg,
		Timestamp: now,
	}
	if err := insertTaskEventTx(ctx, tx, event); err != nil {
		return fmt.Errorf("insert task event for %s: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update task status: %w", err)
	}
	return nil
}

func (db *DB) UpdateTaskResult(ctx context.Context, id string, result models.TaskResult) error {
	resultJSON, err := json.Marshal(slimTaskResult(result))
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	res, err := db.conn.ExecContext(ctx, `UPDATE tasks SET result = ? WHERE id = ?`, string(resultJSON), id)
	if err != nil {
		return fmt.Errorf("update task %s result: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf(errCheckUpdateTask, id, err)
	}
	if n == 0 {
		return fmt.Errorf(errTaskNotFound, id)
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
		return fmt.Errorf(errTaskNotFound, id)
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
		return fmt.Errorf(errTaskNotFound, id)
	}
	return nil
}

func (db *DB) UpdateTask(ctx context.Context, id string, p TaskUpdateParams) error {
	stepsJSON, err := json.Marshal(p.Steps)
	if err != nil {
		return fmt.Errorf(errMarshalSteps, err)
	}
	tagsJSON, err := json.Marshal(p.Tags)
	if err != nil {
		return fmt.Errorf(errMarshalTags, err)
	}
	loggingPolicyJSON, err := json.Marshal(p.LoggingPolicy)
	if err != nil {
		return fmt.Errorf(errMarshalLoggingPolicy, err)
	}

	encUsername, err := crypto.Encrypt(p.ProxyConfig.Username)
	if err != nil {
		return fmt.Errorf(errEncryptProxyUsername, err)
	}
	encPassword, err := crypto.Encrypt(p.ProxyConfig.Password)
	if err != nil {
		return fmt.Errorf(errEncryptProxyPassword, err)
	}

	res, err := db.conn.ExecContext(ctx, `UPDATE tasks SET name = ?, url = ?, steps = ?, proxy_server = ?, proxy_username = ?, proxy_password = ?, proxy_geo = ?, proxy_protocol = ?, priority = ?, tags = ?, timeout_seconds = ?, logging_policy = ? WHERE id = ? AND status IN (?, ?)`,
		p.Name, p.URL, string(stepsJSON), p.ProxyConfig.Server, encUsername, encPassword, p.ProxyConfig.Geo, p.ProxyConfig.Protocol, p.Priority, string(tagsJSON), p.Timeout, string(loggingPolicyJSON), id,
		models.TaskStatusPending, models.TaskStatusFailed)
	if err != nil {
		return fmt.Errorf("update task %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf(errCheckUpdateTask, id, err)
	}
	if n == 0 {
		task, getErr := db.GetTask(ctx, id)
		if getErr != nil {
			return fmt.Errorf(errTaskNotFound, id)
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
		return fmt.Errorf(errTaskNotFound, id)
	}
	return tx.Commit()
}

func (db *DB) GetTaskStats(ctx context.Context) (map[string]int, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT status, COUNT(*) FROM tasks GROUP BY status`)
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
		where += " AND EXISTS (SELECT 1 FROM json_each(tags) WHERE value = ?)"
		args = append(args, tag)
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM tasks " + where
	if err := db.readConn.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return models.PaginatedTasks{}, fmt.Errorf("count tasks: %w", err)
	}

	totalPages := (total + pageSize - 1) / pageSize
	offset := (page - 1) * pageSize

	query := `SELECT id, name, url, batch_id, flow_id, headless, proxy_server, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, timeout_seconds, error, tags, logging_policy, created_at, started_at, completed_at
		FROM tasks ` + where + ` ORDER BY priority DESC, created_at DESC LIMIT ? OFFSET ?`
	queryArgs := make([]any, 0, len(args)+2)
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, pageSize, offset)

	rows, err := db.readConn.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return models.PaginatedTasks{}, fmt.Errorf("query paginated tasks: %w", err)
	}
	defer rows.Close()

	tasks := []models.Task{}
	for rows.Next() {
		task, err := db.scanTaskSummaryRow(rows)
		if err != nil {
			return models.PaginatedTasks{}, fmt.Errorf(errScanTaskRowFmt, err)
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

// ListStaleTasks returns tasks stuck in "running" or "queued" status.
// These are typically left over from a previous crash and need recovery.
func (db *DB) ListStaleTasks(ctx context.Context) ([]models.Task, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, name, url, steps, batch_id, flow_id, headless, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol,
		priority, status, retry_count, max_retries, timeout_seconds, error, result, tags, logging_policy, created_at, started_at, completed_at, webhook_url, webhook_events
		FROM tasks WHERE status IN (?, ?) ORDER BY priority DESC, created_at ASC`,
		models.TaskStatusRunning, models.TaskStatusQueued)
	if err != nil {
		return nil, fmt.Errorf("query stale tasks: %w", err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		task, err := db.scanTaskRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan stale task: %w", err)
		}
		tasks = append(tasks, *task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale tasks: %w", err)
	}
	return tasks, nil
}

// BatchApplyTaskStateChanges updates the status of multiple tasks in a single transaction.
type batchStmts struct {
	running  *sql.Stmt
	terminal *sql.Stmt
	def      *sql.Stmt
	retry    *sql.Stmt
}

type taskState struct {
	status  models.TaskStatus
	batchID string
}

func isTerminal(s models.TaskStatus) bool {
	return s == models.TaskStatusCancelled || s == models.TaskStatusCompleted || s == models.TaskStatusFailed
}

func applyStateChange(ctx context.Context, change TaskStateChange, stateMap map[string]taskState, s batchStmts) (models.TaskLifecycleEvent, bool, error) {
	state, ok := stateMap[change.TaskID]
	if !ok {
		return models.TaskLifecycleEvent{}, false, fmt.Errorf(errTaskNotFound, change.TaskID)
	}
	if isTerminal(state.status) && !isTerminal(change.Status) {
		return models.TaskLifecycleEvent{}, true, nil
	}

	now := time.Now()
	var (
		res sql.Result
		err error
	)
	switch {
	case change.IncrementRetry:
		res, err = s.retry.ExecContext(ctx, models.TaskStatusRetrying, change.TaskID)
	case change.Status == models.TaskStatusRunning:
		res, err = s.running.ExecContext(ctx, change.Status, now, change.TaskID)
	case isTerminal(change.Status):
		res, err = s.terminal.ExecContext(ctx, change.Status, change.Error, now, change.TaskID)
	default:
		res, err = s.def.ExecContext(ctx, change.Status, change.Error, change.TaskID)
	}
	if err != nil {
		return models.TaskLifecycleEvent{}, false, fmt.Errorf("batch update task %s status to %s: %w", change.TaskID, change.Status, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return models.TaskLifecycleEvent{}, false, fmt.Errorf("check batch update result for task %s: %w", change.TaskID, err)
	}
	if n == 0 {
		return models.TaskLifecycleEvent{}, false, fmt.Errorf(errTaskNotFound, change.TaskID)
	}

	event := models.TaskLifecycleEvent{
		ID:        "evt_" + uuid.New().String(),
		TaskID:    change.TaskID,
		BatchID:   state.batchID,
		FromState: state.status,
		ToState:   change.Status,
		Error:     change.Error,
		Timestamp: now,
	}
	return event, false, nil
}

func (db *DB) BatchApplyTaskStateChanges(ctx context.Context, changes []TaskStateChange) error {
	if len(changes) == 0 {
		return nil
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin batch state tx: %w", err)
	}
	defer tx.Rollback()

	prepared, err := prepareBatchStateStmts(ctx, tx)
	if err != nil {
		return err
	}
	defer closeBatchStateStmts(prepared)

	stateMap, err := prefetchTaskStates(ctx, tx, changes)
	if err != nil {
		return err
	}

	events, err := applyStateChanges(ctx, changes, stateMap, prepared)
	if err != nil {
		return err
	}
	if err := insertTaskEventsTx(ctx, tx, events); err != nil {
		return fmt.Errorf("insert task events: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit batch state update: %w", err)
	}
	return nil
}

func prepareBatchStateStmts(ctx context.Context, tx *sql.Tx) (batchStmts, error) {
	runningStmt, err := tx.PrepareContext(ctx, `UPDATE tasks SET status = ?, started_at = ? WHERE id = ?`)
	if err != nil {
		return batchStmts{}, fmt.Errorf("prepare running status update: %w", err)
	}
	terminalStmt, err := tx.PrepareContext(ctx, `UPDATE tasks SET status = ?, error = ?, completed_at = ? WHERE id = ?`)
	if err != nil {
		_ = runningStmt.Close()
		return batchStmts{}, fmt.Errorf("prepare terminal status update: %w", err)
	}
	defaultStmt, err := tx.PrepareContext(ctx, `UPDATE tasks SET status = ?, error = ? WHERE id = ?`)
	if err != nil {
		_ = terminalStmt.Close()
		_ = runningStmt.Close()
		return batchStmts{}, fmt.Errorf("prepare default status update: %w", err)
	}
	retryStmt, err := tx.PrepareContext(ctx, `UPDATE tasks SET retry_count = retry_count + 1, status = ? WHERE id = ?`)
	if err != nil {
		_ = defaultStmt.Close()
		_ = terminalStmt.Close()
		_ = runningStmt.Close()
		return batchStmts{}, fmt.Errorf("prepare retry status update: %w", err)
	}
	return batchStmts{running: runningStmt, terminal: terminalStmt, def: defaultStmt, retry: retryStmt}, nil
}

func closeBatchStateStmts(stmts batchStmts) {
	if stmts.running != nil {
		_ = stmts.running.Close()
	}
	if stmts.terminal != nil {
		_ = stmts.terminal.Close()
	}
	if stmts.def != nil {
		_ = stmts.def.Close()
	}
	if stmts.retry != nil {
		_ = stmts.retry.Close()
	}
}

func prefetchTaskStates(ctx context.Context, tx *sql.Tx, changes []TaskStateChange) (map[string]taskState, error) {
	taskIDs := make([]string, len(changes))
	for i, c := range changes {
		taskIDs[i] = c.TaskID
	}
	placeholders := make([]string, len(taskIDs))
	fetchArgs := make([]any, len(taskIDs))
	for i, id := range taskIDs {
		placeholders[i] = "?"
		fetchArgs[i] = id
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, status, batch_id FROM tasks WHERE id IN (`+strings.Join(placeholders, ",")+`)`, fetchArgs...)
	if err != nil {
		return nil, fmt.Errorf("prefetch task states: %w", err)
	}
	defer rows.Close()

	stateMap := make(map[string]taskState, len(changes))
	for rows.Next() {
		var tid string
		var ts taskState
		if err := rows.Scan(&tid, &ts.status, &ts.batchID); err != nil {
			return nil, fmt.Errorf("scan task state: %w", err)
		}
		stateMap[tid] = ts
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task states: %w", err)
	}
	return stateMap, nil
}

func applyStateChanges(ctx context.Context, changes []TaskStateChange, stateMap map[string]taskState, prepared batchStmts) ([]models.TaskLifecycleEvent, error) {
	events := make([]models.TaskLifecycleEvent, 0, len(changes))
	for _, change := range changes {
		event, skip, err := applyStateChange(ctx, change, stateMap, prepared)
		if err != nil {
			return nil, err
		}
		if !skip {
			events = append(events, event)
		}
	}
	return events, nil
}

func (db *DB) BatchUpdateTaskStatus(ctx context.Context, taskIDs []string, status models.TaskStatus, errMsg string) error {
	if len(taskIDs) == 0 {
		return nil
	}
	changes := make([]TaskStateChange, 0, len(taskIDs))
	for _, id := range taskIDs {
		changes = append(changes, TaskStateChange{TaskID: id, Status: status, Error: errMsg})
	}
	return db.BatchApplyTaskStateChanges(ctx, changes)
}
