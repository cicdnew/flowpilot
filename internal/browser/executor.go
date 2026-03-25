package browser

import (
	"context"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// Executor abstracts chromedp operations for testability.
type Executor interface {
	Run(ctx context.Context, actions ...chromedp.Action) error
	RunResponse(ctx context.Context, actions ...chromedp.Action) (*network.Response, error)
	Targets(ctx context.Context) ([]*target.Info, error)
}

type chromeExecutor struct{}

func (chromeExecutor) Run(ctx context.Context, actions ...chromedp.Action) error {
	return chromedp.Run(ctx, actions...)
}

func (chromeExecutor) RunResponse(ctx context.Context, actions ...chromedp.Action) (*network.Response, error) {
	return chromedp.RunResponse(ctx, actions...)
}

func (chromeExecutor) Targets(ctx context.Context) ([]*target.Info, error) {
	return chromedp.Targets(ctx)
}
