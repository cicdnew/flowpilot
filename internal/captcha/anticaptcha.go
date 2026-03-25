package captcha

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"flowpilot/internal/models"
)

type AntiCaptcha struct {
	apiKey    string
	client    *http.Client
	baseURL   string
	pollDelay time.Duration
	maxWait   time.Duration
	backoff   time.Duration
}

func NewAntiCaptcha(apiKey string) *AntiCaptcha {
	return &AntiCaptcha{
		apiKey:    apiKey,
		client:    &http.Client{Timeout: 30 * time.Second},
		baseURL:   "https://api.anti-captcha.com",
		pollDelay: 5 * time.Second,
		maxWait:   120 * time.Second,
		backoff:   15 * time.Second,
	}
}

type antiCaptchaRequest struct {
	ClientKey string      `json:"clientKey"`
	Task      interface{} `json:"task,omitempty"`
	TaskID    int64       `json:"taskId,omitempty"`
}

type antiCaptchaResponse struct {
	ErrorID          int    `json:"errorId"`
	ErrorCode        string `json:"errorCode"`
	ErrorDescription string `json:"errorDescription"`
	TaskID           int64  `json:"taskId"`
	Status           string `json:"status"`
	Solution         struct {
		GRecaptchaResponse string `json:"gRecaptchaResponse"`
		Token              string `json:"token"`
		Text               string `json:"text"`
	} `json:"solution"`
	Balance float64 `json:"balance"`
}

func (a *AntiCaptcha) Solve(ctx context.Context, req models.CaptchaSolveRequest) (*models.CaptchaSolveResult, error) {
	start := time.Now()

	taskID, err := a.createTask(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create captcha task: %w", err)
	}

	token, err := a.pollResult(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("poll captcha result: %w", err)
	}

	return &models.CaptchaSolveResult{
		Token:    token,
		Duration: time.Since(start),
	}, nil
}

func (a *AntiCaptcha) createTask(ctx context.Context, req models.CaptchaSolveRequest) (int64, error) {
	var task interface{}

	switch req.Type {
	case models.CaptchaTypeRecaptchaV2:
		t := map[string]interface{}{
			"type":       "RecaptchaV2TaskProxyless",
			"websiteURL": req.PageURL,
			"websiteKey": req.SiteKey,
		}
		if req.Invisible {
			t["isInvisible"] = true
		}
		task = t
	case models.CaptchaTypeRecaptchaV3:
		t := map[string]interface{}{
			"type":       "RecaptchaV3TaskProxyless",
			"websiteURL": req.PageURL,
			"websiteKey": req.SiteKey,
		}
		if req.MinScore > 0 {
			t["minScore"] = req.MinScore
		}
		task = t
	case models.CaptchaTypeHCaptcha:
		task = map[string]interface{}{
			"type":       "HCaptchaTaskProxyless",
			"websiteURL": req.PageURL,
			"websiteKey": req.SiteKey,
		}
	case models.CaptchaTypeImage:
		task = map[string]interface{}{
			"type": "ImageToTextTask",
			"body": req.SiteKey,
		}
	default:
		return 0, fmt.Errorf("unsupported captcha type: %s", req.Type)
	}

	body := antiCaptchaRequest{
		ClientKey: a.apiKey,
		Task:      task,
	}

	resp, err := a.doPost(ctx, a.baseURL+"/createTask", body)
	if err != nil {
		return 0, err
	}
	if resp.ErrorID != 0 {
		return 0, fmt.Errorf("anticaptcha create task error: %s (%s)", resp.ErrorCode, resp.ErrorDescription)
	}
	return resp.TaskID, nil
}

func (a *AntiCaptcha) pollResult(ctx context.Context, taskID int64) (string, error) {
	deadline := time.Now().Add(a.maxWait)
	delay := a.pollDelay

	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("captcha solve timed out after %s", a.maxWait)
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}

		body := antiCaptchaRequest{
			ClientKey: a.apiKey,
			TaskID:    taskID,
		}

		resp, err := a.doPost(ctx, a.baseURL+"/getTaskResult", body)
		if err != nil {
			return "", err
		}
		if resp.ErrorID != 0 {
			return "", fmt.Errorf("anticaptcha poll error: %s (%s)", resp.ErrorCode, resp.ErrorDescription)
		}

		switch resp.Status {
		case "processing":
			delay = min(delay*2, a.backoff)
			continue
		case "ready":
			token := resp.Solution.GRecaptchaResponse
			if token == "" {
				token = resp.Solution.Token
			}
			if token == "" {
				token = resp.Solution.Text
			}
			return token, nil
		default:
			return "", fmt.Errorf("anticaptcha unexpected status: %s", resp.Status)
		}
	}
}

func (a *AntiCaptcha) Balance(ctx context.Context) (float64, error) {
	body := antiCaptchaRequest{
		ClientKey: a.apiKey,
	}

	resp, err := a.doPost(ctx, a.baseURL+"/getBalance", body)
	if err != nil {
		return 0, err
	}
	if resp.ErrorID != 0 {
		return 0, fmt.Errorf("anticaptcha balance error: %s (%s)", resp.ErrorCode, resp.ErrorDescription)
	}
	return resp.Balance, nil
}

func (a *AntiCaptcha) doPost(ctx context.Context, url string, payload interface{}) (*antiCaptchaResponse, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(httpResp.Body, 512))
		return nil, fmt.Errorf("HTTP %d: %s — %s", httpResp.StatusCode, httpResp.Status, strings.TrimSpace(string(body)))
	}

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result antiCaptchaResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}
