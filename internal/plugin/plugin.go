package plugin

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sync"
	"time"

	"flowpilot/internal/models"
	sdk "flowpilot/sdk/go/plugin"
)

type (
	HookPoint   = sdk.HookPoint
	StepContext = sdk.StepContext
	StepResult  = sdk.StepResult
	HookEvent   = sdk.HookEvent
)

const (
	HookPreExecute  = sdk.HookPreExecute
	HookPostExecute = sdk.HookPostExecute
	HookOnError     = sdk.HookOnError
)

type Type string

const (
	TypeProcess Type = "process"
	TypeBuiltin Type = "builtin"
)

const (
	CapVariablesRead  = "variables.read"
	CapVariablesWrite = "variables.write"
	CapExtractedWrite = "extracted.write"
)

var knownCapabilities = map[string]bool{
	CapVariablesRead:  true,
	CapVariablesWrite: true,
	CapExtractedWrite: true,
}

var (
	ErrInvalidManifest = errors.New("invalid plugin manifest")
	ErrConflict        = errors.New("plugin conflict")
	ErrNotFound        = errors.New("plugin not found")
	ErrVetoed          = errors.New("step vetoed by plugin")
)

var (
	idPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	actionPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)
)

type ActionSpec struct {
	Name             models.StepAction `json:"name"`
	Description      string            `json:"description,omitempty"`
	SelectorRequired bool              `json:"selector_required,omitempty"`
	ValueRequired    bool              `json:"value_required,omitempty"`
}

type Manifest struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	Description  string         `json:"description,omitempty"`
	Type         Type           `json:"type"`
	Entrypoint   string         `json:"entrypoint,omitempty"`
	Args         []string       `json:"args,omitempty"`
	Actions      []ActionSpec   `json:"actions,omitempty"`
	Hooks        []HookPoint    `json:"hooks,omitempty"`
	Capabilities []string       `json:"capabilities,omitempty"`
	Config       map[string]any `json:"config,omitempty"`
	TimeoutMs    int            `json:"timeout_ms,omitempty"`
	MaxRestarts  int            `json:"max_restarts,omitempty"`
}

func (m Manifest) HasCapability(capability string) bool {
	return slices.Contains(m.Capabilities, capability)
}

func (m Manifest) HasHook(point HookPoint) bool {
	return slices.Contains(m.Hooks, point)
}

func (m Manifest) Validate() error {
	if !idPattern.MatchString(m.ID) {
		return fmt.Errorf("%w: id %q must match %s", ErrInvalidManifest, m.ID, idPattern)
	}
	if m.Name == "" || m.Version == "" {
		return fmt.Errorf("%w: %s: name and version are required", ErrInvalidManifest, m.ID)
	}
	switch m.Type {
	case TypeProcess:
		if m.Entrypoint == "" {
			return fmt.Errorf("%w: %s: process plugins need an entrypoint", ErrInvalidManifest, m.ID)
		}
	case TypeBuiltin:
	default:
		return fmt.Errorf("%w: %s: unsupported type %q", ErrInvalidManifest, m.ID, m.Type)
	}
	if len(m.Actions) == 0 && len(m.Hooks) == 0 {
		return fmt.Errorf("%w: %s: declares no actions or hooks", ErrInvalidManifest, m.ID)
	}
	if m.TimeoutMs < 0 {
		return fmt.Errorf("%w: %s: timeout_ms must not be negative", ErrInvalidManifest, m.ID)
	}
	seen := make(map[models.StepAction]bool, len(m.Actions))
	for _, a := range m.Actions {
		if !actionPattern.MatchString(string(a.Name)) {
			return fmt.Errorf("%w: %s: action %q must match %s", ErrInvalidManifest, m.ID, a.Name, actionPattern)
		}
		if seen[a.Name] {
			return fmt.Errorf("%w: %s: action %q declared twice", ErrInvalidManifest, m.ID, a.Name)
		}
		seen[a.Name] = true
	}
	for _, h := range m.Hooks {
		if h != HookPreExecute && h != HookPostExecute && h != HookOnError {
			return fmt.Errorf("%w: %s: unknown hook %q", ErrInvalidManifest, m.ID, h)
		}
	}
	for _, c := range m.Capabilities {
		if !knownCapabilities[c] {
			return fmt.Errorf("%w: %s: unknown capability %q", ErrInvalidManifest, m.ID, c)
		}
	}
	return nil
}

