package logs

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"flowpilot/internal/database"
	"flowpilot/internal/models"
)

// Exporter handles JSONL/CSV log exports for tasks and batches.
type Exporter struct {
	db     *database.DB
	output string
}

// NewExporter creates a new log exporter.
func NewExporter(db *database.DB, outputDir string) (*Exporter, error) {
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return nil, fmt.Errorf("create export dir: %w", err)
	}
	return &Exporter{db: db, output: outputDir}, nil
}

func (e *Exporter) ExportTaskLogsZip(ctx context.Context, taskID string) (string, error) {
	stepLogs, err := e.db.ListStepLogs(ctx, taskID)
	if err != nil {
		return "", err
	}
	networkLogs, err := e.db.ListNetworkLogs(ctx, taskID)
	if err != nil {
		return "", err
	}

	zipPath := filepath.Join(e.output, fmt.Sprintf("task_%s_%d.zip", taskID, time.Now().Unix()))
	file, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("create task zip: %w", err)
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	jsonlFile, err := zipWriter.Create(fmt.Sprintf("task_%s.jsonl", taskID))
	if err != nil {
		return "", fmt.Errorf("create zip jsonl: %w", err)
	}
	if err := writeJSONLToWriter(jsonlFile, stepLogs, networkLogs); err != nil {
		return "", err
	}

	csvFile, err := zipWriter.Create(fmt.Sprintf("task_%s.csv", taskID))
	if err != nil {
		return "", fmt.Errorf("create zip csv: %w", err)
	}
	if err := writeCSVToWriter(csvFile, stepLogs, networkLogs); err != nil {
		return "", err
	}

	return zipPath, nil
}

// ExportBatchLogs exports logs for all tasks in a batch and returns the ZIP path.
func (e *Exporter) ExportBatchLogs(ctx context.Context, batchID string) (string, error) {
	tasks, err := e.db.ListTasksByBatch(ctx, batchID)
	if err != nil {
		return "", err
	}
	zipPath := filepath.Join(e.output, fmt.Sprintf("batch_%s_%d.zip", batchID, time.Now().Unix()))
	if err := writeBatchZip(ctx, zipPath, e.db, tasks); err != nil {
		return "", err
	}
	return zipPath, nil
}

func writeJSONL(path string, stepLogs []models.StepLog, networkLogs []models.NetworkLog) (retErr error) {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create jsonl: %w", err)
	}
	defer func() {
		if cErr := file.Close(); cErr != nil && retErr == nil {
			retErr = fmt.Errorf("close jsonl: %w", cErr)
		}
	}()
	enc := json.NewEncoder(file)
	for _, log := range stepLogs {
		if err := enc.Encode(log); err != nil {
			return fmt.Errorf("encode step log: %w", err)
		}
	}
	for _, log := range networkLogs {
		if err := enc.Encode(log); err != nil {
			return fmt.Errorf("encode network log: %w", err)
		}
	}
	return nil
}

