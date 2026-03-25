package recorder

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"flowpilot/internal/logs"
	"flowpilot/internal/models"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

func TestRecordStep(t *testing.T) {
	var captured []models.RecordedStep
	handler := func(step models.RecordedStep) {
		captured = append(captured, step)
	}

	// Use a nil context since we won't actually run chromedp
	r := &Recorder{
		handler: handler,
		flowID:  "flow-1",
	}

	r.RecordStep(models.ActionClick, "#btn", "")
	r.RecordStep(models.ActionType, "#input", "hello")
	r.RecordStep(models.ActionNavigate, "", "https://example.com")

	if len(captured) != 3 {
		t.Fatalf("expected 3 captured steps, got %d", len(captured))
	}

	if captured[0].Index != 0 {
		t.Errorf("step[0].Index: got %d, want 0", captured[0].Index)
	}
	if captured[0].Action != models.ActionClick {
		t.Errorf("step[0].Action: got %q, want %q", captured[0].Action, models.ActionClick)
	}
	if captured[0].Selector != "#btn" {
		t.Errorf("step[0].Selector: got %q, want %q", captured[0].Selector, "#btn")
	}

	if captured[1].Index != 1 {
		t.Errorf("step[1].Index: got %d, want 1", captured[1].Index)
	}
	if captured[1].Value != "hello" {
		t.Errorf("step[1].Value: got %q, want %q", captured[1].Value, "hello")
	}

	if captured[2].Index != 2 {
		t.Errorf("step[2].Index: got %d, want 2", captured[2].Index)
	}
	if captured[2].Action != models.ActionNavigate {
		t.Errorf("step[2].Action: got %q, want %q", captured[2].Action, models.ActionNavigate)
	}
	if captured[2].Value != "https://example.com" {
		t.Errorf("step[2].Value: got %q", captured[2].Value)
	}

	if captured[0].Timestamp.IsZero() {
		t.Error("step timestamps should not be zero")
	}
}

func TestRecordStepNilHandler(t *testing.T) {
	r := &Recorder{handler: nil, flowID: "flow-nil"}
	// Should not panic
	r.RecordStep(models.ActionClick, "#btn", "")
}

func TestRecordStepIncrementsIndex(t *testing.T) {
	var captured []models.RecordedStep
	r := &Recorder{
		handler: func(step models.RecordedStep) {
			captured = append(captured, step)
		},
		flowID: "flow-idx",
	}

	for i := 0; i < 10; i++ {
		r.RecordStep(models.ActionClick, "#btn", "")
	}

	for i, step := range captured {
		if step.Index != i {
			t.Errorf("step[%d].Index: got %d, want %d", i, step.Index, i)
		}
	}
}

func TestRankSelectors(t *testing.T) {
	candidates := []models.SelectorCandidate{
		{Selector: ".class", Strategy: models.SelectorCSS, Score: 30},
		{Selector: "#id", Strategy: models.SelectorID, Score: 90},
		{Selector: "[data-testid]", Strategy: models.SelectorDataTestID, Score: 95},
		{Selector: "//div", Strategy: models.SelectorXPath, Score: 10},
	}

	ranked := RankSelectors(candidates)
	if ranked[0].Score != 95 {
		t.Errorf("top rank score: got %d, want 95", ranked[0].Score)
	}
	if ranked[0].Selector != "[data-testid]" {
		t.Errorf("top rank selector: got %q", ranked[0].Selector)
	}
	if ranked[len(ranked)-1].Score != 10 {
		t.Errorf("lowest rank score: got %d, want 10", ranked[len(ranked)-1].Score)
	}
}

func TestRankSelectorsEmpty(t *testing.T) {
	ranked := RankSelectors(nil)
	if len(ranked) != 0 {
		t.Errorf("expected empty result, got %d", len(ranked))
	}
}

func TestRankSelectorsDoesNotMutateOriginal(t *testing.T) {
	candidates := []models.SelectorCandidate{
		{Selector: "c", Score: 30},
		{Selector: "a", Score: 90},
		{Selector: "b", Score: 50},
	}
	origFirst := candidates[0]

	_ = RankSelectors(candidates)

	if candidates[0] != origFirst {
		t.Error("RankSelectors should not mutate the original slice")
	}
}

