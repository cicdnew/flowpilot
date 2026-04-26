# Pattern Library - Reusable Test Patterns

## 🎯 Quick Reference

Common patterns you can copy and customize for your tests.

---

## 1. Basic Patterns

### Simple Navigate + Extract
```json
{
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/page/{{counter}}"},
    {"action": "wait", "value": "2"},
    {"action": "extract", "selector": "h1", "value": "title_{{counter}}"},
    {"action": "screenshot", "value": "page_{{counter}}"}
  ]
}
```

### Search + Click + Extract
```json
{
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk"},
    {"action": "type", "selector": "#search", "value": "query {{counter}}"},
    {"action": "click", "selector": "#search-btn"},
    {"action": "wait", "value": "2"},
    {"action": "extract", "selector": ".result-count", "value": "results_{{counter}}"}
  ]
}
```

---

## 2. E-commerce Patterns

### Add to Cart Flow
```json
{
  "steps": [
    {"action": "navigate", "value": "https://shop.com/product/{{product_id}}"},
    {"action": "wait", "value": "2"},
    {"action": "extract", "selector": ".price", "value": "price_{{product_id}}"},
    {"action": "click", "selector": "#add-to-cart"},
    {"action": "wait", "value": "1"},
    {"action": "extract", "selector": ".cart-count", "value": "cart_{{product_id}}"}
  ]
}
```

### Product Comparison
```json
{
  "steps": [
    {"action": "navigate", "value": "https://shop.com/product/{{product_id}}"},
    {"action": "extract", "selector": ".name", "value": "name_{{product_id}}"},
    {"action": "extract", "selector": ".price", "value": "price_{{product_id}}"},
    {"action": "extract", "selector": ".rating", "value": "rating_{{product_id}}"},
    {"action": "extract", "selector": ".reviews", "value": "reviews_{{product_id}}"}
  ]
}
```

---

## 3. Form Patterns

### Simple Form Submission
```json
{
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/contact"},
    {"action": "type", "selector": "#name", "value": "User {{counter}}"},
    {"action": "type", "selector": "#email", "value": "user{{counter}}@test.com"},
    {"action": "type", "selector": "#message", "value": "Test message {{counter}}"},
    {"action": "click", "selector": "#submit"},
    {"action": "wait", "value": "2"},
    {"action": "extract", "selector": ".confirmation", "value": "result_{{counter}}"}
  ]
}
```

### Multi-Step Form
```json
{
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/signup"},
    {"action": "type", "selector": "#username", "value": "user{{counter}}"},
    {"action": "type", "selector": "#email", "value": "user{{counter}}@test.com"},
    {"action": "click", "selector": ".next-step"},
    {"action": "wait", "value": "1"},
    {"action": "type", "selector": "#password", "value": "Pass{{counter}}!"},
    {"action": "type", "selector": "#confirm-password", "value": "Pass{{counter}}!"},
    {"action": "click", "selector": ".submit"}
  ]
}
```

---

## 4. Authentication Patterns

### Login Flow
```json
{
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/login"},
    {"action": "type", "selector": "#username", "value": "testuser{{counter}}"},
    {"action": "type", "selector": "#password", "value": "TestPass{{counter}}!"},
    {"action": "click", "selector": "#login"},
    {"action": "wait", "value": "3"},
    {"action": "screenshot", "value": "logged_in_{{counter}}"}
  ]
}
```

### Login + Navigate + Extract
```json
{
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/login"},
    {"action": "type", "selector": "#username", "value": "user{{counter}}"},
    {"action": "type", "selector": "#password", "value": "pass{{counter}}"},
    {"action": "click", "selector": "#login"},
    {"action": "wait", "value": "2"},
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/dashboard"},
    {"action": "extract", "selector": ".balance", "value": "balance_{{counter}}"},
    {"action": "extract", "selector": ".username", "value": "name_{{counter}}"}
  ]
}
```

---

## 5. Validation Patterns

### Element Existence Check
```json
{
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/page/{{counter}}"},
    {"action": "wait", "value": "2"},
    {"action": "if_element_exists", "selector": ".error-message", "jumpTo": "error_handler"},
    {"action": "screenshot", "value": "success_{{counter}}"},
    {"action": "goto", "value": "end"},
    {"action": "label", "value": "error_handler"},
    {"action": "screenshot", "value": "error_{{counter}}"},
    {"action": "label", "value": "end"}
  ]
}
```

