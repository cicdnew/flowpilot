# Chhotu.com Testing - Visual Guide

## 📸 Step-by-Step with Screenshots

### Step 1: Open Repeat Task Modal
```
Click: [+ Repeat Task] button in toolbar
```
**You'll see**: A modal with three main sections

---

### Step 2: Task Configuration Section

```
┌─────────────────────────────────────────────────────────┐
│ Task Name:                                              │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ Chhotu Page {{counter}}                             │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ URL:                                                    │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ https://chhotu.com/page/{{counter}}                 │ │
│ └─────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

**Fill in**:
- Name: `Chhotu Page {{counter}}`
- URL: `https://chhotu.com/page/{{counter}}`

---

### Step 3: Repeat Configuration Section

```
┌─────────────────────────────────────────────────────────┐
│ Repeat Configuration                                    │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ Repeat Mode:        Variable Name:                     │
│ ┌─────────────────┐ ┌─────────────────┐               │
│ │ Range ▼         │ │ counter         │               │
│ └─────────────────┘ └─────────────────┘               │
│                                                         │
│ Start Value:  End Value:  Step:                        │
│ ┌─────┐       ┌─────┐     ┌─────┐                     │
│ │ 100 │       │ 110 │     │ 1   │                     │
│ └─────┘       └─────┘     └─────┘                     │
│                                                         │
│ ℹ️ Will create 11 tasks                                │
└─────────────────────────────────────────────────────────┘
```

**Fill in**:
- Mode: `Range`
- Variable: `counter`
- Start: `100`
- End: `110`
- Step: `1`

**Result**: 11 tasks (100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110)

---

### Step 4: Steps Section

```
┌─────────────────────────────────────────────────────────┐
│ Steps                                                   │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ [Navigate ▼] [            ] [https://chhotu.com/pa...] [×]
│                                                         │
│ [Wait     ▼] [            ] [2                      ] [×]
│                                                         │
│ [Screenshot▼] [            ] [chhotu_{{counter}}    ] [×]
│                                                         │
│ [Extract  ▼] [h1,.title   ] [page_{{counter}}_title] [×]
│                                                         │
│                                                         │
│              [+ Add Step]                               │
└─────────────────────────────────────────────────────────┘
```

**Configure steps**:

**Step 1** (auto-filled):
- Action: `Navigate`
- Value: `https://chhotu.com/page/{{counter}}`

**Step 2** (click "+ Add Step"):
- Action: `Wait`
- Value: `2`

**Step 3** (click "+ Add Step"):
- Action: `Screenshot`
- Value: `chhotu_{{counter}}`

**Step 4** (click "+ Add Step"):
- Action: `Extract`
- Selector: `h1, .page-title, .title`
- Value: `page_{{counter}}_title`

---

### Step 5: Options Section

```
┌─────────────────────────────────────────────────────────┐
│ Priority:  Timeout (seconds):                           │
│ ┌─────┐    ┌─────┐                                      │
│ │ 5   │    │ 60  │                                      │
│ └─────┘    └─────┘                                      │
│                                                         │
│ Tags:                                                   │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ chhotu, test                                        │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ ☑ Auto-start tasks                                     │
│ ☑ Headless mode                                        │
└─────────────────────────────────────────────────────────┘
```

**Configure**:
- Priority: `5`
- Timeout: `60`
- Tags: `chhotu, test`
- ☑ Auto-start
- ☑ Headless

---

### Step 6: Create Tasks

```
┌─────────────────────────────────────────────────────────┐
│                                                         │
│                  [Cancel]  [Create Tasks]               │
└─────────────────────────────────────────────────────────┘
```

Click **[Create Tasks]** button

---

### Step 7: Watch Execution

**Task List View**:
```
┌──────────────────────────────────────────────────────────┐
│ Status: [All ▼]  Tag: [chhotu ▼]                        │
├──────────────────────────────────────────────────────────┤
│ Name                   Status      Priority  Created     │
├──────────────────────────────────────────────────────────┤
│ ● Chhotu Page 100     Running     5         11:30:00    │
│ ⏸ Chhotu Page 101     Queued      5         11:30:00    │
│ ⏸ Chhotu Page 102     Queued      5         11:30:00    │
│ ⏸ Chhotu Page 103     Queued      5         11:30:00    │
│ ⏸ Chhotu Page 104     Queued      5         11:30:00    │
│ ⏸ Chhotu Page 105     Queued      5         11:30:00    │
│ ... 5 more tasks                                         │
└──────────────────────────────────────────────────────────┘
```

**Status Legend**:
- ● Running (blue)
- ⏸ Queued (gray)
- ✓ Completed (green)
- ✗ Failed (red)
- ⟳ Retrying (yellow)

---

### Step 8: View Results

**Click any completed task to see**:

```
┌──────────────────────────────────────────────────────────┐
│ Task: Chhotu Page 100                                    │
├──────────────────────────────────────────────────────────┤
│ Status: ✓ Completed                                      │
│ Duration: 3.2s                                           │
│ URL: https://chhotu.com/page/100                         │
├──────────────────────────────────────────────────────────┤
│ Extracted Data:                                          │
│ ┌────────────────────────────────────────────────────┐  │
│ │ page_100_title: "Welcome to Page 100"              │  │
│ └────────────────────────────────────────────────────┘  │
├──────────────────────────────────────────────────────────┤
│ Screenshots:                                             │
│ 📷 chhotu_100.png (1920x1080)                           │
├──────────────────────────────────────────────────────────┤
│ Execution Logs:                                          │
│ ✓ Navigate to https://chhotu.com/page/100              │
│ ✓ Wait 2 seconds                                        │
│ ✓ Screenshot captured: chhotu_100.png                   │
│ ✓ Extracted: page_100_title                             │
└──────────────────────────────────────────────────────────┘
```

---

## 🎯 Common Patterns for Chhotu.com

### Pattern 1: Sequential Pages (1-100)
```
Variable: page_num
Range: 1 to 100, Step: 1
URL: https://chhotu.com/page/{{page_num}}
Result: 100 tasks
```

### Pattern 2: Product IDs (Gaps in sequence)
```
Variable: product_id
Range: 1000 to 2000, Step: 10
URL: https://chhotu.com/product/{{product_id}}
Result: 101 tasks (1000, 1010, 1020... 2000)
```

### Pattern 3: Category Testing
```
Variable: category
Mode: List
Values: electronics, clothing, home, sports
URL: https://chhotu.com/category/{{category}}
Result: 4 tasks
```

---

## 📊 Results Dashboard

**After execution, view in Batch Progress tab**:

```
┌──────────────────────────────────────────────────────────┐
│ Batch: Chhotu Page {{counter}} (repeated 11 times)      │
├──────────────────────────────────────────────────────────┤
│                                                          │
│ Progress: ████████████████░░░░ 80% (8/10 completed)     │
│                                                          │
│ ┌────────────────────────────────────────────────────┐  │
│ │ ✓ Completed: 8                                     │  │
│ │ ● Running:   1                                     │  │
│ │ ⏸ Queued:    2                                     │  │
│ │ ✗ Failed:    0                                     │  │
│ │ ⟳ Retrying:  0                                     │  │
│ └────────────────────────────────────────────────────┘  │
│                                                          │
│ Duration: 28.4s                                          │
│ Avg per task: 3.2s                                       │
│                                                          │
│              [Pause Batch]  [Export Results]             │
└──────────────────────────────────────────────────────────┘
```

---

## 💾 Export Results

**Click "Export CSV" to get**:

```csv
task_id,name,status,url,duration_ms,extracted_data,created_at
uuid-1,Chhotu Page 100,completed,https://chhotu.com/page/100,3200,"{""page_100_title"":""Welcome""}",2026-04-19T11:30:00Z
uuid-2,Chhotu Page 101,completed,https://chhotu.com/page/101,3150,"{""page_101_title"":""Page 101""}",2026-04-19T11:30:03Z
uuid-3,Chhotu Page 102,completed,https://chhotu.com/page/102,3180,"{""page_102_title"":""Page 102""}",2026-04-19T11:30:06Z
...
```

**Open in Excel for analysis**:
- Filter by status
- Calculate average duration
- Analyze extracted data patterns
- Identify failed pages

---

## 🔄 Variable Substitution Examples

### Example 1: URL with counter
```
Input:  https://chhotu.com/page/{{counter}}
Counter: 105
Output: https://chhotu.com/page/105
```

### Example 2: Selector with counter
```
Input:  #product-{{counter}} .add-to-cart
Counter: 205
Output: #product-205 .add-to-cart
```

### Example 3: Value with multiple variables
```
Input:  page_{{counter}}_title_{{index}}
Counter: 150
Index: 51
Output: page_150_title_51
```

### Example 4: List mode
```
Input:  https://chhotu.com/{{category}}/products
Category: "electronics"
Output: https://chhotu.com/electronics/products
```

---

## ⚡ Quick Reference

### Default Values
- Priority: `5`
- Timeout: `60` seconds
- Headless: `true`
- Auto-start: `false`

### Variable Syntax
- Counter/Range: `{{counter}}`, `{{page}}`, `{{id}}`
- Index: `{{index}}` (always 1, 2, 3...)
- List: `{{category}}`, `{{item}}`, `{{query}}`

### Task Limits
- Max tasks per repeat: `10,000`
- Recommended batch size: `100-500`
- Concurrent execution: Configurable (default: based on CPU)

### Step Actions Available
- `navigate` - Go to URL
- `click` - Click element
- `type` - Type text
- `wait` - Wait N seconds
- `screenshot` - Capture screen
- `extract` - Extract text/data
- `select` - Select dropdown option
- `scroll` - Scroll page
- `hover` - Hover over element

---

## 🎓 Learning Path

1. **First Run**: 3 tasks, simple navigate + screenshot
2. **Second Run**: 10 tasks, add extraction
3. **Third Run**: 50 tasks, add interactions (click)
4. **Fourth Run**: 100+ tasks, full flow with validation

**Start simple, scale gradually!** 🚀