func TestBestSelector(t *testing.T) {
	candidates := []models.SelectorCandidate{
		{Selector: ".low", Score: 10},
		{Selector: "#high", Score: 99},
		{Selector: ".mid", Score: 50},
	}

	best := BestSelector(candidates)
	if best != "#high" {
		t.Errorf("BestSelector: got %q, want %q", best, "#high")
	}
}

func TestBestSelectorEmpty(t *testing.T) {
	best := BestSelector(nil)
	if best != "" {
		t.Errorf("BestSelector(nil): got %q, want empty", best)
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal-id", "normal-id"},
		{"path/traversal", "path_traversal"},
		{"back\\slash", "back_slash"},
		{"dot.dot", "dot_dot"},
		{"null\x00byte", "null_byte"},
	}

	for _, tc := range tests {
		got := sanitize(tc.input)
		if got != tc.want {
			t.Errorf("sanitize(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestNewRecorder(t *testing.T) {
	ctx := context.Background()
	var called bool
	handler := func(step models.RecordedStep) {
		called = true
	}

	r := New(ctx, "flow-new", handler)
	if r.flowID != "flow-new" {
		t.Errorf("flowID: got %q, want %q", r.flowID, "flow-new")
	}
	if r.parentCtx != ctx {
		t.Error("parentCtx not set correctly")
	}
	if r.BrowserCtx() != nil {
		t.Error("BrowserCtx should be nil before Start")
	}
	if r.FlowID() != "flow-new" {
		t.Errorf("FlowID(): got %q, want %q", r.FlowID(), "flow-new")
	}

	r.RecordStep(models.ActionClick, "#btn", "")
	if !called {
		t.Error("handler was not called")
	}
}

func TestStopIdempotent(t *testing.T) {
	r := &Recorder{flowID: "flow-stop"}

	r.Stop()
	r.Stop()
}

func TestStopCleansUpContexts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := &Recorder{
		flowID:        "flow-cleanup",
		allocCtx:      ctx,
		allocCancel:   cancel,
		browserCtx:    ctx,
		browserCancel: cancel,
	}

	r.Stop()
	if r.browserCtx != nil {
		t.Error("browserCtx should be nil after Stop")
	}
	if r.allocCtx != nil {
		t.Error("allocCtx should be nil after Stop")
	}
	if r.browserCancel != nil {
		t.Error("browserCancel should be nil after Stop")
	}
	if r.allocCancel != nil {
		t.Error("allocCancel should be nil after Stop")
	}
}

func TestBrowserCtxNilBeforeStart(t *testing.T) {
	r := New(context.Background(), "flow-ctx", nil)
	if r.BrowserCtx() != nil {
		t.Error("BrowserCtx() should return nil before Start")
	}
}

func TestNetworkLogsNilLogger(t *testing.T) {
	r := &Recorder{flowID: "flow-nil-net"}
	logs := r.NetworkLogs()
	if logs != nil {
		t.Errorf("expected nil, got %v", logs)
	}
}

func TestNetworkLogsWithLogger(t *testing.T) {
	r := New(context.Background(), "flow-net", nil)
	r.netLogger = logs.NewNetworkLogger("flow-net")

	logs := r.NetworkLogs()
	if logs == nil {
		t.Fatal("expected non-nil logs slice")
	}
	if len(logs) != 0 {
		t.Errorf("expected empty logs, got %d", len(logs))
	}
}

func TestWebSocketLogsNilLogger(t *testing.T) {
	r := &Recorder{flowID: "flow-nil-ws"}
	wsLogs := r.WebSocketLogs()
	if wsLogs != nil {
		t.Errorf("expected nil, got %v", wsLogs)
	}
}

func TestWebSocketLogsWithLogger(t *testing.T) {
	r := New(context.Background(), "flow-ws", nil)
	r.wsLogger = logs.NewWebSocketLogger("flow-ws")

	wsLogs := r.WebSocketLogs()
	if wsLogs == nil {
		t.Fatal("expected non-nil ws logs slice")
	}
	if len(wsLogs) != 0 {
		t.Errorf("expected empty ws logs, got %d", len(wsLogs))
	}
}

func TestSetSnapshotter(t *testing.T) {
	r := New(context.Background(), "flow-snap", nil)
	if r.snapshotter != nil {
		t.Error("snapshotter should be nil initially")
	}

	dir := t.TempDir()
	s, err := NewSnapshotter(dir)
	if err != nil {
		t.Fatalf("NewSnapshotter: %v", err)
	}
	r.SetSnapshotter(s)
	if r.snapshotter == nil {
		t.Error("snapshotter should be set after SetSnapshotter")
	}
}

func TestNewSnapshotterCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "snap")
	s, err := NewSnapshotter(dir)
	if err != nil {
		t.Fatalf("NewSnapshotter: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil snapshotter")
	}
	if s.outputDir != dir {
		t.Errorf("outputDir: got %q, want %q", s.outputDir, dir)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestNewSnapshotterInvalidPath(t *testing.T) {
	_, err := NewSnapshotter("/dev/null/invalid")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestSanitizeEdgeCases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"abc", "abc"},
		{"a/b\\c.d\x00e", "a_b_c_d_e"},
		{"///", "___"},
		{"no_special-chars_here", "no_special-chars_here"},
		{"日本語", "日本語"},
		{"émojis🎉", "émojis🎉"},
	}

	for _, tc := range tests {
		got := sanitize(tc.input)
		if got != tc.want {
			t.Errorf("sanitize(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestRecordStepConcurrent(t *testing.T) {
	var mu sync.Mutex
	var captured []models.RecordedStep
	r := &Recorder{
		handler: func(step models.RecordedStep) {
			mu.Lock()
			captured = append(captured, step)
			mu.Unlock()
		},
		flowID: "flow-concurrent",
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.RecordStep(models.ActionClick, "#btn", "")
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(captured) != 100 {
		t.Errorf("expected 100 steps, got %d", len(captured))
	}

	indices := make(map[int]bool)
	for _, step := range captured {
		if indices[step.Index] {
			t.Errorf("duplicate step index: %d", step.Index)
		}
		indices[step.Index] = true
	}
}

func TestBestSelectorSingleCandidate(t *testing.T) {
	candidates := []models.SelectorCandidate{
		{Selector: "#only", Score: 50},
	}
	best := BestSelector(candidates)
	if best != "#only" {
		t.Errorf("BestSelector single: got %q, want %q", best, "#only")
	}
}

func TestRankSelectorsEqualScores(t *testing.T) {
	candidates := []models.SelectorCandidate{
		{Selector: "a", Score: 50},
		{Selector: "b", Score: 50},
		{Selector: "c", Score: 50},
	}
	ranked := RankSelectors(candidates)
	if len(ranked) != 3 {
		t.Errorf("expected 3 results, got %d", len(ranked))
	}
	for _, r := range ranked {
		if r.Score != 50 {
			t.Errorf("unexpected score: %d", r.Score)
		}
	}
}

type mockCDP struct {
	mu          sync.Mutex
	runCalls    int
	runErr      error
	runErrAt    int
	listenCalls int
}

func (m *mockCDP) Run(ctx context.Context, actions ...chromedp.Action) error {
	m.mu.Lock()
	m.runCalls++
	n := m.runCalls
	m.mu.Unlock()
	if m.runErrAt > 0 && n == m.runErrAt {
		return m.runErr
	}
	if m.runErrAt == 0 && m.runErr != nil {
		return m.runErr
	}
	return nil
}

func (m *mockCDP) ListenTarget(ctx context.Context, fn func(ev any)) {
	m.mu.Lock()
	m.listenCalls++
	m.mu.Unlock()
}

func (m *mockCDP) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runCalls
}

func TestSetWSCallback(t *testing.T) {
	r := New(context.Background(), "flow-wscb", nil)
	r.wsLogger = logs.NewWebSocketLogger("flow-wscb")

	called := false
	r.SetWSCallback(func(ev models.WebSocketLog) {
		called = true
	})
	_ = called
}

func TestSetWSCallbackNilLogger(t *testing.T) {
	r := New(context.Background(), "flow-wscb-nil", nil)
	r.SetWSCallback(func(ev models.WebSocketLog) {})
}

func TestEnableDomainsSuccess(t *testing.T) {
	mock := &mockCDP{}
	r := &Recorder{
		flowID:     "flow-ed",
		browserCtx: context.Background(),
		cdp:        mock,
	}

	err := r.enableDomains()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.calls() != 4 {
		t.Fatalf("expected 4 Run calls, got %d", mock.calls())
	}
}

func TestEnableDomainsRuntimeFail(t *testing.T) {
	mock := &mockCDP{runErr: errors.New("runtime err"), runErrAt: 1}
	r := &Recorder{
		flowID:     "flow-ed-fail",
		browserCtx: context.Background(),
		cdp:        mock,
	}

	err := r.enableDomains()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "enable runtime domain") {
		t.Errorf("expected runtime domain error, got: %v", err)
	}
}

func TestEnableDomainsPageFail(t *testing.T) {
	mock := &mockCDP{runErr: errors.New("page err"), runErrAt: 2}
	r := &Recorder{
		flowID:     "flow-ed-page",
		browserCtx: context.Background(),
		cdp:        mock,
	}

	err := r.enableDomains()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "enable page domain") {
		t.Errorf("expected page domain error, got: %v", err)
	}
}

func TestEnableDomainsNetworkFail(t *testing.T) {
	mock := &mockCDP{runErr: errors.New("net err"), runErrAt: 3}
	r := &Recorder{
		flowID:     "flow-ed-net",
		browserCtx: context.Background(),
		cdp:        mock,
	}

	err := r.enableDomains()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "enable network domain") {
		t.Errorf("expected network domain error, got: %v", err)
	}
}

