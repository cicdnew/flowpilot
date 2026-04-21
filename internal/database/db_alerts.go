package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"flowpilot/internal/models"
)

// CreateAlertRule inserts a new alert rule.
func (db *DB) CreateAlertRule(ctx context.Context, rule models.AlertRule) error {
	cooldownSecs := int(rule.Cooldown.Seconds())
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO alert_rules (id, name, description, metric, condition, threshold, window_secs, cooldown_secs, severity, enabled, webhook_url, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.Name, rule.Description, rule.Metric, rule.Cond, rule.Threshold,
		rule.Window, cooldownSecs, rule.Severity, rule.Enabled, rule.WebhookURL, rule.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create alert rule: %w", err)
	}
	return nil
}

// UpdateAlertRule updates an existing rule by ID.
func (db *DB) UpdateAlertRule(ctx context.Context, rule models.AlertRule) error {
	cooldownSecs := int(rule.Cooldown.Seconds())
	_, err := db.conn.ExecContext(ctx, `
		UPDATE alert_rules
		SET name = ?, description = ?, metric = ?, condition = ?, threshold = ?,
		    window_secs = ?, cooldown_secs = ?, severity = ?, enabled = ?, webhook_url = ?
		WHERE id = ?`,
		rule.Name, rule.Description, rule.Metric, rule.Cond, rule.Threshold,
		rule.Window, cooldownSecs, rule.Severity, rule.Enabled, rule.WebhookURL, rule.ID,
	)
	if err != nil {
		return fmt.Errorf("update alert rule: %w", err)
	}
	return nil
}

// DeleteAlertRule removes a rule by ID.
func (db *DB) DeleteAlertRule(ctx context.Context, id string) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM alert_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	return nil
}

// GetAlertRule retrieves a single rule by ID.
func (db *DB) GetAlertRule(ctx context.Context, id string) (*models.AlertRule, error) {
	var rule models.AlertRule
	row := db.conn.QueryRowContext(ctx, `
		SELECT id, name, description, metric, condition, threshold, window_secs, cooldown_secs, severity, enabled, webhook_url, created_at
		FROM alert_rules WHERE id = ?`, id,
	)
	err := row.Scan(
		&rule.ID, &rule.Name, &rule.Description, &rule.Metric, &rule.Cond, &rule.Threshold,
		&rule.Window, &rule.CooldownSecs, &rule.Severity, &rule.Enabled, &rule.WebhookURL, &rule.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get alert rule: %w", err)
	}
	rule.Cooldown = time.Duration(rule.CooldownSecs) * time.Second
	return &rule, nil
}

// ListAlertRules returns all rules ordered by creation time.
func (db *DB) ListAlertRules(ctx context.Context) ([]models.AlertRule, error) {
	var rules []models.AlertRule
	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, name, description, metric, condition, threshold, window_secs, cooldown_secs, severity, enabled, webhook_url, created_at
		FROM alert_rules ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list alert rules query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var rule models.AlertRule
		if err := rows.Scan(
			&rule.ID, &rule.Name, &rule.Description, &rule.Metric, &rule.Cond, &rule.Threshold,
			&rule.Window, &rule.CooldownSecs, &rule.Severity, &rule.Enabled, &rule.WebhookURL, &rule.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert rule: %w", err)
		}
		rule.Cooldown = time.Duration(rule.CooldownSecs) * time.Second
		rules = append(rules, rule)
	}
	return rules, nil
}

// SaveAlertFiring inserts a new alert firing.
func (db *DB) SaveAlertFiring(ctx context.Context, firing models.AlertFiring) error {
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO alert_firings (id, rule_id, rule_name, severity, value, threshold, fired_at, notified)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		firing.ID, firing.RuleID, firing.RuleName, firing.Severity, firing.Value, firing.Threshold,
		firing.FiredAt, firing.Notified,
	)
	if err != nil {
		return fmt.Errorf("save alert firing: %w", err)
	}
	return nil
}

// UpdateAlertFiring marks a firing as resolved by setting resolved_at.
func (db *DB) UpdateAlertFiring(ctx context.Context, firing models.AlertFiring) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE alert_firings SET resolved_at = ? WHERE id = ?`,
		firing.ResolvedAt, firing.ID,
	)
	if err != nil {
		return fmt.Errorf("update alert firing: %w", err)
	}
	return nil
}

// ListAlertFirings returns recent firings for a rule.
func (db *DB) ListAlertFirings(ctx context.Context, ruleID string, limit int) ([]models.AlertFiring, error) {
	var firings []models.AlertFiring
	query := `SELECT * FROM alert_firings`
	args := []interface{}{}

	if ruleID != "" {
		query += ` WHERE rule_id = ?`
		args = append(args, ruleID)
	}
	query += ` ORDER BY fired_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list alert firings query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var f models.AlertFiring
		if err := rows.Scan(
			&f.ID, &f.RuleID, &f.RuleName, &f.Severity, &f.Value, &f.Threshold,
			&f.FiredAt, &f.ResolvedAt, &f.Notified,
		); err != nil {
			return nil, fmt.Errorf("scan alert firing: %w", err)
		}
		firings = append(firings, f)
	}
	return firings, nil
}

// ListActiveAlerts returns all currently firing (non-resolved) alerts.
func (db *DB) ListActiveAlerts(ctx context.Context) ([]models.AlertFiring, error) {
	var firings []models.AlertFiring
	rows, err := db.conn.QueryContext(ctx,
		`SELECT * FROM alert_firings WHERE resolved_at IS NULL ORDER BY fired_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list active alerts query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var f models.AlertFiring
		if err := rows.Scan(
			&f.ID, &f.RuleID, &f.RuleName, &f.Severity, &f.Value, &f.Threshold,
			&f.FiredAt, &f.ResolvedAt, &f.Notified,
		); err != nil {
			return nil, fmt.Errorf("scan active alert: %w", err)
		}
		firings = append(firings, f)
	}
	return firings, nil
}
