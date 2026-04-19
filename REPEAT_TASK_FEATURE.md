# Repeated Task Feature

## Overview
The Repeated Task feature allows you to create multiple task instances from a single task definition with automatic parameter substitution. This is perfect for testing the same domain with different values (e.g., testing product pages 100-1000).

## Features

### Three Repeat Modes

1. **Counter Mode**: Simple sequential numbering (1, 2, 3...)
2. **Range Mode**: Custom start/end/step values (e.g., 100, 150, 200 with step=50)
3. **List Mode**: Custom values from a list (e.g., "apple", "banana", "cherry")

### Variable Substitution

Use `{{varName}}` or `{{index}}` in:
- Task name
- URL
- Step selectors
- Step values

**Example:**
```
Task Name: "Test Product {{counter}}"
URL: "https://example.com/product/{{counter}}"
Step Selector: "#add-to-cart-{{counter}}"
```

With range 100-110, step 10, creates:
- Test Product 100 → https://example.com/product/100
- Test Product 110 → https://example.com/product/110

## Backend Implementation

### Models (`internal/models/repeat.go`)

```go
type RepeatConfig struct {
    Mode      RepeatMode // counter, range, or list
    VarName   string     // Variable name for substitution
    StartVal  int        // Start value (counter/range)
    EndVal    int        // End value (counter/range, inclusive)
    Step      int        // Increment step
    Values    []string   // Custom values (list mode)
    BatchSize int        // Optional batch splitting
}

type RepeatTaskInput struct {
    Name          string
    URL           string
    Steps         []TaskStep
    Repeat        RepeatConfig
    ProxyConfig   ProxyConfig
    Priority      int
    AutoStart     bool
    Tags          []string
    Timeout       int
    LoggingPolicy *TaskLoggingPolicy
    Headless      *bool
}
```

### Engine (`internal/repeat/repeat.go`)

The `repeat.Engine` handles:
- Validation of repeat configuration
- Value generation (numeric ranges or custom lists)
- Variable substitution in task fields
- Batch group creation with transaction safety
- Integration with existing task queue

### API (`app_repeat.go`)

```go
func (a *App) CreateRepeatedTask(input RepeatTaskInput) (BatchGroup, error)
```

Validates all inputs and creates repeated task instances in a single transaction.

## Frontend UI (`frontend/src/components/RepeatTaskModal.svelte`)

### Form Fields

1. **Task Configuration**
   - Name (with {{varName}} placeholder)
   - URL (with {{varName}} placeholder)

2. **Repeat Configuration**
   - Mode selector (Counter/Range/List)
   - Variable name input
   - For Counter/Range: Start, End, Step
   - For List: Multi-line text area

3. **Steps Builder**
   - Dynamic step addition/removal
   - Action selector (navigate, click, type, etc.)
   - Selector and value fields with substitution support

4. **Task Options**
   - Priority (1-10)
   - Timeout
   - Tags
   - Auto-start checkbox
   - Headless mode checkbox

### Live Preview

Shows the number of tasks that will be created based on current configuration.

## Usage Examples

### Example 1: Test Product Pages 100-1000

```javascript
{
  name: "Product Test {{counter}}",
  url: "https://shop.example.com/product/{{counter}}",
  steps: [
    { action: "navigate", value: "https://shop.example.com/product/{{counter}}" },
    { action: "click", selector: "#add-to-cart" },
    { action: "extract", selector: ".price", value: "price_{{counter}}" }
  ],
  repeat: {
    mode: "range",
    varName: "counter",
    startVal: 100,
    endVal: 1000,
    step: 10
  }
}
```

**Result**: 91 tasks testing products 100, 110, 120... 1000

### Example 2: Test Multiple Categories

```javascript
{
  name: "Category {{item}}",
  url: "https://shop.example.com/category/{{item}}",
  steps: [
    { action: "navigate", value: "https://shop.example.com/category/{{item}}" },
    { action: "screenshot" }
  ],
  repeat: {
    mode: "list",
    varName: "item",
    values: ["electronics", "clothing", "home", "sports", "toys"]
  }
}
```

**Result**: 5 tasks, one per category

### Example 3: Paginated Scraping

```javascript
{
  name: "Scrape Page {{index}}",
  url: "https://api.example.com/data?page={{index}}",
  steps: [
    { action: "navigate", value: "https://api.example.com/data?page={{index}}" },
    { action: "extract", selector: "body", value: "page_{{index}}_data" }
  ],
  repeat: {
    mode: "counter",
    varName: "index",
    startVal: 1,
    endVal: 50,
    step: 1
  }
}
```

**Result**: 50 tasks scraping pages 1-50

## Integration with Existing Systems

### Queue Integration
- Tasks are submitted as a batch via `queue.SubmitBatch()`
- Batch control (pause/resume) works seamlessly
- Retry logic applies per task instance

### Database
- Uses existing batch_groups table (FlowID is empty for repeated tasks)
- Tasks linked via BatchID for group operations
- Full transaction safety on creation

### Proxy Management
- Supports proxy configuration per batch
- Auto-proxy with geo routing works
- Concurrency limits respected

### Metrics & Monitoring
- Each task tracked independently
- Batch progress API shows aggregate status
- Standard task events emitted

## Validation & Limits

- Max tasks per repeat: 10,000 (models.MaxBatchSize)
- Variable name required for all non-none modes
- Step must be > 0 for counter/range modes
- EndVal must be >= StartVal
- Values list cannot be empty for list mode

## Testing

All features tested in `internal/repeat/repeat_test.go`:
- ✓ Repeat config validation
- ✓ Value generation (counter, range, list)
- ✓ Task creation with substitution
- ✓ Index variable support
- ✓ Database transaction safety

Run tests:
```bash
go test -tags=dev ./internal/repeat -v
```

## Future Enhancements

Potential additions:
- CSV import for list mode
- Batch splitting for large repeats (use BatchSize field)
- Variable interpolation in more fields (tags, proxy config)
- Template preview before creation
- Incremental start (resume from last index)