func TestEnableDomainsBindingFail(t *testing.T) {
	mock := &mockCDP{runErr: errors.New("binding err"), runErrAt: 4}
	r := &Recorder{
		flowID:     "flow-ed-bind",
		browserCtx: context.Background(),
		cdp:        mock,
	}

	err := r.enableDomains()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "add binding") {
		t.Errorf("expected binding error, got: %v", err)
	}
}

func TestInstallCaptureScriptSuccess(t *testing.T) {
	mock := &mockCDP{}
	r := &Recorder{
		flowID:     "flow-ics",
		browserCtx: context.Background(),
		cdp:        mock,
	}

	err := r.installCaptureScript()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.calls() != 1 {
		t.Fatalf("expected 1 Run call, got %d", mock.calls())
	}
}

func TestInstallCaptureScriptError(t *testing.T) {
	mock := &mockCDP{runErr: errors.New("install err")}
	r := &Recorder{
		flowID:     "flow-ics-err",
		browserCtx: context.Background(),
		cdp:        mock,
	}

	err := r.installCaptureScript()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "install capture script") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestInjectCaptureScriptSuccess(t *testing.T) {
	mock := &mockCDP{}
	r := &Recorder{
		flowID:     "flow-inject",
		browserCtx: context.Background(),
		cdp:        mock,
	}

	err := r.injectCaptureScript()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInjectCaptureScriptError(t *testing.T) {
	mock := &mockCDP{runErr: errors.New("inject err")}
	r := &Recorder{
		flowID:     "flow-inject-err",
		browserCtx: context.Background(),
		cdp:        mock,
	}

	err := r.injectCaptureScript()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRegisterListeners(t *testing.T) {
	mock := &mockCDP{}
	r := &Recorder{
		flowID:     "flow-rl",
		browserCtx: context.Background(),
		cdp:        mock,
	}

	r.registerListeners()
	if mock.listenCalls != 1 {
		t.Fatalf("expected 1 ListenTarget call, got %d", mock.listenCalls)
	}
}

func TestHandleEventBindingCalled(t *testing.T) {
	var captured []models.RecordedStep
	r := &Recorder{
		handler: func(step models.RecordedStep) {
			captured = append(captured, step)
		},
		flowID: "flow-he-bind",
		cdp:    &mockCDP{},
	}

	r.handleEvent(&runtime.EventBindingCalled{
		Name:    bindingName,
		Payload: `{"action":"click","selector":"#btn","value":""}`,
	})

	if len(captured) != 1 {
		t.Fatalf("expected 1 captured step, got %d", len(captured))
	}
	if captured[0].Action != models.ActionClick {
		t.Errorf("expected click action, got %q", captured[0].Action)
	}
}

func TestHandleEventBindingCalledWrongName(t *testing.T) {
	var captured []models.RecordedStep
	r := &Recorder{
		handler: func(step models.RecordedStep) {
			captured = append(captured, step)
		},
		flowID: "flow-he-wrong",
		cdp:    &mockCDP{},
	}

	r.handleEvent(&runtime.EventBindingCalled{
		Name:    "otherBinding",
		Payload: `{"action":"click","selector":"#btn","value":""}`,
	})

	if len(captured) != 0 {
		t.Fatalf("expected no captured steps, got %d", len(captured))
	}
}

func TestHandleEventFrameNavigated(t *testing.T) {
	var captured []models.RecordedStep
	r := &Recorder{
		handler: func(step models.RecordedStep) {
			captured = append(captured, step)
		},
		flowID: "flow-he-nav",
		cdp:    &mockCDP{},
	}

	r.handleEvent(&page.EventFrameNavigated{
		Frame: &cdp.Frame{
			URL: "https://example.com",
		},
	})

	if len(captured) != 1 {
		t.Fatalf("expected 1 captured step, got %d", len(captured))
	}
	if captured[0].Action != models.ActionNavigate {
		t.Errorf("expected navigate action, got %q", captured[0].Action)
	}
	if captured[0].Value != "https://example.com" {
		t.Errorf("expected URL in value, got %q", captured[0].Value)
	}
}

func TestHandleEventFrameNavigatedSubframe(t *testing.T) {
	var captured []models.RecordedStep
	r := &Recorder{
		handler: func(step models.RecordedStep) {
			captured = append(captured, step)
		},
		flowID: "flow-he-sub",
		cdp:    &mockCDP{},
	}

	r.handleEvent(&page.EventFrameNavigated{
		Frame: &cdp.Frame{
			ParentID: "parent-frame",
			URL:      "https://example.com/iframe",
		},
	})

	if len(captured) != 0 {
		t.Fatal("should not record subframe navigation")
	}
}

func TestHandleEventFrameNavigatedNilFrame(t *testing.T) {
	var captured []models.RecordedStep
	r := &Recorder{
		handler: func(step models.RecordedStep) {
			captured = append(captured, step)
		},
		flowID: "flow-he-nilf",
		cdp:    &mockCDP{},
	}

	r.handleEvent(&page.EventFrameNavigated{Frame: nil})

	if len(captured) != 0 {
		t.Fatal("should not record when frame is nil")
	}
}

func TestHandleEventTargetInfoChangedTabSwitch(t *testing.T) {
	var captured []models.RecordedStep
	r := &Recorder{
		handler: func(step models.RecordedStep) {
			captured = append(captured, step)
		},
		flowID:      "flow-he-tab",
		activeTabID: target.ID("tab-1"),
		cdp:         &mockCDP{},
	}

	r.handleEvent(&target.EventTargetInfoChanged{
		TargetInfo: &target.Info{
			TargetID: target.ID("tab-2"),
			Type:     "page",
			URL:      "https://other.com",
		},
	})

	if len(captured) != 1 {
		t.Fatalf("expected 1 captured step, got %d", len(captured))
	}
	if captured[0].Action != models.ActionTabSwitch {
		t.Errorf("expected tab_switch action, got %q", captured[0].Action)
	}
}

func TestHandleEventTargetInfoChangedSameTab(t *testing.T) {
	var captured []models.RecordedStep
	r := &Recorder{
		handler: func(step models.RecordedStep) {
			captured = append(captured, step)
		},
		flowID:      "flow-he-same",
		activeTabID: target.ID("tab-1"),
		cdp:         &mockCDP{},
	}

	r.handleEvent(&target.EventTargetInfoChanged{
		TargetInfo: &target.Info{
			TargetID: target.ID("tab-1"),
			Type:     "page",
			URL:      "https://example.com",
		},
	})

	if len(captured) != 0 {
		t.Fatal("should not record when same tab")
	}
}

func TestHandleEventTargetInfoChangedInitialTab(t *testing.T) {
	r := &Recorder{
		handler: func(step models.RecordedStep) {},
		flowID:  "flow-he-init",
		cdp:     &mockCDP{},
	}

	r.handleEvent(&target.EventTargetInfoChanged{
		TargetInfo: &target.Info{
			TargetID: target.ID("tab-1"),
			Type:     "page",
			URL:      "https://example.com",
		},
	})

	if r.activeTabID != target.ID("tab-1") {
		t.Errorf("expected activeTabID to be set, got %q", r.activeTabID)
	}
}

func TestHandleEventTargetInfoChangedNilInfo(t *testing.T) {
	r := &Recorder{
		handler: func(step models.RecordedStep) {},
		flowID:  "flow-he-nil-info",
		cdp:     &mockCDP{},
	}

	r.handleEvent(&target.EventTargetInfoChanged{TargetInfo: nil})
}

func TestHandleEventTargetInfoChangedNonPage(t *testing.T) {
	var captured []models.RecordedStep
	r := &Recorder{
		handler: func(step models.RecordedStep) {
			captured = append(captured, step)
		},
		flowID:      "flow-he-nonpage",
		activeTabID: target.ID("tab-1"),
		cdp:         &mockCDP{},
	}

	r.handleEvent(&target.EventTargetInfoChanged{
		TargetInfo: &target.Info{
			TargetID: target.ID("sw-1"),
			Type:     "service_worker",
			URL:      "https://example.com/sw.js",
		},
	})

	if len(captured) != 0 {
		t.Fatal("should not record non-page target changes")
	}
}

func TestHandleEventNetworkRequest(t *testing.T) {
	r := &Recorder{
		flowID:    "flow-he-net",
		netLogger: logs.NewNetworkLogger("flow-he-net"),
		cdp:       &mockCDP{},
	}

	r.handleEvent(&network.EventRequestWillBeSent{
		RequestID: "req-1",
		Request:   &network.Request{URL: "https://example.com/api"},
	})

	r.handleEvent(&network.EventResponseReceived{
		RequestID: "req-1",
		Response:  &network.Response{URL: "https://example.com/api", Status: 200},
	})

	r.handleEvent(&network.EventLoadingFinished{
		RequestID: "req-1",
	})

	netLogs := r.NetworkLogs()
	if len(netLogs) != 1 {
		t.Fatalf("expected 1 network log, got %d", len(netLogs))
	}
}

func TestHandleEventNetworkResponse(t *testing.T) {
	r := &Recorder{
		flowID:    "flow-he-resp",
		netLogger: logs.NewNetworkLogger("flow-he-resp"),
		cdp:       &mockCDP{},
	}

	r.handleEvent(&network.EventRequestWillBeSent{
		RequestID: "req-1",
		Request:   &network.Request{URL: "https://example.com/api"},
	})

	r.handleEvent(&network.EventResponseReceived{
		RequestID: "req-1",
		Response:  &network.Response{URL: "https://example.com/api", Status: 200},
	})
}

func TestHandleEventNetworkLoadingFinished(t *testing.T) {
	r := &Recorder{
		flowID:    "flow-he-load",
		netLogger: logs.NewNetworkLogger("flow-he-load"),
		cdp:       &mockCDP{},
	}

	r.handleEvent(&network.EventLoadingFinished{
		RequestID: "req-1",
	})
}

func TestHandleEventWebSocket(t *testing.T) {
	r := &Recorder{
		flowID:   "flow-he-ws",
		wsLogger: logs.NewWebSocketLogger("flow-he-ws"),
		cdp:      &mockCDP{},
	}

	r.handleEvent(&network.EventWebSocketCreated{
		RequestID: "ws-1",
		URL:       "wss://example.com/ws",
	})
	r.handleEvent(&network.EventWebSocketHandshakeResponseReceived{
		RequestID: "ws-1",
	})
	r.handleEvent(&network.EventWebSocketFrameSent{
		RequestID: "ws-1",
		Response:  &network.WebSocketFrame{PayloadData: "hello"},
	})
	r.handleEvent(&network.EventWebSocketFrameReceived{
		RequestID: "ws-1",
		Response:  &network.WebSocketFrame{PayloadData: "world"},
	})
	r.handleEvent(&network.EventWebSocketClosed{
		RequestID: "ws-1",
	})
	r.handleEvent(&network.EventWebSocketFrameError{
		RequestID:    "ws-1",
		ErrorMessage: "connection reset",
	})

	wsLogs := r.WebSocketLogs()
	if len(wsLogs) == 0 {
		t.Fatal("expected ws logs")
	}
}

func TestHandleEventNilLoggers(t *testing.T) {
	r := &Recorder{
		flowID: "flow-he-nillog",
		cdp:    &mockCDP{},
	}

	r.handleEvent(&network.EventRequestWillBeSent{
		RequestID: "req-1",
		Request:   &network.Request{URL: "https://example.com"},
	})
	r.handleEvent(&network.EventResponseReceived{
		RequestID: "req-1",
	})
	r.handleEvent(&network.EventLoadingFinished{RequestID: "req-1"})
	r.handleEvent(&network.EventWebSocketCreated{RequestID: "ws-1"})
	r.handleEvent(&network.EventWebSocketHandshakeResponseReceived{RequestID: "ws-1"})
	r.handleEvent(&network.EventWebSocketFrameSent{RequestID: "ws-1"})
	r.handleEvent(&network.EventWebSocketFrameReceived{RequestID: "ws-1"})
	r.handleEvent(&network.EventWebSocketClosed{RequestID: "ws-1"})
	r.handleEvent(&network.EventWebSocketFrameError{RequestID: "ws-1"})
}

func TestHandleEventUnknownType(t *testing.T) {
	r := &Recorder{
		flowID: "flow-he-unknown",
		cdp:    &mockCDP{},
	}
	r.handleEvent("some unknown event type")
}

func TestStartSuccess(t *testing.T) {
	mock := &mockCDP{}
	r := &Recorder{
		parentCtx: context.Background(),
		flowID:    "flow-start",
		handler:   func(step models.RecordedStep) {},
		cdp:       mock,
	}

	r.allocCtx, r.allocCancel = context.WithCancel(r.parentCtx)
	r.browserCtx, r.browserCancel = context.WithCancel(r.allocCtx)
	r.netLogger = logs.NewNetworkLogger(r.flowID)
	r.wsLogger = logs.NewWebSocketLogger(r.flowID)

	err := r.enableDomains()
	if err != nil {
		t.Fatalf("enableDomains: %v", err)
	}

	err = r.installCaptureScript()
	if err != nil {
		t.Fatalf("installCaptureScript: %v", err)
	}

	err = r.injectCaptureScript()
	if err != nil {
		t.Fatalf("injectCaptureScript: %v", err)
	}

	if mock.calls() != 6 {
		t.Fatalf("expected 6 Run calls, got %d", mock.calls())
	}

	r.Stop()
}

func TestStartEnableDomainsError(t *testing.T) {
	mock := &mockCDP{runErr: errors.New("enable fail"), runErrAt: 1}
	r := &Recorder{
		parentCtx: context.Background(),
		flowID:    "flow-start-err",
		handler:   func(step models.RecordedStep) {},
		cdp:       mock,
	}

	r.allocCtx, r.allocCancel = context.WithCancel(r.parentCtx)
	r.browserCtx, r.browserCancel = context.WithCancel(r.allocCtx)
	r.netLogger = logs.NewNetworkLogger(r.flowID)
	r.wsLogger = logs.NewWebSocketLogger(r.flowID)

	err := r.enableDomains()
	if err == nil {
		t.Fatal("expected error")
	}

	r.Stop()
}

func TestCaptureSnapshotWithMock(t *testing.T) {
	dir := t.TempDir()
	mock := &mockCDP{}
	s := &Snapshotter{outputDir: dir, cdp: mock}

	snap, err := s.CaptureSnapshot(context.Background(), "flow-snap", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.FlowID != "flow-snap" {
		t.Errorf("expected FlowID 'flow-snap', got %q", snap.FlowID)
	}
	if snap.StepIndex != 0 {
		t.Errorf("expected StepIndex 0, got %d", snap.StepIndex)
	}
	if !strings.HasPrefix(snap.ScreenshotPath, dir) {
		t.Errorf("screenshot path not under output dir: %s", snap.ScreenshotPath)
	}
	if snap.ID == "" {
		t.Error("expected non-empty ID")
	}
	if mock.calls() != 2 {
		t.Fatalf("expected 2 Run calls (capture + location), got %d", mock.calls())
	}
}

func TestCaptureSnapshotRunError(t *testing.T) {
	dir := t.TempDir()
	mock := &mockCDP{runErr: errors.New("capture err")}
	s := &Snapshotter{outputDir: dir, cdp: mock}

	_, err := s.CaptureSnapshot(context.Background(), "flow-snap-err", 0)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "capture dom snapshot") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestCaptureSnapshotWriteError(t *testing.T) {
	mock := &mockCDP{}
	s := &Snapshotter{outputDir: "/nonexistent/dir/that/should/not/exist", cdp: mock}

	_, err := s.CaptureSnapshot(context.Background(), "flow-snap-write", 0)
	if err == nil {
		t.Fatal("expected write error")
	}
	if !strings.Contains(err.Error(), "write snapshot") {
		t.Errorf("expected 'write snapshot' error, got: %v", err)
	}
}

func TestRecordStepWithSnapshotter(t *testing.T) {
	dir := t.TempDir()
	mock := &mockCDP{}

	var captured []models.RecordedStep
	r := &Recorder{
		handler: func(step models.RecordedStep) {
			captured = append(captured, step)
		},
		flowID:     "flow-rs-snap",
		browserCtx: context.Background(),
		cdp:        mock,
	}

	s := &Snapshotter{outputDir: dir, cdp: mock}
	r.SetSnapshotter(s)

	r.RecordStep(models.ActionClick, "#btn", "")

	if len(captured) != 1 {
		t.Fatalf("expected 1 step, got %d", len(captured))
	}
	if captured[0].SnapshotID == "" {
		t.Error("expected non-empty SnapshotID when snapshotter is set")
	}
}

func TestRecordStepWithSnapshotterError(t *testing.T) {
	mock := &mockCDP{runErr: errors.New("snap fail")}

	var captured []models.RecordedStep
	r := &Recorder{
		handler: func(step models.RecordedStep) {
			captured = append(captured, step)
		},
		flowID:     "flow-rs-snap-err",
		browserCtx: context.Background(),
		cdp:        mock,
	}

	s := &Snapshotter{outputDir: t.TempDir(), cdp: mock}
	r.SetSnapshotter(s)

	r.RecordStep(models.ActionClick, "#btn", "")

	if len(captured) != 1 {
		t.Fatalf("expected 1 step, got %d", len(captured))
	}
	if captured[0].SnapshotID != "" {
		t.Error("expected empty SnapshotID when snapshot fails")
	}
}

func TestHandleBindingCallInvalid(t *testing.T) {
	var captured []models.RecordedStep
	r := &Recorder{
		handler: func(step models.RecordedStep) {
			captured = append(captured, step)
		},
		flowID: "flow-hbc-bad",
		cdp:    &mockCDP{},
	}

	r.handleBindingCall("not valid json")

	if len(captured) != 0 {
		t.Fatal("should not record step for invalid payload")
	}
}

func TestNewCreatesDefaultCDP(t *testing.T) {
	r := New(context.Background(), "flow-default-cdp", nil)
	if r.cdp == nil {
		t.Fatal("expected non-nil cdp client")
	}
}

func TestSnapshotterHasCDP(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSnapshotter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.cdp == nil {
		t.Fatal("expected non-nil cdp client on snapshotter")
	}
}

func TestCaptureSnapshotSanitizesFlowID(t *testing.T) {
	dir := t.TempDir()
	mock := &mockCDP{}
	s := &Snapshotter{outputDir: dir, cdp: mock}

	snap, err := s.CaptureSnapshot(context.Background(), "flow/with.dots\\and/slashes", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	filename := filepath.Base(snap.ScreenshotPath)
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		t.Errorf("filename not properly sanitized: %s", filename)
	}

	if _, err := os.Stat(snap.ScreenshotPath); err != nil {
		t.Errorf("screenshot file should exist: %v", err)
	}
}
