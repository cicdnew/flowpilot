//go:build realweb

// Real-website, multi-action scenarios for the browser Runner.
//
// These tests drive real Chrome (a visible window when DISPLAY is set; set
// REALWEB_HEADLESS=1 for headless) against public sites that are built
// for automation practice and explicitly allow scraping:
//
//	the-internet.herokuapp.com, quotes.toscrape.com, books.toscrape.com,
//	httpbin.org
//
// They need network access, so they are opt-in:
//
//	go test -tags=realweb -v -run RealWeb ./internal/browser/
//
// Set REALWEB_ARTIFACTS=/some/dir to keep screenshots after the run.
package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"flowpilot/internal/models"
)

const (
	theInternet = "https://the-internet.herokuapp.com"
	quotesSite  = "https://quotes.toscrape.com"
	booksSite   = "https://books.toscrape.com"
	httpbinSite = "https://httpbin.org"
)

// realWebHeaded reports whether to show a visible Chrome window. Headed is the
// default when a display is available; set REALWEB_HEADLESS=1 to force headless.
func realWebHeaded() bool {
	return os.Getenv("REALWEB_HEADLESS") != "1" && os.Getenv("DISPLAY") != ""
}

func realWebChromeOpts(t *testing.T) []chromedp.ExecAllocatorOption {
	if !realWebHeaded() {
		t.Log("browser mode: headless")
		return chromeOpts()
	}
	t.Logf("browser mode: headed (visible window on DISPLAY=%s)", os.Getenv("DISPLAY"))
	return append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
	)
}

// newRealWebRunner returns a Runner (headed by default) with its own browser pool.
func newRealWebRunner(t *testing.T) *Runner {
	t.Helper()
	requireChrome(t)
	dir := os.Getenv("REALWEB_ARTIFACTS")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create artifacts dir: %v", err)
	}
	runner, err := NewRunner(dir)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	runner.SetForceHeadless(!realWebHeaded())
	runner.SetAllowEval(true)
	pool := NewBrowserPool(PoolConfig{Size: 1, MaxTabs: 2, AcquireTimeout: 60 * time.Second}, realWebChromeOpts(t))
	t.Cleanup(func() { pool.Stop() })
	runner.SetPool(pool)
	return runner
}

// requireSite skips (rather than fails) when a site is unreachable, so a
// third-party outage is not reported as a FlowPilot failure.
func requireSite(t *testing.T, url string) {
	t.Helper()
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Skipf("site unreachable, skipping: %s: %v", url, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 500 {
		t.Skipf("site unhealthy, skipping: %s: HTTP %d", url, resp.StatusCode)
	}
}

// runRealWebTask runs the task and logs a per-step trace as evidence.
func runRealWebTask(t *testing.T, runner *Runner, task models.Task) *models.TaskResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := runner.RunTask(ctx, task)
	if result == nil {
		t.Fatalf("RunTask returned nil result (err=%v)", err)
	}
	t.Logf("task %s: success=%t steps=%d duration=%s", task.ID, result.Success, len(task.Steps), result.Duration.Round(time.Millisecond))
	for _, s := range result.StepLogs {
		status := "ok"
		if s.ErrorMsg != "" {
			status = "ERROR " + s.ErrorMsg
		}
		t.Logf("  step %2d %-17s %-45q %5dms %s", s.StepIndex+1, s.Action, s.Selector, s.DurationMs, status)
	}
	for _, shot := range result.Screenshots {
		t.Logf("  screenshot: %s", shot)
	}
	if err != nil || result.Error != "" {
		t.Fatalf("task failed: err=%v result.Error=%q", err, result.Error)
	}
	return result
}

func wantEqual(t *testing.T, data map[string]string, key, want string) {
	t.Helper()
	if got := strings.TrimSpace(data[key]); got != want {
		t.Errorf("%s: got %q, want %q", key, got, want)
	} else {
		t.Logf("  verified %s = %q", key, got)
	}
}

