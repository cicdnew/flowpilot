package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"flowpilot/internal/models"
	"flowpilot/internal/vision"
)

func pathWithinBase(basePath, targetPath string) bool {
	rel, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func resolveExistingPathWithinBase(baseDir, targetPath string) (string, error) {
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve base dir: %w", err)
	}
	baseResolved, err := filepath.EvalSymlinks(baseAbs)
	if err != nil {
		return "", fmt.Errorf("eval base dir symlinks: %w", err)
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("resolve target path: %w", err)
	}
	targetResolved, err := filepath.EvalSymlinks(targetAbs)
	if err != nil {
		return "", fmt.Errorf("eval target path symlinks: %w", err)
	}
	if !pathWithinBase(baseResolved, targetResolved) {
		return "", fmt.Errorf("path must be within application data directory")
	}
	return targetResolved, nil
}

func resolveNewPathWithinBase(baseDir, targetPath string) (string, error) {
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve base dir: %w", err)
	}
	baseResolved, err := filepath.EvalSymlinks(baseAbs)
	if err != nil {
		return "", fmt.Errorf("eval base dir symlinks: %w", err)
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("resolve target path: %w", err)
	}
	parentResolved, err := filepath.EvalSymlinks(filepath.Dir(targetAbs))
	if err != nil {
		return "", fmt.Errorf("eval target parent symlinks: %w", err)
	}
	if !pathWithinBase(baseResolved, parentResolved) {
		return "", fmt.Errorf("path must be within application data directory")
	}
	if !pathWithinBase(parentResolved, targetAbs) {
		return "", fmt.Errorf("path escapes permitted directory")
	}
	return targetAbs, nil
}

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

	resolvedPath, err := resolveExistingPathWithinBase(a.dataDir, screenshotPath)
	if err != nil {
		return nil, fmt.Errorf("create visual baseline: %w", err)
	}
	f, err := os.Open(resolvedPath)
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
		ScreenshotPath: resolvedPath,
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

	baselinePath, err := resolveExistingPathWithinBase(a.dataDir, baseline.ScreenshotPath)
	if err != nil {
		return nil, fmt.Errorf("compare visual: invalid baseline path: %w", err)
	}

	task, err := a.db.GetTask(a.ctx, req.TaskID)
	if err != nil {
		return nil, fmt.Errorf("compare visual: get task: %w", err)
	}

	if task.Result == nil || len(task.Result.Screenshots) == 0 {
		return nil, fmt.Errorf("compare visual: task %s has no screenshots", req.TaskID)
	}

	screenshotPath, err := resolveExistingPathWithinBase(a.dataDir, task.Result.Screenshots[0])
	if err != nil {
		return nil, fmt.Errorf("compare visual: invalid screenshot path: %w", err)
	}

	diffsDir := filepath.Join(a.dataDir, "diffs")
	if err := os.MkdirAll(diffsDir, 0o700); err != nil {
		return nil, fmt.Errorf("compare visual: create diffs directory: %w", err)
	}

	diffID := uuid.New().String()
	diffOutputPath, err := resolveNewPathWithinBase(diffsDir, filepath.Join(diffsDir, diffID+".png"))
	if err != nil {
		return nil, fmt.Errorf("compare visual: invalid diff output path: %w", err)
	}

	result, err := vision.Compare(baselinePath, screenshotPath, diffOutputPath)
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