func (m Manifest) CallTimeout() time.Duration {
	if m.TimeoutMs > 0 {
		return time.Duration(m.TimeoutMs) * time.Millisecond
	}
	return 30 * time.Second
}

type Plugin interface {
	Manifest() Manifest
	Init(ctx context.Context, config map[string]any) error
	Execute(ctx context.Context, sc StepContext) (StepResult, error)
	Hook(ctx context.Context, ev HookEvent) error
	Shutdown(ctx context.Context) error
}

type Info struct {
	Manifest
	Source   string         `json:"source,omitempty"`
	LoadedAt time.Time      `json:"loadedAt"`
	Status   *RuntimeStatus `json:"status,omitempty"`
}

type statusReporter interface {
	Status() RuntimeStatus
}

type entry struct {
	plugin   Plugin
	manifest Manifest
	source   string
	loadedAt time.Time
}

type Registry struct {
	mu      sync.RWMutex
	order   []string
	entries map[string]*entry
	actions map[models.StepAction]*entry
}

func NewRegistry() *Registry {
	return &Registry{entries: map[string]*entry{}, actions: map[models.StepAction]*entry{}}
}

var defaultRegistry = NewRegistry()

func Default() *Registry { return defaultRegistry }

func (r *Registry) Register(ctx context.Context, p Plugin, source string) error {
	m := p.Manifest()
	if err := m.Validate(); err != nil {
		return err
	}
	if err := r.checkConflicts(m); err != nil {
		return err
	}
	initCtx, cancel := context.WithTimeout(ctx, m.CallTimeout())
	defer cancel()
	if err := p.Init(initCtx, m.Config); err != nil {
		_ = p.Shutdown(context.WithoutCancel(ctx))
		return fmt.Errorf("init plugin %s: %w", m.ID, err)
	}

	r.mu.Lock()
	if err := r.checkConflictsLocked(m); err != nil {
		r.mu.Unlock()
		_ = p.Shutdown(context.WithoutCancel(ctx))
		return err
	}
	e := &entry{plugin: p, manifest: m, source: source, loadedAt: time.Now()}
	r.entries[m.ID] = e
	r.order = append(r.order, m.ID)
	for _, a := range m.Actions {
		r.actions[a.Name] = e
	}
	r.mu.Unlock()
	return nil
}

func (r *Registry) checkConflicts(m Manifest) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.checkConflictsLocked(m)
}

func (r *Registry) checkConflictsLocked(m Manifest) error {
	if _, exists := r.entries[m.ID]; exists {
		return fmt.Errorf("%w: plugin %q is already loaded", ErrConflict, m.ID)
	}
	for _, a := range m.Actions {
		if models.IsKnownAction(a.Name) {
			return fmt.Errorf("%w: %s: action %q is built in", ErrConflict, m.ID, a.Name)
		}
		if owner, taken := r.actions[a.Name]; taken {
			return fmt.Errorf("%w: %s: action %q already provided by %s", ErrConflict, m.ID, a.Name, owner.manifest.ID)
		}
	}
	return nil
}

