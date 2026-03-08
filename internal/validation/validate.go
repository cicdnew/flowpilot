package validation

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode"

	"flowpilot/internal/models"
)

var (
	ErrEmptyName           = errors.New("name must not be empty")
	ErrNameTooLong         = errors.New("name must not exceed 255 characters")
	ErrNameControlChars    = errors.New("name must not contain control characters")
	ErrEmptyURL            = errors.New("url must not be empty")
	ErrInvalidURLScheme    = errors.New("url must use http or https scheme")
	ErrInvalidURL          = errors.New("url is not valid")
	ErrNoSteps             = errors.New("at least one step is required")
	ErrInvalidStepAction   = errors.New("invalid step action")
	ErrStepMissingValue    = errors.New("step requires a value")
	ErrStepInvalidURL      = errors.New("step navigate value must be a valid http/https url")
	ErrStepMissingSelector = errors.New("step requires a non-empty selector")
	ErrEvalNotAllowed      = errors.New("eval steps are not allowed unless explicitly enabled")
	ErrEmptyServer         = errors.New("proxy server must not be empty")
	ErrInvalidServer       = errors.New("proxy server must be in host:port format")
	ErrInvalidPriority     = errors.New("priority must be 1, 5, or 10")
	ErrInvalidProtocol     = errors.New("protocol must be http, https, or socks5")
	ErrTooManyTags         = errors.New("too many tags (max 20)")
	ErrTagTooLong          = errors.New("tag must not exceed 50 characters")
	ErrTagEmpty            = errors.New("tag must not be empty")
	ErrTagControlChars     = errors.New("tag must not contain control characters")
	ErrInvalidStatus       = errors.New("invalid task status")
	ErrInvalidBatchSize    = errors.New("batch size exceeds limit")
	ErrInvalidTemplate     = errors.New("invalid naming template")
	ErrInvalidPage         = errors.New("page must be a positive integer")
	ErrInvalidPageSize     = errors.New("pageSize must be between 1 and 200")
	ErrInvalidFilterStatus = errors.New("invalid filter status")
	ErrTagFilterTooLong    = errors.New("tag filter must not exceed 50 characters")
	ErrTagFilterControl    = errors.New("tag filter must not contain control characters")
)

var validActions = map[models.StepAction]bool{
	models.ActionNavigate:   true,
	models.ActionClick:      true,
	models.ActionType:       true,
	models.ActionWait:       true,
	models.ActionScreenshot: true,
	models.ActionExtract:    true,
	models.ActionScroll:     true,
	models.ActionSelect:     true,
	models.ActionEval:       true,
	models.ActionTabSwitch:  true,
}

var selectorRequiredActions = map[models.StepAction]bool{
	models.ActionClick:   true,
	models.ActionType:    true,
	models.ActionExtract: true,
	models.ActionSelect:  true,
}

var validPriorities = map[models.TaskPriority]bool{
	models.PriorityLow:    true,
	models.PriorityNormal: true,
	models.PriorityHigh:   true,
}

var validProtocols = map[models.ProxyProtocol]bool{
	models.ProxyHTTP:   true,
	models.ProxyHTTPS:  true,
	models.ProxySOCKS5: true,
}

func ValidatePagination(page, pageSize int, status, tag string) error {
	if page < 1 {
		return ErrInvalidPage
	}
	if pageSize < 1 || pageSize > 200 {
		return ErrInvalidPageSize
	}
	if status != "" && status != "all" {
		if !validStatuses[status] {
			return fmt.Errorf("%w: %s", ErrInvalidFilterStatus, status)
		}
	}
	if tag != "" {
		if len(tag) > 50 {
			return ErrTagFilterTooLong
		}
		for _, r := range tag {
			if unicode.IsControl(r) {
				return ErrTagFilterControl
			}
		}
	}
	return nil
}

// ValidateTaskName checks that the name is non-empty, within bounds, and has no control characters.
func ValidateTaskName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrEmptyName
	}
	if len(name) > 255 {
		return ErrNameTooLong
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return ErrNameControlChars
		}
	}
	return nil
}

// ValidateTaskURL checks that the URL is valid and uses http or https.
func ValidateTaskURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return ErrEmptyURL
	}
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("validate task url: %w", ErrInvalidURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrInvalidURLScheme
	}
	if u.Host == "" {
		return fmt.Errorf("validate task url: %w", ErrInvalidURL)
	}
	return nil
}

