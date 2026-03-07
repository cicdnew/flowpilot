package batch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"flowpilot/internal/database"
	"flowpilot/internal/models"
	"flowpilot/internal/validation"

	"github.com/google/uuid"
)

// Engine creates batch tasks from recorded flows.
type Engine struct {
	db *database.DB
}

// New creates a batch engine.
func New(db *database.DB) *Engine {
	return &Engine{db: db}
}

// CreateBatchFromFlow creates tasks from a flow with shared steps and returns the batch ID.
func (e *Engine) CreateBatchFromFlow(ctx context.Context, flow models.RecordedFlow, input models.AdvancedBatchInput) (models.BatchGroup, []models.Task, error) {
	if err := validation.ValidateBatchInput(input); err != nil {
		return models.BatchGroup{}, nil, err
	}

	steps := models.FlowToTaskSteps(flow)
	batchID := uuid.New().String()
	nameTemplate := input.NamingTemplate
	if strings.TrimSpace(nameTemplate) == "" {
		nameTemplate = DefaultNameTemplate()
	}
	if !ValidateTemplate(nameTemplate) {
		return models.BatchGroup{}, nil, fmt.Errorf("invalid naming template")
	}

	created := make([]models.Task, 0, len(input.URLs))
	for i, rawURL := range input.URLs {
		index := i + 1
		vars := TemplateVars{
			URL:    rawURL,
			Domain: ExtractDomain(rawURL),
			Index:  index,
		}
		name := ApplyTemplate(nameTemplate, vars)
		vars.Name = name

		adjustedSteps := make([]models.TaskStep, len(steps))
		for sIdx, step := range steps {
			stepCopy := step
			stepCopy.Value = ApplyTemplate(stepCopy.Value, vars)
			stepCopy.Selector = ApplyTemplate(stepCopy.Selector, vars)
			adjustedSteps[sIdx] = stepCopy
		}

		if len(adjustedSteps) > 0 && adjustedSteps[0].Action == models.ActionNavigate && strings.TrimSpace(adjustedSteps[0].Value) == "" {
			adjustedSteps[0].Value = rawURL
		}

		task := models.Task{
			ID:         uuid.New().String(),
			Name:       name,
			URL:        rawURL,
			Steps:      adjustedSteps,
			Proxy:      input.Proxy,
			Priority:   models.TaskPriority(input.Priority),
			Status:     models.TaskStatusPending,
			MaxRetries: 3,
			Tags:       input.Tags,
			CreatedAt:  time.Now(),
			BatchID:    batchID,
			FlowID:     flow.ID,
			Headless:   input.BatchHeadless(),
		}

		if err := e.db.CreateTask(task); err != nil {
			return models.BatchGroup{}, created, fmt.Errorf("create task %d: %w", len(created), err)
		}
		created = append(created, task)
	}

	group := models.BatchGroup{
		ID:      batchID,
		FlowID:  flow.ID,
		TaskIDs: collectTaskIDs(created),
		Total:   len(created),
		Name:    flow.Name,
	}
	if err := e.db.CreateBatchGroup(group); err != nil {
		return group, created, fmt.Errorf("create batch group: %w", err)
	}

	return group, created, nil
}

func collectTaskIDs(tasks []models.Task) []string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}
