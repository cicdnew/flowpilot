package repeat

import (
	"context"
	"path/filepath"
	"testing"

	"flowpilot/internal/crypto"
	"flowpilot/internal/database"
	"flowpilot/internal/models"
)

func setupTestDB(t *testing.T) *database.DB {
	t.Helper()
	crypto.ResetForTest()
	if err := crypto.InitKeyWithBytes([]byte("0123456789abcdef0123456789abcdef")); err != nil {
		t.Fatalf("init crypto: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRepeatConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  models.RepeatConfig
		wantErr bool
	}{
		{
			name:    "none mode valid",
			config:  models.RepeatConfig{Mode: models.RepeatModeNone},
			wantErr: false,
		},
		{
			name: "counter mode valid",
			config: models.RepeatConfig{
				Mode:     models.RepeatModeCounter,
				VarName:  "counter",
				StartVal: 1,
				EndVal:   10,
				Step:     1,
			},
			wantErr: false,
		},
		{
			name: "range mode valid",
			config: models.RepeatConfig{
				Mode:     models.RepeatModeRange,
				VarName:  "value",
				StartVal: 100,
				EndVal:   200,
				Step:     10,
			},
			wantErr: false,
		},
		{
			name: "list mode valid",
			config: models.RepeatConfig{
				Mode:    models.RepeatModeList,
				VarName: "item",
				Values:  []string{"apple", "banana", "cherry"},
			},
			wantErr: false,
		},
		{
			name: "missing varName",
			config: models.RepeatConfig{
				Mode:     models.RepeatModeCounter,
				StartVal: 1,
				EndVal:   10,
				Step:     1,
			},
			wantErr: true,
		},
		{
			name: "invalid range",
			config: models.RepeatConfig{
				Mode:     models.RepeatModeCounter,
				VarName:  "counter",
				StartVal: 10,
				EndVal:   1,
				Step:     1,
			},
			wantErr: true,
		},
		{
			name: "zero step",
			config: models.RepeatConfig{
				Mode:     models.RepeatModeCounter,
				VarName:  "counter",
				StartVal: 1,
				EndVal:   10,
				Step:     0,
			},
			wantErr: true,
		},
		{
			name: "empty list",
			config: models.RepeatConfig{
				Mode:    models.RepeatModeList,
				VarName: "item",
				Values:  []string{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRepeatConfigGenerateValues(t *testing.T) {
	tests := []struct {
		name       string
		config     models.RepeatConfig
		wantValues []string
	}{
		{
			name: "counter 1 to 5",
			config: models.RepeatConfig{
				Mode:     models.RepeatModeCounter,
				VarName:  "counter",
				StartVal: 1,
				EndVal:   5,
				Step:     1,
			},
			wantValues: []string{"1", "2", "3", "4", "5"},
		},
		{
			name: "range 100 to 200 step 50",
			config: models.RepeatConfig{
				Mode:     models.RepeatModeRange,
				VarName:  "value",
				StartVal: 100,
				EndVal:   200,
				Step:     50,
			},
			wantValues: []string{"100", "150", "200"},
		},
		{
			name: "list mode",
			config: models.RepeatConfig{
				Mode:    models.RepeatModeList,
				VarName: "item",
				Values:  []string{"apple", "banana"},
			},
			wantValues: []string{"apple", "banana"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := tt.config.GenerateValues()
			if len(values) != len(tt.wantValues) {
				t.Errorf("GenerateValues() count = %d, want %d", len(values), len(tt.wantValues))
				return
			}
			for i, v := range values {
				if v != tt.wantValues[i] {
					t.Errorf("GenerateValues()[%d] = %s, want %s", i, v, tt.wantValues[i])
				}
			}
		})
	}
}

func TestCreateRepeatedTasks(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)
	ctx := context.Background()

	input := models.RepeatTaskInput{
		Name: "Test Task {{counter}}",
		URL:  "https://example.com/page/{{counter}}",
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: "https://example.com/page/{{counter}}"},
			{Action: models.ActionClick, Selector: "#item-{{counter}}"},
		},
		Repeat: models.RepeatConfig{
			Mode:     models.RepeatModeCounter,
			VarName:  "counter",
			StartVal: 1,
			EndVal:   3,
			Step:     1,
		},
		Priority: 5,
	}

	group, tasks, err := engine.CreateRepeatedTasks(ctx, input)
	if err != nil {
		t.Fatalf("CreateRepeatedTasks() error = %v", err)
	}

	if len(tasks) != 3 {
		t.Errorf("CreateRepeatedTasks() created %d tasks, want 3", len(tasks))
	}

	if group.Total != 3 {
		t.Errorf("BatchGroup.Total = %d, want 3", group.Total)
	}

	// Verify task substitutions
	expectedNames := []string{"Test Task 1", "Test Task 2", "Test Task 3"}
	expectedURLs := []string{
		"https://example.com/page/1",
		"https://example.com/page/2",
		"https://example.com/page/3",
	}

	for i, task := range tasks {
		if task.Name != expectedNames[i] {
			t.Errorf("Task[%d].Name = %s, want %s", i, task.Name, expectedNames[i])
		}
		if task.URL != expectedURLs[i] {
			t.Errorf("Task[%d].URL = %s, want %s", i, task.URL, expectedURLs[i])
		}
		if task.Steps[0].Value != expectedURLs[i] {
			t.Errorf("Task[%d].Steps[0].Value = %s, want %s", i, task.Steps[0].Value, expectedURLs[i])
		}
		expectedSelector := "#item-" + expectedNames[i][len(expectedNames[i])-1:]
		if task.Steps[1].Selector != expectedSelector {
			t.Errorf("Task[%d].Steps[1].Selector = %s, want %s", i, task.Steps[1].Selector, expectedSelector)
		}
	}
}

func TestCreateRepeatedTasksWithIndexVar(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)
	ctx := context.Background()

	input := models.RepeatTaskInput{
		Name: "Task #{{index}}",
		URL:  "https://example.com",
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: "https://example.com"},
			{Action: models.ActionType, Selector: "#input", Value: "test-{{index}}"},
		},
		Repeat: models.RepeatConfig{
			Mode:     models.RepeatModeCounter,
			VarName:  "counter",
			StartVal: 10,
			EndVal:   12,
			Step:     1,
		},
		Priority: 5,
	}

	_, tasks, err := engine.CreateRepeatedTasks(ctx, input)
	if err != nil {
		t.Fatalf("CreateRepeatedTasks() error = %v", err)
	}

	// Verify {{index}} substitution (should be 1, 2, 3)
	expectedNames := []string{"Task #1", "Task #2", "Task #3"}
	for i, task := range tasks {
		if task.Name != expectedNames[i] {
			t.Errorf("Task[%d].Name = %s, want %s", i, task.Name, expectedNames[i])
		}
	}
}
