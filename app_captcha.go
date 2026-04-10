package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"flowpilot/internal/captcha"
	"flowpilot/internal/models"

	"github.com/google/uuid"
)

const errSaveCaptchaConfig = "save captcha config: %w"

func (a *App) SaveCaptchaConfig(provider, apiKey string) (*models.CaptchaConfig, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if provider == "" {
		return nil, fmt.Errorf("save captcha config: provider is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("save captcha config: apiKey is required")
	}

	p := models.CaptchaProvider(provider)
	if p != models.CaptchaProvider2Captcha && p != models.CaptchaProviderAntiCaptcha {
		return nil, fmt.Errorf("save captcha config: unsupported provider %q", provider)
	}

	existing, err := a.db.GetActiveCaptchaConfig(a.ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf(errSaveCaptchaConfig, err)
	}
	if existing != nil {
		existing.Provider = p
		existing.APIKey = apiKey
		existing.Enabled = true
		if err := a.db.UpdateCaptchaConfig(a.ctx, *existing); err != nil {
			return nil, fmt.Errorf(errSaveCaptchaConfig, err)
		}
		existing.APIKey = maskCredential(existing.APIKey)
		a.refreshCaptchaSolver()
		return existing, nil
	}

	now := time.Now()
	c := models.CaptchaConfig{
		ID:        uuid.New().String(),
		Provider:  p,
		APIKey:    apiKey,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := a.db.CreateCaptchaConfig(a.ctx, c); err != nil {
		return nil, fmt.Errorf(errSaveCaptchaConfig, err)
	}
	c.APIKey = maskCredential(c.APIKey)
	a.refreshCaptchaSolver()
	return &c, nil
}

func (a *App) GetCaptchaConfig() (*models.CaptchaConfig, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	c, err := a.db.GetActiveCaptchaConfig(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("get captcha config: %w", err)
	}
	if c == nil {
		return nil, nil
	}
	c.APIKey = maskCredential(c.APIKey)
	return c, nil
}

func (a *App) ListCaptchaConfigs() ([]models.CaptchaConfig, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	configs, err := a.db.ListCaptchaConfigs(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("list captcha configs: %w", err)
	}
	for i := range configs {
		configs[i].APIKey = maskCredential(configs[i].APIKey)
	}
	return configs, nil
}

func (a *App) DeleteCaptchaConfig(id string) error {
	if err := a.ready(); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("delete captcha config: id is required")
	}
	if err := a.db.DeleteCaptchaConfig(a.ctx, id); err != nil {
		return fmt.Errorf("delete captcha config: %w", err)
	}
	a.refreshCaptchaSolver()
	return nil
}

func (a *App) TestCaptchaConfig(id string) (float64, error) {
	if err := a.ready(); err != nil {
		return 0, err
	}
	if id == "" {
		return 0, fmt.Errorf("test captcha config: id is required")
	}
	c, err := a.db.GetCaptchaConfig(a.ctx, id)
	if err != nil {
		return 0, fmt.Errorf(errTestCaptchaConfig, err)
	}
	solver, err := captcha.NewSolver(*c)
	if err != nil {
		return 0, fmt.Errorf(errTestCaptchaConfig, err)
	}
	balance, err := solver.Balance(a.ctx)
	if err != nil {
		return 0, fmt.Errorf(errTestCaptchaConfig, err)
	}
	return balance, nil
}

func (a *App) refreshCaptchaSolver() {
	c, err := a.db.GetActiveCaptchaConfig(a.ctx)
	if err != nil || c == nil {
		a.runner.SetCaptchaSolver(nil)
		return
	}
	solver, err := captcha.NewSolver(*c)
	if err != nil {
		a.runner.SetCaptchaSolver(nil)
		return
	}
	a.runner.SetCaptchaSolver(solver)
}
