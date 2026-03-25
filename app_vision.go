package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"flowpilot/internal/models"
	"flowpilot/internal/vision"
)

func (a *App) CreateVisualBaseline(name, taskID, screenshotPath string) (*models.VisualBaseline, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("create visual baseline: name is required")
	}
	if screenshotPath == "" {
		return nil, fmt.Errorf("create visual baseline: screenshotPath is required")
	}

	f, err := os.Open(screenshotPath)
	if err != nil {
		return nil, fmt.Errorf("create visual baseline: open screenshot: %w", err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("create visual baseline: decode screenshot: %w", err)
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	url := ""
	if taskID != "" {
		task, err := a.db.GetTask(a.ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("create visual baseline: get task: %w", err)
		}
		url = task.URL
	}

	baseline := models.VisualBaseline{
		ID:             uuid.New().String(),
		Name:           name,
		TaskID:         taskID,
		URL:            url,
		ScreenshotPath: screenshotPath,
		Width:          width,
		Height:         height,
		CreatedAt:      time.Now(),
	}

	if err := a.db.CreateVisualBaseline(a.ctx, baseline); err != nil {
		return nil, fmt.Errorf("create visual baseline: %w", err)
	}

	return &baseline, nil
}

func (a *App) ListVisualBaselines() ([]models.VisualBaseline, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.db.ListVisualBaselines(a.ctx)
}

func (a *App) DeleteVisualBaseline(id string) error {
	if err := a.ready(); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("delete visual baseline: id is required")
	}
	return a.db.DeleteVisualBaseline(a.ctx, id)
}

func (a *App) CompareVisual(req models.DiffRequest) (*models.VisualDiff, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if req.BaselineID == "" {
		return nil, fmt.Errorf("compare visual: baselineId is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("compare visual: taskId is required")
	}

	baseline, err := a.db.GetVisualBaseline(a.ctx, req.BaselineID)
	if err != nil {
		return nil, fmt.Errorf("compare visual: %w", err)
	}

	task, err := a.db.GetTask(a.ctx, req.TaskID)
	if err != nil {
		return nil, fmt.Errorf("compare visual: get task: %w", err)
	}

	if task.Result == nil || len(task.Result.Screenshots) == 0 {
		return nil, fmt.Errorf("compare visual: task %s has no screenshots", req.TaskID)
	}

	screenshotPath := task.Result.Screenshots[0]

	diffsDir := filepath.Join(a.dataDir, "diffs")
	if err := os.MkdirAll(diffsDir, 0o700); err != nil {
		return nil, fmt.Errorf("compare visual: create diffs directory: %w", err)
	}

	diffID := uuid.New().String()
	diffOutputPath := filepath.Join(diffsDir, diffID+".png")

	result, err := vision.Compare(baseline.ScreenshotPath, screenshotPath, diffOutputPath)
	if err != nil {
		return nil, fmt.Errorf("compare visual: %w", err)
	}

	threshold := req.Threshold
	if threshold <= 0 {
		threshold = 5.0
	}

	diff := models.VisualDiff{
		ID:             diffID,
		BaselineID:     req.BaselineID,
		TaskID:         req.TaskID,
		ScreenshotPath: screenshotPath,
		DiffImagePath:  diffOutputPath,
		DiffPercent:    result.DiffPercent,
		PixelCount:     result.PixelCount,
		Threshold:      threshold,
		Passed:         result.DiffPercent <= threshold,
		Width:          result.Width,
		Height:         result.Height,
		CreatedAt:      time.Now(),
	}

	if err := a.db.CreateVisualDiff(a.ctx, diff); err != nil {
		return nil, fmt.Errorf("compare visual: save diff: %w", err)
	}

	return &diff, nil
}

func (a *App) ListVisualDiffs(baselineID string) ([]models.VisualDiff, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if baselineID == "" {
		return nil, fmt.Errorf("list visual diffs: baselineId is required")
	}
	return a.db.ListVisualDiffs(a.ctx, baselineID)
}

func (a *App) ListVisualDiffsByTask(taskID string) ([]models.VisualDiff, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if taskID == "" {
		return nil, fmt.Errorf("list visual diffs by task: taskId is required")
	}
	return a.db.ListVisualDiffsByTask(a.ctx, taskID)
}

func (a *App) GetVisualDiff(id string) (*models.VisualDiff, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if id == "" {
		return nil, fmt.Errorf("get visual diff: id is required")
	}
	return a.db.GetVisualDiff(a.ctx, id)
}
