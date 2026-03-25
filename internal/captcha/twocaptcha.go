package captcha

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"flowpilot/internal/models"
)

const (
	twoCaptchaInPath  = "/in.php"
	twoCaptchaResPath = "/res.php"
)

type TwoCaptcha struct {
	apiKey      string
	client      *http.Client
	baseURL     string
	pollDelay   time.Duration
	maxWait     time.Duration
	backoffMax  time.Duration
}

func NewTwoCaptcha(apiKey string) *TwoCaptcha {
	return &TwoCaptcha{
		apiKey:     apiKey,
		client:     &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://2captcha.com",
		pollDelay:  5 * time.Second,
		maxWait:    120 * time.Second,
		backoffMax: 15 * time.Second,
	}
}

func (t *TwoCaptcha) Solve(ctx context.Context, req models.CaptchaSolveRequest) (*models.CaptchaSolveResult, error) {
	start := time.Now()

	taskID, err := t.submit(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("submit captcha: %w", err)
	}

	token, err := t.poll(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("poll captcha result: %w", err)
	}

	return &models.CaptchaSolveResult{
		Token:    token,
		Duration: time.Since(start),
	}, nil
}

func (t *TwoCaptcha) submit(ctx context.Context, req models.CaptchaSolveRequest) (string, error) {
	params := url.Values{
		"key":  {t.apiKey},
		"json": {"0"},
	}

	switch req.Type {
	case models.CaptchaTypeRecaptchaV2:
		params.Set("method", "userrecaptcha")
		params.Set("googlekey", req.SiteKey)
		params.Set("pageurl", req.PageURL)
		if req.Invisible {
			params.Set("invisible", "1")
		}
	case models.CaptchaTypeRecaptchaV3:
		params.Set("method", "userrecaptcha")
		params.Set("version", "v3")
		params.Set("googlekey", req.SiteKey)
		params.Set("pageurl", req.PageURL)
		if req.MinScore > 0 {
			params.Set("min_score", fmt.Sprintf("%.1f", req.MinScore))
		}
	case models.CaptchaTypeHCaptcha:
		params.Set("method", "hcaptcha")
		params.Set("sitekey", req.SiteKey)
		params.Set("pageurl", req.PageURL)
	case models.CaptchaTypeImage:
		params.Set("method", "base64")
		params.Set("body", req.SiteKey)
	default:
		return "", fmt.Errorf("unsupported captcha type: %s", req.Type)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+twoCaptchaInPath, strings.NewReader(params.Encode()))
	if err != nil {
		return "", fmt.Errorf("create submit request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("send submit request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("HTTP %d: %s — %s", resp.StatusCode, resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		return "", fmt.Errorf("read submit response: %w", err)
	}

	text := strings.TrimSpace(string(body))
	if !strings.HasPrefix(text, "OK|") {
		return "", fmt.Errorf("2captcha submit error: %s", text)
	}

	return strings.TrimPrefix(text, "OK|"), nil
}

func (t *TwoCaptcha) poll(ctx context.Context, taskID string) (string, error) {
	deadline := time.Now().Add(t.maxWait)
	delay := t.pollDelay

	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("captcha solve timed out after %s", t.maxWait)
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}

		params := url.Values{
			"key":    {t.apiKey},
			"action": {"get"},
			"id":     {taskID},
			"json":   {"0"},
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL+twoCaptchaResPath+"?"+params.Encode(), nil)
		if err != nil {
			return "", fmt.Errorf("create poll request: %w", err)
		}

		resp, err := t.client.Do(httpReq)
		if err != nil {
			return "", fmt.Errorf("send poll request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			resp.Body.Close()
			return "", fmt.Errorf("HTTP %d: %s — %s", resp.StatusCode, resp.Status, strings.TrimSpace(string(body)))
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB limit
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("read poll response: %w", err)
		}

		text := strings.TrimSpace(string(body))
		if text == "CAPCHA_NOT_READY" {
			delay = min(delay*2, t.backoffMax)
			continue
		}
		if strings.HasPrefix(text, "OK|") {
			return strings.TrimPrefix(text, "OK|"), nil
		}
		return "", fmt.Errorf("2captcha poll error: %s", text)
	}
}

func (t *TwoCaptcha) Balance(ctx context.Context) (float64, error) {
	params := url.Values{
		"key":    {t.apiKey},
		"action": {"getbalance"},
		"json":   {"0"},
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL+twoCaptchaResPath+"?"+params.Encode(), nil)
	if err != nil {
		return 0, fmt.Errorf("create balance request: %w", err)
	}

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("send balance request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("HTTP %d: %s — %s", resp.StatusCode, resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		return 0, fmt.Errorf("read balance response: %w", err)
	}

	text := strings.TrimSpace(string(body))
	var balance float64
	if _, err := fmt.Sscanf(text, "%f", &balance); err != nil {
		return 0, fmt.Errorf("parse balance %q: %w", text, err)
	}
	return balance, nil
}
