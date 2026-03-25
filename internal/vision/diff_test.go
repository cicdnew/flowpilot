package vision

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func createTestPNG(t *testing.T, path string, width, height int, fill color.Color) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, fill)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test png: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
}

func TestCompareIdentical(t *testing.T) {
	dir := t.TempDir()
	baseline := filepath.Join(dir, "baseline.png")
	screenshot := filepath.Join(dir, "screenshot.png")
	diffPath := filepath.Join(dir, "diff.png")

	createTestPNG(t, baseline, 100, 100, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	createTestPNG(t, screenshot, 100, 100, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	result, err := Compare(baseline, screenshot, diffPath)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if result.DiffPercent != 0 {
		t.Errorf("expected 0%% diff, got %.2f%%", result.DiffPercent)
	}
	if result.PixelCount != 0 {
		t.Errorf("expected 0 diff pixels, got %d", result.PixelCount)
	}

	if _, err := os.Stat(diffPath); os.IsNotExist(err) {
		t.Error("diff image was not created")
	}
}

func TestCompareDifferent(t *testing.T) {
	dir := t.TempDir()
	baseline := filepath.Join(dir, "baseline.png")
	screenshot := filepath.Join(dir, "screenshot.png")
	diffPath := filepath.Join(dir, "diff.png")

	createTestPNG(t, baseline, 100, 100, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	createTestPNG(t, screenshot, 100, 100, color.RGBA{R: 0, G: 0, B: 255, A: 255})

	result, err := Compare(baseline, screenshot, diffPath)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if result.DiffPercent == 0 {
		t.Error("expected non-zero diff")
	}
	if result.PixelCount == 0 {
		t.Error("expected non-zero diff pixel count")
	}
}

func TestCompareDifferentSizes(t *testing.T) {
	dir := t.TempDir()
	baseline := filepath.Join(dir, "baseline.png")
	screenshot := filepath.Join(dir, "screenshot.png")
	diffPath := filepath.Join(dir, "diff.png")

	createTestPNG(t, baseline, 100, 100, color.RGBA{R: 255, A: 255})
	createTestPNG(t, screenshot, 150, 120, color.RGBA{R: 255, A: 255})

	result, err := Compare(baseline, screenshot, diffPath)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if result.Width != 150 {
		t.Errorf("expected width 150, got %d", result.Width)
	}
	if result.Height != 120 {
		t.Errorf("expected height 120, got %d", result.Height)
	}
}

func TestCompareMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := Compare(filepath.Join(dir, "missing.png"), filepath.Join(dir, "also_missing.png"), filepath.Join(dir, "diff.png"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestColorDiff(t *testing.T) {
	c1 := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	c2 := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	if d := colorDiff(c1, c2); d != 0 {
		t.Errorf("identical colors should have 0 diff, got %f", d)
	}

	c3 := color.RGBA{R: 0, G: 255, B: 0, A: 255}
	if d := colorDiff(c1, c3); d == 0 {
		t.Error("different colors should have non-zero diff")
	}
}
