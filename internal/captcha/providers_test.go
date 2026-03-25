package captcha

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"flowpilot/internal/models"
)

func newTwoCaptchaTestSolver(apiKey, baseURL string) *TwoCaptcha {
	return &TwoCaptcha{
		apiKey:     apiKey,
		client:     &http.Client{Timeout: 5 * time.Second},
		baseURL:    baseURL,
		pollDelay:  time.Millisecond,
		maxWait:    100 * time.Millisecond,
		backoffMax: 2 * time.Millisecond,
	}
}

func newAntiCaptchaTestSolver(apiKey, baseURL string) *AntiCaptcha {
	return &AntiCaptcha{
		apiKey:    apiKey,
		client:    &http.Client{Timeout: 5 * time.Second},
		baseURL:   baseURL,
		pollDelay: time.Millisecond,
		maxWait:   100 * time.Millisecond,
		backoff:   2 * time.Millisecond,
	}
}

func TestTwoCaptchaSubmitByType(t *testing.T) {
	tests := []struct {
		name string
		req  models.CaptchaSolveRequest
		want map[string]string
	}{
		{
			name: "recaptcha v2 invisible",
			req:  models.CaptchaSolveRequest{Type: models.CaptchaTypeRecaptchaV2, SiteKey: "site-v2", PageURL: "https://example.com/v2", Invisible: true},
			want: map[string]string{"method": "userrecaptcha", "googlekey": "site-v2", "pageurl": "https://example.com/v2", "invisible": "1"},
		},
		{
			name: "recaptcha v3",
			req:  models.CaptchaSolveRequest{Type: models.CaptchaTypeRecaptchaV3, SiteKey: "site-v3", PageURL: "https://example.com/v3", MinScore: 0.7},
			want: map[string]string{"method": "userrecaptcha", "version": "v3", "googlekey": "site-v3", "pageurl": "https://example.com/v3", "min_score": "0.7"},
		},
		{
			name: "hcaptcha",
			req:  models.CaptchaSolveRequest{Type: models.CaptchaTypeHCaptcha, SiteKey: "site-h", PageURL: "https://example.com/h"},
			want: map[string]string{"method": "hcaptcha", "sitekey": "site-h", "pageurl": "https://example.com/h"},
		},
		{
			name: "image",
			req:  models.CaptchaSolveRequest{Type: models.CaptchaTypeImage, SiteKey: "base64-body"},
			want: map[string]string{"method": "base64", "body": "base64-body"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got url.Values
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != twoCaptchaInPath {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				if err := r.ParseForm(); err != nil {
					t.Fatalf("ParseForm: %v", err)
				}
				got = r.PostForm
				_, _ = w.Write([]byte("OK|task-123"))
			}))
			defer ts.Close()

			solver := newTwoCaptchaTestSolver("api-key", ts.URL)
			taskID, err := solver.submit(context.Background(), tc.req)
			if err != nil {
				t.Fatalf("submit: %v", err)
			}
			if taskID != "task-123" {
				t.Fatalf("taskID = %q, want task-123", taskID)
			}
			if got.Get("key") != "api-key" || got.Get("json") != "0" {
				t.Fatalf("common params mismatch: %v", got)
			}
			for key, want := range tc.want {
				if got.Get(key) != want {
					t.Fatalf("param %q = %q, want %q", key, got.Get(key), want)
				}
			}
		})
	}
}

