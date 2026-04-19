# Chhotu.com Testing Scenarios

## Overview
Sample repeated task configurations for testing chhotu.com domain with various parameters and scenarios.

## Scenario 1: Basic Page Range Test (100-200)

**Use Case**: Test all pages from 100 to 200 to verify page existence and basic content

```json
{
  "name": "Chhotu.com Page {{counter}}",
  "url": "https://chhotu.com/page/{{counter}}",
  "repeat": {
    "mode": "range",
    "varName": "counter",
    "startVal": 100,
    "endVal": 200,
    "step": 1
  },
  "steps": [
    {"action": "navigate", "value": "https://chhotu.com/page/{{counter}}"},
    {"action": "wait", "value": "2"},
    {"action": "screenshot", "value": "chhotu_page_{{counter}}"},
    {"action": "extract", "selector": "h1", "value": "title_{{counter}}"}
  ],
  "priority": 5,
  "autoStart": true,
  "tags": ["chhotu", "pages-100-200"]
}
```

**Creates**: 101 tasks testing pages 100-200
**Duration**: ~5 minutes (assuming 3s per task)
**Result**: Screenshots + extracted titles for each page

---

## Scenario 2: Large Scale Test (1-1000, step 10)

**Use Case**: Quick smoke test across wide range of pages

```json
{
  "name": "Chhotu.com Quick Test {{counter}}",
  "url": "https://chhotu.com/page/{{counter}}",
  "repeat": {
    "mode": "range",
    "varName": "counter",
    "startVal": 1,
    "endVal": 1000,
    "step": 10
  },
  "steps": [
    {"action": "navigate", "value": "https://chhotu.com/page/{{counter}}"},
    {"action": "wait", "value": "1"},
    {"action": "get_title", "value": "page_{{counter}}_title"}
  ],
  "priority": 7,
  "autoStart": true,
  "tags": ["chhotu", "smoke-test", "1000-pages"]
}
```

**Creates**: 100 tasks (1, 11, 21, 31... 991)
**Duration**: ~2-3 minutes
**Result**: Quick validation of page accessibility

---

## Scenario 3: Product Page Deep Test

**Use Case**: Comprehensive testing of product pages with form interactions

```json
{
  "name": "Chhotu Product {{product_id}}",
  "url": "https://chhotu.com/product/{{product_id}}",
  "repeat": {
    "mode": "range",
    "varName": "product_id",
    "startVal": 1001,
    "endVal": 1100,
    "step": 1
  },
  "steps": [
    {"action": "navigate", "value": "https://chhotu.com/product/{{product_id}}"},
    {"action": "wait", "value": "2"},
    {"action": "screenshot", "value": "product_{{product_id}}_initial"},
    {"action": "extract", "selector": ".product-name", "value": "name_{{product_id}}"},
    {"action": "extract", "selector": ".price", "value": "price_{{product_id}}"},
    {"action": "extract", "selector": ".stock-status", "value": "stock_{{product_id}}"},
    {"action": "click", "selector": "#add-to-cart, .add-to-cart-btn"},
    {"action": "wait", "value": "1"},
    {"action": "screenshot", "value": "product_{{product_id}}_after_add"}
  ],
  "priority": 8,
  "autoStart": false,
  "tags": ["chhotu", "products", "deep-test"],
  "timeout": 90
}
```

**Creates**: 100 tasks testing products 1001-1100
**Duration**: ~8-10 minutes
**Result**: Product info + before/after screenshots

---

## Scenario 4: Category Testing (List Mode)

**Use Case**: Test different categories/sections of the site

```json
{
  "name": "Chhotu Category {{category}}",
  "url": "https://chhotu.com/category/{{category}}",
  "repeat": {
    "mode": "list",
    "varName": "category",
    "values": [
      "electronics",
      "clothing",
      "home-garden",
      "sports-outdoors",
      "books-media",
      "toys-games",
      "health-beauty",
      "automotive",
      "food-grocery",
      "pet-supplies"
    ]
  },
  "steps": [
    {"action": "navigate", "value": "https://chhotu.com/category/{{category}}"},
    {"action": "wait", "value": "3"},
    {"action": "screenshot", "value": "category_{{category}}"},
    {"action": "extract", "selector": ".item-count", "value": "{{category}}_count"},
    {"action": "extract", "selector": ".category-description", "value": "{{category}}_desc"},
    {"action": "click", "selector": ".sort-by-price"},
    {"action": "wait", "value": "2"},
    {"action": "screenshot", "value": "category_{{category}}_sorted"}
  ],
  "priority": 9,
  "autoStart": true,
  "tags": ["chhotu", "categories"]
}
```

**Creates**: 10 tasks (one per category)
**Duration**: ~1-2 minutes
**Result**: Category screenshots + item counts

---

## Scenario 5: Search Query Testing

**Use Case**: Test search functionality with various queries