func wantContains(t *testing.T, data map[string]string, key, want string) {
	t.Helper()
	if got := data[key]; !strings.Contains(got, want) {
		t.Errorf("%s: got %q, want it to contain %q", key, got, want)
	} else {
		t.Logf("  verified %s contains %q", key, want)
	}
}

// probe returns an eval script that writes a JS expression's value into a
// #fp-probe element, so page state can be read back with an extract step.
func probe(expr string) string {
	return `(function(){var d=document.getElementById('fp-probe');if(!d){d=document.createElement('div');d.id='fp-probe';document.body.appendChild(d);}d.textContent=String(` + expr + `);return true;})()`
}

// Login, conditional branch on URL, logout, and read the flash messages.
func TestRealWebInternetLoginLogout(t *testing.T) {
	requireSite(t, theInternet+"/login")
	runner := newRealWebRunner(t)
	result := runRealWebTask(t, runner, models.Task{
		ID: "realweb-internet-login", URL: theInternet + "/login",
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: theInternet + "/login"},
			{Action: models.ActionType, Selector: "#username", Value: "tomsmith"},
			{Action: models.ActionType, Selector: "#password", Value: "SuperSecretPassword!"},
			{Action: models.ActionClick, Selector: "button[type=submit]"},
			{Action: models.ActionWait, Selector: "#flash.success"},
			{Action: models.ActionExtract, Selector: "#flash", VarName: "login_flash"},
			// Only a successful login reaches /secure; otherwise fall through to a step that fails.
			{Action: models.ActionIfURL, Condition: "contains:/secure", JumpTo: "logout"},
			{Action: models.ActionWait, Selector: "#login-did-not-reach-secure-area", Timeout: 1000},
			{Action: models.ActionClick, Selector: "a[href='/logout']", Label: "logout"},
			{Action: models.ActionWait, Selector: "#flash"},
			{Action: models.ActionExtract, Selector: "#flash", VarName: "logout_flash"},
			{Action: models.ActionGetTitle, Value: "title"},
		},
	})
	wantContains(t, result.ExtractedData, "login_flash", "You logged into a secure area!")
	wantContains(t, result.ExtractedData, "logout_flash", "You logged out of the secure area!")
	wantEqual(t, result.ExtractedData, "title", "The Internet")
}

// Negative control: a wrong password must be rejected, proving the assertions
// above are not vacuous.
func TestRealWebInternetLoginRejectsBadPassword(t *testing.T) {
	requireSite(t, theInternet+"/login")
	runner := newRealWebRunner(t)
	result := runRealWebTask(t, runner, models.Task{
		ID: "realweb-internet-badlogin", URL: theInternet + "/login",
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: theInternet + "/login"},
			{Action: models.ActionType, Selector: "#username", Value: "tomsmith"},
			{Action: models.ActionType, Selector: "#password", Value: "wrong-password"},
			{Action: models.ActionClick, Selector: "button[type=submit]"},
			{Action: models.ActionWait, Selector: "#flash.error"},
			{Action: models.ActionExtract, Selector: "#flash", VarName: "error_flash"},
			{Action: models.ActionIfElement, Selector: "a[href='/logout']", Condition: "exists", JumpTo: "wrongly-logged-in"},
			{Action: models.ActionGetTitle, Value: "title"},
			{Action: models.ActionGoto, JumpTo: "end"},
			{Action: models.ActionWait, Selector: "#bad-password-was-accepted", Timeout: 1000, Label: "wrongly-logged-in"},
			{Action: models.ActionWait, Value: "1", Label: "end"},
		},
	})
	wantContains(t, result.ExtractedData, "error_flash", "Your password is invalid!")
}

