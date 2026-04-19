# Quick Start: Testing Chhotu.com with Repeated Tasks

## 🚀 5-Minute Setup

### Step 1: Start FlowPilot
```bash
cd /path/to/flowpilot
wails dev
```

### Step 2: Open Repeat Task Modal
1. Navigate to **Tasks** tab
2. Click **"+ Repeat Task"** button

### Step 3: Configure for Chhotu.com

#### Basic Configuration (Test 10 Pages)
```
Task Name: Chhotu Page {{counter}}
URL: https://chhotu.com/page/{{counter}}
```

#### Repeat Settings
```
Mode: Range
Variable Name: counter
Start Value: 100
End Value: 110
Step: 1
```
**Preview**: Will create 11 tasks (100, 101, 102... 110)

#### Steps to Execute
**Step 1**: Navigate (auto-filled from URL)

**Step 2**: Wait for page load
- Action: `Wait`
- Value: `2` (seconds)

**Step 3**: Take screenshot
- Action: `Screenshot`
- Value: `chhotu_{{counter}}`

**Step 4**: Extract page title
- Action: `Extract`
- Selector: `h1, .page-title, .title`
- Value: `page_{{counter}}_title`

#### Options
```
Priority: 5
Tags: chhotu, test-run-1
✅ Auto-start tasks
✅ Headless mode
```

### Step 4: Create & Watch
1. Click **"Create Tasks"** button
2. Tasks will appear in the task list
3. Watch them execute in real-time
4. Check results in **Task Detail** panel

---

## 📊 Expected Results

### What You'll Get
- ✅ 11 tasks created and executed
- ✅ Screenshots: `chhotu_100.png` through `chhotu_110.png`
- ✅ Extracted titles for each page
- ✅ Execution logs for debugging
- ✅ Success/failure status per task

### Where to Find Results
1. **Screenshots**: In your FlowPilot screenshots directory
2. **Extracted Data**: Click any task → View "Extracted Data" section
3. **Logs**: Click any task → Scroll to "Execution Logs"
4. **Export**: Click "Export CSV" for spreadsheet analysis

---

## 🎯 Real-World Scenarios

### Scenario 1: Quick Health Check (20 pages, step 10)
```
Name: Chhotu Health {{counter}}
URL: https://chhotu.com/page/{{counter}}
Range: 1 to 200, Step: 10
Result: 20 tasks checking every 10th page
Duration: ~1 minute
```

### Scenario 2: Product Catalog (100 products)
```
Name: Product {{counter}}
URL: https://chhotu.com/product/{{counter}}
Range: 1001 to 1100, Step: 1
Steps:
  - Navigate
  - Extract: .product-name → name_{{counter}}
  - Extract: .price → price_{{counter}}
  - Screenshot
Result: Product details for 100 items
Duration: ~3-4 minutes
```

### Scenario 3: Category Pages
```
Name: Category {{category}}
URL: https://chhotu.com/category/{{category}}
Mode: List
Values:
  electronics
  clothing
  home
  sports
Result: 4 tasks, one per category
Duration: ~30 seconds
```

---

## 💡 Pro Tips for Chhotu.com

### 1. Find the Right Selectors
Open chhotu.com in Chrome:
1. Right-click on element → Inspect
2. Copy selector (right-click in DevTools → Copy → Copy selector)
3. Paste into FlowPilot selector field

### 2. Test Small First
Always test 2-3 pages before scaling to 100+:
```
Start: 100, End: 102 (3 tasks)
✅ Verify it works
Then: Start: 100, End: 200 (101 tasks)
```

### 3. Use Meaningful Variable Names
- `counter` for generic numbering
- `product_id` for product pages
- `page_num` for pagination
- `item` for list values

### 4. Tag Your Tests
Use tags for organization:
- `chhotu` - All chhotu.com tests
- `smoke` - Quick health checks
- `deep` - Comprehensive tests
- `prod-test` - Product testing
- `daily` - Regular monitoring

### 5. Monitor Resource Usage
```
Small test (10 tasks): ~30 seconds
Medium test (100 tasks): ~5 minutes
Large test (1000 tasks, step 10): ~3-5 minutes
```

---

## 🔍 Troubleshooting

### Issue: Tasks timing out
**Solution**: Increase timeout from 60 to 120 seconds

### Issue: Selectors not found
**Solution**: 
1. Open chhotu.com manually
2. Verify element exists on page
3. Use browser DevTools to find correct selector
4. Try alternatives: `h1`, `.title`, `#page-title`

### Issue: Pages load slowly
**Solution**: Increase wait time from 2 to 5 seconds

### Issue: Too many tasks created
**Solution**: Check your step value
- Range 1-100, Step 1 = 100 tasks
- Range 1-100, Step 10 = 10 tasks

---

## 📈 Analyzing Results

### View in FlowPilot UI
1. Click "Batch Progress" tab for aggregate stats
2. Filter by tag: Select "chhotu" from tag dropdown
3. Click individual tasks to see details

### Export for Analysis
1. Click "Export CSV" button
2. Open in Excel/Google Sheets
3. Analyze columns:
   - Task Name
   - Status
   - Extracted Data (JSON)
   - Duration
   - Error Message (if failed)

### Check Screenshots
Navigate to screenshots folder (shown in logs) and review visual captures.

---

## 🎉 Next Steps

### Once Basic Test Works

1. **Scale Up**: Increase range to test more pages
2. **Add Interactions**: Add click/type steps
3. **Extract More Data**: Add more extract steps for prices, descriptions, etc.
4. **Schedule**: Use Schedule tab for recurring tests
5. **Use Proxies**: Configure proxies for distributed testing

### Advanced Scenarios

Explore `chhotu_com_scenarios.md` for:
- User journey simulations
- Search functionality testing
- API endpoint validation
- Multi-step flows

---

## 🛟 Need Help?

- **Documentation**: See `REPEAT_TASK_FEATURE.md`
- **More Examples**: See `chhotu_com_scenarios.md`
- **Technical Details**: See `IMPLEMENTATION_SUMMARY.md`

---

**Ready to test chhotu.com at scale? Start with the 5-minute setup above!** 🚀
