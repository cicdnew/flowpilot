package vision

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

type CompareResult struct {
	DiffPercent float64
	PixelCount  int64
	TotalPixels int64
	Width       int
	Height      int
}

func Compare(baselinePath, screenshotPath, diffOutputPath string) (*CompareResult, error) {
	baseImg, err := loadPNG(baselinePath)
	if err != nil {
		return nil, fmt.Errorf("load baseline: %w", err)
	}

	newImg, err := loadPNG(screenshotPath)
	if err != nil {
		return nil, fmt.Errorf("load screenshot: %w", err)
	}

	baseBounds := baseImg.Bounds()
	newBounds := newImg.Bounds()

	width := max(baseBounds.Dx(), newBounds.Dx())
	height := max(baseBounds.Dy(), newBounds.Dy())

	diffImg := image.NewRGBA(image.Rect(0, 0, width, height))

	var diffPixels int64
	totalPixels := int64(width) * int64(height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var c1, c2 color.Color
			if image.Pt(x+baseBounds.Min.X, y+baseBounds.Min.Y).In(baseBounds) {
				c1 = baseImg.At(x+baseBounds.Min.X, y+baseBounds.Min.Y)
			} else {
				c1 = color.Transparent
			}
			if image.Pt(x+newBounds.Min.X, y+newBounds.Min.Y).In(newBounds) {
				c2 = newImg.At(x+newBounds.Min.X, y+newBounds.Min.Y)
			} else {
				c2 = color.Transparent
			}

			if colorDiff(c1, c2) > 0.1 {
				diffPixels++
				diffImg.Set(x, y, color.RGBA{R: 255, A: 200})
			} else {
				r, g, b, a := c2.RGBA()
				diffImg.Set(x, y, color.RGBA{
					R: uint8(r >> 9),
					G: uint8(g >> 9),
					B: uint8(b >> 9),
					A: uint8(a >> 8),
				})
			}
		}
	}

	if err := savePNG(diffOutputPath, diffImg); err != nil {
		return nil, fmt.Errorf("save diff image: %w", err)
	}

	diffPercent := 0.0
	if totalPixels > 0 {
		diffPercent = float64(diffPixels) / float64(totalPixels) * 100.0
	}

	return &CompareResult{
		DiffPercent: math.Round(diffPercent*100) / 100,
		PixelCount:  diffPixels,
		TotalPixels: totalPixels,
		Width:       width,
		Height:      height,
	}, nil
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode png %s: %w", path, err)
	}
	return img, nil
}

func savePNG(path string, img image.Image) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encode png %s: %w", path, err)
	}
	return nil
}

func colorDiff(c1, c2 color.Color) float64 {
	r1, g1, b1, a1 := c1.RGBA()
	r2, g2, b2, a2 := c2.RGBA()

	dr := float64(r1) - float64(r2)
	dg := float64(g1) - float64(g2)
	db := float64(b1) - float64(b2)
	da := float64(a1) - float64(a2)

	maxDist := 65535.0 * 2.0
	dist := math.Sqrt(dr*dr+dg*dg+db*db+da*da) / maxDist
	return dist
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