### Content Validation
```json
{
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/page/{{counter}}"},
    {"action": "wait", "value": "2"},
    {"action": "extract", "selector": "h1", "value": "title_{{counter}}"},
    {"action": "extract", "selector": ".author", "value": "author_{{counter}}"},
    {"action": "extract", "selector": ".date", "value": "date_{{counter}}"},
    {"action": "if_text_contains", "selector": ".status", "value": "Active", "jumpTo": "active_flow"}
  ]
}
```

---

## 6. Scraping Patterns

### Data Extraction
```json
{
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/data/{{counter}}"},
    {"action": "wait", "value": "2"},
    {"action": "scroll", "value": "bottom"},
    {"action": "wait", "value": "1"},
    {"action": "extract", "selector": ".title", "value": "title_{{counter}}"},
    {"action": "extract", "selector": ".content", "value": "content_{{counter}}"},
    {"action": "extract", "selector": ".meta", "value": "meta_{{counter}}"}
  ]
}
```

### Paginated Scraping
```json
{
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/list?page={{counter}}"},
    {"action": "wait", "value": "2"},
    {"action": "extract", "selector": ".item:nth-child(1) .title", "value": "p{{counter}}_item1"},
    {"action": "extract", "selector": ".item:nth-child(2) .title", "value": "p{{counter}}_item2"},
    {"action": "extract", "selector": ".item:nth-child(3) .title", "value": "p{{counter}}_item3"},
    {"action": "screenshot", "value": "page_{{counter}}"}
  ]
}
```

---

## 7. Performance Patterns

### Load Time Measurement
```json
{
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/page/{{counter}}"},
    {"action": "wait", "value": "2"},
    {"action": "eval", "value": "performance.timing"},
    {"action": "extract", "selector": "body", "value": "timing_{{counter}}"},
    {"action": "eval", "value": "document.querySelectorAll('*').length"},
    {"action": "extract", "selector": "body", "value": "dom_count_{{counter}}"}
  ]
}
```

---

## 8. Mobile/Responsive Patterns

### Multi-Device Testing
```json
{
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/page/{{counter}}"},
    {"action": "wait", "value": "2"},
    {"action": "screenshot", "value": "desktop_{{counter}}"},
    {"action": "emulate_device", "value": "mobile"},
    {"action": "wait", "value": "1"},
    {"action": "screenshot", "value": "mobile_{{counter}}"},
    {"action": "emulate_device", "value": "tablet"},
    {"action": "screenshot", "value": "tablet_{{counter}}"}
  ]
}
```

---

## 9. Interaction Patterns

### Click Through Flow
```json
{
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk"},
    {"action": "click", "selector": ".menu-item:nth-child({{counter}})"},
    {"action": "wait", "value": "2"},
    {"action": "screenshot", "value": "menu_{{counter}}"},
    {"action": "extract", "selector": ".page-title", "value": "title_{{counter}}"}
  ]
}
```

### Hover and Extract
```json
{
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/product/{{counter}}"},
    {"action": "hover", "selector": ".product-image"},
    {"action": "wait", "value": "1"},
    {"action": "screenshot", "value": "hover_{{counter}}"},
    {"action": "extract", "selector": ".tooltip", "value": "tooltip_{{counter}}"}
  ]
}
```

---

## 10. Variable Combinations

### Multiple Variables
```json
{
  "name": "User {{user_id}} - Product {{product_id}}",
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/user/{{user_id}}"},
    {"action": "click", "selector": "#product-{{product_id}}"},
    {"action": "extract", "selector": ".result", "value": "u{{user_id}}_p{{product_id}}"}
  ]
}
```

### Index + Value
```json
{
  "name": "Test #{{index}} - {{category}}",
  "steps": [
    {"action": "navigate", "value": "http://chhotu-bin.infy.uk/{{category}}"},
    {"action": "extract", "selector": ".count", "value": "test{{index}}_{{category}}_count"}
  ]
}
```

---

## 💡 Customization Tips

### Replace These Values
- `chhotu-bin.infy.uk` → Your domain
- `#search`, `.title` → Your actual selectors
- `counter`, `product_id` → Your variable names
- Wait times → Adjust for your site's speed

### Finding Selectors
1. Open your site in Chrome
2. Right-click element → Inspect
3. In DevTools, right-click element → Copy → Copy selector
4. Paste into FlowPilot

### Testing Pattern
1. Start with 2-3 tasks
2. Verify selectors work
3. Check extracted data
4. Scale up gradually

---

## 📚 Related Resources

- `ADVANCED_TEST_FLOWS.md` - Complex scenarios
- `FLOW_TEMPLATES.json` - Ready-to-use configs
- `chhotu_com_scenarios.md` - Domain-specific examples
