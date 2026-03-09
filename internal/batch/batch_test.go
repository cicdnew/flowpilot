package batch

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"flowpilot/internal/crypto"
	"flowpilot/internal/database"
	"flowpilot/internal/models"
)

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
	dbPath := filepath.Join(dir, "test.db")
	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("create test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func makeFlow() models.RecordedFlow {
	return models.RecordedFlow{
		ID:   "flow-1",
		Name: "Test Flow",
		Steps: []models.RecordedStep{
			{Index: 0, Action: models.ActionNavigate, Value: "https://origin.example.com"},
			{Index: 1, Action: models.ActionClick, Selector: "#btn-{{domain}}"},
			{Index: 2, Action: models.ActionType, Selector: "#input", Value: "{{url}}"},
		},
		OriginURL: "https://origin.example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func makeBatchInput(urls []string) models.AdvancedBatchInput {
	return models.AdvancedBatchInput{
		FlowID:         "flow-1",
		URLs:           urls,
		NamingTemplate: "",
		Priority:       5,
		Proxy:          models.ProxyConfig{},
		Tags:           []string{"batch-test"},
	}
}

// --- CreateBatchFromFlow Tests ---

func TestCreateBatchFromFlow_Basic(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)
	flow := makeFlow()
	input := makeBatchInput([]string{
		"https://example.com",
		"https://other.com",
	})

	group, tasks, err := engine.CreateBatchFromFlow(context.Background(), flow, input)
	if err != nil {
		t.Fatalf("CreateBatchFromFlow: %v", err)
	}

	if group.ID == "" {
		t.Error("batch group ID should not be empty")
	}
	if group.FlowID != "flow-1" {
		t.Errorf("FlowID: got %q, want %q", group.FlowID, "flow-1")
	}
	if group.Total != 2 {
		t.Errorf("Total: got %d, want 2", group.Total)
	}
	if len(group.TaskIDs) != 2 {
		t.Errorf("TaskIDs count: got %d, want 2", len(group.TaskIDs))
	}
	if len(tasks) != 2 {
		t.Fatalf("tasks count: got %d, want 2", len(tasks))
	}

	// Each task should have a unique ID and correct batch ID
	if tasks[0].ID == tasks[1].ID {
		t.Error("tasks should have unique IDs")
	}
	for _, task := range tasks {
		if task.BatchID != group.ID {
			t.Errorf("task BatchID %q should match group ID %q", task.BatchID, group.ID)
		}
		if task.FlowID != "flow-1" {
			t.Errorf("task FlowID: got %q, want %q", task.FlowID, "flow-1")
		}
		if task.Status != models.TaskStatusPending {
			t.Errorf("task Status: got %q, want %q", task.Status, models.TaskStatusPending)
		}
		if task.MaxRetries != 3 {
			t.Errorf("MaxRetries: got %d, want 3", task.MaxRetries)
		}
	}
}

func TestCreateBatchFromFlow_DefaultNamingTemplate(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)
	flow := makeFlow()
	input := makeBatchInput([]string{"https://example.com"})
	input.NamingTemplate = "" // blank should use default

	_, tasks, err := engine.CreateBatchFromFlow(context.Background(), flow, input)
	if err != nil {
		t.Fatalf("CreateBatchFromFlow: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	// Default template is "Task {{index}} - {{domain}}"
	// For https://example.com with index 1: "Task 1 - example.com"
	want := "Task 1 - example.com"
	if tasks[0].Name != want {
		t.Errorf("Name: got %q, want %q", tasks[0].Name, want)
	}
}

func TestCreateBatchFromFlow_CustomNamingTemplate(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)
	flow := makeFlow()
	input := makeBatchInput([]string{"https://example.com", "https://test.org"})
	input.NamingTemplate = "Batch #{{index}} {{domain}}"

	_, tasks, err := engine.CreateBatchFromFlow(context.Background(), flow, input)
	if err != nil {
		t.Fatalf("CreateBatchFromFlow: %v", err)
	}

	if tasks[0].Name != "Batch #1 example.com" {
		t.Errorf("tasks[0].Name: got %q, want %q", tasks[0].Name, "Batch #1 example.com")
	}
	if tasks[1].Name != "Batch #2 test.org" {
		t.Errorf("tasks[1].Name: got %q, want %q", tasks[1].Name, "Batch #2 test.org")
	}
}

