package browser

import (
	"context"
	"testing"
	"time"
)

func TestDebugControllerPauseResume(t *testing.T) {
	dc := newDebugController()
	dc.pause()

	released := make(chan error, 1)
	go func() {
		released <- dc.waitIfPaused(context.Background())
	}()

	select {
	case err := <-released:
		t.Fatalf("waitIfPaused returned early: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	dc.resume()

	select {
	case err := <-released:
		if err != nil {
			t.Fatalf("waitIfPaused returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("waitIfPaused did not resume")
	}
}

func TestDebugControllerStepAllowsProgressWhilePaused(t *testing.T) {
	dc := newDebugController()
	dc.pause()

	released := make(chan error, 1)
	go func() {
		released <- dc.waitIfPaused(context.Background())
	}()

	select {
	case err := <-released:
		t.Fatalf("waitIfPaused returned early: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	dc.step()

	select {
	case err := <-released:
		if err != nil {
			t.Fatalf("waitIfPaused returned error after step: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("waitIfPaused did not progress after step")
	}
}
