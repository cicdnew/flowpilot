package models

import "fmt"

// RepeatMode specifies how a task should be repeated.
type RepeatMode string

const (
	RepeatModeNone     RepeatMode = "none"     // Single execution
	RepeatModeCounter  RepeatMode = "counter"  // Repeat with incrementing counter
	RepeatModeRange    RepeatMode = "range"    // Repeat with custom range values
	RepeatModeList     RepeatMode = "list"     // Repeat with custom list of values
)

// RepeatConfig defines how to repeat a task with parameter substitution.
type RepeatConfig struct {
	Mode      RepeatMode `json:"mode"`      // Repeat mode
	VarName   string     `json:"varName"`   // Variable name for substitution (e.g., "counter", "value")
	StartVal  int        `json:"startVal"`  // Start value for counter/range mode
	EndVal    int        `json:"endVal"`    // End value for counter/range mode (inclusive)
	Step      int        `json:"step"`      // Step increment (default 1)
	Values    []string   `json:"values"`    // Custom values for list mode
	BatchSize int        `json:"batchSize"` // Optional: split into batches of this size
}

// Validate checks if the repeat configuration is valid.
func (r *RepeatConfig) Validate() error {
	if r.Mode == RepeatModeNone {
		return nil
	}

	if r.VarName == "" {
		return fmt.Errorf("varName is required for repeat mode %s", r.Mode)
	}

	switch r.Mode {
	case RepeatModeCounter, RepeatModeRange:
		if r.EndVal < r.StartVal {
			return fmt.Errorf("endVal (%d) must be >= startVal (%d)", r.EndVal, r.StartVal)
		}
		if r.Step <= 0 {
			return fmt.Errorf("step must be > 0")
		}
		count := (r.EndVal-r.StartVal)/r.Step + 1
		if count > MaxBatchSize {
			return fmt.Errorf("repeat count %d exceeds max batch size %d", count, MaxBatchSize)
		}

	case RepeatModeList:
		if len(r.Values) == 0 {
			return fmt.Errorf("values list cannot be empty for list mode")
		}
		if len(r.Values) > MaxBatchSize {
			return fmt.Errorf("values count %d exceeds max batch size %d", len(r.Values), MaxBatchSize)
		}

	default:
		return fmt.Errorf("unsupported repeat mode: %s", r.Mode)
	}

	return nil
}

// GenerateValues returns the list of values to iterate over.
func (r *RepeatConfig) GenerateValues() []string {
	if r.Mode == RepeatModeNone {
		return []string{""}
	}

	switch r.Mode {
	case RepeatModeCounter, RepeatModeRange:
		step := r.Step
		if step <= 0 {
			step = 1
		}
		values := []string{}
		for i := r.StartVal; i <= r.EndVal; i += step {
			values = append(values, fmt.Sprintf("%d", i))
		}
		return values

	case RepeatModeList:
		return r.Values

	default:
		return []string{""}
	}
}

// Count returns the total number of iterations.
func (r *RepeatConfig) Count() int {
	return len(r.GenerateValues())
}

// RepeatTaskInput defines input for creating repeated task instances.
type RepeatTaskInput struct {
	Name          string             `json:"name"`
	URL           string             `json:"url"`
	Steps         []TaskStep         `json:"steps"`
	Repeat        RepeatConfig       `json:"repeat"`
	ProxyConfig   ProxyConfig        `json:"proxy"`
	Priority      int                `json:"priority"`
	AutoStart     bool               `json:"autoStart"`
	Tags          []string           `json:"tags,omitempty"`
	Timeout       int                `json:"timeout,omitempty"`
	LoggingPolicy *TaskLoggingPolicy `json:"loggingPolicy,omitempty"`
	Headless      *bool              `json:"headless,omitempty"`
}
