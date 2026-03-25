package models

import "time"

type CaptchaProvider string

const (
	CaptchaProvider2Captcha    CaptchaProvider = "2captcha"
	CaptchaProviderAntiCaptcha CaptchaProvider = "anticaptcha"
)

type CaptchaType string

const (
	CaptchaTypeRecaptchaV2 CaptchaType = "recaptcha_v2"
	CaptchaTypeRecaptchaV3 CaptchaType = "recaptcha_v3"
	CaptchaTypeHCaptcha    CaptchaType = "hcaptcha"
	CaptchaTypeImage       CaptchaType = "image"
)

type CaptchaConfig struct {
	ID           string          `json:"id"`
	Provider     CaptchaProvider `json:"provider"`
	APIKey       string          `json:"apiKey"`
	Enabled      bool            `json:"enabled"`
	BalanceCache float64         `json:"balance,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

type CaptchaSolveRequest struct {
	Type      CaptchaType `json:"type"`
	SiteKey   string      `json:"siteKey"`
	PageURL   string      `json:"pageUrl"`
	Invisible bool        `json:"invisible,omitempty"`
	MinScore  float64     `json:"minScore,omitempty"`
}

type CaptchaSolveResult struct {
	Token    string        `json:"token"`
	Duration time.Duration `json:"duration"`
}
