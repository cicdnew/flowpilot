# Advanced Test Flows - FlowPilot Repeated Tasks

## 🚀 Complex Automation Scenarios

This guide covers advanced test flows combining multiple techniques for real-world automation needs.

---

## 📋 Table of Contents

1. [Multi-Step E-commerce Flow](#1-multi-step-e-commerce-flow)
2. [Form Submission with Dynamic Data](#2-form-submission-with-dynamic-data)
3. [Login + Navigate + Extract Pattern](#3-login--navigate--extract-pattern)
4. [Conditional Testing with If/While](#4-conditional-testing-with-ifwhile)
5. [API Testing with Authentication](#5-api-testing-with-authentication)
6. [Web Scraping with Pagination](#6-web-scraping-with-pagination)
7. [Performance Testing with Metrics](#7-performance-testing-with-metrics)
8. [Visual Regression Testing](#8-visual-regression-testing)
9. [Multi-Tab Workflow Testing](#9-multi-tab-workflow-testing)
10. [CAPTCHA Handling Flow](#10-captcha-handling-flow)

---

## 1. Multi-Step E-commerce Flow

**Use Case**: Test complete purchase flow across multiple products

### Configuration
```json
{
  "name": "E-commerce Flow - Product {{product_id}}",
  "url": "https://chhotu.com",
  "repeat": {
    "mode": "range",
    "varName": "product_id",
    "startVal": 1001,
    "endVal": 1050,
    "step": 1
  },
  "steps": [
    {
      "action": "navigate",
      "value": "https://chhotu.com"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "type",
      "selector": "#search-input",
      "value": "product-{{product_id}}"
    },
    {
      "action": "click",
      "selector": "#search-button"
    },
    {
      "action": "wait",
      "value": "3"
    },
    {
      "action": "screenshot",
      "value": "search_result_{{product_id}}"
    },
    {
      "action": "click",
      "selector": ".product-card:first-child"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "extract",
      "selector": ".product-name",
      "value": "name_{{product_id}}"
    },
    {
      "action": "extract",
      "selector": ".price",
      "value": "price_{{product_id}}"
    },
    {
      "action": "extract",
      "selector": ".stock-status",
      "value": "stock_{{product_id}}"
    },
    {
      "action": "screenshot",
      "value": "product_page_{{product_id}}"
    },
    {
      "action": "click",
      "selector": "#add-to-cart, .add-to-cart-btn"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "screenshot",
      "value": "cart_added_{{product_id}}"
    },
    {
      "action": "navigate",
      "value": "https://chhotu.com/cart"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "extract",
      "selector": ".cart-total",
      "value": "cart_total_{{product_id}}"
    },
    {
      "action": "screenshot",
      "value": "final_cart_{{product_id}}"
    }
  ],
  "priority": 8,
  "autoStart": false,
  "tags": ["e-commerce", "end-to-end", "products"],
  "timeout": 120
}
```

**What It Does**:
- Searches for each product
- Clicks first result
- Extracts product details
- Adds to cart
- Validates cart total
- Takes screenshots at each step

**Expected Duration**: ~15-20s per product (50 products = ~15min)

---

## 2. Form Submission with Dynamic Data

**Use Case**: Test form submission with different input combinations

### Configuration
```json
{
  "name": "Contact Form Test - User {{user_id}}",
  "url": "https://chhotu.com/contact",
  "repeat": {
    "mode": "counter",
    "varName": "user_id",
    "startVal": 1,
    "endVal": 100,
    "step": 1
  },
  "steps": [
    {
      "action": "navigate",
      "value": "https://chhotu.com/contact"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "type",
      "selector": "#name, input[name='name']",
      "value": "Test User {{user_id}}"
    },
    {
      "action": "type",
      "selector": "#email, input[name='email']",
      "value": "user{{user_id}}@test.com"
    },
    {
      "action": "type",
      "selector": "#phone, input[name='phone']",
      "value": "555-010{{user_id}}"
    },
    {
      "action": "select",
      "selector": "#subject, select[name='subject']",
      "value": "Support"
    },
    {
      "action": "type",
      "selector": "#message, textarea[name='message']",
      "value": "This is test message number {{user_id}} for form validation testing."
    },
    {
      "action": "screenshot",
      "value": "form_filled_{{user_id}}"
    },
    {
      "action": "click",
      "selector": "#submit, button[type='submit']"
    },
    {
      "action": "wait",
      "value": "3"
    },
    {
      "action": "screenshot",
      "value": "form_submitted_{{user_id}}"
    },
    {
      "action": "extract",
      "selector": ".success-message, .confirmation",
      "value": "confirmation_{{user_id}}"
    },
    {
      "action": "extract",
      "selector": ".reference-number, .ticket-id",
      "value": "reference_{{user_id}}"
    }
  ],
  "priority": 7,
  "tags": ["forms", "submission", "validation"],
  "timeout": 90
}
```

**Advanced Variation - Multiple Form Types**:
```json
{
  "repeat": {
    "mode": "list",
    "varName": "form_type",
    "values": ["contact", "support", "feedback", "inquiry", "complaint"]
  },
  "steps": [
    {
      "action": "navigate",
      "value": "https://chhotu.com/{{form_type}}"
    }
    // ... rest of form steps
  ]
}
```

---

## 3. Login + Navigate + Extract Pattern

**Use Case**: Test authenticated areas with different user accounts

### Configuration
```json
{
  "name": "User Dashboard Test - Account {{account_id}}",
  "url": "https://chhotu.com/login",
  "repeat": {
    "mode": "range",
    "varName": "account_id",
    "startVal": 100,
    "endVal": 200,
    "step": 1
  },
  "steps": [
    {
      "action": "navigate",
      "value": "https://chhotu.com/login"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "type",
      "selector": "#username, input[name='username']",
      "value": "testuser{{account_id}}"
    },
    {
      "action": "type",
      "selector": "#password, input[name='password']",
      "value": "TestPass{{account_id}}!"
    },
    {
      "action": "click",
      "selector": "#login-btn, button[type='submit']"
    },
    {
      "action": "wait",
      "value": "3"
    },
    {
      "action": "screenshot",
      "value": "login_success_{{account_id}}"
    },
    {
      "action": "navigate",
      "value": "https://chhotu.com/dashboard"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "extract",
      "selector": ".user-name, .username-display",
      "value": "username_{{account_id}}"
    },
    {
      "action": "extract",
      "selector": ".account-balance, .balance",
      "value": "balance_{{account_id}}"
    },
    {
      "action": "extract",
      "selector": ".order-count",
      "value": "orders_{{account_id}}"
    },
    {
      "action": "screenshot",
      "value": "dashboard_{{account_id}}"
    },
    {
      "action": "navigate",
      "value": "https://chhotu.com/profile"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "extract",
      "selector": ".email",
      "value": "email_{{account_id}}"
    },
    {
      "action": "extract",
      "selector": ".member-since",
      "value": "joined_{{account_id}}"
    },
    {
      "action": "screenshot",
      "value": "profile_{{account_id}}"
    },
    {
      "action": "click",
      "selector": "#logout, .logout-btn"
    },
    {
      "action": "wait",
      "value": "2"
    }
  ],
  "priority": 9,
  "tags": ["authentication", "dashboard", "user-data"],
  "timeout": 120
}
```

**Security Note**: Don't use real credentials in automated tests. Use test accounts only.

---

## 4. Conditional Testing with If/While

**Use Case**: Handle dynamic content with conditional logic

### Configuration with Conditions
```json
{
  "name": "Conditional Product Test {{product_id}}",
  "url": "https://chhotu.com/product/{{product_id}}",
  "repeat": {
    "mode": "range",
    "varName": "product_id",
    "startVal": 1,
    "endVal": 100,
    "step": 1
  },
  "steps": [
    {
      "action": "navigate",
      "value": "https://chhotu.com/product/{{product_id}}"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "screenshot",
      "value": "product_initial_{{product_id}}"
    },
    {
      "action": "if_element_exists",
      "selector": ".out-of-stock",
      "jumpTo": "handle_out_of_stock"
    },
    {
      "action": "extract",
      "selector": ".price",
      "value": "price_{{product_id}}"
    },
    {
      "action": "click",
      "selector": "#add-to-cart"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "screenshot",
      "value": "added_to_cart_{{product_id}}"
    },
    {
      "action": "goto",
      "value": "end"
    },
    {
      "action": "label",
      "value": "handle_out_of_stock"
    },
    {
      "action": "extract",
      "selector": ".out-of-stock-message",
      "value": "stock_status_{{product_id}}"
    },
    {
      "action": "screenshot",
      "value": "out_of_stock_{{product_id}}"
    },
    {
      "action": "label",
      "value": "end"
    }
  ],
  "priority": 8,
  "tags": ["conditional", "products", "stock-check"],
  "timeout": 90
}
```

### While Loop for Pagination
```json
{
  "name": "Paginated Results - Query {{index}}",
  "url": "https://chhotu.com/search?q=test",
  "repeat": {
    "mode": "counter",
    "varName": "query_id",
    "startVal": 1,
    "endVal": 10,
    "step": 1
  },
  "steps": [
    {
      "action": "navigate",
      "value": "https://chhotu.com/search?q=query{{query_id}}"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "label",
      "value": "page_loop"
    },
    {
      "action": "extract",
      "selector": ".result-count",
      "value": "results_{{query_id}}_page_{{index}}"
    },
    {
      "action": "screenshot",
      "value": "page_{{query_id}}_{{index}}"
    },
    {
      "action": "while_condition",
      "selector": ".next-page:not(.disabled)",
      "condition": "exists",
      "maxIterations": 10
    },
    {
      "action": "click",
      "selector": ".next-page"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "end_while"
    }
  ],
  "priority": 7,
  "tags": ["pagination", "search", "multi-page"]
}
```

---

## 5. API Testing with Authentication

**Use Case**: Test API endpoints with different parameters

### REST API Testing
```json
{
  "name": "API Endpoint Test - ID {{api_id}}",
  "url": "https://chhotu.com/api/v1/items/{{api_id}}",
  "repeat": {
    "mode": "range",
    "varName": "api_id",
    "startVal": 1000,
    "endVal": 2000,
    "step": 10
  },
  "steps": [
    {
      "action": "navigate",
      "value": "https://chhotu.com/api/login"
    },
    {
      "action": "wait",
      "value": "1"
    },
    {
      "action": "extract",
      "selector": "body",
      "value": "auth_token_{{api_id}}"
    },
    {
      "action": "navigate",
      "value": "https://chhotu.com/api/v1/items/{{api_id}}"
    },
    {
      "action": "wait",
      "value": "1"
    },
    {
      "action": "extract",
      "selector": "body",
      "value": "api_response_{{api_id}}"
    },
    {
      "action": "screenshot",
      "value": "api_{{api_id}}"
    }
  ],
  "priority": 8,
  "tags": ["api", "rest", "automation"],
  "timeout": 60,
  "headless": true
}
```

### GraphQL API Testing
```json
{
  "name": "GraphQL Query Test {{query_num}}",
  "url": "https://chhotu.com/graphql",
  "repeat": {
    "mode": "list",
    "varName": "entity_type",
    "values": ["users", "products", "orders", "reviews", "categories"]
  },
  "steps": [
    {
      "action": "navigate",
      "value": "https://chhotu.com/graphql"
    },
    {
      "action": "eval",
      "value": "fetch('/graphql', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({query:'{ {{entity_type}}(limit:10) { id name } }'})}).then(r=>r.json()).then(d=>console.log(d))"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "extract",
      "selector": "body",
      "value": "graphql_{{entity_type}}"
    }
  ],
  "priority": 7,
  "tags": ["api", "graphql"]
}
```

---

## 6. Web Scraping with Pagination

**Use Case**: Extract data from multiple pages systematically

### Configuration
```json
{
  "name": "Scrape Category - Page {{page_num}}",
  "url": "https://chhotu.com/category/electronics?page={{page_num}}",
  "repeat": {
    "mode": "range",
    "varName": "page_num",
    "startVal": 1,
    "endVal": 50,
    "step": 1
  },
  "steps": [
    {
      "action": "navigate",
      "value": "https://chhotu.com/category/electronics?page={{page_num}}"
    },
    {
      "action": "wait",
      "value": "3"
    },
    {
      "action": "scroll",
      "value": "bottom"
    },
    {
      "action": "wait",
      "value": "1"
    },
    {
      "action": "extract",
      "selector": ".product-grid .product-card:nth-child(1) .name",
      "value": "page{{page_num}}_product1_name"
    },
    {
      "action": "extract",
      "selector": ".product-grid .product-card:nth-child(1) .price",
      "value": "page{{page_num}}_product1_price"
    },
    {
      "action": "extract",
      "selector": ".product-grid .product-card:nth-child(2) .name",
      "value": "page{{page_num}}_product2_name"
    },
    {
      "action": "extract",
      "selector": ".product-grid .product-card:nth-child(2) .price",
      "value": "page{{page_num}}_product2_price"
    },
    {
      "action": "extract",
      "selector": ".pagination .total-items",
      "value": "page{{page_num}}_total_count"
    },
    {
      "action": "screenshot",
      "value": "category_page_{{page_num}}"
    }
  ],
  "priority": 6,
  "tags": ["scraping", "category", "pagination"],
  "timeout": 90,
  "loggingPolicy": {
    "networkLogs": false,
    "screenshots": true,
    "stepLogs": true
  }
}
```

### Advanced: Extract All Products on Page
```json
{
  "steps": [
    {
      "action": "navigate",
      "value": "https://chhotu.com/category/all?page={{page_num}}"
    },
    {
      "action": "wait",
      "value": "3"
    },
    {
      "action": "eval",
      "value": "Array.from(document.querySelectorAll('.product-card')).map((p,i)=>({id:i+1, name:p.querySelector('.name')?.textContent, price:p.querySelector('.price')?.textContent})).forEach((p)=>console.log(JSON.stringify(p)))"
    },
    {
      "action": "wait",
      "value": "1"
    },
    {
      "action": "extract",
      "selector": "body",
      "value": "all_products_page_{{page_num}}"
    }
  ]
}
```

---

## 7. Performance Testing with Metrics

**Use Case**: Measure page load times and performance across pages

### Configuration
```json
{
  "name": "Performance Test - Page {{page_id}}",
  "url": "https://chhotu.com/page/{{page_id}}",
  "repeat": {
    "mode": "range",
    "varName": "page_id",
    "startVal": 1,
    "endVal": 100,
    "step": 1
  },
  "steps": [
    {
      "action": "navigate",
      "value": "https://chhotu.com/page/{{page_id}}"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "eval",
      "value": "performance.timing"
    },
    {
      "action": "extract",
      "selector": "body",
      "value": "timing_{{page_id}}"
    },
    {
      "action": "eval",
      "value": "document.querySelectorAll('*').length"
    },
    {
      "action": "extract",
      "selector": "body",
      "value": "dom_elements_{{page_id}}"
    },
    {
      "action": "eval",
      "value": "performance.getEntriesByType('resource').length"
    },
    {
      "action": "extract",
      "selector": "body",
      "value": "resource_count_{{page_id}}"
    },
    {
      "action": "screenshot",
      "value": "perf_{{page_id}}"
    }
  ],
  "priority": 7,
  "tags": ["performance", "metrics", "load-time"],
  "timeout": 60,
  "loggingPolicy": {
    "networkLogs": true,
    "screenshots": false,
    "stepLogs": true
  }
}
```

**Analysis**: Export CSV and analyze load times, DOM complexity, resource counts.

---

## 8. Visual Regression Testing

**Use Case**: Compare visual appearance across versions/environments

### Configuration
```json
{
  "name": "Visual Regression - Component {{component_id}}",
  "url": "https://chhotu.com/components/{{component_id}}",
  "repeat": {
    "mode": "list",
    "varName": "component_id",
    "values": ["header", "footer", "sidebar", "product-card", "checkout", "profile"]
  },
  "steps": [
    {
      "action": "navigate",
      "value": "https://chhotu.com/components/{{component_id}}"
    },
    {
      "action": "wait",
      "value": "3"
    },
    {
      "action": "screenshot",
      "value": "baseline_{{component_id}}_desktop"
    },
    {
      "action": "emulate_device",
      "value": "mobile"
    },
    {
      "action": "wait",
      "value": "1"
    },
    {
      "action": "screenshot",
      "value": "baseline_{{component_id}}_mobile"
    },
    {
      "action": "emulate_device",
      "value": "tablet"
    },
    {
      "action": "wait",
      "value": "1"
    },
    {
      "action": "screenshot",
      "value": "baseline_{{component_id}}_tablet"
    }
  ],
  "priority": 8,
  "tags": ["visual-regression", "components", "responsive"],
  "timeout": 90
}
```

**Use FlowPilot's visual diff feature** to compare screenshots across runs.

---

## 9. Multi-Tab Workflow Testing

**Use Case**: Test workflows involving multiple tabs/windows

### Configuration
```json
{
  "name": "Multi-Tab Flow - Session {{session_id}}",
  "url": "https://chhotu.com",
  "repeat": {
    "mode": "counter",
    "varName": "session_id",
    "startVal": 1,
    "endVal": 20,
    "step": 1
  },
  "steps": [
    {
      "action": "navigate",
      "value": "https://chhotu.com"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "click",
      "selector": "a[href='/product/{{session_id}}'][target='_blank']"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "tab_switch",
      "value": "1"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "extract",
      "selector": ".product-name",
      "value": "product_tab_{{session_id}}"
    },
    {
      "action": "screenshot",
      "value": "new_tab_{{session_id}}"
    },
    {
      "action": "tab_switch",
      "value": "0"
    },
    {
      "action": "wait",
      "value": "1"
    },
    {
      "action": "screenshot",
      "value": "original_tab_{{session_id}}"
    }
  ],
  "priority": 7,
  "tags": ["multi-tab", "navigation", "tabs"]
}
```

---

## 10. CAPTCHA Handling Flow

**Use Case**: Test flows with CAPTCHA using solver integration

### Configuration
```json
{
  "name": "CAPTCHA Flow - Request {{request_id}}",
  "url": "https://chhotu.com/register",
  "repeat": {
    "mode": "counter",
    "varName": "request_id",
    "startVal": 1,
    "endVal": 10,
    "step": 1
  },
  "steps": [
    {
      "action": "navigate",
      "value": "https://chhotu.com/register"
    },
    {
      "action": "wait",
      "value": "2"
    },
    {
      "action": "type",
      "selector": "#username",
      "value": "testuser{{request_id}}"
    },
    {
      "action": "type",
      "selector": "#email",
      "value": "user{{request_id}}@test.com"
    },
    {
      "action": "type",
      "selector": "#password",
      "value": "TestPass{{request_id}}!"
    },
    {
      "action": "screenshot",
      "value": "before_captcha_{{request_id}}"
    },
    {
      "action": "solve_captcha",
      "selector": ".g-recaptcha, .h-captcha",
      "value": "recaptcha"
    },
    {
      "action": "wait",
      "value": "3"
    },
    {
      "action": "screenshot",
      "value": "after_captcha_{{request_id}}"
    },
    {
      "action": "click",
      "selector": "#submit"
    },
    {
      "action": "wait",
      "value": "3"
    },
    {
      "action": "extract",
      "selector": ".success-message",
      "value": "registration_result_{{request_id}}"
    },
    {
      "action": "screenshot",
      "value": "registration_complete_{{request_id}}"
    }
  ],
  "priority": 9,
  "tags": ["captcha", "registration", "automation"],
  "timeout": 120
}
```

**Note**: Requires CAPTCHA solver configuration in FlowPilot settings.

---

## 💡 Best Practices for Advanced Flows

### 1. Error Handling
- Use `if_element_exists` to handle optional elements
- Set appropriate timeouts (60-120s for complex flows)
- Use `wait_for_selector` instead of fixed waits when possible

### 2. Performance
- Disable network logs for large batches (unless debugging)
- Take screenshots only at critical steps
- Use headless mode when visual validation isn't needed

### 3. Data Extraction
- Use unique extraction keys: `{{varName}}_{{index}}_description`
- Extract timestamps for performance analysis
- Capture error messages for debugging

### 4. Maintainability
- Use descriptive task names
- Tag flows by category
- Document selector logic in comments (using screenshot names)

### 5. Scalability
- Start with small ranges (10-20) to validate
- Use step values for large datasets
- Monitor system resources during execution

---

## 🎯 Combining Patterns

### Pattern: E-commerce + Performance + Visual
```json
{
  "name": "Full Product Test {{product_id}}",
  "steps": [
    // Navigate
    {"action": "navigate", "value": "https://chhotu.com/product/{{product_id}}"},
    
    // Performance metrics
    {"action": "eval", "value": "performance.timing"},
    {"action": "extract", "selector": "body", "value": "timing_{{product_id}}"},
    
    // Visual regression
    {"action": "screenshot", "value": "desktop_{{product_id}}"},
    {"action": "emulate_device", "value": "mobile"},
    {"action": "screenshot", "value": "mobile_{{product_id}}"},
    
    // Functionality test
    {"action": "click", "selector": "#add-to-cart"},
    {"action": "extract", "selector": ".cart-count", "value": "cart_{{product_id}}"},
    
    // Final validation
    {"action": "screenshot", "value": "final_{{product_id}}"}
  ]
}
```

---

## 📊 Example Results Analysis

After running advanced flows, analyze results:

### CSV Export Columns
```
task_id, name, status, duration_ms, 
timing_data, cart_total, stock_status, 
error_message, screenshots
```

### SQL-like Analysis (in spreadsheet)
```sql
-- Average load time
=AVERAGE(timing_column)

-- Success rate
=COUNTIF(status, "completed") / COUNT(status)

-- Failed products
=FILTER(name, status="failed")

-- Price range analysis
=MIN(price_column), MAX(price_column), AVERAGE(price_column)
```

---

## 🚀 Next Steps

1. **Start Simple**: Begin with one advanced pattern
2. **Validate**: Run 2-3 tasks to verify selectors
3. **Scale Up**: Increase range once validated
4. **Monitor**: Watch first few executions
5. **Analyze**: Export and review results
6. **Iterate**: Refine based on findings

---

## 📚 Related Documentation

- `REPEAT_TASK_FEATURE.md` - Core feature documentation
- `examples/chhotu_com_scenarios.md` - Basic scenarios
- `DEMO_GUIDE.md` - Getting started guide
- FlowPilot action reference - See `internal/models/task.go`

---

**These advanced flows showcase the full power of FlowPilot's repeated task system!** 🚀