```json
{
  "name": "Chhotu Search Test #{{index}}",
  "url": "https://chhotu.com/search?q={{query}}",
  "repeat": {
    "mode": "list",
    "varName": "query",
    "values": [
      "laptop",
      "phone",
      "tablet",
      "headphones",
      "camera",
      "watch",
      "keyboard",
      "mouse",
      "monitor",
      "speaker"
    ]
  },
  "steps": [
    {"action": "navigate", "value": "https://chhotu.com"},
    {"action": "type", "selector": "#search-input, input[type='search']", "value": "{{query}}"},
    {"action": "click", "selector": "#search-button, button[type='submit']"},
    {"action": "wait", "value": "3"},
    {"action": "screenshot", "value": "search_{{query}}"},
    {"action": "extract", "selector": ".result-count", "value": "results_{{query}}"}
  ],
  "priority": 6,
  "autoStart": true,
  "tags": ["chhotu", "search-test"]
}
```

**Creates**: 10 tasks (one per search query)
**Duration**: ~1-2 minutes
**Result**: Search result screenshots + counts

---

## Scenario 6: User Journey Simulation

**Use Case**: Simulate realistic user behavior across multiple pages

```json
{
  "name": "Chhotu User Journey {{user_id}}",
  "url": "https://chhotu.com",
  "repeat": {
    "mode": "counter",
    "varName": "user_id",
    "startVal": 1,
    "endVal": 50,
    "step": 1
  },
  "steps": [
    {"action": "navigate", "value": "https://chhotu.com"},
    {"action": "wait", "value": "2"},
    {"action": "click", "selector": ".featured-product:nth-child({{user_id}})"},
    {"action": "wait", "value": "2"},
    {"action": "screenshot", "value": "user{{user_id}}_product_view"},
    {"action": "click", "selector": "#add-to-cart"},
    {"action": "wait", "value": "1"},
    {"action": "click", "selector": ".cart-icon, #view-cart"},
    {"action": "wait", "value": "2"},
    {"action": "screenshot", "value": "user{{user_id}}_cart"},
    {"action": "extract", "selector": ".cart-total", "value": "user{{user_id}}_total"}
  ],
  "priority": 5,
  "autoStart": false,
  "tags": ["chhotu", "user-journey", "e2e"]
}
```

**Creates**: 50 tasks simulating user journeys
**Duration**: ~5-7 minutes
**Result**: End-to-end flow validation

---

## Scenario 7: API Endpoint Testing

**Use Case**: Test API endpoints with different parameters

```json
{
  "name": "Chhotu API Item {{item_id}}",
  "url": "https://chhotu.com/api/v1/items/{{item_id}}",
  "repeat": {
    "mode": "range",
    "varName": "item_id",
    "startVal": 5000,
    "endVal": 5100,
    "step": 1
  },
  "steps": [
    {"action": "navigate", "value": "https://chhotu.com/api/v1/items/{{item_id}}"},
    {"action": "wait", "value": "1"},
    {"action": "extract", "selector": "body", "value": "api_response_{{item_id}}"}
  ],
  "priority": 7,
  "autoStart": true,
  "tags": ["chhotu", "api-test"],
  "headless": true
}
```

**Creates**: 101 tasks testing API endpoints 5000-5100
**Duration**: ~2-3 minutes
**Result**: API response data for each endpoint

---

## Usage Instructions

### Method 1: Via UI (Recommended)
1. Start FlowPilot: `wails dev`
2. Click "**+ Repeat Task**" button
3. Copy values from desired scenario above
4. Adjust parameters as needed
5. Click "Create Tasks"

### Method 2: Import JSON (Advanced)
```bash
# Use the sample JSON file
cat examples/chhotu_com_sample.json
```

### Method 3: Programmatically
```go
// In Go code
input := models.RepeatTaskInput{
    Name: "Chhotu.com Page {{counter}}",
    URL:  "https://chhotu.com/page/{{counter}}",
    // ... rest of config
}
group, err := app.CreateRepeatedTask(input)
```

---

## Best Practices for Chhotu.com Testing

1. **Start Small**: Test 2-3 pages first to validate your configuration
2. **Use Tags**: Tag all chhotu.com tests for easy filtering
3. **Monitor Load**: Don't overwhelm the server - use reasonable step values
4. **Check Selectors**: Verify CSS selectors match actual site structure
5. **Adjust Timeouts**: Increase timeout for slow-loading pages
6. **Enable Logging**: Use networkLogs and screenshots for debugging
7. **Use Headless**: Enable headless mode for faster execution

---

## Expected Results

### Success Indicators
- ✓ All tasks complete with "Completed" status
- ✓ Screenshots captured for each page
- ✓ Extracted data appears in task results
- ✓ No timeout errors

### Common Issues & Solutions

**Issue**: Many tasks failing with timeout
**Solution**: Increase timeout value or reduce concurrent tasks

**Issue**: Selectors not found
**Solution**: Verify selectors using browser DevTools on actual site

**Issue**: Pages loading slowly
**Solution**: Add longer wait times or use wait_for_selector action

**Issue**: Too many requests
**Solution**: Increase step value or reduce range

---

## Monitoring Your Tests

1. **Task List**: View all tasks and their status
2. **Batch Progress**: See aggregate completion percentage
3. **Individual Results**: Click any task to view logs and extracted data
4. **Export Results**: Use CSV/JSON export for analysis
5. **Screenshots**: Check screenshots folder for visual validation

---

## Next Steps

After running tests:
1. Review extracted data in CSV export
2. Check screenshots for visual issues
3. Analyze patterns in failed tasks
4. Adjust selectors/timeouts as needed
5. Create focused tests for problem areas

For more details, see `REPEAT_TASK_FEATURE.md` and `DEMO_GUIDE.md`.
