package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"flowpilot/internal/crypto"
	"flowpilot/internal/models"
)

func (db *DB) scanSchedule(row scanner) (*models.Schedule, error) {
	var s models.Schedule
	var tagsJSON string
	var headlessInt, enabledInt int
	var lastRunAt, nextRunAt sql.NullTime

	err := row.Scan(
		&s.ID, &s.Name, &s.CronExpr, &s.FlowID, &s.URL,
		&s.ProxyConfig.Server, &s.ProxyConfig.Username, &s.ProxyConfig.Password,
		&s.ProxyConfig.Geo, &s.ProxyConfig.Protocol,
		&s.Priority, &headlessInt, &tagsJSON, &enabledInt,
		&lastRunAt, &nextRunAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	s.Headless = headlessInt != 0
	s.Enabled = enabledInt != 0

	if lastRunAt.Valid {
		s.LastRunAt = &lastRunAt.Time
	}
	if nextRunAt.Valid {
		s.NextRunAt = &nextRunAt.Time
	}

	if tagsJSON != "" {
		if err := json.Unmarshal([]byte(tagsJSON), &s.Tags); err != nil {
			return nil, fmt.Errorf("parse schedule tags JSON: %w", err)
		}
	}

	if s.ProxyConfig.Username != "" {
		decUser, err := crypto.Decrypt(s.ProxyConfig.Username)
		if err != nil {
			return nil, fmt.Errorf("decrypt schedule proxy username for %s: %w", s.ID, err)
		}
		s.ProxyConfig.Username = decUser
	}
	if s.ProxyConfig.Password != "" {
		decPass, err := crypto.Decrypt(s.ProxyConfig.Password)
		if err != nil {
			return nil, fmt.Errorf("decrypt schedule proxy password for %s: %w", s.ID, err)
		}
		s.ProxyConfig.Password = decPass
	}

	return &s, nil
}

func (db *DB) CreateSchedule(ctx context.Context, s models.Schedule) error {
	tagsJSON, err := json.Marshal(s.Tags)
	if err != nil {
		return fmt.Errorf("marshal schedule tags: %w", err)
	}

	encUsername, err := crypto.Encrypt(s.ProxyConfig.Username)
	if err != nil {
		return fmt.Errorf("encrypt schedule proxy username: %w", err)
	}
	encPassword, err := crypto.Encrypt(s.ProxyConfig.Password)
	if err != nil {
		return fmt.Errorf("encrypt schedule proxy password: %w", err)
	}

	headless := 1
	if !s.Headless {
		headless = 0
	}
	enabled := 1
	if !s.Enabled {
		enabled = 0
	}

	_, err = db.conn.ExecContext(ctx, `
		INSERT INTO schedules (id, name, cron_expr, flow_id, url, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol, priority, headless, tags, enabled, last_run_at, next_run_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.CronExpr, s.FlowID, s.URL,
		s.ProxyConfig.Server, encUsername, encPassword, s.ProxyConfig.Geo, string(s.ProxyConfig.Protocol),
		s.Priority, headless, string(tagsJSON), enabled,
		s.LastRunAt, s.NextRunAt, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert schedule %s: %w", s.ID, err)
	}
	return nil
}

func (db *DB) GetSchedule(ctx context.Context, id string) (*models.Schedule, error) {
	row := db.readConn.QueryRowContext(ctx, `
		SELECT id, name, cron_expr, flow_id, url, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol, priority, headless, tags, enabled, last_run_at, next_run_at, created_at, updated_at
		FROM schedules WHERE id = ?`, id)
	s, err := db.scanSchedule(row)
	if err != nil {
		return nil, fmt.Errorf("get schedule %s: %w", id, err)
	}
	return s, nil
}

func (db *DB) ListSchedules(ctx context.Context) ([]models.Schedule, error) {
	rows, err := db.readConn.QueryContext(ctx, `
		SELECT id, name, cron_expr, flow_id, url, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol, priority, headless, tags, enabled, last_run_at, next_run_at, created_at, updated_at
		FROM schedules ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query schedules: %w", err)
	}
	defer rows.Close()

	var schedules []models.Schedule
	for rows.Next() {
		s, err := db.scanSchedule(rows)
		if err != nil {
			return nil, fmt.Errorf("scan schedule row: %w", err)
		}
		schedules = append(schedules, *s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schedules: %w", err)
	}
	return schedules, nil
}

func (db *DB) UpdateSchedule(ctx context.Context, s models.Schedule) error {
	tagsJSON, err := json.Marshal(s.Tags)
	if err != nil {
		return fmt.Errorf("marshal schedule tags: %w", err)
	}

	encUsername, err := crypto.Encrypt(s.ProxyConfig.Username)
	if err != nil {
		return fmt.Errorf("encrypt schedule proxy username: %w", err)
	}
	encPassword, err := crypto.Encrypt(s.ProxyConfig.Password)
	if err != nil {
		return fmt.Errorf("encrypt schedule proxy password: %w", err)
	}

	headless := 1
	if !s.Headless {
		headless = 0
	}
	enabled := 1
	if !s.Enabled {
		enabled = 0
	}

	res, err := db.conn.ExecContext(ctx, `
		UPDATE schedules SET name = ?, cron_expr = ?, flow_id = ?, url = ?, proxy_server = ?, proxy_username = ?, proxy_password = ?, proxy_geo = ?, proxy_protocol = ?, priority = ?, headless = ?, tags = ?, enabled = ?, next_run_at = ?, updated_at = ?
		WHERE id = ?`,
		s.Name, s.CronExpr, s.FlowID, s.URL,
		s.ProxyConfig.Server, encUsername, encPassword, s.ProxyConfig.Geo, string(s.ProxyConfig.Protocol),
		s.Priority, headless, string(tagsJSON), enabled, s.NextRunAt, s.UpdatedAt,
		s.ID,
	)
	if err != nil {
		return fmt.Errorf("update schedule %s: %w", s.ID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update result for schedule %s: %w", s.ID, err)
	}
	if n == 0 {
		return fmt.Errorf("schedule %s not found", s.ID)
	}
	return nil
}

func (db *DB) DeleteSchedule(ctx context.Context, id string) error {
	res, err := db.conn.ExecContext(ctx, `DELETE FROM schedules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete schedule %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete result for schedule %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("schedule %s not found", id)
	}
	return nil
}

func (db *DB) ListDueSchedules(ctx context.Context, now time.Time) ([]models.Schedule, error) {
	rows, err := db.readConn.QueryContext(ctx, `
		SELECT id, name, cron_expr, flow_id, url, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol, priority, headless, tags, enabled, last_run_at, next_run_at, created_at, updated_at
		FROM schedules WHERE enabled = 1 AND next_run_at <= ?
		ORDER BY next_run_at ASC`, now)
	if err != nil {
		return nil, fmt.Errorf("query due schedules: %w", err)
	}
	defer rows.Close()

	var schedules []models.Schedule
	for rows.Next() {
		s, err := db.scanSchedule(rows)
		if err != nil {
			log.Printf("skip invalid due schedule row: %v", err)
			continue
		}
		schedules = append(schedules, *s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due schedules: %w", err)
	}
	return schedules, nil
}

func (db *DB) UpdateScheduleRun(ctx context.Context, id string, lastRun, nextRun time.Time) error {
	res, err := db.conn.ExecContext(ctx, `
		UPDATE schedules SET last_run_at = ?, next_run_at = ?, updated_at = ?
		WHERE id = ?`, lastRun, nextRun, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update schedule run %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update run result for schedule %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("schedule %s not found", id)
	}
	return nil
}
