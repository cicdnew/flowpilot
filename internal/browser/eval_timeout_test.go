package browser

import (
	"context"
	"errors"
	"testing"
	"time"

	"flowpilot/internal/models"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

type blockingExecutor struct{}

func (b *blockingExecutor) Run(ctx context.Context, actions ...chromedp.Action) error {
	<-ctx.Done()
	return ctx.Err()
}

func (b *blockingExecutor) RunResponse(ctx context.Context, actions ...chromedp.Action) (*network.Response, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (b *blockingExecutor) Targets(ctx context.Context) ([]*target.Info, error) {
	return nil, nil
}

func TestExecEvalUsesStepTimeout(t *testing.T) {
	r := &Runner{exec: &blockingExecutor{}}
	r.allowEval.Store(true)

	start := time.Now()
	err := r.execEval(context.Background(), models.TaskStep{Action: models.ActionEval, Value: "1+1", Timeout: 1})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("elapsed = %s, want about 1s timeout", elapsed)
	}
}
