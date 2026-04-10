package database

import (
	"context"
	"encoding/json"
	"fmt"

	"flowpilot/internal/models"
)

func (db *DB) CreateRecordedFlow(ctx context.Context, flow models.RecordedFlow) error {
	stepsJSON, err := json.Marshal(flow.Steps)
	if err != nil {
		return fmt.Errorf("marshal flow steps: %w", err)
	}
	var loggingPolicyJSON string
	if flow.LoggingPolicy != nil {
		b, err := json.Marshal(flow.LoggingPolicy)
		if err != nil {
			return fmt.Errorf("marshal flow logging policy: %w", err)
		}
		loggingPolicyJSON = string(b)
	}
	_, err = db.conn.ExecContext(ctx, `INSERT INTO recorded_flows (id, name, description, steps, origin_url, timeout, logging_policy, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		flow.ID, flow.Name, flow.Description, string(stepsJSON), flow.OriginURL, flow.Timeout, loggingPolicyJSON, flow.CreatedAt, flow.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert recorded flow %s: %w", flow.ID, err)
	}
	return nil
}

func (db *DB) UpdateRecordedFlow(ctx context.Context, flow models.RecordedFlow) error {
	stepsJSON, err := json.Marshal(flow.Steps)
	if err != nil {
		return fmt.Errorf("marshal flow steps: %w", err)
	}
	var loggingPolicyJSON string
	if flow.LoggingPolicy != nil {
		b, err := json.Marshal(flow.LoggingPolicy)
		if err != nil {
			return fmt.Errorf("marshal flow logging policy: %w", err)
		}
		loggingPolicyJSON = string(b)
	}
	res, err := db.conn.ExecContext(ctx, `UPDATE recorded_flows SET name = ?, description = ?, steps = ?, origin_url = ?, timeout = ?, logging_policy = ?, updated_at = ? WHERE id = ?`,
		flow.Name, flow.Description, string(stepsJSON), flow.OriginURL, flow.Timeout, loggingPolicyJSON, flow.UpdatedAt, flow.ID)
	if err != nil {
		return fmt.Errorf("update recorded flow %s: %w", flow.ID, err)
	}
	if rows, err := res.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return fmt.Errorf("check update flow %s: %w", flow.ID, err)
		}
		return fmt.Errorf(errFlowNotFound, flow.ID)
	}
	return nil
}

func (db *DB) GetRecordedFlow(ctx context.Context, id string) (*models.RecordedFlow, error) {
	row := db.readConn.QueryRowContext(ctx, `SELECT id, name, description, steps, origin_url, timeout, logging_policy, created_at, updated_at FROM recorded_flows WHERE id = ?`, id)
	var flow models.RecordedFlow
	var stepsJSON, loggingPolicyJSON string
	if err := row.Scan(&flow.ID, &flow.Name, &flow.Description, &stepsJSON, &flow.OriginURL, &flow.Timeout, &loggingPolicyJSON, &flow.CreatedAt, &flow.UpdatedAt); err != nil {
		return nil, fmt.Errorf("get recorded flow %s: %w", id, err)
	}
	if stepsJSON != "" {
		if err := json.Unmarshal([]byte(stepsJSON), &flow.Steps); err != nil {
			return nil, fmt.Errorf("parse flow steps: %w", err)
		}
	}
	if loggingPolicyJSON != "" {
		var lp models.TaskLoggingPolicy
		if err := json.Unmarshal([]byte(loggingPolicyJSON), &lp); err != nil {
			return nil, fmt.Errorf("parse flow logging policy: %w", err)
		}
		flow.LoggingPolicy = &lp
	}
	return &flow, nil
}

func (db *DB) ListRecordedFlows(ctx context.Context) ([]models.RecordedFlow, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, name, description, steps, origin_url, timeout, logging_policy, created_at, updated_at FROM recorded_flows ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list recorded flows: %w", err)
	}
	defer rows.Close()

	flows := []models.RecordedFlow{}
	for rows.Next() {
		var flow models.RecordedFlow
		var stepsJSON, loggingPolicyJSON string
		if err := rows.Scan(&flow.ID, &flow.Name, &flow.Description, &stepsJSON, &flow.OriginURL, &flow.Timeout, &loggingPolicyJSON, &flow.CreatedAt, &flow.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan recorded flow: %w", err)
		}
		if stepsJSON != "" {
			if err := json.Unmarshal([]byte(stepsJSON), &flow.Steps); err != nil {
				return nil, fmt.Errorf("parse flow steps: %w", err)
			}
		}
		if loggingPolicyJSON != "" {
			var lp models.TaskLoggingPolicy
			if err := json.Unmarshal([]byte(loggingPolicyJSON), &lp); err != nil {
				return nil, fmt.Errorf("parse flow logging policy: %w", err)
			}
			flow.LoggingPolicy = &lp
		}
		flows = append(flows, flow)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recorded flows: %w", err)
	}
	return flows, nil
}

func (db *DB) DeleteRecordedFlow(ctx context.Context, id string) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete flow tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM dom_snapshots WHERE flow_id = ?`, id); err != nil {
		return fmt.Errorf("delete dom snapshots for flow %s: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM websocket_logs WHERE flow_id = ?`, id); err != nil {
		return fmt.Errorf("delete websocket logs for flow %s: %w", id, err)
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM recorded_flows WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete recorded flow %s: %w", id, err)
	}
	if rows, err := res.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return fmt.Errorf("check delete flow %s: %w", id, err)
		}
		return fmt.Errorf(errFlowNotFound, id)
	}
	return tx.Commit()
}

func (db *DB) CreateDOMSnapshot(ctx context.Context, snapshot models.DOMSnapshot) error {
	_, err := db.conn.ExecContext(ctx, `INSERT INTO dom_snapshots (id, flow_id, step_index, html, screenshot_path, url, captured_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		snapshot.ID, snapshot.FlowID, snapshot.StepIndex, snapshot.HTML, snapshot.ScreenshotPath, snapshot.URL, snapshot.CapturedAt)
	if err != nil {
		return fmt.Errorf("insert dom snapshot %s: %w", snapshot.ID, err)
	}
	return nil
}

func (db *DB) ListDOMSnapshots(ctx context.Context, flowID string) ([]models.DOMSnapshot, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, flow_id, step_index, html, screenshot_path, url, captured_at FROM dom_snapshots WHERE flow_id = ? ORDER BY step_index ASC`, flowID)
	if err != nil {
		return nil, fmt.Errorf("list dom snapshots: %w", err)
	}
	defer rows.Close()

	snapshots := []models.DOMSnapshot{}
	for rows.Next() {
		var s models.DOMSnapshot
		if err := rows.Scan(&s.ID, &s.FlowID, &s.StepIndex, &s.HTML, &s.ScreenshotPath, &s.URL, &s.CapturedAt); err != nil {
			return nil, fmt.Errorf("scan dom snapshot: %w", err)
		}
		snapshots = append(snapshots, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dom snapshots: %w", err)
	}
	return snapshots, nil
}