func TestCreateBatchFromFlow_InvalidNamingTemplate(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)
	flow := makeFlow()
	input := makeBatchInput([]string{"https://example.com"})
	input.NamingTemplate = "Task {{invalid_var}}"

	_, _, err := engine.CreateBatchFromFlow(context.Background(), flow, input)
	if err == nil {
		t.Fatal("expected error for invalid naming template")
	}
	if !strings.Contains(err.Error(), "invalid naming template") {
		t.Errorf("error should mention invalid naming template, got: %v", err)
	}
}

func TestCreateBatchFromFlow_StepTemplateSubstitution(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)
	flow := makeFlow() // step[1].Selector = "#btn-{{domain}}", step[2].Value = "{{url}}"
	input := makeBatchInput([]string{"https://example.com"})

	_, tasks, err := engine.CreateBatchFromFlow(context.Background(), flow, input)
	if err != nil {
		t.Fatalf("CreateBatchFromFlow: %v", err)
	}

	task := tasks[0]
	if len(task.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(task.Steps))
	}

	// Step 1 (click): selector should have {{domain}} replaced
	if task.Steps[1].Selector != "#btn-example.com" {
		t.Errorf("Step 1 selector: got %q, want %q", task.Steps[1].Selector, "#btn-example.com")
	}

	// Step 2 (type): value should have {{url}} replaced
	if task.Steps[2].Value != "https://example.com" {
		t.Errorf("Step 2 value: got %q, want %q", task.Steps[2].Value, "https://example.com")
	}
}

func TestCreateBatchFromFlow_NavigateStepURLFallback(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)

	// Flow with navigate step that has empty value (will be filled with rawURL)
	flow := models.RecordedFlow{
		ID:   "flow-nav",
		Name: "Navigate Flow",
		Steps: []models.RecordedStep{
			{Index: 0, Action: models.ActionNavigate, Value: ""},
			{Index: 1, Action: models.ActionClick, Selector: "#btn"},
		},
		OriginURL: "https://origin.example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	input := makeBatchInput([]string{"https://target.com"})

	_, tasks, err := engine.CreateBatchFromFlow(context.Background(), flow, input)
	if err != nil {
		t.Fatalf("CreateBatchFromFlow: %v", err)
	}

	// First navigate step should get the rawURL as fallback
	if tasks[0].Steps[0].Value != "https://target.com" {
		t.Errorf("Navigate fallback: got %q, want %q", tasks[0].Steps[0].Value, "https://target.com")
	}
}

func TestCreateBatchFromFlow_NavigateStepPreservesExplicitURL(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)

	// Flow with navigate step that has an explicit URL
	flow := models.RecordedFlow{
		ID:   "flow-explicit",
		Name: "Explicit Navigate",
		Steps: []models.RecordedStep{
			{Index: 0, Action: models.ActionNavigate, Value: "https://explicit.com"},
		},
		OriginURL: "https://origin.example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	input := makeBatchInput([]string{"https://target.com"})

	_, tasks, err := engine.CreateBatchFromFlow(context.Background(), flow, input)
	if err != nil {
		t.Fatalf("CreateBatchFromFlow: %v", err)
	}

	// Navigate step should keep its explicit value
	if tasks[0].Steps[0].Value != "https://explicit.com" {
		t.Errorf("Navigate preserved: got %q, want %q", tasks[0].Steps[0].Value, "https://explicit.com")
	}
}

func TestCreateBatchFromFlow_HeadlessDefault(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)
	flow := makeFlow()
	input := makeBatchInput([]string{"https://example.com"})

	_, tasks, err := engine.CreateBatchFromFlow(context.Background(), flow, input)
	if err != nil {
		t.Fatalf("CreateBatchFromFlow: %v", err)
	}

	if !tasks[0].Headless {
		t.Error("batch-created tasks should default to Headless=true")
	}
}

