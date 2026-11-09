package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"unicode"

	sdk "flowpilot/sdk/go/plugin"
)

var (
	placeholder = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)
	nonSlug     = regexp.MustCompile(`[^a-z0-9]+`)
)

type textTools struct {
	sdk.Base
	blockedHosts []string
}

func (t *textTools) Init(_ context.Context, config map[string]any) error {
	raw, ok := config["blocked_hosts"]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return errors.New("config.blocked_hosts must be a list of host names")
	}
	for _, v := range list {
		host, ok := v.(string)
		if !ok || strings.TrimSpace(host) == "" {
			return errors.New("config.blocked_hosts must contain non-empty strings")
		}
		t.blockedHosts = append(t.blockedHosts, strings.ToLower(strings.TrimSpace(host)))
	}
	return nil
}

func (t *textTools) Execute(_ context.Context, sc sdk.StepContext) (sdk.StepResult, error) {
	if sc.Step.Action != "text_transform" {
		return t.Base.Execute(context.Background(), sc)
	}
	if sc.Step.VarName == "" {
		return sdk.StepResult{}, errors.New("text_transform: varName is required")
	}
	text := placeholder.ReplaceAllStringFunc(sc.Step.Value, func(m string) string {
		return sc.Variables[placeholder.FindStringSubmatch(m)[1]]
	})
	out, err := transform(sc.Step.Operator, text)
	if err != nil {
		return sdk.StepResult{}, err
	}
	return sdk.StepResult{
		Variables: map[string]string{sc.Step.VarName: out},
		Logs:      []string{fmt.Sprintf("%s -> %s (%d chars)", sc.Step.Operator, sc.Step.VarName, len(out))},
	}, nil
}

func transform(op, s string) (string, error) {
	switch op {
	case "", "trim":
		return strings.TrimSpace(s), nil
	case "upper":
		return strings.ToUpper(s), nil
	case "lower":
		return strings.ToLower(s), nil
	case "title":
		prev := ' '
		return strings.Map(func(r rune) rune {
			defer func() { prev = r }()
			if unicode.IsSpace(prev) {
				return unicode.ToUpper(r)
			}
			return unicode.ToLower(r)
		}, s), nil
	case "reverse":
		r := []rune(s)
		slices.Reverse(r)
		return string(r), nil
	case "slug":
		return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-"), nil
	default:
		return "", fmt.Errorf("text_transform: unsupported operator %q (use trim, upper, lower, title, reverse or slug)", op)
	}
}

func (t *textTools) Hook(_ context.Context, ev sdk.HookEvent) error {
	if ev.Point != sdk.HookPreExecute || ev.Context.Step.Action != "navigate" || len(t.blockedHosts) == 0 {
		return nil
	}
	u, err := url.Parse(ev.Context.Step.Value)
	if err != nil {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	for _, blocked := range t.blockedHosts {
		if host == blocked || strings.HasSuffix(host, "."+blocked) {
			return fmt.Errorf("navigation to %s is blocked by policy", host)
		}
	}
	return nil
}

func main() {
	if err := sdk.Serve(&textTools{}); err != nil {
		sdk.Logf("text-tools: %v", err)
		os.Exit(1)
	}
}
