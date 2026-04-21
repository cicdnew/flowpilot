package models

import "time"

// AlertSeverity defines the importance level of an alert.
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// AlertRule represents a persisted alert configuration.
type AlertRule struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Metric       string        `json:"metric"`
	Cond         string        `json:"condition"` // "gt", "lt", "rate_gt"
	Threshold    float64       `json:"threshold"`
	Window       int           `json:"window_secs"`
	Cooldown     time.Duration `json:"cooldown"`
	CooldownSecs int           `json:"-"` // Temporary field for database scanning
	Severity     AlertSeverity `json:"severity"`
	Enabled      bool          `json:"enabled"`
	WebhookURL   string        `json:"webhook_url,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
}

// AlertFiring represents a single firing of an alert rule.
type AlertFiring struct {
	ID         string        `json:"id"`
	RuleID     string        `json:"rule_id"`
	RuleName   string        `json:"rule_name"`
	Severity   AlertSeverity `json:"severity"`
	Value      float64       `json:"current_value"`
	Threshold  float64       `json:"threshold"`
	FiredAt    time.Time     `json:"fired_at"`
	ResolvedAt *time.Time    `json:"resolved_at,omitempty"`
	Notified   bool          `json:"notified"`
}