func TestCreateBatchFromFlow_ProxyConfigPropagated(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)
	flow := makeFlow()
	input := makeBatchInput([]string{"https://example.com"})
	input.Proxy = models.ProxyConfig{
		Server:   "proxy.test:8080",
		Username: "user",
		Password: "pass",
	}

	_, tasks, err := engine.CreateBatchFromFlow(context.Background(), flow, input)
	if err != nil {
		t.Fatalf("CreateBatchFromFlow: %v", err)
	}

	if tasks[0].Proxy.Server != "proxy.test:8080" {
		t.Errorf("Proxy.Server: got %q, want %q", tasks[0].Proxy.Server, "proxy.test:8080")
	}
}

func TestCreateBatchFromFlow_TagsPropagated(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)
	flow := makeFlow()
	input := makeBatchInput([]string{"https://example.com"})
	input.Tags = []string{"production", "nightly"}

	_, tasks, err := engine.CreateBatchFromFlow(context.Background(), flow, input)
	if err != nil {
		t.Fatalf("CreateBatchFromFlow: %v", err)
	}

	if len(tasks[0].Tags) != 2 || tasks[0].Tags[0] != "production" {
		t.Errorf("Tags: got %v, want [production nightly]", tasks[0].Tags)
	}
}

func TestCreateBatchFromFlow_TaskPersistedInDB(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)
	flow := makeFlow()
	input := makeBatchInput([]string{"https://example.com"})

	_, tasks, err := engine.CreateBatchFromFlow(context.Background(), flow, input)
	if err != nil {
		t.Fatalf("CreateBatchFromFlow: %v", err)
	}

	// Verify the task can be retrieved from DB
	got, err := db.GetTask(context.Background(), tasks[0].ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Name != tasks[0].Name {
		t.Errorf("persisted Name: got %q, want %q", got.Name, tasks[0].Name)
	}
	if got.URL != "https://example.com" {
		t.Errorf("persisted URL: got %q, want %q", got.URL, "https://example.com")
	}
}

func TestCreateBatchFromFlow_ValidationRejectsEmptyURLs(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)
	flow := makeFlow()
	input := makeBatchInput([]string{}) // empty URLs

	_, _, err := engine.CreateBatchFromFlow(context.Background(), flow, input)
	if err == nil {
		t.Fatal("expected validation error for empty URLs")
	}
}

// --- Template and Naming Tests ---

func TestDefaultNameTemplate(t *testing.T) {
	tmpl := DefaultNameTemplate()
	if tmpl == "" {
		t.Fatal("DefaultNameTemplate should not be empty")
	}
	if !strings.Contains(tmpl, "{{index}}") {
		t.Error("default template should contain {{index}}")
	}
	if !strings.Contains(tmpl, "{{domain}}") {
		t.Error("default template should contain {{domain}}")
	}
}

func TestValidateTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		valid    bool
	}{
		{"default template", "Task {{index}} - {{domain}}", true},
		{"all variables", "{{url}} {{domain}} {{index}} {{name}}", true},
		{"plain text only", "Static Name", true},
		{"single variable", "{{index}}", true},
		{"invalid variable", "{{badvar}}", false},
		{"unclosed brace", "Task {{index", false},
		{"mixed valid and invalid", "{{index}} {{bad}}", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateTemplate(tc.template)
			if got != tc.valid {
				t.Errorf("ValidateTemplate(%q): got %v, want %v", tc.template, got, tc.valid)
			}
		})
	}
}

