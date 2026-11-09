package browser

import (
	"context"
	"fmt"
	"strings"
	"time"

	"flowpilot/internal/models"
	"flowpilot/internal/plugin"
	sdk "flowpilot/sdk/go/plugin"
)

const (
	variablePrefix   = "var_"
	defaultHookGrace = 5 * time.Second
)

func (r *Runner) SetPluginRegistry(reg *plugin.Registry) {
	r.plugins.Store(reg)
}

func (r *Runner) pluginRegistry() *plugin.Registry {
	if reg := r.plugins.Load(); reg != nil {
		return reg
	}
	return plugin.Default()
}

func pluginStepContext(taskID string, stepIndex int, step models.TaskStep) plugin.StepContext {
	return plugin.StepContext{
		TaskID:    taskID,
		StepIndex: stepIndex,
		Step: sdk.Step{
			Action:    string(step.Action),
			Selector:  step.Selector,
			Value:     step.Value,
			Timeout:   step.Timeout,
			Condition: step.Condition,
			Label:     step.Label,
			JumpTo:    step.JumpTo,
			VarName:   step.VarName,
			Operator:  step.Operator,
			MaxLoops:  step.MaxLoops,
			Target:    step.Target,
			Source:    step.Source,
			Keys:      step.Keys,
			Duration:  step.Duration,
		},
	}
}

func taskVariables(result *models.TaskResult) map[string]string {
	vars := make(map[string]string)
	for k, v := range result.ExtractedData {
		if name, ok := strings.CutPrefix(k, variablePrefix); ok {
			vars[name] = v
		}
	}
	return vars
}

func (r *Runner) executePluginStep(ctx context.Context, stepIndex int, step models.TaskStep, result *models.TaskResult) error {
	reg := r.pluginRegistry()
	if _, ok := reg.Action(step.Action); !ok {
		return fmt.Errorf("unknown action: %s", step.Action)
	}
	res, m, err := reg.Execute(ctx, pluginStepContext(result.TaskID, stepIndex, step), taskVariables(result))
	if err != nil {
		return err
	}
	for k := range res.Extracted {
		if strings.HasPrefix(k, variablePrefix) {
			return fmt.Errorf("plugin %s: extracted key %q uses the reserved %q prefix", m.ID, k, variablePrefix)
		}
	}
	if result.ExtractedData == nil {
		result.ExtractedData = make(map[string]string)
	}
	for k, v := range res.Extracted {
		result.ExtractedData[k] = v
	}
	for k, v := range res.Variables {
		result.ExtractedData[variablePrefix+k] = v
	}
	for _, line := range res.Logs {
		r.addLog(result, "info", fmt.Sprintf("[plugin %s] %s", m.ID, line))
	}
	return nil
}

func (r *Runner) runPluginStep(ctx context.Context, stepIndex int, step models.TaskStep, result *models.TaskResult) error {
	reg := r.pluginRegistry()
	if !reg.HasHooks() {
		return r.executeStepAt(ctx, stepIndex, step, result)
	}
	sc := pluginStepContext(result.TaskID, stepIndex, step)
	if err := reg.RunHooks(ctx, plugin.HookPreExecute, sc, taskVariables(result), nil); err != nil {
		return err
	}
	stepErr := r.executeStepAt(ctx, stepIndex, step, result)
	point := plugin.HookPostExecute
	if stepErr != nil {
		point = plugin.HookOnError
	}
	hookCtx := ctx
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		hookCtx, cancel = context.WithTimeout(context.WithoutCancel(ctx), defaultHookGrace)
		defer cancel()
	}
	if hookErr := reg.RunHooks(hookCtx, point, sc, taskVariables(result), stepErr); hookErr != nil {
		r.addLog(result, "warn", hookErr.Error())
	}
	return stepErr
}

func (r *Runner) executeStepAt(ctx context.Context, stepIndex int, step models.TaskStep, result *models.TaskResult) error {
	if !models.IsKnownAction(step.Action) {
		return r.executePluginStep(ctx, stepIndex, step, result)
	}
	return r.executeStep(ctx, step, result)
}
