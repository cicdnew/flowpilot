package database

import (
	"context"
	"database/sql"
	"fmt"

	"flowpilot/internal/models"
)

func (db *DB) CreateVisualBaseline(ctx context.Context, b models.VisualBaseline) error {
	_, err := db.conn.ExecContext(ctx, `INSERT INTO visual_baselines (id, name, task_id, url, screenshot_path, width, height, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.Name, b.TaskID, b.URL, b.ScreenshotPath, b.Width, b.Height, b.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert visual baseline %s: %w", b.ID, err)
	}
	return nil
}

func (db *DB) GetVisualBaseline(ctx context.Context, id string) (*models.VisualBaseline, error) {
	var b models.VisualBaseline
	err := db.readConn.QueryRowContext(ctx, `SELECT id, name, task_id, url, screenshot_path, width, height, created_at FROM visual_baselines WHERE id = ?`, id).
		Scan(&b.ID, &b.Name, &b.TaskID, &b.URL, &b.ScreenshotPath, &b.Width, &b.Height, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("visual baseline %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get visual baseline %s: %w", id, err)
	}
	return &b, nil
}

func (db *DB) ListVisualBaselines(ctx context.Context) ([]models.VisualBaseline, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, name, task_id, url, screenshot_path, width, height, created_at FROM visual_baselines ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query visual baselines: %w", err)
	}
	defer rows.Close()

	var baselines []models.VisualBaseline
	for rows.Next() {
		var b models.VisualBaseline
		if err := rows.Scan(&b.ID, &b.Name, &b.TaskID, &b.URL, &b.ScreenshotPath, &b.Width, &b.Height, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan visual baseline: %w", err)
		}
		baselines = append(baselines, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate visual baselines: %w", err)
	}
	return baselines, nil
}

func (db *DB) DeleteVisualBaseline(ctx context.Context, id string) error {
	res, err := db.conn.ExecContext(ctx, `DELETE FROM visual_baselines WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete visual baseline %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete result for visual baseline %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("visual baseline %s not found", id)
	}
	return nil
}

func (db *DB) CreateVisualDiff(ctx context.Context, d models.VisualDiff) error {
	passed := 0
	if d.Passed {
		passed = 1
	}
	_, err := db.conn.ExecContext(ctx, `INSERT INTO visual_diffs (id, baseline_id, task_id, screenshot_path, diff_image_path, diff_percent, pixel_count, threshold, passed, width, height, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.BaselineID, d.TaskID, d.ScreenshotPath, d.DiffImagePath, d.DiffPercent, d.PixelCount, d.Threshold, passed, d.Width, d.Height, d.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert visual diff %s: %w", d.ID, err)
	}
	return nil
}

func (db *DB) GetVisualDiff(ctx context.Context, id string) (*models.VisualDiff, error) {
	var d models.VisualDiff
	var passedInt int
	err := db.readConn.QueryRowContext(ctx, `SELECT id, baseline_id, task_id, screenshot_path, diff_image_path, diff_percent, pixel_count, threshold, passed, width, height, created_at FROM visual_diffs WHERE id = ?`, id).
		Scan(&d.ID, &d.BaselineID, &d.TaskID, &d.ScreenshotPath, &d.DiffImagePath, &d.DiffPercent, &d.PixelCount, &d.Threshold, &passedInt, &d.Width, &d.Height, &d.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("visual diff %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get visual diff %s: %w", id, err)
	}
	d.Passed = passedInt != 0
	return &d, nil
}

func (db *DB) ListVisualDiffs(ctx context.Context, baselineID string) ([]models.VisualDiff, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, baseline_id, task_id, screenshot_path, diff_image_path, diff_percent, pixel_count, threshold, passed, width, height, created_at FROM visual_diffs WHERE baseline_id = ? ORDER BY created_at DESC`, baselineID)
	if err != nil {
		return nil, fmt.Errorf("query visual diffs for baseline %s: %w", baselineID, err)
	}
	defer rows.Close()

	var diffs []models.VisualDiff
	for rows.Next() {
		var d models.VisualDiff
		var passedInt int
		if err := rows.Scan(&d.ID, &d.BaselineID, &d.TaskID, &d.ScreenshotPath, &d.DiffImagePath, &d.DiffPercent, &d.PixelCount, &d.Threshold, &passedInt, &d.Width, &d.Height, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan visual diff: %w", err)
		}
		d.Passed = passedInt != 0
		diffs = append(diffs, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate visual diffs: %w", err)
	}
	return diffs, nil
}

func (db *DB) ListVisualDiffsByTask(ctx context.Context, taskID string) ([]models.VisualDiff, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, baseline_id, task_id, screenshot_path, diff_image_path, diff_percent, pixel_count, threshold, passed, width, height, created_at FROM visual_diffs WHERE task_id = ? ORDER BY created_at DESC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("query visual diffs for task %s: %w", taskID, err)
	}
	defer rows.Close()

	var diffs []models.VisualDiff
	for rows.Next() {
		var d models.VisualDiff
		var passedInt int
		if err := rows.Scan(&d.ID, &d.BaselineID, &d.TaskID, &d.ScreenshotPath, &d.DiffImagePath, &d.DiffPercent, &d.PixelCount, &d.Threshold, &passedInt, &d.Width, &d.Height, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan visual diff: %w", err)
		}
		d.Passed = passedInt != 0
		diffs = append(diffs, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate visual diffs by task: %w", err)
	}
	return diffs, nil
}

func (db *DB) DeleteVisualDiff(ctx context.Context, id string) error {
	res, err := db.conn.ExecContext(ctx, `DELETE FROM visual_diffs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete visual diff %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete result for visual diff %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("visual diff %s not found", id)
	}
	return nil
}