func TestApplyTemplate(t *testing.T) {
	vars := TemplateVars{
		URL:    "https://example.com",
		Domain: "example.com",
		Index:  3,
		Name:   "Test Task",
	}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{"index and domain", "Task {{index}} - {{domain}}", "Task 3 - example.com"},
		{"url", "Visit {{url}}", "Visit https://example.com"},
		{"name", "Name: {{name}}", "Name: Test Task"},
		{"no variables", "Static", "Static"},
		{"all variables", "{{index}} {{domain}} {{url}} {{name}}", "3 example.com https://example.com Test Task"},
		{"repeated variable", "{{index}}-{{index}}", "3-3"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ApplyTemplate(tc.template, vars)
			if got != tc.want {
				t.Errorf("ApplyTemplate(%q): got %q, want %q", tc.template, got, tc.want)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"standard url", "https://example.com/path", "example.com"},
		{"with port", "https://example.com:8080/path", "example.com"},
		{"http", "http://test.org", "test.org"},
		{"subdomain", "https://sub.domain.com", "sub.domain.com"},
		{"invalid url", "not-a-url", ""},
		{"empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractDomain(tc.url)
			if got != tc.want {
				t.Errorf("ExtractDomain(%q): got %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

// --- CSV/URL Parsing Tests ---

func TestParseURLList(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"basic", "https://a.com\nhttps://b.com", 2},
		{"with blanks", "https://a.com\n\nhttps://b.com\n\n", 2},
		{"single", "https://a.com", 1},
		{"empty", "", 0},
		{"only whitespace", "  \n  \n  ", 0},
		{"with spaces", "  https://a.com  \n  https://b.com  ", 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			urls, err := ParseURLList(tc.input)
			if err != nil {
				t.Fatalf("ParseURLList: %v", err)
			}
			if len(urls) != tc.want {
				t.Errorf("count: got %d, want %d", len(urls), tc.want)
			}
		})
	}
}

func TestParseCSVURLs(t *testing.T) {
	tests := []struct {
		name string
		csv  string
		want int
	}{
		{"basic", "https://a.com\nhttps://b.com\n", 2},
		{"multi-column", "https://a.com,col2\nhttps://b.com,col2\n", 2},
		{"empty", "", 0},
		{"with empty first col", ",col2\nhttps://a.com,col2\n", 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			urls, err := ParseCSVURLs(strings.NewReader(tc.csv))
			if err != nil {
				t.Fatalf("ParseCSVURLs: %v", err)
			}
			if len(urls) != tc.want {
				t.Errorf("count: got %d, want %d", len(urls), tc.want)
			}
		})
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func TestCreateBatchFromFlow_HeadlessExplicitFalse(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)
	flow := makeFlow()
	input := makeBatchInput([]string{"https://example.com"})
	input.Headless = boolPtr(false)

	_, tasks, err := engine.CreateBatchFromFlow(context.Background(), flow, input)
	if err != nil {
		t.Fatalf("CreateBatchFromFlow: %v", err)
	}

	if tasks[0].Headless {
		t.Error("expected Headless=false when explicitly set to false")
	}
}

func TestCreateBatchFromFlow_HeadlessExplicitTrue(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)
	flow := makeFlow()
	input := makeBatchInput([]string{"https://example.com"})
	input.Headless = boolPtr(true)

	_, tasks, err := engine.CreateBatchFromFlow(context.Background(), flow, input)
	if err != nil {
		t.Fatalf("CreateBatchFromFlow: %v", err)
	}

	if !tasks[0].Headless {
		t.Error("expected Headless=true when explicitly set to true")
	}
}

func TestCreateBatchFromFlow_HeadlessNilDefaultsTrue(t *testing.T) {
	db := setupTestDB(t)
	engine := New(db)
	flow := makeFlow()
	input := makeBatchInput([]string{"https://example.com"})
	input.Headless = nil // explicitly nil

	_, tasks, err := engine.CreateBatchFromFlow(context.Background(), flow, input)
	if err != nil {
		t.Fatalf("CreateBatchFromFlow: %v", err)
	}

	if !tasks[0].Headless {
		t.Error("expected Headless=true when nil (backwards compatible default)")
	}
}

func TestBatchHeadlessHelper(t *testing.T) {
	// nil -> true
	input := models.AdvancedBatchInput{}
	if !input.BatchHeadless() {
		t.Error("BatchHeadless() should return true for nil")
	}

	// explicit true
	input.Headless = boolPtr(true)
	if !input.BatchHeadless() {
		t.Error("BatchHeadless() should return true for *true")
	}

	// explicit false
	input.Headless = boolPtr(false)
	if input.BatchHeadless() {
		t.Error("BatchHeadless() should return false for *false")
	}
}

func TestExtractDomainParseError(t *testing.T) {
	// url.Parse returns error for URLs with invalid percent-encoding
	got := ExtractDomain("https://example.com/%zz")
	if got != "" {
		t.Errorf("expected empty for invalid URL, got %q", got)
	}
}
