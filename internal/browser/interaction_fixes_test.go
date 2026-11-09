package browser

// Local (no internet) regression tests for interaction bugs found by the
// real-website suite (realweb_test.go). They run against an httptest server
// in real Chrome and skip when Chrome is not installed.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"flowpilot/internal/models"
)

func newFixRunner(t *testing.T) *Runner {
	t.Helper()
	requireChrome(t)
	r, err := NewRunner(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r.SetForceHeadless(true)
	r.SetAllowEval(true)
	pool := NewBrowserPool(PoolConfig{Size: 1, MaxTabs: 1, AcquireTimeout: 30 * time.Second}, chromeOpts())
	t.Cleanup(func() { pool.Stop() })
	r.SetPool(pool)
	return r
}

func fixSite(t *testing.T, pages map[string]string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, ok := pages[req.URL.Path]
		if !ok {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func runFixTask(t *testing.T, r *Runner, steps []models.TaskStep) (*models.TaskResult, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	task := models.Task{ID: "fix-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-")), Name: t.Name(), Steps: steps, Headless: true}
	res, err := r.RunTask(ctx, task)
	if res != nil {
		for i, l := range res.StepLogs {
			t.Logf("  step %2d %-14s %-22q %6dms %s", i, l.Action, l.Selector, l.DurationMs, l.ErrorMsg)
		}
	}
	return res, err
}

// outScript copies a JS expression's value into #out so extract can read it.
func outScript(expr string) string {
	return fmt.Sprintf(`document.getElementById('out').textContent = String(%s)`, expr)
}

func wantOut(t *testing.T, res *models.TaskResult, err error, want string) {
	t.Helper()
	if err != nil {
		t.Fatalf("task failed: %v", err)
	}
	if got := strings.TrimSpace(res.ExtractedData["out"]); got != want {
		t.Fatalf("out = %q, want %q", got, want)
	}
}

const outDiv = `<div id="out"></div>`

func TestTypeIntoEmptyAndPrefilledFields(t *testing.T) {
	r := newFixRunner(t)
	base := fixSite(t, map[string]string{"/": `<html><body>
		<textarea id="empty"></textarea>
		<textarea id="filled">old text</textarea>
		<input id="input" value="old value">` + outDiv + `</body></html>`})
	res, err := runFixTask(t, r, []models.TaskStep{
		{Action: models.ActionNavigate, Value: base + "/"},
		{Action: models.ActionType, Selector: "#empty", Value: "hello"},
		{Action: models.ActionType, Selector: "#filled", Value: "new text"},
		{Action: models.ActionType, Selector: "#input", Value: "new value"},
		{Action: models.ActionEval, Value: outScript(`[empty.value, filled.value, input.value].join('|')`)},
		{Action: models.ActionExtract, Selector: "#out", VarName: "out"},
	})
	wantOut(t, res, err, "hello|new text|new value")
}

func TestHoverTriggersCSSHover(t *testing.T) {
	r := newFixRunner(t)
	base := fixSite(t, map[string]string{"/": `<html><head><style>
		.menu { width: 200px; height: 60px; background: #ddd; margin: 40px; }
		.menu .item { display: none; }
		.menu:hover .item { display: block; }
		</style></head><body>
		<div class="menu">Menu<div class="item">Revealed by hover</div></div>` + outDiv + `
		<script>document.querySelector('.menu').addEventListener('mouseenter', function(){ window.entered = true; });</script>
		</body></html>`})
	res, err := runFixTask(t, r, []models.TaskStep{
		{Action: models.ActionNavigate, Value: base + "/"},
		{Action: models.ActionHover, Selector: ".menu"},
		{Action: models.ActionEval, Value: outScript(`getComputedStyle(document.querySelector('.item')).display + '|' + !!window.entered`)},
		{Action: models.ActionExtract, Selector: "#out", VarName: "out"},
	})
	wantOut(t, res, err, "block|true")
}

func TestDragDropHTML5(t *testing.T) {
	r := newFixRunner(t)
	base := fixSite(t, map[string]string{"/": `<html><body>
		<div id="src" draggable="true" style="width:100px;height:50px;background:#acf">drag me</div>
		<div id="dst" style="width:200px;height:100px;margin-top:40px;background:#fca">drop here</div>` + outDiv + `
		<script>
		src.addEventListener('dragstart', function(e){ e.dataTransfer.setData('text/plain', 'payload-42'); });
		dst.addEventListener('dragover', function(e){ e.preventDefault(); });
		dst.addEventListener('drop', function(e){ e.preventDefault(); dst.dataset.got = e.dataTransfer.getData('text/plain'); });
		src.addEventListener('dragend', function(){ src.dataset.ended = 'yes'; });
		</script></body></html>`})
	res, err := runFixTask(t, r, []models.TaskStep{
		{Action: models.ActionNavigate, Value: base + "/"},
		{Action: models.ActionDragDrop, Selector: "#src", Target: "#dst"},
		{Action: models.ActionEval, Value: outScript(`(dst.dataset.got || 'none') + '|' + (src.dataset.ended || 'no')`)},
		{Action: models.ActionExtract, Selector: "#out", VarName: "out"},
	})
	wantOut(t, res, err, "payload-42|yes")
}

// Guard: mouse-event based drag (jQuery UI style) must keep working.
func TestDragDropMouseBased(t *testing.T) {
	r := newFixRunner(t)
	base := fixSite(t, map[string]string{"/": `<html><body>
		<div id="src" style="width:100px;height:50px;background:#acf">handle</div>
		<div id="dst" style="width:200px;height:100px;margin-top:40px;background:#fca">zone</div>` + outDiv + `
		<script>
		var dragging = false;
		src.addEventListener('mousedown', function(){ dragging = true; });
		document.addEventListener('mouseup', function(e){
			if (!dragging) return; dragging = false;
			var r = dst.getBoundingClientRect();
			window.droppedInZone = e.clientX >= r.left && e.clientX <= r.right && e.clientY >= r.top && e.clientY <= r.bottom;
		});
		</script></body></html>`})
	res, err := runFixTask(t, r, []models.TaskStep{
		{Action: models.ActionNavigate, Value: base + "/"},
		{Action: models.ActionDragDrop, Selector: "#src", Target: "#dst"},
		{Action: models.ActionEval, Value: outScript(`!!window.droppedInZone`)},
		{Action: models.ActionExtract, Selector: "#out", VarName: "out"},
	})
	wantOut(t, res, err, "true")
}

// The button is already inside the viewport (so no scroll happens before the
// click) but sits under a fixed footer, like quotes.toscrape.com's login
// button in a small headless window. Content below allows centring it.
const footerPage = `<html><body style="margin:0">
	<div style="height:calc(100vh - 110px)">top content</div>
	<button id="go" style="height:40px">Go</button>
	<div style="height:1200px">more content below</div>
	<div id="footer" style="position:fixed;left:0;right:0;bottom:0;height:160px;background:#333;color:#fff">sticky footer</div>
	` + outDiv + `
	<script>
	go.addEventListener('click', function(){ out.textContent = 'button'; });
	footer.addEventListener('click', function(){ out.textContent = 'footer'; });
	</script></body></html>`

// Before the fix the click silently landed on the footer.
func TestClickScrollsTargetClearOfFixedFooter(t *testing.T) {
	r := newFixRunner(t)
	base := fixSite(t, map[string]string{"/": footerPage})
	res, err := runFixTask(t, r, []models.TaskStep{
		{Action: models.ActionNavigate, Value: base + "/"},
		{Action: models.ActionClick, Selector: "#go"},
		{Action: models.ActionExtract, Selector: "#out", VarName: "out"},
	})
	wantOut(t, res, err, "button")
}

// A target that stays covered must fail loudly, not click something else.
func TestClickFailsWhenTargetIsCovered(t *testing.T) {
	r := newFixRunner(t)
	base := fixSite(t, map[string]string{"/": `<html><body>
		<button id="go">Go</button>` + outDiv + `
		<div id="modal" style="position:fixed;inset:0;background:rgba(0,0,0,.5)">modal overlay</div>
		<script>go.addEventListener('click', function(){ out.textContent = 'button'; });
		modal.addEventListener('click', function(){ out.textContent = 'modal'; });</script>
		</body></html>`})
	_, err := runFixTask(t, r, []models.TaskStep{
		{Action: models.ActionNavigate, Value: base + "/"},
		{Action: models.ActionClick, Selector: "#go", Timeout: 3000},
	})
	if err == nil || !strings.Contains(err.Error(), "covered") {
		t.Fatalf("expected a 'covered' error, got %v", err)
	}
	t.Logf("got expected error: %v", err)
}

func TestNavigateBackForwardWithBFCache(t *testing.T) {
	r := newFixRunner(t)
	base := fixSite(t, map[string]string{
		"/a": `<html><body><h1>Page A</h1><a id="next" href="/b">to b</a></body></html>`,
		"/b": `<html><body><h1>Page B</h1></body></html>`,
	})
	start := time.Now()
	res, err := runFixTask(t, r, []models.TaskStep{
		{Action: models.ActionNavigate, Value: base + "/a"},
		{Action: models.ActionClick, Selector: "#next"},
		{Action: models.ActionWait, Selector: "h1"},
		{Action: models.ActionNavigateBack, Timeout: 8000},
		{Action: models.ActionExtract, Selector: "h1", VarName: "back"},
		{Action: models.ActionNavigateForward, Timeout: 8000},
		{Action: models.ActionExtract, Selector: "h1", VarName: "forward"},
	})
	if err != nil {
		t.Fatalf("task failed after %s: %v", time.Since(start).Round(time.Millisecond), err)
	}
	if res.ExtractedData["back"] != "Page A" || res.ExtractedData["forward"] != "Page B" {
		t.Fatalf("back=%q forward=%q, want Page A / Page B", res.ExtractedData["back"], res.ExtractedData["forward"])
	}
}

func TestWaitFunctionSurvivesNavigation(t *testing.T) {
	r := newFixRunner(t)
	base := fixSite(t, map[string]string{
		"/start": `<html><body>starting<script>setTimeout(function(){ location.href = '/done'; }, 400);</script></body></html>`,
		"/done":  `<html><body><h1 id="done">Done</h1></body></html>`,
	})
	res, err := runFixTask(t, r, []models.TaskStep{
		{Action: models.ActionNavigate, Value: base + "/start"},
		{Action: models.ActionWaitFunction, Value: "location.pathname === '/done' && document.readyState === 'complete'", Timeout: 10000},
		{Action: models.ActionExtract, Selector: "#done", VarName: "done"},
	})
	if err != nil {
		t.Fatalf("task failed: %v", err)
	}
	if res.ExtractedData["done"] != "Done" {
		t.Fatalf("done = %q", res.ExtractedData["done"])
	}
}

// Guard: a wait_function that never becomes true must still time out.
func TestWaitFunctionStillTimesOut(t *testing.T) {
	r := newFixRunner(t)
	base := fixSite(t, map[string]string{"/": `<html><body>static</body></html>`})
	start := time.Now()
	_, err := runFixTask(t, r, []models.TaskStep{
		{Action: models.ActionNavigate, Value: base + "/"},
		{Action: models.ActionWaitFunction, Value: "window.neverTrue === true", Timeout: 1500},
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Fatalf("timed out too slowly: %s", d)
	}
}
