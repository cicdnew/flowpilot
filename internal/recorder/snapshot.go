package recorder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"flowpilot/internal/models"

	"github.com/chromedp/chromedp"
)

// Snapshotter captures DOM HTML and screenshots for recorded steps.
type Snapshotter struct {
	outputDir string
	cdp       CDPClient
}

// NewSnapshotter creates a snapshotter for a given output directory.
func NewSnapshotter(outputDir string) (*Snapshotter, error) {
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return nil, fmt.Errorf("create snapshot dir: %w", err)
	}
	return &Snapshotter{outputDir: outputDir, cdp: chromeCDPClient{}}, nil
}

// CaptureSnapshot captures the current DOM HTML and a full screenshot.
func (s *Snapshotter) CaptureSnapshot(ctx context.Context, flowID string, stepIndex int) (models.DOMSnapshot, error) {
	var html string
	var buf []byte
	if err := s.cdp.Run(ctx,
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
		chromedp.FullScreenshot(&buf, 90),
	); err != nil {
		return models.DOMSnapshot{}, fmt.Errorf("capture dom snapshot: %w", err)
	}

	filename := fmt.Sprintf("%s_step_%d_%d.png", sanitize(flowID), stepIndex, time.Now().UnixMilli())
	path := filepath.Join(s.outputDir, filename)
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		return models.DOMSnapshot{}, fmt.Errorf("write snapshot: %w", err)
	}

	var url string
	_ = s.cdp.Run(ctx, chromedp.Location(&url))

	return models.DOMSnapshot{
		ID:             fmt.Sprintf("snap_%d", time.Now().UnixNano()),
		FlowID:         flowID,
		StepIndex:      stepIndex,
		HTML:           html,
		ScreenshotPath: path,
		URL:            url,
		CapturedAt:     time.Now(),
	}, nil
}

func sanitize(value string) string {
	result := make([]rune, 0, len(value))
	for _, r := range value {
		switch r {
		case '/', '\\', '.', '\x00':
			result = append(result, '_')
		default:
			result = append(result, r)
		}
	}
	return string(result)
}