// Dropdown, checkbox state, and waiting for content loaded asynchronously.
func TestRealWebInternetFormControlsAndDynamicContent(t *testing.T) {
	requireSite(t, theInternet+"/dropdown")
	runner := newRealWebRunner(t)
	result := runRealWebTask(t, runner, models.Task{
		ID: "realweb-internet-controls", URL: theInternet + "/dropdown",
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: theInternet + "/dropdown"},
			{Action: models.ActionSelect, Selector: "#dropdown", Value: "2"},
			{Action: models.ActionEval, Value: probe("document.querySelector('#dropdown').value")},
			{Action: models.ActionExtract, Selector: "#fp-probe", VarName: "dropdown_value"},

			{Action: models.ActionNavigate, Value: theInternet + "/checkboxes"},
			{Action: models.ActionClick, Selector: "#checkboxes input:nth-of-type(1)"},
			{Action: models.ActionClick, Selector: "#checkboxes input:nth-of-type(2)"},
			{Action: models.ActionEval, Value: probe("Array.from(document.querySelectorAll('#checkboxes input')).map(function(c){return c.checked}).join(',')")},
			{Action: models.ActionExtract, Selector: "#fp-probe", VarName: "checkbox_states"},

			{Action: models.ActionNavigate, Value: theInternet + "/dynamic_loading/1"},
			{Action: models.ActionClick, Selector: "#start button"},
			{Action: models.ActionWait, Selector: "#finish", Timeout: 20000},
			{Action: models.ActionExtract, Selector: "#finish h4", VarName: "dynamic_text"},
		},
	})
	wantEqual(t, result.ExtractedData, "dropdown_value", "2")
	// Initial state is [unchecked, checked]; clicking both flips them.
	wantEqual(t, result.ExtractedData, "checkbox_states", "true,false")
	wantEqual(t, result.ExtractedData, "dynamic_text", "Hello World!")
}

// Hover must reveal a CSS :hover caption, as it would for a real user.
func TestRealWebInternetHoverRevealsCaption(t *testing.T) {
	requireSite(t, theInternet+"/hovers")
	runner := newRealWebRunner(t)
	result := runRealWebTask(t, runner, models.Task{
		ID: "realweb-internet-hover", URL: theInternet + "/hovers",
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: theInternet + "/hovers"},
			{Action: models.ActionHover, Selector: ".figure"},
			{Action: models.ActionEval, Value: probe("getComputedStyle(document.querySelector('.figure .figcaption')).display")},
			{Action: models.ActionExtract, Selector: "#fp-probe", VarName: "caption_display"},
		},
	})
	if got := result.ExtractedData["caption_display"]; got == "none" {
		t.Errorf("hover did not reveal the caption (figcaption display=%q): the hover action does not produce a real pointer hover", got)
	} else {
		t.Logf("  verified caption visible after hover (display=%q)", got)
	}
}

// Drag-and-drop must swap the two HTML5 drag-and-drop columns.
func TestRealWebInternetDragAndDrop(t *testing.T) {
	requireSite(t, theInternet+"/drag_and_drop")
	runner := newRealWebRunner(t)
	result := runRealWebTask(t, runner, models.Task{
		ID: "realweb-internet-dnd", URL: theInternet + "/drag_and_drop",
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: theInternet + "/drag_and_drop"},
			{Action: models.ActionDragDrop, Selector: "#column-a", Target: "#column-b"},
			{Action: models.ActionExtract, Selector: "#column-a header", VarName: "column_a"},
		},
	})
	if got := strings.TrimSpace(result.ExtractedData["column_a"]); got != "B" {
		t.Errorf("drag_drop did not move column A onto B (column A header=%q, want \"B\")", got)
	} else {
		t.Logf("  verified columns swapped (column A header=%q)", got)
	}
}

// wait_function must keep polling across the navigation the click starts.
func TestRealWebQuotesWaitFunctionAcrossNavigation(t *testing.T) {
	requireSite(t, quotesSite)
	r := newRealWebRunner(t)
	res := runRealWebTask(t, r, models.Task{ID: "rw-quotes-waitfn", Name: "quotes wait_function", Steps: []models.TaskStep{
		{Action: models.ActionNavigate, Value: quotesSite + "/"},
		{Action: models.ActionClick, Selector: "li.next a"},
		{Action: models.ActionWaitFunction, Value: "location.pathname === '/page/2/' && document.readyState === 'complete'", Timeout: 15000},
		{Action: models.ActionExtract, Selector: ".quote .author", VarName: "page2_author"},
	}})
	wantContains(t, res.ExtractedData, "page2_author", "Marilyn Monroe")
}

