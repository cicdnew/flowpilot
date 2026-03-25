package recorder

import (
	"context"

	"github.com/chromedp/chromedp"
)

type CDPClient interface {
	Run(ctx context.Context, actions ...chromedp.Action) error
	ListenTarget(ctx context.Context, fn func(ev any))
}

type chromeCDPClient struct{}

func (chromeCDPClient) Run(ctx context.Context, actions ...chromedp.Action) error {
	return chromedp.Run(ctx, actions...)
}

func (chromeCDPClient) ListenTarget(ctx context.Context, fn func(ev any)) {
	chromedp.ListenTarget(ctx, fn)
}