func TestTwoCaptchaSubmitUnsupportedType(t *testing.T) {
	solver := NewTwoCaptcha("api-key")
	_, err := solver.submit(context.Background(), models.CaptchaSolveRequest{Type: "unknown"})
	if err == nil || !strings.Contains(err.Error(), "unsupported captcha type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTwoCaptchaSolve(t *testing.T) {
	var mu sync.Mutex
	pollCalls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case twoCaptchaResPath:
			query := r.URL.Query()
			if query.Get("action") != "get" {
				t.Fatalf("unexpected action: %s", query.Get("action"))
			}
			mu.Lock()
			pollCalls++
			call := pollCalls
			mu.Unlock()
			if call == 1 {
				_, _ = w.Write([]byte("CAPCHA_NOT_READY"))
				return
			}
			_, _ = w.Write([]byte("OK|solved-token"))
		case twoCaptchaInPath:
			_, _ = w.Write([]byte("OK|task-999"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	solver := newTwoCaptchaTestSolver("api-key", ts.URL)
	res, err := solver.Solve(context.Background(), models.CaptchaSolveRequest{Type: models.CaptchaTypeImage, SiteKey: "body"})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}
	if res.Token != "solved-token" {
		t.Fatalf("token = %q, want solved-token", res.Token)
	}
	if res.Duration <= 0 {
		t.Fatalf("expected positive duration, got %s", res.Duration)
	}
}

func TestTwoCaptchaBalance(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != twoCaptchaResPath || r.URL.Query().Get("action") != "getbalance" {
			t.Fatalf("unexpected request: %s %s", r.URL.Path, r.URL.Query().Get("action"))
		}
		_, _ = w.Write([]byte("12.34"))
	}))
	defer ts.Close()

	solver := newTwoCaptchaTestSolver("api-key", ts.URL)
	balance, err := solver.Balance(context.Background())
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if balance != 12.34 {
		t.Fatalf("balance = %v, want 12.34", balance)
	}
}

func TestTwoCaptchaPollContextCanceled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-a-number"))
	}))
	defer ts.Close()

	solver := newTwoCaptchaTestSolver("api-key", ts.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := solver.poll(ctx, "task-1")
	if err != context.Canceled {
		t.Fatalf("poll error = %v, want %v", err, context.Canceled)
	}
}

func TestTwoCaptchaBalanceParseError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-a-number"))
	}))
	defer ts.Close()

	solver := newTwoCaptchaTestSolver("api-key", ts.URL)
	_, err := solver.Balance(context.Background())
	if err == nil || !strings.Contains(err.Error(), "parse balance") {
		t.Fatalf("unexpected balance error: %v", err)
	}
}

func TestTwoCaptchaNetworkFailure(t *testing.T) {
	solver := newTwoCaptchaTestSolver("api-key", "http://127.0.0.1:1")
	_, err := solver.submit(context.Background(), models.CaptchaSolveRequest{Type: models.CaptchaTypeImage, SiteKey: "body"})
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
	if !strings.Contains(err.Error(), "send submit request") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAntiCaptchaCreateTaskByType(t *testing.T) {
	tests := []struct {
		name     string
		req      models.CaptchaSolveRequest
		wantType string
		assert   func(t *testing.T, task map[string]any)
	}{
		{
			name:     "recaptcha v2 invisible",
			req:      models.CaptchaSolveRequest{Type: models.CaptchaTypeRecaptchaV2, SiteKey: "site-v2", PageURL: "https://example.com/v2", Invisible: true},
			wantType: "RecaptchaV2TaskProxyless",
			assert: func(t *testing.T, task map[string]any) {
				if task["websiteKey"] != "site-v2" || task["websiteURL"] != "https://example.com/v2" || task["isInvisible"] != true {
					t.Fatalf("unexpected task payload: %#v", task)
				}
			},
		},
		{
			name:     "recaptcha v3",
			req:      models.CaptchaSolveRequest{Type: models.CaptchaTypeRecaptchaV3, SiteKey: "site-v3", PageURL: "https://example.com/v3", MinScore: 0.9},
			wantType: "RecaptchaV3TaskProxyless",
			assert: func(t *testing.T, task map[string]any) {
				if task["websiteKey"] != "site-v3" || task["websiteURL"] != "https://example.com/v3" || task["minScore"] != 0.9 {
					t.Fatalf("unexpected task payload: %#v", task)
				}
			},
		},
		{
			name:     "hcaptcha",
			req:      models.CaptchaSolveRequest{Type: models.CaptchaTypeHCaptcha, SiteKey: "site-h", PageURL: "https://example.com/h"},
			wantType: "HCaptchaTaskProxyless",
			assert: func(t *testing.T, task map[string]any) {
				if task["websiteKey"] != "site-h" || task["websiteURL"] != "https://example.com/h" {
					t.Fatalf("unexpected task payload: %#v", task)
				}
			},
		},
		{
			name:     "image",
			req:      models.CaptchaSolveRequest{Type: models.CaptchaTypeImage, SiteKey: "base64-body"},
			wantType: "ImageToTextTask",
			assert: func(t *testing.T, task map[string]any) {
				if task["body"] != "base64-body" {
					t.Fatalf("unexpected task payload: %#v", task)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var payload antiCaptchaRequest
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/createTask" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatalf("decode: %v", err)
				}
				_ = json.NewEncoder(w).Encode(antiCaptchaResponse{TaskID: 123})
			}))
			defer ts.Close()

			solver := newAntiCaptchaTestSolver("api-key", ts.URL)
			taskID, err := solver.createTask(context.Background(), tc.req)
			if err != nil {
				t.Fatalf("createTask: %v", err)
			}
			if taskID != 123 {
				t.Fatalf("taskID = %d, want 123", taskID)
			}
			if payload.ClientKey != "api-key" {
				t.Fatalf("client key = %q, want api-key", payload.ClientKey)
			}
			task, ok := payload.Task.(map[string]any)
			if !ok {
				t.Fatalf("task payload type = %T, want map[string]any", payload.Task)
			}
			if task["type"] != tc.wantType {
				t.Fatalf("task type = %v, want %s", task["type"], tc.wantType)
			}
			tc.assert(t, task)
		})
	}
}

