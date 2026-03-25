package captcha

import (
	"testing"

	"flowpilot/internal/models"
)

func TestNewSolverTwoCaptcha(t *testing.T) {
	solver, err := NewSolver(models.CaptchaConfig{
		Provider: models.CaptchaProvider2Captcha,
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("NewSolver: %v", err)
	}
	if solver == nil {
		t.Fatal("solver is nil")
	}
}

func TestNewSolverAntiCaptcha(t *testing.T) {
	solver, err := NewSolver(models.CaptchaConfig{
		Provider: models.CaptchaProviderAntiCaptcha,
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("NewSolver: %v", err)
	}
	if solver == nil {
		t.Fatal("solver is nil")
	}
}

func TestNewSolverInvalid(t *testing.T) {
	_, err := NewSolver(models.CaptchaConfig{
		Provider: "invalid",
		APIKey:   "test-key",
	})
	if err == nil {
		t.Error("expected error for invalid provider")
	}
}
