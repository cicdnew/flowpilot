# Troubleshooting Guide - Repeated Tasks

## 🔧 Common Issues and Solutions

Quick reference for solving common problems with repeated tasks in FlowPilot.

---

## Table of Contents

1. [Task Creation Issues](#task-creation-issues)
2. [Execution Problems](#execution-problems)
3. [Selector Issues](#selector-issues)
4. [Variable Substitution](#variable-substitution)
5. [Performance Issues](#performance-issues)
6. [Data Extraction](#data-extraction)
7. [Network and Timeouts](#network-and-timeouts)
8. [Queue and Batch](#queue-and-batch)

---

## Task Creation Issues

### Issue: "Variable name is required"

**Symptom**: Error when clicking "Create Tasks"

**Solution**:
```
✗ Wrong: Leave varName empty
✓ Right: Enter a variable name like "counter", "id", "page"
```

### Issue: "Start value must be <= end value"

**Symptom**: Validation error in range mode

**Solution**:
```
✗ Wrong: Start: 100, End: 50
✓ Right: Start: 50, End: 100
```

### Issue: "Step must be > 0"

**Symptom**: Cannot create tasks with step value

**Solution**:
```
✗ Wrong: Step: 0 or negative
✓ Right: Step: 1 (or any positive number)
```

### Issue: "Values list cannot be empty"

**Symptom**: Error in list mode

**Solution**:
```
✗ Wrong: Empty text area in list mode
✓ Right: Enter at least one value per line
```

Example:
```
electronics
clothing
home
```

### Issue: Too many tasks created

**Symptom**: Created 1000+ tasks when expecting 100

**Solution**: Check your step value
```
Range: 1 to 1000, Step: 1 = 1000 tasks
Range: 1 to 1000, Step: 10 = 100 tasks ✓
```

Formula: `(EndVal - StartVal) / Step + 1`

---

## Execution Problems

### Issue: Tasks stuck in "Queued" status

**Possible Causes**:
1. Queue at max concurrency
2. Proxy limit reached
3. Batch is paused

**Solutions**:
```bash
# Check queue status
- View "Batch Progress" tab
- Check running task count
- Verify batch isn't paused

# Increase concurrency (if system can handle it)
- Adjust MaxConcurrency in settings
```

### Issue: All tasks timing out

**Symptom**: Tasks fail with "timeout exceeded"

**Solution 1**: Increase timeout
```json
{
  "timeout": 120  // Increase from 60 to 120 seconds
}
```

**Solution 2**: Add more wait time
```json
{
  "steps": [
    {"action": "navigate", "value": "..."},
    {"action": "wait", "value": "5"}  // Increase from 2 to 5
  ]
}
```

**Solution 3**: Use wait_for_selector (when available)
```json
{
  "action": "wait_for_selector",
  "selector": ".content-loaded",
  "timeout": 10
}
```

### Issue: Tasks fail immediately

**Symptom**: Tasks move from Queued → Failed instantly

**Check**:
1. URL is accessible (try manually in browser)
2. Selectors exist on page
3. No JavaScript errors blocking page load
4. Proxy configuration (if using)

**Debug steps**:
```
1. Click failed task to view logs
2. Check error message
3. Look at screenshot (if captured)
4. Try with headless: false to see browser
```

---

## Selector Issues

### Issue: "Selector not found"

**Symptom**: Step fails with selector not found error

**Solution**: Find the correct selector

**Method 1 - Chrome DevTools**:
```
1. Open page in Chrome
2. Right-click element → Inspect
3. In DevTools, right-click element
4. Copy → Copy selector
5. Paste into FlowPilot
```

**Method 2 - Try alternatives**:
```css
/* Try multiple selectors */
"h1, .page-title, .title, #title"

/* Use nth-child for specific items */
".product-card:nth-child(1)"

/* Use attribute selectors */
"button[type='submit']"
"input[name='email']"
```

### Issue: Selector works manually but not in automation

**Possible Causes**:
1. Element not loaded yet
2. Element in iframe
3. Element hidden/not visible
4. Dynamic selector (ID changes)

**Solutions**:
```json
// Add wait before action
{"action": "wait", "value": "3"},
{"action": "click", "selector": ".my-selector"}

// Use more stable selectors (avoid auto-generated IDs)
✗ "#button-abc123-xyz"
✓ ".submit-button"
✓ "button[type='submit']"
```

### Issue: Multiple elements match selector

**Symptom**: Extracts/clicks wrong element

**Solution**: Make selector more specific
```css
/* Too broad */
".title"

/* More specific */
".product-card .title"
".product-card:first-child .title"
".product-card:nth-child(2) .title"
```

---

## Variable Substitution

### Issue: Variables not substituting

**Symptom**: Literal `{{counter}}` appears instead of value

**Check**:
1. Variable name matches exactly (case-sensitive)
2. Using correct syntax: `{{varName}}` not `{varName}` or `${varName}`
3. Variable is defined in repeat config

**Example**:
```json
{
  "repeat": {
    "varName": "counter"  // Define here
  },
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/{{counter}}"}  // Use here
  ]
}
```

### Issue: {{index}} not working

**Symptom**: {{index}} shows as literal text

**Solution**: {{index}} is always available, but check:
```
1. Are you in list mode? {{index}} goes 1, 2, 3... regardless of list values
2. Spelling: {{index}} not {{idx}} or {{i}}
```

**Example**:
```json
{
  "repeat": {
    "mode": "list",
    "varName": "category",
    "values": ["electronics", "clothing"]
  },
  "name": "Test #{{index}} - {{category}}"
}
// Results:
// "Test #1 - electronics"
// "Test #2 - clothing"
```

### Issue: Substitution in wrong field

**Where it works**:
- ✓ Task name
- ✓ URL
- ✓ Step selectors
- ✓ Step values

**Where it doesn't work**:
- ✗ Priority (same for all tasks)
- ✗ Timeout (same for all tasks)
- ✗ Tags (same for all tasks)

---

## Performance Issues

### Issue: Tasks running very slowly

**Possible Causes**:
1. Too many concurrent tasks
2. Site is rate-limiting
3. Heavy screenshots
4. Network logs enabled

**Solutions**:
```json
// Disable unnecessary logging
{
  "loggingPolicy": {
    "networkLogs": false,     // Disable if not needed
    "screenshots": false,     // Only enable for critical steps
    "stepLogs": true
  }
}

// Use headless mode
{
  "headless": true
}

// Reduce concurrency if overwhelming site
// (adjust in FlowPilot settings)
```

### Issue: System resource exhaustion

**Symptom**: Browser crashes, system slow

**Solutions**:
1. Reduce concurrent tasks
2. Use headless mode
3. Disable screenshots
4. Increase step value (fewer tasks)
5. Split into multiple smaller batches

---

## Data Extraction

### Issue: Extracted data is empty

**Check**:
1. Element exists on page (inspect manually)
2. Selector is correct
3. Element is visible (not hidden)
4. Page has loaded completely

**Debug**:
```json
{
  "steps": [
    {"action": "navigate", "value": "..."},
    {"action": "wait", "value": "5"},  // Add more wait
    {"action": "screenshot", "value": "before_extract"},  // Visual check
    {"action": "extract", "selector": ".my-data", "value": "data_{{counter}}"}
  ]
}
```

### Issue: Extracted data is cut off

**Symptom**: Only partial text extracted

**Possible Causes**:
1. Selector targets only part of text
2. Text is truncated with CSS
3. Multiple elements, only getting first

**Solutions**:
```css
/* Get full container */
".product-description"  not  ".product-description span:first-child"

/* Try parent element */
".price-container"  instead of  ".price .amount"
```

### Issue: Cannot extract from multiple elements

**Symptom**: Need data from 10 products on one page

**Solution**: Use multiple extract steps
```json
{
  "steps": [
    {"action": "extract", "selector": ".item:nth-child(1) .name", "value": "item1_name"},
    {"action": "extract", "selector": ".item:nth-child(2) .name", "value": "item2_name"},
    {"action": "extract", "selector": ".item:nth-child(3) .name", "value": "item3_name"}
  ]
}
```

Or use eval for complex extraction:
```json
{
  "action": "eval",
  "value": "Array.from(document.querySelectorAll('.item')).map(i=>i.textContent).join('|')"
}
```

---

## Network and Timeouts

### Issue: "Navigation timeout"

**Symptom**: Task fails during navigate action

**Solutions**:
```json
// Increase timeout for specific step
{
  "action": "navigate",
  "value": "https://slow-site.com",
  "timeout": 60  // Override default
}

// Or increase task-level timeout
{
  "timeout": 120  // For entire task
}
```

### Issue: Intermittent failures

**Symptom**: Same task succeeds sometimes, fails others

**Possible Causes**:
1. Network instability
2. Site performance varies
3. Rate limiting

**Solutions**:
```json
// Use retries (already configured in queue)
// Tasks will auto-retry on failure

// Add longer waits
{"action": "wait", "value": "5"}

// Use less aggressive concurrency
```

### Issue: "ERR_CONNECTION_REFUSED"

**Check**:
1. URL is correct (including http:// or https://)
2. Site is accessible manually
3. Proxy configuration (if using)
4. Firewall/network restrictions

---

## Queue and Batch

### Issue: Cannot pause batch

**Symptom**: Pause button doesn't work

**Check**:
- Batch ID is correct
- Some tasks are still queued (can't pause if all running/completed)

### Issue: Tasks not starting after resume

**Solution**: 
```
1. Check Batch Progress tab
2. Click "Resume Batch" button
3. Wait a few seconds for queue to process
```

### Issue: Duplicate tasks created

**Symptom**: Same tasks appear multiple times

**Cause**: Clicked "Create Tasks" multiple times

**Solution**:
- Delete duplicates manually
- Or use tags to filter and bulk delete

### Issue: Cannot cancel running task

**Symptom**: Task keeps running after cancel

**Reason**: Task may be in critical section

**Solution**: Wait a few seconds, task will cancel at next checkpoint

---

## Debug Checklist

When tasks fail, check in this order:

### 1. View Task Details
```
✓ Click failed task
✓ Read error message
✓ Check execution logs
✓ View screenshots (if available)
```

### 2. Validate Configuration
```
✓ URL is accessible
✓ Selectors are correct
✓ Variable names match
✓ Timeout is sufficient
```

### 3. Test Manually
```
✓ Open URL in browser
✓ Verify elements exist
✓ Check page load time
✓ Test selectors in console
```

### 4. Simplify and Test
```
✓ Remove all steps except navigate
✓ Add steps back one by one
✓ Test with 2-3 tasks first
✓ Scale up after validation
```

---

## Getting More Help

### Check Logs
```bash
# FlowPilot logs location
# Check console output in wails dev mode
# View task execution logs in UI
```

### Enable Debug Mode
```json
{
  "loggingPolicy": {
    "networkLogs": true,
    "screenshots": true,
    "stepLogs": true,
    "consoleLogs": true
  },
  "headless": false  // See browser in action
}
```

### Test in Isolation
```
1. Create single task (not repeated)
2. Test with headless: false
3. Watch browser actions
4. Fix issues
5. Convert to repeated task
```

---

## Quick Fixes Reference

| Problem | Quick Fix |
|---------|-----------|
| Timeout errors | Increase timeout to 120s |
| Selector not found | Add `wait` before step, verify selector |
| Variables not working | Check varName spelling, use {{varName}} |
| Tasks stuck queued | Check concurrency limit, proxy limit |
| Empty extractions | Add longer wait, check selector |
| Too many tasks | Check step value calculation |
| Slow execution | Disable logs, use headless mode |
| Duplicate data | Make selectors more specific |

---

## Prevention Tips

### Before Creating Large Batches

1. **Test with 2-3 tasks first**
   ```
   Start: 100, End: 102  (not 100 to 1000)
   ```

2. **Verify selectors manually**
   ```
   Open page → Inspect → Copy selector → Test
   ```

3. **Check timing**
   ```
   Add waits after navigate, click, type actions
   ```

4. **Enable debugging for test run**
   ```json
   {
     "headless": false,
     "loggingPolicy": {"screenshots": true}
   }
   ```

5. **Monitor first few tasks**
   ```
   Watch first 5 tasks complete successfully
   Then let batch continue
   ```

---

## Still Having Issues?

1. **Check Documentation**:
   - `REPEAT_TASK_FEATURE.md` - Feature guide
   - `ADVANCED_TEST_FLOWS.md` - Complex scenarios
   - `PATTERN_LIBRARY.md` - Code patterns

2. **Review Examples**:
   - `chhotu_com_scenarios.md` - Working examples
   - `FLOW_TEMPLATES.json` - Copy-paste templates

3. **Test Environment**:
   - Try on simple site first (httpbin.org)
   - Validate basic flow works
   - Then apply to target site

---

**Remember**: Start small, validate, then scale! 🚀