func TestAntiCaptchaCreateTaskUnsupportedType(t *testing.T) {
	solver := NewAntiCaptcha("api-key")
	_, err := solver.createTask(context.Background(), models.CaptchaSolveRequest{Type: "unknown"})
	if err == nil || !strings.Contains(err.Error(), "unsupported captcha type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAntiCaptchaSolve(t *testing.T) {
	var mu sync.Mutex
	pollCalls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/createTask":
			_ = json.NewEncoder(w).Encode(antiCaptchaResponse{TaskID: 456})
		case "/getTaskResult":
			mu.Lock()
			pollCalls++
			call := pollCalls
			mu.Unlock()
			if call == 1 {
				_ = json.NewEncoder(w).Encode(antiCaptchaResponse{Status: "processing"})
				return
			}
			_ = json.NewEncoder(w).Encode(antiCaptchaResponse{Status: "ready", Solution: struct {
				GRecaptchaResponse string `json:"gRecaptchaResponse"`
				Token              string `json:"token"`
				Text               string `json:"text"`
			}{Token: "anti-token"}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	solver := newAntiCaptchaTestSolver("api-key", ts.URL)
	res, err := solver.Solve(context.Background(), models.CaptchaSolveRequest{Type: models.CaptchaTypeImage, SiteKey: "body"})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}
	if res.Token != "anti-token" {
		t.Fatalf("token = %q, want anti-token", res.Token)
	}
	if res.Duration <= 0 {
		t.Fatalf("expected positive duration, got %s", res.Duration)
	}
}

func TestAntiCaptchaBalance(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getBalance" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(antiCaptchaResponse{Balance: 4.56})
	}))
	defer ts.Close()

	solver := newAntiCaptchaTestSolver("api-key", ts.URL)
	balance, err := solver.Balance(context.Background())
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if balance != 4.56 {
		t.Fatalf("balance = %v, want 4.56", balance)
	}
}

func TestAntiCaptchaPollContextCanceled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer ts.Close()

	solver := newAntiCaptchaTestSolver("api-key", ts.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := solver.pollResult(ctx, 1)
	if err != context.Canceled {
		t.Fatalf("pollResult error = %v, want %v", err, context.Canceled)
	}
}

func TestAntiCaptchaDoPostInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer ts.Close()

	solver := newAntiCaptchaTestSolver("api-key", ts.URL)
	_, err := solver.doPost(context.Background(), ts.URL+"/bad", antiCaptchaRequest{ClientKey: "api-key"})
	if err == nil || !strings.Contains(err.Error(), "unmarshal response") {
		t.Fatalf("unexpected doPost error: %v", err)
	}
}

func TestAntiCaptchaNetworkFailure(t *testing.T) {
	solver := newAntiCaptchaTestSolver("api-key", "http://127.0.0.1:1")
	_, err := solver.doPost(context.Background(), "http://127.0.0.1:1/createTask", antiCaptchaRequest{ClientKey: "api-key"})
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
	if !strings.Contains(err.Error(), "send request") {
		t.Fatalf("unexpected error: %v", err)
	}
}
