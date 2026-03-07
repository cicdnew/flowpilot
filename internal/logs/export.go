package logs

import (
	"archive/zip"
	"bytes"
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

// ExportTaskLogs writes JSONL and CSV files for a task and returns the file paths.
func (e *Exporter) ExportTaskLogs(taskID string) (string, string, error) {
	stepLogs, err := e.db.ListStepLogs(taskID)
	if err != nil {
		return "", "", err
	}
	networkLogs, err := e.db.ListNetworkLogs(taskID)
	if err != nil {
		return "", "", err
	}

	jsonlPath := filepath.Join(e.output, fmt.Sprintf("task_%s_%d.jsonl", taskID, time.Now().Unix()))
	if err := writeJSONL(jsonlPath, stepLogs, networkLogs); err != nil {
		return "", "", err
	}

	csvPath := filepath.Join(e.output, fmt.Sprintf("task_%s_%d.csv", taskID, time.Now().Unix()))
	if err := writeCSV(csvPath, stepLogs, networkLogs); err != nil {
		return "", "", err
	}
	return jsonlPath, csvPath, nil
}

// ExportBatchLogs exports logs for all tasks in a batch and returns the ZIP path.
func (e *Exporter) ExportBatchLogs(batchID string) (string, error) {
	tasks, err := e.db.ListTasksByBatch(batchID)
	if err != nil {
		return "", err
	}
	zipPath := filepath.Join(e.output, fmt.Sprintf("batch_%s_%d.zip", batchID, time.Now().Unix()))
	if err := writeBatchZip(zipPath, e.db, tasks); err != nil {
		return "", err
	}
	return zipPath, nil
}

func writeJSONL(path string, stepLogs []models.StepLog, networkLogs []models.NetworkLog) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create jsonl: %w", err)
	}
	defer file.Close()
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

func writeCSV(path string, stepLogs []models.StepLog, networkLogs []models.NetworkLog) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv: %w", err)
	}
	defer file.Close()

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

func writeBatchZip(path string, db *database.DB, tasks []models.Task) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	for _, task := range tasks {
		stepLogs, err := db.ListStepLogs(task.ID)
		if err != nil {
			return err
		}
		networkLogs, err := db.ListNetworkLogs(task.ID)
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