// Log in, then paginate with a while loop while a "Next" link exists.
func TestRealWebQuotesLoginAndPaginate(t *testing.T) {
	requireSite(t, quotesSite+"/login")
	runner := newRealWebRunner(t)
	result := runRealWebTask(t, runner, models.Task{
		ID: "realweb-quotes", URL: quotesSite + "/login",
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: quotesSite + "/login"},
			{Action: models.ActionType, Selector: "#username", Value: "flowpilot"},
			{Action: models.ActionType, Selector: "#password", Value: "flowpilot"},
			{Action: models.ActionClick, Selector: "input[type=submit]"},
			{Action: models.ActionWait, Selector: "a[href='/logout']"},
			{Action: models.ActionExtract, Selector: ".quote .author", VarName: "page1_author"},
			{Action: models.ActionWhile, Selector: "li.next a", Condition: "exists", MaxLoops: 3},
			// Mark the current document, click, then wait until the marker is
			// gone (new document) before touching the next page. Waiting on
			// ".quote" alone passes instantly on the old page, and wait_function
			// cannot poll across a navigation (CDP -32000), so use DOM waits.
			{Action: models.ActionEval, Value: probe("'stale'")},
			{Action: models.ActionClick, Selector: "li.next a"},
			{Action: models.ActionWaitNotPresent, Selector: "#fp-probe", Timeout: 15000},
			{Action: models.ActionWait, Selector: ".quote .author"},
			{Action: models.ActionEndWhile},
			{Action: models.ActionIfURL, Condition: "contains:/page/4/", JumpTo: "on-page-4"},
			{Action: models.ActionWait, Selector: "#loop-did-not-reach-page-4", Timeout: 1000},
			{Action: models.ActionExtract, Selector: ".quote .author", VarName: "page4_author", Label: "on-page-4"},
			{Action: models.ActionExtract, Selector: "a[href='/logout']", VarName: "still_logged_in"},
		},
	})
	wantEqual(t, result.ExtractedData, "page1_author", "Albert Einstein")
	wantEqual(t, result.ExtractedData, "page4_author", "Dr. Seuss")
	wantEqual(t, result.ExtractedData, "still_logged_in", "Logout")
}