func writeCSV(path string, stepLogs []models.StepLog, networkLogs []models.NetworkLog) (retErr error) {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv: %w", err)
	}
	defer func() {
		if cErr := file.Close(); cErr != nil && retErr == nil {
			retErr = fmt.Errorf("close csv: %w", cErr)
		}
	}()

	writer := csv.NewWriter(file)
	if err := writer.Write([]string{"type", "taskId", "stepIndex", "action", "selector", "value", "snapshotId", "errorCode", "errorMsg", "durationMs", "timestamp", "url", "method", "statusCode", "mimeType"}); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for _, log := range stepLogs {
		row := []string{"step", log.TaskID, fmt.Sprintf("%d", log.StepIndex), string(log.Action), log.Selector, log.Value, log.SnapshotID, log.ErrorCode, log.ErrorMsg, fmt.Sprintf("%d", log.DurationMs), log.StartedAt.Format(time.RFC3339), "", "", "", ""}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("write step csv: %w", err)
		}
	}
	for _, log := range networkLogs {
		row := []string{"network", log.TaskID, fmt.Sprintf("%d", log.StepIndex), "", "", "", "", "", log.Error, fmt.Sprintf("%d", log.DurationMs), log.Timestamp.Format(time.RFC3339), log.RequestURL, log.Method, fmt.Sprintf("%d", log.StatusCode), log.MimeType}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("write network csv: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}

func writeBatchZip(ctx context.Context, path string, db *database.DB, tasks []models.Task) (retErr error) {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer func() {
		if cErr := file.Close(); cErr != nil && retErr == nil {
			retErr = fmt.Errorf("close zip file: %w", cErr)
		}
	}()

	zipWriter := zip.NewWriter(file)
	defer func() {
		if cErr := zipWriter.Close(); cErr != nil && retErr == nil {
			retErr = fmt.Errorf("close zip writer: %w", cErr)
		}
	}()

	for _, task := range tasks {
		stepLogs, err := db.ListStepLogs(ctx, task.ID)
		if err != nil {
			return err
		}
		networkLogs, err := db.ListNetworkLogs(ctx, task.ID)
		if err != nil {
			return err
		}

		jsonlName := fmt.Sprintf("task_%s.jsonl", task.ID)
		jsonlFile, err := zipWriter.Create(jsonlName)
		if err != nil {
			return fmt.Errorf("create zip jsonl: %w", err)
		}
		if err := writeJSONLToWriter(jsonlFile, stepLogs, networkLogs); err != nil {
			return err
		}

		csvName := fmt.Sprintf("task_%s.csv", task.ID)
		csvFile, err := zipWriter.Create(csvName)
		if err != nil {
			return fmt.Errorf("create zip csv: %w", err)
		}
		if err := writeCSVToWriter(csvFile, stepLogs, networkLogs); err != nil {
			return err
		}
	}

	return nil
}

func writeJSONLToWriter(writer io.Writer, stepLogs []models.StepLog, networkLogs []models.NetworkLog) error {
	enc := json.NewEncoder(writer)
	for _, log := range stepLogs {
		if err := enc.Encode(log); err != nil {
			return fmt.Errorf("encode step log: %w", err)
		}
	}
	for _, log := range networkLogs {
		if err := enc.Encode(log); err != nil {
			return fmt.Errorf("encode network log: %w", err)
		}
	}
	return nil
}

func writeCSVToWriter(writer io.Writer, stepLogs []models.StepLog, networkLogs []models.NetworkLog) error {
	buffer := &bytes.Buffer{}
	csvWriter := csv.NewWriter(buffer)
	if err := csvWriter.Write([]string{"type", "taskId", "stepIndex", "action", "selector", "value", "snapshotId", "errorCode", "errorMsg", "durationMs", "timestamp", "url", "method", "statusCode", "mimeType"}); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}
	for _, log := range stepLogs {
		row := []string{"step", log.TaskID, fmt.Sprintf("%d", log.StepIndex), string(log.Action), log.Selector, log.Value, log.SnapshotID, log.ErrorCode, log.ErrorMsg, fmt.Sprintf("%d", log.DurationMs), log.StartedAt.Format(time.RFC3339), "", "", "", ""}
		if err := csvWriter.Write(row); err != nil {
			return fmt.Errorf("write step csv: %w", err)
		}
	}
	for _, log := range networkLogs {
		row := []string{"network", log.TaskID, fmt.Sprintf("%d", log.StepIndex), "", "", "", "", "", log.Error, fmt.Sprintf("%d", log.DurationMs), log.Timestamp.Format(time.RFC3339), log.RequestURL, log.Method, fmt.Sprintf("%d", log.StatusCode), log.MimeType}
		if err := csvWriter.Write(row); err != nil {
			return fmt.Errorf("write network csv: %w", err)
		}
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	_, err := writer.Write(buffer.Bytes())
	return err
}
