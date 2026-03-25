package browser

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"flowpilot/internal/models"

	"github.com/chromedp/chromedp"
)

type elementState struct {
	Exists  bool `json:"exists"`
	Visible bool `json:"visible"`
	Enabled bool `json:"enabled"`
}

func (r *Runner) getElementState(ctx context.Context, selector string) (elementState, error) {
	var state elementState
	checkJS := fmt.Sprintf(`(function() {
		var el = document.querySelector(%q);
		if (!el) return {exists:false, visible:false, enabled:false};
		var style = window.getComputedStyle(el);
		return {
			exists: true,
			visible: el.offsetWidth > 0 && el.offsetHeight > 0 && style.display !== 'none' && style.visibility !== 'hidden',
			enabled: !el.disabled && !el.readOnly
		};
	})()`, selector)
	if err := r.exec.Run(ctx, chromedp.Evaluate(checkJS, &state)); err != nil {
		return elementState{}, err
	}
	return state, nil
}

func (r *Runner) evaluateCondition(ctx context.Context, step models.TaskStep, vars map[string]string) (bool, error) {
	switch step.Action {
	case models.ActionIfElement:
		state, err := r.getElementState(ctx, step.Selector)
		if err != nil {
			return false, fmt.Errorf("if_element check: %w", err)
		}
		switch step.Condition {
		case "not_exists":
			return !state.Exists, nil
		default:
			return state.Exists, nil
		}

	case models.ActionIfText:
		var text string
		if err := r.exec.Run(ctx,
			chromedp.Text(step.Selector, &text, chromedp.ByQuery),
		); err != nil {
			return false, fmt.Errorf("if_text get text: %w", err)
		}
		return evaluateTextCondition(step.Condition, text, vars)

	case models.ActionIfURL:
		var currentURL string
		if err := r.exec.Run(ctx, chromedp.Location(&currentURL)); err != nil {
			return false, fmt.Errorf("if_url get location: %w", err)
		}
		return evaluateTextCondition(step.Condition, currentURL, vars)

	case models.ActionWhile:
		if step.Selector != "" {
			switch step.Condition {
			case "", "exists", "not_exists":
				state, err := r.getElementState(ctx, step.Selector)
				if err != nil {
					return false, fmt.Errorf("while_condition element check: %w", err)
				}
				if step.Condition == "not_exists" {
					return !state.Exists, nil
				}
				return state.Exists, nil
			default:
				var text string
				if err := r.exec.Run(ctx, chromedp.Text(step.Selector, &text, chromedp.ByQuery)); err != nil {
					return false, fmt.Errorf("while_condition text check: %w", err)
				}
				return evaluateTextCondition(step.Condition, text, vars)
			}
		}
		var currentURL string
		if err := r.exec.Run(ctx, chromedp.Location(&currentURL)); err != nil {
			return false, fmt.Errorf("while_condition get location: %w", err)
		}
		return evaluateTextCondition(step.Condition, currentURL, vars)

	default:
		return false, fmt.Errorf("unknown condition action: %s", step.Action)
	}
}

func evaluateTextCondition(condition, text string, vars map[string]string) (bool, error) {
	for k, v := range vars {
		condition = strings.ReplaceAll(condition, "{{"+k+"}}", v)
	}

	parts := strings.SplitN(condition, ":", 2)
	if len(parts) != 2 {
		return strings.Contains(text, condition), nil
	}

	op, val := parts[0], parts[1]
	switch op {
	case "contains":
		return strings.Contains(text, val), nil
	case "not_contains":
		return !strings.Contains(text, val), nil
	case "equals":
		return text == val, nil
	case "not_equals":
		return text != val, nil
	case "starts_with":
		return strings.HasPrefix(text, val), nil
	case "ends_with":
		return strings.HasSuffix(text, val), nil
	case "matches":
		re, err := regexp.Compile(val)
		if err != nil {
			return false, fmt.Errorf("invalid regex in condition: %w", err)
		}
		return re.MatchString(text), nil
	default:
		return false, fmt.Errorf("unknown condition operator: %s", op)
	}
}

func buildLabelIndex(steps []models.TaskStep) map[string]int {
	idx := make(map[string]int, len(steps))
	for i, s := range steps {
		if s.Label != "" {
			idx[s.Label] = i
		}
	}
	return idx
}

func findEndLoop(steps []models.TaskStep, loopPC int) int {
	depth := 0
	for i := loopPC; i < len(steps); i++ {
		if steps[i].Action == models.ActionLoop {
			depth++
		}
		if steps[i].Action == models.ActionEndLoop {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func findEndWhile(steps []models.TaskStep, whilePC int) int {
	depth := 0
	for i := whilePC; i < len(steps); i++ {
		if steps[i].Action == models.ActionWhile {
			depth++
		}
		if steps[i].Action == models.ActionEndWhile {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
