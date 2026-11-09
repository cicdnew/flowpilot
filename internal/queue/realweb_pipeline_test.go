//go:build realweb

// End-to-end: real websites through the real queue, database and browser
// runner. Opt-in because it needs network access:
//
//	go test -tags=realweb -v -run RealWeb ./internal/queue/
package queue

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"flowpilot/internal/browser"
	"flowpilot/internal/crypto"
	"flowpilot/internal/database"
	"flowpilot/internal/models"
)

func TestRealWebPipelineThroughQueue(t *testing.T) {
	if _, err := exec.LookPath("google-chrome"); err != nil {
		if _, err := exec.LookPath("chromium"); err != nil {
			t.Skip("Chrome not found")
		}
	}
	for _, u := range []string{
		"https://the-internet.herokuapp.com/login", "https://quotes.toscrape.com/",
		"https://books.toscrape.com/", "https://httpbin.org/forms/post",
	} {
		resp, err := (&http.Client{Timeout: 15 * time.Second}).Get(u)
		if err != nil {
			t.Skipf("site unreachable, skipping: %s: %v", u, err)
		}
		_ = resp.Body.Close()
	}

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	crypto.ResetForTest()
	if err := crypto.InitKeyWithBytes(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(crypto.ResetForTest)

	dir := t.TempDir()
	db, err := database.New(filepath.Join(dir, "realweb.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	runner, err := browser.NewRunner(filepath.Join(dir, "screenshots"))
	if err != nil {
		t.Fatal(err)
	}
	// Visible windows by default when a display exists; REALWEB_HEADLESS=1 forces headless.
	headed := os.Getenv("REALWEB_HEADLESS") != "1" && os.Getenv("DISPLAY") != ""
	runner.SetForceHeadless(!headed)
	t.Logf("browser mode: headed=%t DISPLAY=%q", headed, os.Getenv("DISPLAY"))
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", !headed), chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true), chromedp.Flag("no-first-run", true))
	pool := browser.NewBrowserPool(browser.PoolConfig{Size: 2, MaxTabs: 2, AcquireTimeout: 60 * time.Second}, opts)
	t.Cleanup(func() { pool.Stop() })
	runner.SetPool(pool)

	var mu sync.Mutex
	terminal := map[string]models.TaskStatus{}
	q := New(db, runner, 2, func(e models.TaskEvent) {
		mu.Lock()
		defer mu.Unlock()
		switch e.Status {
		case models.TaskStatusCompleted, models.TaskStatusFailed, models.TaskStatusCancelled:
			terminal[e.TaskID] = e.Status
			t.Logf("event %s -> %s %s", e.TaskID, e.Status, e.Error)
		}
	})
	t.Cleanup(q.Stop)

	type check struct{ key, want string }
	cases := []struct {
		task   models.Task
		checks []check
	}{
		{
			task: models.Task{ID: "pipe-internet-login", Name: "the-internet login", Steps: []models.TaskStep{
				{Action: models.ActionNavigate, Value: "https://the-internet.herokuapp.com/login"},
				{Action: models.ActionType, Selector: "#username", Value: "tomsmith"},
				{Action: models.ActionType, Selector: "#password", Value: "SuperSecretPassword!"},
				{Action: models.ActionClick, Selector: "button[type=submit]"},
				{Action: models.ActionWait, Selector: "#flash.success"},
				{Action: models.ActionExtract, Selector: "#flash", VarName: "flash"},
			}},
			checks: []check{{"flash", "You logged into a secure area!"}},
		},
		{
			task: models.Task{ID: "pipe-quotes-scrape", Name: "quotes scrape", Steps: []models.TaskStep{
				{Action: models.ActionNavigate, Value: "https://quotes.toscrape.com/"},
				{Action: models.ActionExtract, Selector: ".quote .author", VarName: "page1_author"},
				{Action: models.ActionNavigate, Value: "https://quotes.toscrape.com/tag/love/"},
				{Action: models.ActionExtract, Selector: ".tags-box .tag-item a", VarName: "top_tag"},
				{Action: models.ActionGetTitle, Value: "title"},
			}},
			checks: []check{{"page1_author", "Albert Einstein"}, {"top_tag", "love"}, {"title", "Quotes to Scrape"}},
		},
		{
			task: models.Task{ID: "pipe-books-product", Name: "books product", Steps: []models.TaskStep{
				{Action: models.ActionNavigate, Value: "https://books.toscrape.com/"},
				{Action: models.ActionClick, Selector: "a[href='catalogue/category/books/poetry_23/index.html']"},
				{Action: models.ActionWait, Selector: ".page-header h1"},
				{Action: models.ActionClick, Selector: "article.product_pod h3 a"},
				{Action: models.ActionWait, Selector: ".product_main h1"},
				{Action: models.ActionExtract, Selector: ".product_main h1", VarName: "book"},
				{Action: models.ActionExtract, Selector: ".product_main .price_color", VarName: "price"},
				{Action: models.ActionScreenshot},
			}},
			checks: []check{{"book", "A Light in the Attic"}, {"price", "£51.77"}},
		},
		{
			task: models.Task{ID: "pipe-httpbin-form", Name: "httpbin form", Steps: []models.TaskStep{
				{Action: models.ActionNavigate, Value: "https://httpbin.org/forms/post"},
				{Action: models.ActionType, Selector: "input[name=custname]", Value: "Pipeline"},
				{Action: models.ActionClick, Selector: "input[name=size][value=large]"},
				{Action: models.ActionSubmitForm, Selector: "form"},
				{Action: models.ActionWait, Selector: "pre", Timeout: 20000},
				{Action: models.ActionExtract, Selector: "pre", VarName: "response"},
			}},
		},
	}

	ctx := context.Background()
	start := time.Now()
	for _, c := range cases {
		task := c.task
		task.URL = task.Steps[0].Value
		task.Status = models.TaskStatusPending
		task.Priority = models.PriorityNormal
		task.Headless = !headed
		task.CreatedAt = time.Now()
		if err := db.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask %s: %v", task.ID, err)
		}
		if err := q.Submit(ctx, task); err != nil {
			t.Fatalf("Submit %s: %v", task.ID, err)
		}
	}

	deadline := time.Now().Add(3 * time.Minute)
	for {
		mu.Lock()
		done := len(terminal)
		mu.Unlock()
		if done == len(cases) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d/%d tasks reached a terminal state", done, len(cases))
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Logf("all %d tasks finished in %s (concurrency 2)", len(cases), time.Since(start).Round(time.Millisecond))

	for _, c := range cases {
		stored, err := db.GetTask(ctx, c.task.ID)
		if err != nil {
			t.Fatalf("GetTask %s: %v", c.task.ID, err)
		}
		if stored.Status != models.TaskStatusCompleted || stored.Result == nil {
			t.Errorf("%s: stored status %q result=%v error=%q", c.task.ID, stored.Status, stored.Result != nil, stored.Error)
			continue
		}
		t.Logf("%s: stored completed, %d steps logged, %s", c.task.ID, len(stored.Result.StepLogs), stored.Result.Duration.Round(time.Millisecond))
		for _, ch := range c.checks {
			if got := strings.TrimSpace(stored.Result.ExtractedData[ch.key]); !strings.Contains(got, ch.want) {
				t.Errorf("%s %s: got %q, want %q", c.task.ID, ch.key, got, ch.want)
			} else {
				t.Logf("  verified %s = %q (read back from DB)", ch.key, got)
			}
		}
		if c.task.ID == "pipe-httpbin-form" {
			var echoed struct{ Form map[string]any }
			_ = json.Unmarshal([]byte(stored.Result.ExtractedData["response"]), &echoed)
			if echoed.Form["custname"] != "Pipeline" || echoed.Form["size"] != "large" {
				t.Errorf("httpbin echoed form %v, want custname=Pipeline size=large", echoed.Form)
			} else {
				t.Logf("  verified server echoed custname=Pipeline size=large")
			}
		}
	}

	m := q.Metrics()
	t.Logf("queue metrics: submitted=%d completed=%d failed=%d cancelled=%d", m.TotalSubmitted, m.TotalCompleted, m.TotalFailed, m.TotalCancelled)
	if m.TotalCompleted != int64(len(cases)) || m.TotalFailed != 0 || m.TotalCancelled != 0 {
		t.Errorf("metrics: completed=%d failed=%d cancelled=%d, want %d/0/0", m.TotalCompleted, m.TotalFailed, m.TotalCancelled, len(cases))
	}
}
