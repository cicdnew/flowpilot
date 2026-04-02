package logs

import (
	"archive/zip"
	"context"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"flowpilot/internal/crypto"
	"flowpilot/internal/database"
	"flowpilot/internal/models"
)

func TestWriteJSONLToWriter(t *testing.T) {
	stepLogs := []models.StepLog{
		{TaskID: "task-1", StepIndex: 0, Action: models.ActionNavigate, Value: "https://example.com", DurationMs: 100, StartedAt: time.Now()},
		{TaskID: "task-1", StepIndex: 1, Action: models.ActionClick, Selector: "#btn", DurationMs: 50, StartedAt: time.Now()},
	}
	networkLogs := []models.NetworkLog{
		{TaskID: "task-1", StepIndex: 0, RequestURL: "https://example.com", Method: "GET", StatusCode: 200, DurationMs: 150, Timestamp: time.Now()},
	}

	var buf bytes.Buffer
	err := writeJSONLToWriter(&buf, stepLogs, networkLogs)
	if err != nil {
		t.Fatalf("writeJSONLToWriter: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	var step models.StepLog
	if err := json.Unmarshal([]byte(lines[0]), &step); err != nil {
		t.Fatalf("parse first line: %v", err)
	}
	if step.TaskID != "task-1" {
		t.Errorf("step.TaskID: got %q, want %q", step.TaskID, "task-1")
	}
	if step.Action != models.ActionNavigate {
		t.Errorf("step.Action: got %q, want %q", step.Action, models.ActionNavigate)
	}

	var net models.NetworkLog
	if err := json.Unmarshal([]byte(lines[2]), &net); err != nil {
		t.Fatalf("parse third line: %v", err)
	}
	if net.Method != "GET" {
		t.Errorf("net.Method: got %q, want GET", net.Method)
	}
}

func TestWriteJSONLToWriterEmpty(t *testing.T) {
	var buf bytes.Buffer
	err := writeJSONLToWriter(&buf, nil, nil)
	if err != nil {
		t.Fatalf("writeJSONLToWriter(empty): %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %d bytes", buf.Len())
	}
}

func TestWriteCSVToWriter(t *testing.T) {
	stepLogs := []models.StepLog{
		{TaskID: "task-1", StepIndex: 0, Action: models.ActionNavigate, Value: "https://example.com", DurationMs: 100, StartedAt: time.Now()},
	}
	networkLogs := []models.NetworkLog{
		{TaskID: "task-1", StepIndex: 0, RequestURL: "https://example.com", Method: "GET", StatusCode: 200, DurationMs: 150, Timestamp: time.Now()},
	}

	var buf bytes.Buffer
	err := writeCSVToWriter(&buf, stepLogs, networkLogs)
	if err != nil {
		t.Fatalf("writeCSVToWriter: %v", err)
	}

	reader := csv.NewReader(bytes.NewReader(buf.Bytes()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 CSV rows (header + 2 data), got %d", len(records))
	}

	header := records[0]
	if header[0] != "type" {
		t.Errorf("header[0]: got %q, want %q", header[0], "type")
	}

	stepRow := records[1]
	if stepRow[0] != "step" {
		t.Errorf("step row type: got %q, want %q", stepRow[0], "step")
	}
	if stepRow[1] != "task-1" {
		t.Errorf("step row taskId: got %q, want %q", stepRow[1], "task-1")
	}

	netRow := records[2]
	if netRow[0] != "network" {
		t.Errorf("network row type: got %q, want %q", netRow[0], "network")
	}
}

func TestWriteCSVToWriterEmpty(t *testing.T) {
	var buf bytes.Buffer
	err := writeCSVToWriter(&buf, nil, nil)
	if err != nil {
		t.Fatalf("writeCSVToWriter(empty): %v", err)
	}

	reader := csv.NewReader(bytes.NewReader(buf.Bytes()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("expected 1 row (header only), got %d", len(records))
	}
}

func TestWriteJSONL(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.jsonl"

	stepLogs := []models.StepLog{
		{TaskID: "task-1", StepIndex: 0, Action: models.ActionClick, Selector: "#btn", DurationMs: 42, StartedAt: time.Now()},
	}
	networkLogs := []models.NetworkLog{
		{TaskID: "task-1", StepIndex: 0, RequestURL: "https://example.com", Method: "POST", StatusCode: 201, DurationMs: 99, Timestamp: time.Now()},
	}

	err := writeJSONL(path, stepLogs, networkLogs)
	if err != nil {
		t.Fatalf("writeJSONL: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestWriteCSV(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.csv"

	stepLogs := []models.StepLog{
		{TaskID: "task-1", StepIndex: 0, Action: models.ActionNavigate, Value: "https://example.com", DurationMs: 100, StartedAt: time.Now()},
	}

	err := writeCSV(path, stepLogs, nil)
	if err != nil {
		t.Fatalf("writeCSV: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 rows (header + 1 data), got %d", len(records))
	}
}

func TestWriteJSONLInvalidPath(t *testing.T) {
	err := writeJSONL("/nonexistent/dir/file.jsonl", nil, nil)
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestWriteCSVInvalidPath(t *testing.T) {
	err := writeCSV("/nonexistent/dir/file.csv", nil, nil)
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestNewExporter(t *testing.T) {
	dir := t.TempDir()
	_, err := NewExporter(nil, dir+"/exports")
	if err != nil {
		t.Fatalf("NewExporter: %v", err)
	}

	if _, statErr := os.Stat(dir + "/exports"); os.IsNotExist(statErr) {
		t.Error("export directory should be created")
	}
}

func TestNewExporterCreatesNestedDir(t *testing.T) {
	dir := t.TempDir()
	nested := dir + "/a/b/c/exports"
	_, err := NewExporter(nil, nested)
	if err != nil {
		t.Fatalf("NewExporter: %v", err)
	}

	if _, statErr := os.Stat(nested); os.IsNotExist(statErr) {
		t.Error("nested export directory should be created")
	}
}

func TestWriteCSVToWriterStepAndNetworkRows(t *testing.T) {
	stepLogs := []models.StepLog{
		{TaskID: "t1", StepIndex: 0, Action: models.ActionClick, Selector: "#a", SnapshotID: "snap-1", ErrorCode: "TIMEOUT", ErrorMsg: "timed out", DurationMs: 200, StartedAt: time.Now()},
	}
	networkLogs := []models.NetworkLog{
		{TaskID: "t1", StepIndex: 1, RequestURL: "https://api.test.com", Method: "PUT", StatusCode: 204, MimeType: "application/json", Error: "partial", DurationMs: 80, Timestamp: time.Now()},
	}

	var buf bytes.Buffer
	err := writeCSVToWriter(&buf, stepLogs, networkLogs)
	if err != nil {
		t.Fatalf("writeCSVToWriter: %v", err)
	}

	reader := csv.NewReader(bytes.NewReader(buf.Bytes()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(records))
	}

	stepRow := records[1]
	if stepRow[4] != "#a" {
		t.Errorf("selector: got %q, want #a", stepRow[4])
	}
	if stepRow[6] != "snap-1" {
		t.Errorf("snapshotId: got %q, want snap-1", stepRow[6])
	}
	if stepRow[7] != "TIMEOUT" {
		t.Errorf("errorCode: got %q, want TIMEOUT", stepRow[7])
	}
	if stepRow[8] != "timed out" {
		t.Errorf("errorMsg: got %q, want 'timed out'", stepRow[8])
	}

	netRow := records[2]
	if netRow[11] != "https://api.test.com" {
		t.Errorf("url: got %q, want https://api.test.com", netRow[11])
	}
	if netRow[12] != "PUT" {
		t.Errorf("method: got %q, want PUT", netRow[12])
	}
	if netRow[14] != "application/json" {
		t.Errorf("mimeType: got %q, want application/json", netRow[14])
	}
}

func TestWriteJSONLEmptyLists(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/empty.jsonl"

	err := writeJSONL(path, []models.StepLog{}, []models.NetworkLog{})
	if err != nil {
		t.Fatalf("writeJSONL(empty): %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(data))
	}
}

func TestWriteCSVEmptyLists(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/empty.csv"

	err := writeCSV(path, []models.StepLog{}, []models.NetworkLog{})
	if err != nil {
		t.Fatalf("writeCSV(empty): %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("expected 1 row (header only), got %d", len(records))
	}
}

func TestWriteCSVMultipleNetworkLogs(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/multi_net.csv"

	networkLogs := []models.NetworkLog{
		{TaskID: "t1", StepIndex: 0, RequestURL: "https://a.com", Method: "GET", StatusCode: 200, MimeType: "text/html", DurationMs: 100, Timestamp: time.Now()},
		{TaskID: "t1", StepIndex: 1, RequestURL: "https://b.com", Method: "POST", StatusCode: 201, MimeType: "application/json", Error: "partial error", DurationMs: 200, Timestamp: time.Now()},
	}

	err := writeCSV(path, nil, networkLogs)
	if err != nil {
		t.Fatalf("writeCSV: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 rows (header + 2 data), got %d", len(records))
	}
	if records[1][0] != "network" {
		t.Errorf("first data row type: got %q, want network", records[1][0])
	}
	if records[2][12] != "POST" {
		t.Errorf("second data row method: got %q, want POST", records[2][12])
	}
}

func setupTestDB(t *testing.T) *database.DB {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	crypto.ResetForTest()
	if err := crypto.InitKeyWithBytes(key); err != nil {
		t.Fatalf("init crypto key: %v", err)
	}
	t.Cleanup(func() { crypto.ResetForTest() })

	dir := t.TempDir()
	db, err := database.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestExportBatchLogs(t *testing.T) {
	db := setupTestDB(t)
	dir := t.TempDir()

	for i := 0; i < 2; i++ {
		task := models.Task{
			ID:        "batch-export-" + string(rune('0'+i)),
			Name:      "Batch Export Task",
			URL:       "https://example.com",
			Status:    models.TaskStatusCompleted,
			BatchID:   "batch-export-1",
			CreatedAt: time.Now(),
		}
		if err := db.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}

		stepLogs := []models.StepLog{
			{TaskID: task.ID, StepIndex: 0, Action: models.ActionClick, Selector: "#btn", DurationMs: 50, StartedAt: time.Now()},
		}
		if err := db.InsertStepLogs(context.Background(), task.ID, stepLogs); err != nil {
			t.Fatalf("InsertStepLogs %d: %v", i, err)
		}
	}

	exporter, err := NewExporter(db, filepath.Join(dir, "exports"))
	if err != nil {
		t.Fatalf("NewExporter: %v", err)
	}

	zipPath, err := exporter.ExportBatchLogs(context.Background(), "batch-export-1")
	if err != nil {
		t.Fatalf("ExportBatchLogs: %v", err)
	}

	if zipPath == "" {
		t.Error("zipPath should not be empty")
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()

	if len(r.File) != 4 {
		t.Errorf("expected 4 files in zip (2 JSONL + 2 CSV), got %d", len(r.File))
	}
}

func TestExportBatchLogsEmpty(t *testing.T) {
	db := setupTestDB(t)
	dir := t.TempDir()

	exporter, err := NewExporter(db, filepath.Join(dir, "exports"))
	if err != nil {
		t.Fatalf("NewExporter: %v", err)
	}

	zipPath, err := exporter.ExportBatchLogs(context.Background(), "nonexistent-batch")
	if err != nil {
		t.Fatalf("ExportBatchLogs: %v", err)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()

	if len(r.File) != 0 {
		t.Errorf("expected 0 files in zip for empty batch, got %d", len(r.File))
	}
}

func TestWriteJSONLToWriterMultiple(t *testing.T) {
	steps := make([]models.StepLog, 5)
	for i := range steps {
		steps[i] = models.StepLog{
			TaskID:     "task-multi",
			StepIndex:  i,
			Action:     models.ActionClick,
			DurationMs: 10,
			StartedAt:  time.Now(),
		}
	}
	nets := make([]models.NetworkLog, 3)
	for i := range nets {
		nets[i] = models.NetworkLog{
			TaskID:     "task-multi",
			StepIndex:  i,
			RequestURL: "https://example.com",
			Method:     "GET",
			StatusCode: 200,
			Timestamp:  time.Now(),
		}
	}

	var buf bytes.Buffer
	err := writeJSONLToWriter(&buf, steps, nets)
	if err != nil {
		t.Fatalf("writeJSONLToWriter: %v", err)
	}

	decoder := json.NewDecoder(&buf)
	count := 0
	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("decode line %d: %v", count, err)
		}
		count++
	}

	if count != 8 {
		t.Errorf("expected 8 JSON lines (5 steps + 3 nets), got %d", count)
	}
}

func TestNewExporterInvalidPath(t *testing.T) {
	// /proc/... is a read-only filesystem on Linux, MkdirAll should fail
	_, err := NewExporter(nil, "/proc/fakedir/exports")
	if err == nil {
		t.Error("expected error for unwritable path")
	}
}

func TestWriteBatchZipInvalidPath(t *testing.T) {
	tasks := []models.Task{{ID: "zip-err-1"}}
	err := writeBatchZip(context.Background(), "/proc/fakedir/batch.zip", nil, tasks)
	if err == nil {
		t.Error("expected error for unwritable zip path")
	}
}

func TestExportBatchLogsVerifyZipContents(t *testing.T) {
	db := setupTestDB(t)
	dir := t.TempDir()

	task := models.Task{
		ID:        "batch-verify-1",
		Name:      "Batch Verify",
		URL:       "https://example.com",
		Status:    models.TaskStatusCompleted,
		BatchID:   "batch-verify",
		CreatedAt: time.Now(),
	}
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	stepLogs := []models.StepLog{
		{TaskID: "batch-verify-1", StepIndex: 0, Action: models.ActionClick, Selector: "#go", DurationMs: 30, StartedAt: time.Now()},
	}
	if err := db.InsertStepLogs(context.Background(), "batch-verify-1", stepLogs); err != nil {
		t.Fatalf("InsertStepLogs: %v", err)
	}

	networkLogs := []models.NetworkLog{
		{TaskID: "batch-verify-1", StepIndex: 0, RequestURL: "https://example.com/api", Method: "GET", StatusCode: 200, DurationMs: 100, Timestamp: time.Now()},
	}
	if err := db.InsertNetworkLogs(context.Background(), "batch-verify-1", networkLogs); err != nil {
		t.Fatalf("InsertNetworkLogs: %v", err)
	}

	exporter, err := NewExporter(db, filepath.Join(dir, "exports"))
	if err != nil {
		t.Fatalf("NewExporter: %v", err)
	}

	zipPath, err := exporter.ExportBatchLogs(context.Background(), "batch-verify")
	if err != nil {
		t.Fatalf("ExportBatchLogs: %v", err)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()

	if len(r.File) != 2 {
		t.Fatalf("expected 2 files in zip (1 JSONL + 1 CSV), got %d", len(r.File))
	}

	// Verify JSONL content inside zip
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, ".jsonl") {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open zip entry: %v", err)
			}
			data, _ := io.ReadAll(rc)
			rc.Close()
			if len(data) == 0 {
				t.Error("JSONL zip entry should not be empty")
			}
		}
		if strings.HasSuffix(f.Name, ".csv") {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open zip entry: %v", err)
			}
			data, _ := io.ReadAll(rc)
			rc.Close()
			if len(data) == 0 {
				t.Error("CSV zip entry should not be empty")
			}
		}
	}
}

func TestExportTaskLogsZip(t *testing.T) {
	db := setupTestDB(t)
	dir := t.TempDir()

	task := models.Task{
		ID:        "zip-task-1",
		Name:      "Zip Export Test",
		URL:       "https://example.com",
		Status:    models.TaskStatusCompleted,
		CreatedAt: time.Now(),
	}
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	stepLogs := []models.StepLog{
		{TaskID: "zip-task-1", StepIndex: 0, Action: models.ActionNavigate, Value: "https://example.com", DurationMs: 100, StartedAt: time.Now()},
	}
	if err := db.InsertStepLogs(context.Background(), "zip-task-1", stepLogs); err != nil {
		t.Fatalf("InsertStepLogs: %v", err)
	}

	networkLogs := []models.NetworkLog{
		{TaskID: "zip-task-1", StepIndex: 0, RequestURL: "https://example.com", Method: "GET", StatusCode: 200, DurationMs: 50, Timestamp: time.Now()},
	}
	if err := db.InsertNetworkLogs(context.Background(), "zip-task-1", networkLogs); err != nil {
		t.Fatalf("InsertNetworkLogs: %v", err)
	}

	exporter, err := NewExporter(db, filepath.Join(dir, "exports"))
	if err != nil {
		t.Fatalf("NewExporter: %v", err)
	}

	zipPath, err := exporter.ExportTaskLogsZip(context.Background(), "zip-task-1")
	if err != nil {
		t.Fatalf("ExportTaskLogsZip: %v", err)
	}

	if zipPath == "" {
		t.Fatal("zipPath should not be empty")
	}
	if !strings.HasSuffix(zipPath, ".zip") {
		t.Errorf("expected .zip extension, got %q", zipPath)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()

	if len(r.File) != 2 {
		t.Fatalf("expected 2 files in zip (1 JSONL + 1 CSV), got %d", len(r.File))
	}

	var hasJSONL, hasCSV bool
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %q: %v", f.Name, err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		if len(data) == 0 {
			t.Errorf("zip entry %q should not be empty", f.Name)
		}
		if strings.HasSuffix(f.Name, ".jsonl") {
			hasJSONL = true
		}
		if strings.HasSuffix(f.Name, ".csv") {
			hasCSV = true
		}
	}
	if !hasJSONL {
		t.Error("zip should contain a .jsonl file")
	}
	if !hasCSV {
		t.Error("zip should contain a .csv file")
	}
}

func TestExportTaskLogsZipNoData(t *testing.T) {
	db := setupTestDB(t)
	dir := t.TempDir()

	exporter, err := NewExporter(db, filepath.Join(dir, "exports"))
	if err != nil {
		t.Fatalf("NewExporter: %v", err)
	}

	zipPath, err := exporter.ExportTaskLogsZip(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("ExportTaskLogsZip: %v", err)
	}

	if zipPath == "" {
		t.Fatal("zipPath should not be empty even for no data")
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()

	if len(r.File) != 2 {
		t.Fatalf("expected 2 files in zip (empty JSONL + header-only CSV), got %d", len(r.File))
	}
}

func TestExportTaskLogsZipInvalidPath(t *testing.T) {
	db := setupTestDB(t)

	exporter := &Exporter{db: db, output: "/proc/fakedir"}
	_, err := exporter.ExportTaskLogsZip(context.Background(), "task-1")
	if err == nil {
		t.Error("expected error for unwritable path")
	}
}
