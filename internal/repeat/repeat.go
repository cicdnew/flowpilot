package repeat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"flowpilot/internal/database"
	"flowpilot/internal/models"

	"github.com/google/uuid"
)

// Engine creates repeated task instances with parameter substitution.
type Engine struct {
	db *database.DB
}

// New creates a repeat engine.
func New(db *database.DB) *Engine {
	return &Engine{db: db}
}

// applyRepeatVar replaces the repeat variable in a string.
func applyRepeatVar(template, varName, value string, index int) string {
	// Support both {{varName}} and {{index}} patterns
	result := strings.ReplaceAll(template, "{{"+varName+"}}", value)
	result = strings.ReplaceAll(result, "{{index}}", fmt.Sprintf("%d", index))
	return result
}

// CreateRepeatedTasks creates multiple task instances from a template with parameter substitution.
func (e *Engine) CreateRepeatedTasks(ctx context.Context, input models.RepeatTaskInput) (models.BatchGroup, []models.Task, error) {
	if err := input.Repeat.Validate(); err != nil {
		return models.BatchGroup{}, nil, fmt.Errorf("invalid repeat config: %w", err)
	}

	values := input.Repeat.GenerateValues()
	if len(values) == 0 {
		return models.BatchGroup{}, nil, fmt.Errorf("no values generated from repeat config")
	}

	batchID := uuid.New().String()

	tx, err := e.db.BeginTx(ctx)
	if err != nil {
		return models.BatchGroup{}, nil, fmt.Errorf("begin repeat tx: %w", err)
	}
	defer tx.Rollback()

	created := make([]models.Task, 0, len(values))

	for i, value := range values {
		index := i + 1
		varName := input.Repeat.VarName

		// Apply substitution to name
		taskName := applyRepeatVar(input.Name, varName, value, index)

		// Apply substitution to URL
		taskURL := applyRepeatVar(input.URL, varName, value, index)

		// Apply substitution to all step fields
		adjustedSteps := make([]models.TaskStep, len(input.Steps))
		for sIdx, step := range input.Steps {
			stepCopy := step
			stepCopy.Value = applyRepeatVar(stepCopy.Value, varName, value, index)
			stepCopy.Selector = applyRepeatVar(stepCopy.Selector, varName, value, index)
			adjustedSteps[sIdx] = stepCopy
		}

		headless := true
		if input.Headless != nil {
			headless = *input.Headless
		}

		task := models.Task{
			ID:            uuid.New().String(),
			Name:          taskName,
			URL:           taskURL,
			Steps:         adjustedSteps,
			Proxy:         input.ProxyConfig,
			Priority:      models.TaskPriority(input.Priority),
			Status:        models.TaskStatusPending,
			MaxRetries:    models.DefaultMaxRetries,
			Tags:          input.Tags,
			CreatedAt:     time.Now(),
			BatchID:       batchID,
			Headless:      headless,
			Timeout:       input.Timeout,
			LoggingPolicy: input.LoggingPolicy,
		}

		if err := e.db.CreateTaskTx(ctx, tx, task); err != nil {
			return models.BatchGroup{}, nil, fmt.Errorf("create task %d: %w", i, err)
		}
		created = append(created, task)
	}

	group := models.BatchGroup{
		ID:        batchID,
		FlowID:    "", // Not from a flow
		TaskIDs:   collectTaskIDs(created),
		Total:     len(created),
		Name:      fmt.Sprintf("%s (repeated %d times)", input.Name, len(values)),
		CreatedAt: time.Now(),
	}

	if err := e.db.CreateBatchGroupTx(ctx, tx, group); err != nil {
		return models.BatchGroup{}, nil, fmt.Errorf("create batch group: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return models.BatchGroup{}, nil, fmt.Errorf("commit repeat tx: %w", err)
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
