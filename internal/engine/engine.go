package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/awantoch/beemflow/internal/adapter"
	"github.com/awantoch/beemflow/internal/model"
	"github.com/awantoch/beemflow/internal/templater"
)

type Engine struct {
	Adapters  *adapter.Registry
	Templater *templater.Templater
	// TODO: add storage, blob, eventbus, adapter registry, etc.
}

func NewEngine() *Engine {
	reg := adapter.NewRegistry()
	reg.Register(&adapter.CoreEchoAdapter{})
	reg.Register(adapter.NewMCPAdapter())
	return &Engine{
		Adapters:  reg,
		Templater: templater.NewTemplater(),
	}
}

func (e *Engine) Execute(ctx context.Context, flow *model.Flow, event map[string]any) (map[string]any, error) {
	if flow == nil {
		return nil, nil
	}
	if flow.Steps == nil || len(flow.Steps) == 0 {
		return nil, nil
	}
	stepCtx := &StepContext{
		Event:   event,
		Vars:    flow.Vars,
		Outputs: make(map[string]any),
	}
	for label, step := range flow.Steps {
		if step.Use == "" && step.Parallel == nil && step.Foreach == "" {
			continue
		}
		// IF logic
		if step.If != "" {
			cond, err := e.Templater.Render(step.If, map[string]any{
				"event":   stepCtx.Event,
				"vars":    stepCtx.Vars,
				"outputs": stepCtx.Outputs,
			})
			if err != nil {
				return nil, fmt.Errorf("template error in step %s: %w", label, err)
			}
			if cond == "false" || cond == "0" || cond == "" {
				continue
			}
		}
		// FOREACH logic
		if step.Foreach != "" {
			arrVal, err := e.Templater.Render(step.Foreach, map[string]any{
				"event":   stepCtx.Event,
				"vars":    stepCtx.Vars,
				"outputs": stepCtx.Outputs,
			})
			if err != nil {
				return nil, fmt.Errorf("template error in foreach %s: %w", label, err)
			}
			// For now, expect arrVal to be a JSON array string
			var arr []any
			if err := json.Unmarshal([]byte(arrVal), &arr); err != nil {
				return nil, fmt.Errorf("foreach expects JSON array, got: %s", arrVal)
			}
			for _, item := range arr {
				for _, subStep := range step.Do {
					// Set loop var in context
					loopCtx := &StepContext{
						Event:   stepCtx.Event,
						Vars:    stepCtx.Vars,
						Outputs: stepCtx.Outputs,
					}
					if step.As != "" {
						loopCtx.Vars = make(map[string]any)
						for k, v := range stepCtx.Vars {
							loopCtx.Vars[k] = v
						}
						loopCtx.Vars[step.As] = item
					}
					if err := e.executeStepWithWaitAndAwait(ctx, &subStep, loopCtx, label); err != nil {
						return nil, err
					}
				}
			}
			continue
		}
		// PARALLEL logic
		if step.Parallel != nil && len(step.Parallel) > 0 {
			var wg sync.WaitGroup
			errCh := make(chan error, len(step.Parallel))
			for _, p := range step.Parallel {
				wg.Add(1)
				go func(parLabel string) {
					defer wg.Done()
					parStep, ok := flow.Steps[parLabel]
					if !ok {
						errCh <- fmt.Errorf("parallel step not found: %s", parLabel)
						return
					}
					if err := e.executeStepWithWaitAndAwait(ctx, &parStep, stepCtx, parLabel); err != nil {
						errCh <- err
					}
				}(p)
			}
			wg.Wait()
			close(errCh)
			for err := range errCh {
				if err != nil {
					return nil, err
				}
			}
			continue
		}
		// RETRY logic
		attempts := 1
		delay := 0
		if step.Retry != nil {
			if step.Retry.Attempts > 0 {
				attempts = step.Retry.Attempts
			}
			if step.Retry.DelaySec > 0 {
				delay = step.Retry.DelaySec
			}
		}
		var lastErr error
		for i := 0; i < attempts; i++ {
			lastErr = e.executeStepWithWaitAndAwait(ctx, &step, stepCtx, label)
			if lastErr == nil {
				break
			}
			if delay > 0 && i < attempts-1 {
				time.Sleep(time.Duration(delay) * time.Second)
			}
		}
		if lastErr != nil {
			// CATCH logic
			if flow.Catch != nil && len(flow.Catch) > 0 {
				catchCtx := &StepContext{
					Event:   stepCtx.Event,
					Vars:    make(map[string]any),
					Outputs: stepCtx.Outputs,
				}
				for k, v := range stepCtx.Vars {
					catchCtx.Vars[k] = v
				}
				catchCtx.Vars["error"] = lastErr.Error()
				for catchLabel, catchStep := range flow.Catch {
					if err := e.executeStepWithWaitAndAwait(ctx, &catchStep, catchCtx, catchLabel); err != nil {
						return nil, fmt.Errorf("catch step %s failed: %w", catchLabel, err)
					}
				}
				return stepCtx.Outputs, nil
			}
			return nil, lastErr
		}
	}
	return stepCtx.Outputs, nil
}

// executeStepWithWaitAndAwait handles wait and await_event before running the step
func (e *Engine) executeStepWithWaitAndAwait(ctx context.Context, step *model.Step, stepCtx *StepContext, label string) error {
	// WAIT logic
	if step.Wait != nil {
		if step.Wait.Seconds > 0 {
			time.Sleep(time.Duration(step.Wait.Seconds) * time.Second)
		}
		if step.Wait.Until != "" {
			// For now, just skip (simulate instant)
		}
	}
	// AWAIT_EVENT logic
	if step.AwaitEvent != nil {
		// For now, simulate by returning a special error
		return fmt.Errorf("step %s is waiting for event (await_event stub)", label)
	}
	return e.executeStep(ctx, step, stepCtx, label)
}

// executeStep runs a single step (use/with) and stores output
func (e *Engine) executeStep(ctx context.Context, step *model.Step, stepCtx *StepContext, label string) error {
	if step.Use == "" {
		return nil
	}
	adapterInst, ok := e.Adapters.Get(step.Use)
	if !ok {
		if strings.HasPrefix(step.Use, "mcp://") {
			adapterInst, ok = e.Adapters.Get("mcp")
			if !ok {
				return fmt.Errorf("MCPAdapter not registered")
			}
		} else {
			return fmt.Errorf("adapter not found: %s", step.Use)
		}
	}
	inputs := make(map[string]any)
	for k, v := range step.With {
		if str, ok := v.(string); ok {
			rendered, err := e.Templater.Render(str, map[string]any{
				"event":   stepCtx.Event,
				"vars":    stepCtx.Vars,
				"outputs": stepCtx.Outputs,
			})
			if err != nil {
				return fmt.Errorf("template error in step %s: %w", label, err)
			}
			inputs[k] = rendered
		} else {
			inputs[k] = v
		}
	}
	if strings.HasPrefix(step.Use, "mcp://") {
		inputs["__use"] = step.Use
	}
	outputs, err := adapterInst.Execute(ctx, inputs)
	if err != nil {
		return fmt.Errorf("step %s failed: %w", label, err)
	}
	stepCtx.Outputs[label] = outputs
	return nil
}

// StepContext holds context for step execution (event, vars, outputs, etc.)
type StepContext struct {
	Event   map[string]any
	Vars    map[string]any
	Outputs map[string]any
}

// CronScheduler is a stub for cron-based triggers.
type CronScheduler struct {
	// TODO: implement cron scheduling
}

func NewCronScheduler() *CronScheduler {
	return &CronScheduler{}
}
