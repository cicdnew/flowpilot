# Repeated Task Feature - Quick Demo Guide

## 🚀 Getting Started

### Start the Application
```bash
wails dev
```

## 📝 Demo Scenario: Testing Product Pages 100-110

### Step 1: Open Repeat Task Modal
1. Navigate to the **Tasks** tab
2. Click the **"+ Repeat Task"** button in the toolbar

### Step 2: Configure Basic Task
```
Task Name: Product Test {{counter}}
URL: https://example.com/product/{{counter}}
```

### Step 3: Configure Repeat Settings
```
Mode: Range
Variable Name: counter
Start Value: 100
End Value: 110
Step: 2
```

**Preview**: Will create 6 tasks (100, 102, 104, 106, 108, 110)

### Step 4: Add Steps
Default navigate step is included. Add more:
1. Click **"+ Add Step"**
2. Select action: **Click**
3. Selector: `#add-to-cart`
4. Value: (leave empty)

Add extraction step:
1. Click **"+ Add Step"**
2. Select action: **Extract**
3. Selector: `.price`
4. Value: `price_{{counter}}`

### Step 5: Configure Options
- Priority: **5**
- Timeout: **0** (use default)
- Tags: `demo, products`
- ✅ Auto-start tasks
- ✅ Headless mode

### Step 6: Create Tasks
Click **"Create Tasks"** button

### Step 7: Watch Execution
- Tasks appear in the task list
- If auto-start is enabled, they begin executing
- View progress in real-time
- Check **Batch Progress** tab for aggregate stats

## 🎯 More Examples

### Example 1: Simple Counter (1-10)
```
Name: Test {{index}}
URL: https://httpbin.org/delay/1
Mode: Counter
Start: 1, End: 10, Step: 1
Result: 10 tasks numbered 1-10
```

### Example 2: Large Range (100-1000, step 10)
```
Name: Load Test {{counter}}
URL: https://example.com/api/item/{{counter}}
Mode: Range
Start: 100, End: 1000, Step: 10
Result: 91 tasks (100, 110, 120... 1000)
```

### Example 3: List Mode (Categories)
```
Name: Category {{item}}
URL: https://shop.example.com/{{item}}
Mode: List
Values:
  electronics
  clothing
  home
  sports
Result: 4 tasks, one per category
```

### Example 4: Paginated Scraping
```
Name: Scrape Page {{index}}
URL: https://api.example.com/data?page={{index}}
Steps:
  1. Navigate to URL
  2. Extract: selector=body, value=page_{{index}}_data
Mode: Counter
Start: 1, End: 50, Step: 1
Result: 50 tasks scraping pages 1-50
```

## 🔍 Understanding Variable Substitution

### Available Variables
- `{{varName}}`: The actual value (e.g., `counter` → "100", "102", "104")
- `{{index}}`: Sequential position (1, 2, 3... regardless of actual values)

### Example with Both
```
Name: Task #{{index}} - Product {{counter}}
Start: 100, End: 104, Step: 2

Results in:
- Task #1 - Product 100
- Task #2 - Product 102
- Task #3 - Product 104
```

### Where Variables Work
✅ Task name
✅ URL
✅ Step values
✅ Step selectors
❌ Priority, timeout, tags (use same values for all)

## 📊 Monitoring Repeated Tasks

### Task List
- All tasks appear with their substituted names
- Filter by tag to see just your batch
- Click any task to view details

### Batch Progress Tab
- Shows aggregate status (pending, running, completed, failed)
- Pause/Resume entire batch
- Retry failed tasks

### Individual Task Details
- View execution logs
- See extracted data with substituted keys
- Check network logs
- Download screenshots

## 💡 Pro Tips

1. **Test Small First**: Start with 2-3 tasks to verify your template works
2. **Use Tags**: Tag repeated batches for easy filtering
3. **Watch Limits**: Max 10,000 tasks per repeat
4. **Step Order Matters**: Variables are substituted before execution
5. **Proxy Support**: Works with proxy configuration and auto-selection
6. **Save Templates**: Copy successful configs for reuse

## 🛠️ Troubleshooting

### "Variable name is required"
- Make sure you've entered a variable name (e.g., "counter", "page", "id")

### "Start value must be <= end value"
- Check your range: Start should be less than or equal to End

### "Values list is required for list mode"
- In list mode, enter at least one value in the text area

### Tasks not substituting correctly
- Check your placeholders: `{{varName}}` not `{varName}` or `{{var_name}}`
- Variable names are case-sensitive

### Too many tasks created
- Double-check your Step value
- Formula: (EndVal - StartVal) / Step + 1

## 🎉 Success!

You've successfully created and executed repeated tasks! The feature integrates seamlessly with:
- Queue management (pause/resume/cancel)
- Retry logic (per-task retries)
- Proxy rotation
- Export functionality (CSV/JSON)
- Audit trail

For more details, see `REPEAT_TASK_FEATURE.md`.