func (r *Registry) Unload(ctx context.Context, id string) error {
	r.mu.Lock()
	e, ok := r.entries[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	delete(r.entries, id)
	r.order = slices.DeleteFunc(r.order, func(s string) bool { return s == id })
	for _, a := range e.manifest.Actions {
		delete(r.actions, a.Name)
	}
	r.mu.Unlock()

	shutdownCtx, cancel := context.WithTimeout(ctx, e.manifest.CallTimeout())
	defer cancel()
	return e.plugin.Shutdown(shutdownCtx)
}

func (r *Registry) Close(ctx context.Context) error {
	r.mu.RLock()
	ids := slices.Clone(r.order)
	r.mu.RUnlock()
	var errs []error
	for i := len(ids) - 1; i >= 0; i-- {
		if err := r.Unload(ctx, ids[i]); err != nil && !errors.Is(err, ErrNotFound) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (r *Registry) List() []Info {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Info, 0, len(r.order))
	for _, id := range r.order {
		e := r.entries[id]
		info := Info{Manifest: e.manifest, Source: e.source, LoadedAt: e.loadedAt}
		if sr, ok := e.plugin.(statusReporter); ok {
			st := sr.Status()
			info.Status = &st
		}
		out = append(out, info)
	}
	return out
}

func (r *Registry) Action(action models.StepAction) (ActionSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.actions[action]
	if !ok {
		return ActionSpec{}, false
	}
	for _, a := range e.manifest.Actions {
		if a.Name == action {
			return a, true
		}
	}
	return ActionSpec{}, false
}

func (r *Registry) Execute(ctx context.Context, sc StepContext, variables map[string]string) (StepResult, Manifest, error) {
	r.mu.RLock()
	e, ok := r.actions[models.StepAction(sc.Step.Action)]
	r.mu.RUnlock()
	if !ok {
		return StepResult{}, Manifest{}, fmt.Errorf("%w: no plugin provides action %q", ErrNotFound, sc.Step.Action)
	}
	m := e.manifest
	if m.HasCapability(CapVariablesRead) {
		sc.Variables = variables
	} else {
		sc.Variables = nil
	}
	callCtx, cancel := context.WithTimeout(ctx, m.CallTimeout())
	defer cancel()
	res, err := e.plugin.Execute(callCtx, sc)
	if err != nil {
		return StepResult{}, m, fmt.Errorf("plugin %s action %s: %w", m.ID, sc.Step.Action, err)
	}
	if len(res.Variables) > 0 && !m.HasCapability(CapVariablesWrite) {
		return StepResult{}, m, fmt.Errorf("plugin %s action %s: returned variables without the %s capability", m.ID, sc.Step.Action, CapVariablesWrite)
	}
	if len(res.Extracted) > 0 && !m.HasCapability(CapExtractedWrite) {
		return StepResult{}, m, fmt.Errorf("plugin %s action %s: returned extracted data without the %s capability", m.ID, sc.Step.Action, CapExtractedWrite)
	}
	return res, m, nil
}

func (r *Registry) hookEntries(point HookPoint) []*entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*entry
	for _, id := range r.order {
		if e := r.entries[id]; e.manifest.HasHook(point) {
			out = append(out, e)
		}
	}
	return out
}

func (r *Registry) HasHooks() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, e := range r.entries {
		if len(e.manifest.Hooks) > 0 {
			return true
		}
	}
	return false
}

func (r *Registry) RunHooks(ctx context.Context, point HookPoint, sc StepContext, variables map[string]string, stepErr error) error {
	var errs []error
	for _, e := range r.hookEntries(point) {
		ev := HookEvent{Point: point, Context: sc}
		if e.manifest.HasCapability(CapVariablesRead) {
			ev.Context.Variables = variables
		} else {
			ev.Context.Variables = nil
		}
		if stepErr != nil {
			ev.Error = stepErr.Error()
		}
		callCtx, cancel := context.WithTimeout(ctx, e.manifest.CallTimeout())
		err := e.plugin.Hook(callCtx, ev)
		cancel()
		if err == nil {
			continue
		}
		if point == HookPreExecute {
			return fmt.Errorf("%w: %s: %v", ErrVetoed, e.manifest.ID, err)
		}
		errs = append(errs, fmt.Errorf("plugin %s %s hook: %w", e.manifest.ID, point, err))
	}
	return errors.Join(errs...)
}
