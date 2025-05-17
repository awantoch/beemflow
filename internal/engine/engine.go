package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	reg.Register(&adapter.HTTPFetchAdapter{})
	reg.Register(&adapter.OpenAIChatAdapter{})

	// Auto-register all tools in tools/ directory
	toolsDir := "tools"
	entries, err := os.ReadDir(toolsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			name := entry.Name()[:len(entry.Name())-len(".json")]
			_ = reg.LoadAndRegisterTool(name, toolsDir) // ignore errors for now
		}
	}

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

	// Build step map for id lookup
	stepMap := make(map[string]*model.Step)
	for i := range flow.Steps {
		step := &flow.Steps[i]
		if step.ID == "" {
			return nil, fmt.Errorf("step at index %d missing id", i)
		}
		if _, exists := stepMap[step.ID]; exists {
			return nil, fmt.Errorf("duplicate step id: %s", step.ID)
		}
		stepMap[step.ID] = step
	}

	// Track completed steps
	completed := make(map[string]bool)
	outputs := make(map[string]any)
	secrets := map[string]string{}
	for _, env := range os.Environ() {
		if eq := strings.Index(env, "="); eq != -1 {
			k := env[:eq]
			v := env[eq+1:]
			secrets[k] = v
		}
	}
	stepCtx := &StepContext{
		Event:   event,
		Vars:    flow.Vars,
		Outputs: outputs,
		Secrets: secrets,
	}

	totalSteps := len(flow.Steps)
	for len(completed) < totalSteps {
		ready := []*model.Step{}
		for i := range flow.Steps {
			step := &flow.Steps[i]
			if completed[step.ID] {
				continue
			}
			// Check dependencies
			depsMet := true
			for _, dep := range step.DependsOn {
				if !completed[dep] {
					depsMet = false
					break
				}
			}
			if depsMet {
				ready = append(ready, step)
			}
		}
		if len(ready) == 0 {
			return nil, fmt.Errorf("circular or unsatisfiable dependencies detected")
		}
		var wg sync.WaitGroup
		errCh := make(chan error, len(ready))
		for _, step := range ready {
			if step.Parallel {
				wg.Add(1)
				go func(s *model.Step) {
					defer wg.Done()
					err := e.executeStepWithWaitAndAwait(ctx, s, stepCtx, s.ID)
					if err != nil {
						errCh <- fmt.Errorf("step %s failed: %w", s.ID, err)
						return
					}
					completed[s.ID] = true
				}(step)
			} else {
				err := e.executeStepWithWaitAndAwait(ctx, step, stepCtx, step.ID)
				if err != nil {
					return nil, fmt.Errorf("step %s failed: %w", step.ID, err)
				}
				completed[step.ID] = true
			}
		}
		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				return nil, err
			}
		}
	}
	return stepCtx.Outputs, nil
}

// executeStepWithWaitAndAwait handles wait and await_event before running the step
func (e *Engine) executeStepWithWaitAndAwait(ctx context.Context, step *model.Step, stepCtx *StepContext, stepID string) error {
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
		return fmt.Errorf("step %s is waiting for event (await_event stub)", stepID)
	}
	return e.executeStep(ctx, step, stepCtx, stepID)
}

// renderValue recursively renders template strings in nested values.
func (e *Engine) renderValue(val any, data map[string]any) (any, error) {
	switch x := val.(type) {
	case string:
		return e.Templater.Render(x, data)
	case []any:
		for i, elem := range x {
			rendered, err := e.renderValue(elem, data)
			if err != nil {
				return nil, err
			}
			x[i] = rendered
		}
		return x, nil
	case map[string]any:
		for k, elem := range x {
			rendered, err := e.renderValue(elem, data)
			if err != nil {
				return nil, err
			}
			x[k] = rendered
		}
		return x, nil
	default:
		return val, nil
	}
}

// executeStep runs a single step (use/with) and stores output
func (e *Engine) executeStep(ctx context.Context, step *model.Step, stepCtx *StepContext, stepID string) error {
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
		rendered, err := e.renderValue(v, map[string]any{
			"event":   stepCtx.Event,
			"vars":    stepCtx.Vars,
			"outputs": stepCtx.Outputs,
			"secrets": stepCtx.Secrets,
		})
		if err != nil {
			return fmt.Errorf("template error in step %s: %w", stepID, err)
		}
		inputs[k] = rendered
	}
	// Debug: log fully rendered payload for openai.chat
	if step.Use == "openai.chat" {
		payload, _ := json.Marshal(inputs)
		fmt.Printf("[beemflow] [debug] openai.chat payload: %s\n", payload)
	}
	if strings.HasPrefix(step.Use, "mcp://") {
		inputs["__use"] = step.Use
	}
	outputs, err := adapterInst.Execute(ctx, inputs)
	if err != nil {
		return fmt.Errorf("step %s failed: %w", stepID, err)
	}
	stepCtx.Outputs[stepID] = outputs
	return nil
}

// StepContext holds context for step execution (event, vars, outputs, etc.)
type StepContext struct {
	Event   map[string]any
	Vars    map[string]any
	Outputs map[string]any
	Secrets map[string]string
}

// CronScheduler is a stub for cron-based triggers.
type CronScheduler struct {
	// TODO: implement cron scheduling
}

func NewCronScheduler() *CronScheduler {
	return &CronScheduler{}
}
