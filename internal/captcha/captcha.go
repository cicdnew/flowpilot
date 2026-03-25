package captcha

import (
	"context"
	"fmt"

	"flowpilot/internal/models"
)

type Solver interface {
	Solve(ctx context.Context, req models.CaptchaSolveRequest) (*models.CaptchaSolveResult, error)
	Balance(ctx context.Context) (float64, error)
}

func NewSolver(config models.CaptchaConfig) (Solver, error) {
	switch config.Provider {
	case models.CaptchaProvider2Captcha:
		return NewTwoCaptcha(config.APIKey), nil
	case models.CaptchaProviderAntiCaptcha:
		return NewAntiCaptcha(config.APIKey), nil
	default:
		return nil, fmt.Errorf("unsupported captcha provider: %s", config.Provider)
	}
}
