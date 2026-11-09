package plugin

import "context"

type Builtin struct {
	Spec       Manifest
	OnInit     func(ctx context.Context, config map[string]any) error
	OnExecute  func(ctx context.Context, sc StepContext) (StepResult, error)
	OnHook     func(ctx context.Context, ev HookEvent) error
	OnShutdown func(ctx context.Context) error
}

func (b *Builtin) Manifest() Manifest {
	m := b.Spec
	m.Type = TypeBuiltin
	return m
}

func (b *Builtin) Init(ctx context.Context, config map[string]any) error {
	if b.OnInit == nil {
		return nil
	}
	return b.OnInit(ctx, config)
}

func (b *Builtin) Execute(ctx context.Context, sc StepContext) (StepResult, error) {
	if b.OnExecute == nil {
		return StepResult{}, nil
	}
	return b.OnExecute(ctx, sc)
}

func (b *Builtin) Hook(ctx context.Context, ev HookEvent) error {
	if b.OnHook == nil {
		return nil
	}
	return b.OnHook(ctx, ev)
}

func (b *Builtin) Shutdown(ctx context.Context) error {
	if b.OnShutdown == nil {
		return nil
	}
	return b.OnShutdown(ctx)
}
