package models

import "time"

type Schedule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	CronExpr    string       `json:"cronExpr"`
	FlowID      string       `json:"flowId"`
	URL         string       `json:"url"`
	ProxyConfig ProxyConfig  `json:"proxy"`
	Priority    TaskPriority `json:"priority"`
	Headless    bool         `json:"headless"`
	Tags        []string     `json:"tags,omitempty"`
	Enabled     bool         `json:"enabled"`
	LastRunAt   *time.Time   `json:"lastRunAt,omitempty"`
	NextRunAt   *time.Time   `json:"nextRunAt,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

const MaxSchedules = 100
