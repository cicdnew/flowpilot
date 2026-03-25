package models

import "time"

type VisualBaseline struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	TaskID         string    `json:"taskId,omitempty"`
	URL            string    `json:"url"`
	ScreenshotPath string    `json:"screenshotPath"`
	Width          int       `json:"width"`
	Height         int       `json:"height"`
	CreatedAt      time.Time `json:"createdAt"`
}

type VisualDiff struct {
	ID             string    `json:"id"`
	BaselineID     string    `json:"baselineId"`
	TaskID         string    `json:"taskId"`
	ScreenshotPath string    `json:"screenshotPath"`
	DiffImagePath  string    `json:"diffImagePath"`
	DiffPercent    float64   `json:"diffPercent"`
	PixelCount     int64     `json:"pixelCount"`
	Threshold      float64   `json:"threshold"`
	Passed         bool      `json:"passed"`
	Width          int       `json:"width"`
	Height         int       `json:"height"`
	CreatedAt      time.Time `json:"createdAt"`
}

type DiffRequest struct {
	BaselineID string  `json:"baselineId"`
	TaskID     string  `json:"taskId"`
	Threshold  float64 `json:"threshold"`
}