// ValidateTaskSteps checks that each step has a valid action and required fields.
// allowEval controls whether eval steps are permitted.
func ValidateTaskSteps(steps []models.TaskStep, allowEval bool) error {
	if len(steps) == 0 {
		return ErrNoSteps
	}
	for i, step := range steps {
		if !validActions[step.Action] {
			return fmt.Errorf("step %d: %w: %s", i, ErrInvalidStepAction, step.Action)
		}

		if step.Action == models.ActionEval && !allowEval {
			return fmt.Errorf("step %d: %w", i, ErrEvalNotAllowed)
		}

		if step.Action == models.ActionNavigate {
			if strings.TrimSpace(step.Value) == "" {
				return fmt.Errorf("step %d: %w", i, ErrStepMissingValue)
			}
			if err := ValidateTaskURL(step.Value); err != nil {
				return fmt.Errorf("step %d: %w", i, ErrStepInvalidURL)
			}
		}

		if selectorRequiredActions[step.Action] && strings.TrimSpace(step.Selector) == "" {
			return fmt.Errorf("step %d: %w", i, ErrStepMissingSelector)
		}
	}
	return nil
}

// ValidateProxyServer checks that the server is in valid host:port format.
func ValidateProxyServer(server string) error {
	if strings.TrimSpace(server) == "" {
		return ErrEmptyServer
	}
	host, port, err := net.SplitHostPort(server)
	if err != nil {
		return fmt.Errorf("validate proxy server: %w", ErrInvalidServer)
	}
	if host == "" || port == "" {
		return fmt.Errorf("validate proxy server: %w", ErrInvalidServer)
	}
	return nil
}

// ValidatePriority checks that the priority is a valid value (1, 5, or 10).
func ValidatePriority(priority models.TaskPriority) error {
	if !validPriorities[priority] {
		return ErrInvalidPriority
	}
	return nil
}

// ValidateProxyProtocol checks that the protocol is http, https, or socks5.
func ValidateProxyProtocol(protocol models.ProxyProtocol) error {
	if !validProtocols[protocol] {
		return ErrInvalidProtocol
	}
	return nil
}

// ValidateTags checks that tags are reasonable in length and content.
func ValidateTags(tags []string) error {
	if len(tags) > 20 {
		return ErrTooManyTags
	}
	for i, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			return fmt.Errorf("tag %d: %w", i, ErrTagEmpty)
		}
		if len(tag) > 50 {
			return fmt.Errorf("tag %d: %w", i, ErrTagTooLong)
		}
		for _, r := range tag {
			if unicode.IsControl(r) {
				return fmt.Errorf("tag %d: %w", i, ErrTagControlChars)
			}
		}
	}
	return nil
}

// ValidateTask validates all fields of a task for creation.
// allowEval controls whether eval steps are permitted.
func ValidateTask(name, rawURL string, steps []models.TaskStep, priority models.TaskPriority, allowEval bool) error {
	if err := ValidateTaskName(name); err != nil {
		return fmt.Errorf("validate task: %w", err)
	}
	if err := ValidateTaskURL(rawURL); err != nil {
		return fmt.Errorf("validate task: %w", err)
	}
	if err := ValidateTaskSteps(steps, allowEval); err != nil {
		return fmt.Errorf("validate task: %w", err)
	}
	if err := ValidatePriority(priority); err != nil {
		return fmt.Errorf("validate task: %w", err)
	}
	return nil
}

// ValidateProxy validates proxy server and protocol for creation.
func ValidateProxy(server string, protocol models.ProxyProtocol) error {
	if err := ValidateProxyServer(server); err != nil {
		return fmt.Errorf("validate proxy: %w", err)
	}
	if err := ValidateProxyProtocol(protocol); err != nil {
		return fmt.Errorf("validate proxy: %w", err)
	}
	return nil
}

// validStatuses defines the set of valid task status values.
var validStatuses = map[string]bool{
	"pending":   true,
	"queued":    true,
	"running":   true,
	"completed": true,
	"failed":    true,
	"cancelled": true,
	"retrying":  true,
}

// ValidateStatus checks that the status is a valid task status value.
func ValidateStatus(status string) error {
	if !validStatuses[status] {
		return fmt.Errorf("%w: %s", ErrInvalidStatus, status)
	}
	return nil
}

// ValidateBatchInput validates batch creation inputs.
func ValidateBatchInput(input models.AdvancedBatchInput) error {
	if len(input.URLs) == 0 {
		return fmt.Errorf("batch input: %w", ErrEmptyURL)
	}
	if len(input.URLs) > models.MaxBatchSize {
		return fmt.Errorf("batch input: %w", ErrInvalidBatchSize)
	}
	if strings.TrimSpace(input.FlowID) == "" {
		return fmt.Errorf("batch input: flowId is required")
	}
	if err := ValidatePriority(models.TaskPriority(input.Priority)); err != nil {
		return fmt.Errorf("batch input: %w", err)
	}
	for i, rawURL := range input.URLs {
		if err := ValidateTaskURL(rawURL); err != nil {
			return fmt.Errorf("batch input url %d: %w", i, err)
		}
	}
	if input.NamingTemplate != "" && !models.ValidateBatchTemplate(input.NamingTemplate) {
		return fmt.Errorf("batch input: %w", ErrInvalidTemplate)
	}
	if err := ValidateTags(input.Tags); err != nil {
		return fmt.Errorf("batch input: %w", err)
	}
	return nil
}