// Browse a category, open a product, use back/forward/reload, cookies, screenshot.
func TestRealWebBooksBrowseAndHistory(t *testing.T) {
	requireSite(t, booksSite+"/")
	runner := newRealWebRunner(t)
	result := runRealWebTask(t, runner, models.Task{
		ID: "realweb-books", URL: booksSite + "/",
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: booksSite + "/"},
			{Action: models.ActionClick, Selector: "a[href='catalogue/category/books/poetry_23/index.html']"},
			{Action: models.ActionWait, Selector: ".page-header h1"},
			{Action: models.ActionExtract, Selector: ".page-header h1", VarName: "category"},
			{Action: models.ActionClick, Selector: "article.product_pod h3 a"},
			{Action: models.ActionWait, Selector: ".product_main h1"},
			{Action: models.ActionExtract, Selector: ".product_main h1", VarName: "book_title"},
			{Action: models.ActionExtract, Selector: ".product_main .price_color", VarName: "book_price"},
			{Action: models.ActionNavigateBack},
			{Action: models.ActionWait, Selector: ".page-header h1"},
			{Action: models.ActionExtract, Selector: ".page-header h1", VarName: "after_back"},
			{Action: models.ActionNavigateForward},
			{Action: models.ActionWait, Selector: ".product_main h1"},
			{Action: models.ActionReload},
			{Action: models.ActionWait, Selector: ".product_main h1"},
			{Action: models.ActionExtract, Selector: ".product_main h1", VarName: "after_reload"},
			{Action: models.ActionSetCookie, Name: "fp_realweb", Value: "ok", Path: "/"},
			{Action: models.ActionGetCookies, VarName: "ck"},
			{Action: models.ActionScrollIntoView, Selector: "#product_description"},
			{Action: models.ActionScreenshot},
			{Action: models.ActionGetTitle, Value: "title"},
		},
	})
	wantEqual(t, result.ExtractedData, "category", "Poetry")
	wantEqual(t, result.ExtractedData, "book_title", "A Light in the Attic")
	wantEqual(t, result.ExtractedData, "book_price", "£51.77")
	wantEqual(t, result.ExtractedData, "after_back", "Poetry")
	wantEqual(t, result.ExtractedData, "after_reload", "A Light in the Attic")
	wantContains(t, result.ExtractedData, "title", "A Light in the Attic")
	found := false
	for k, v := range result.ExtractedData {
		if strings.HasPrefix(k, "ck_") && strings.HasSuffix(k, "_name") && v == "fp_realweb" {
			found = true
		}
	}
	if !found {
		t.Errorf("cookie fp_realweb not returned by get_cookies: %v", result.ExtractedData)
	} else {
		t.Logf("  verified cookie fp_realweb round-tripped via set_cookie/get_cookies")
	}
	if len(result.Screenshots) != 1 {
		t.Fatalf("screenshots: got %d, want 1", len(result.Screenshots))
	}
	if st, err := os.Stat(result.Screenshots[0]); err != nil || st.Size() == 0 {
		t.Errorf("screenshot file missing or empty: %v", err)
	} else {
		t.Logf("  verified screenshot %d bytes", st.Size())
	}
}

// Fill and submit a real form; the server echoes back exactly what it received.
func TestRealWebHttpbinFormSubmit(t *testing.T) {
	requireSite(t, httpbinSite+"/forms/post")
	runner := newRealWebRunner(t)
	result := runRealWebTask(t, runner, models.Task{
		ID: "realweb-httpbin", URL: httpbinSite + "/forms/post",
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: httpbinSite + "/forms/post"},
			{Action: models.ActionType, Selector: "input[name=custname]", Value: "FlowPilot Test"},
			{Action: models.ActionType, Selector: "input[name=custtel]", Value: "5551234"},
			{Action: models.ActionType, Selector: "input[name=custemail]", Value: "qa@example.com"},
			{Action: models.ActionClick, Selector: "input[name=size][value=medium]"},
			{Action: models.ActionClick, Selector: "input[name=topping][value=cheese]"},
			{Action: models.ActionClick, Selector: "input[name=topping][value=onion]"},
			{Action: models.ActionType, Selector: "textarea[name=comments]", Value: "multi-action run"},
			{Action: models.ActionSubmitForm, Selector: "form"},
			{Action: models.ActionWait, Selector: "pre", Timeout: 20000},
			{Action: models.ActionExtract, Selector: "pre", VarName: "response"},
		},
	})
	var echoed struct {
		Form map[string]any `json:"form"`
	}
	if err := json.Unmarshal([]byte(result.ExtractedData["response"]), &echoed); err != nil {
		t.Fatalf("response is not JSON: %v\n%s", err, result.ExtractedData["response"])
	}
	t.Logf("  server received form: %v", echoed.Form)
	for field, want := range map[string]string{
		"custname": "FlowPilot Test", "custtel": "5551234", "custemail": "qa@example.com",
		"size": "medium", "comments": "multi-action run",
	} {
		if got, _ := echoed.Form[field].(string); got != want {
			t.Errorf("form %s: got %q, want %q", field, got, want)
		}
	}
	toppings, _ := echoed.Form["topping"].([]any)
	if len(toppings) != 2 || toppings[0] != "cheese" || toppings[1] != "onion" {
		t.Errorf("form topping: got %v, want [cheese onion]", echoed.Form["topping"])
	}
}
