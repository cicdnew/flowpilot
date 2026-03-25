package models

import "time"

// RecordedFlow represents a reusable automation flow captured from a live session.
type RecordedFlow struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Description   string             `json:"description,omitempty"`
	Steps         []RecordedStep     `json:"steps"`
	OriginURL     string             `json:"originUrl"`
	Timeout       int                `json:"timeout,omitempty"`
	LoggingPolicy *TaskLoggingPolicy `json:"loggingPolicy,omitempty"`
	CreatedAt     time.Time          `json:"createdAt"`
	UpdatedAt     time.Time          `json:"updatedAt"`
}

// RecordedStep is a single captured action from a live recording session,
// enriched with DOM snapshot references and selector alternatives.
type RecordedStep struct {
	Index      int        `json:"index"`
	Action     StepAction `json:"action"`
	Selector   string     `json:"selector,omitempty"`
	Value      string     `json:"value,omitempty"`
	Timeout    int        `json:"timeout,omitempty"`
	SnapshotID string     `json:"snapshotId,omitempty"`
	// SelectorCandidates holds alternative selectors ranked by stability.
	SelectorCandidates []SelectorCandidate `json:"selectorCandidates,omitempty"`
	Timestamp          time.Time           `json:"timestamp"`
}

// SelectorCandidate is a ranked selector option for a recorded step.
type SelectorCandidate struct {
	Selector string       `json:"selector"`
	Strategy SelectorType `json:"strategy"`
	Score    int          `json:"score"` // 1-100, higher is more stable
}

// SelectorType identifies how a selector was derived.
type SelectorType string

const (
	SelectorDataTestID SelectorType = "data-testid"
	SelectorID         SelectorType = "id"
	SelectorRole       SelectorType = "role"
	SelectorCSS        SelectorType = "css"
	SelectorXPath      SelectorType = "xpath"
)

// DOMSnapshot holds a captured DOM state for a single step.
type DOMSnapshot struct {
	ID             string    `json:"id"`
	FlowID         string    `json:"flowId"`
	StepIndex      int       `json:"stepIndex"`
	HTML           string    `json:"html"`
	ScreenshotPath string    `json:"screenshotPath"`
	URL            string    `json:"url"`
	CapturedAt     time.Time `json:"capturedAt"`
}

// ToTaskStep converts a RecordedStep to a TaskStep for execution.
func (rs RecordedStep) ToTaskStep() TaskStep {
	return TaskStep{
		Action:   rs.Action,
		Selector: rs.Selector,
		Value:    rs.Value,
		Timeout:  rs.Timeout,
	}
}

// FlowToTaskSteps converts all steps in a RecordedFlow to TaskStep slice.
func FlowToTaskSteps(flow RecordedFlow) []TaskStep {
	steps := make([]TaskStep, len(flow.Steps))
	for i, rs := range flow.Steps {
		steps[i] = rs.ToTaskStep()
	}
	return steps
}
