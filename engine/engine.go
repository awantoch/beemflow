package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/awantoch/beemflow/adapter"
	"github.com/awantoch/beemflow/event"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/templater"
)

// Engine is the core runtime for executing BeemFlow flows. It manages adapters, templating, event bus, and in-memory state.
type Engine struct {
	Adapters  *adapter.Registry
	Templater *templater.Templater
	EventBus  event.EventBus
	// In-memory state for waiting runs: token -> *PausedRun
	waiting map[string]*PausedRun
	mu      sync.Mutex
	// Store completed outputs for resumed runs (token -> outputs)
	completedOutputs map[string]map[string]any
	// NOTE: Storage, blob, eventbus, and cron are pluggable; in-memory is the default for now.
	// Call Close() to clean up resources (e.g., MCPAdapter subprocesses) when done.
}

type PausedRun struct {
	Flow    *model.Flow
	StepIdx int
	StepCtx *StepContext
	Outputs map[string]any
	Token   string
}

// NewEngine creates a new Engine with built-in adapters and default in-memory state.
func NewEngine() *Engine {
	reg := adapter.NewRegistry()
	reg.Register(&adapter.CoreAdapter{})
	reg.Register(adapter.NewMCPAdapter())
	reg.Register(&adapter.HTTPFetchAdapter{})

	// Load openai.chat manifest
	var openaiManifest *adapter.ToolManifest
	manifestPath := filepath.Join("tools", "openai.json")
	if f, err := os.ReadFile(manifestPath); err == nil {
		var m adapter.ToolManifest
		if err := json.Unmarshal(f, &m); err == nil {
			openaiManifest = &m
		}
	}
	reg.Register(&adapter.OpenAIAdapter{ManifestField: openaiManifest})

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
		Adapters:         reg,
		Templater:        templater.NewTemplater(),
		EventBus:         event.NewInProcEventBus(),
		waiting:          make(map[string]*PausedRun),
		completedOutputs: make(map[string]map[string]any),
	}
}

// Execute now supports pausing and resuming at await_event.
func (e *Engine) Execute(ctx context.Context, flow *model.Flow, event map[string]any) (map[string]any, error) {
	if flow == nil {
		return nil, nil
	}
	if flow.Steps == nil || len(flow.Steps) == 0 {
		return nil, nil
	}
	outputs := make(map[string]any)
	secrets := map[string]string{}
	for _, env := range os.Environ() {
		if eq := strings.Index(env, "="); eq != -1 {
			k := env[:eq]
			v := env[eq+1:]
			secrets[k] = v
		}
	}
	// Register a 'secrets' helper for this execution
	secretsCopy := secrets
	if event != nil {
		if s, ok := event["secrets"].(map[string]any); ok {
			for k, v := range s {
				if str, ok := v.(string); ok {
					secretsCopy[k] = str
				}
			}
		}
	}
	e.Templater.RegisterHelpers(map[string]any{
		"secrets": func(key string) string {
			return secretsCopy[key]
		},
	})
	stepCtx := &StepContext{
		Event:   event,
		Vars:    flow.Vars,
		Outputs: outputs,
		Secrets: secretsCopy,
	}
	outputs, err := e.executeStepsWithPause(ctx, flow, stepCtx, 0)
	if err != nil && flow.Catch != nil && len(flow.Catch) > 0 {
		// Run catch steps if error and catch block exists
		catchOutputs := map[string]any{}
		for id, step := range flow.Catch {
			err2 := e.executeStep(ctx, &step, stepCtx, id)
			if err2 == nil {
				catchOutputs[id] = stepCtx.Outputs[id]
			}
		}
		return catchOutputs, err
	}
	return outputs, err
}

// executeStepsWithPause executes steps, pausing at await_event and resuming as needed.
func (e *Engine) executeStepsWithPause(ctx context.Context, flow *model.Flow, stepCtx *StepContext, startIdx int) (map[string]any, error) {
	for i := startIdx; i < len(flow.Steps); i++ {
		step := &flow.Steps[i]
		if step.AwaitEvent != nil {
			// Render token from match (support template)
			match := step.AwaitEvent.Match
			tokenRaw, _ := match["token"].(string)
			if tokenRaw == "" {
				return nil, fmt.Errorf("await_event step missing token in match")
			}
			// Render the token template
			data := map[string]any{
				"event":   stepCtx.Event,
				"vars":    stepCtx.Vars,
				"outputs": stepCtx.Outputs,
				"secrets": stepCtx.Secrets,
			}
			renderedToken, err := e.Templater.Render(tokenRaw, data)
			if err != nil {
				return nil, fmt.Errorf("failed to render token template: %w", err)
			}
			token := renderedToken
			// Pause: store state and subscribe for resume
			e.mu.Lock()
			e.waiting[token] = &PausedRun{
				Flow:    flow,
				StepIdx: i,
				StepCtx: stepCtx,
				Outputs: stepCtx.Outputs,
				Token:   token,
			}
			e.mu.Unlock()
			e.EventBus.Subscribe("resume:"+token, func(payload any) {
				resumeEvent, ok := payload.(map[string]any)
				if !ok {
					return
				}
				e.Resume(token, resumeEvent)
			})
			return nil, fmt.Errorf("step %s is waiting for event (await_event pause)", step.ID)
		}
		err := e.executeStep(ctx, step, stepCtx, step.ID)
		if err != nil {
			return stepCtx.Outputs, err
		}
	}
	return stepCtx.Outputs, nil
}

// Resume resumes a paused run with the given token and event.
func (e *Engine) Resume(token string, resumeEvent map[string]any) {
	debugLog("[DEBUG] Resume called for token %s with event: %+v", token, resumeEvent)
	e.mu.Lock()
	paused, ok := e.waiting[token]
	if !ok {
		e.mu.Unlock()
		return
	}
	delete(e.waiting, token)
	e.mu.Unlock()
	// Update event context
	for k, v := range resumeEvent {
		paused.StepCtx.Event[k] = v
	}
	debugLog("[DEBUG] Outputs map before resume for token %s: %+v", token, paused.StepCtx.Outputs)
	// Continue execution from next step
	outputs, _ := e.executeStepsWithPause(context.Background(), paused.Flow, paused.StepCtx, paused.StepIdx+1)
	// Merge outputs from before and after resume
	allOutputs := make(map[string]any)
	for k, v := range paused.StepCtx.Outputs {
		allOutputs[k] = v
	}
	for k, v := range outputs {
		allOutputs[k] = v
	}
	debugLog("[DEBUG] Outputs map after resume for token %s: %+v", token, allOutputs)
	e.mu.Lock()
	e.completedOutputs[token] = allOutputs
	e.mu.Unlock()
}

// GetCompletedOutputs returns and clears the outputs for a completed resumed run.
func (e *Engine) GetCompletedOutputs(token string) map[string]any {
	debugLog("[DEBUG] GetCompletedOutputs called for token %s", token)
	e.mu.Lock()
	defer e.mu.Unlock()
	outputs := e.completedOutputs[token]
	debugLog("[DEBUG] GetCompletedOutputs for token %s returns: %+v", token, outputs)
	delete(e.completedOutputs, token)
	return outputs
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
	// Foreach logic: handle steps with Foreach and Do
	if step.Foreach != "" {
		s := strings.TrimSpace(step.Foreach)
		if strings.HasPrefix(s, "{{") && strings.HasSuffix(s, "}}") {
			key := strings.TrimSpace(s[2 : len(s)-2])
			val, ok := stepCtx.Event[key]
			if !ok {
				return fmt.Errorf("foreach variable not found: %s", key)
			}
			list, ok := val.([]any)
			if !ok {
				return fmt.Errorf("foreach variable %s is not a list", key)
			}
			if len(list) == 0 {
				stepCtx.Outputs[stepID] = make(map[string]any)
				return nil
			}
			if step.Parallel {
				var wg sync.WaitGroup
				errChan := make(chan error, len(list))
				for range list {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for _, inner := range step.Do {
							if err := e.executeStep(ctx, &inner, stepCtx, inner.ID); err != nil {
								errChan <- err
								return
							}
						}
					}()
				}
				wg.Wait()
				close(errChan)
				for err := range errChan {
					if err != nil {
						return err
					}
				}
			} else {
				for range list {
					for _, inner := range step.Do {
						if err := e.executeStep(ctx, &inner, stepCtx, inner.ID); err != nil {
							return err
						}
					}
				}
			}
			stepCtx.Outputs[stepID] = make(map[string]any)
			return nil
		}
		return fmt.Errorf("unsupported foreach expression: %s", step.Foreach)
	}
	if step.Use == "" {
		return nil
	}
	adapterInst, ok := e.Adapters.Get(step.Use)
	if !ok {
		if strings.HasPrefix(step.Use, "mcp://") {
			adapterInst, ok = e.Adapters.Get("mcp")
			if !ok {
				stepCtx.Outputs[stepID] = make(map[string]any)
				return fmt.Errorf("MCPAdapter not registered")
			}
		} else {
			stepCtx.Outputs[stepID] = make(map[string]any)
			return fmt.Errorf("adapter not found: %s", step.Use)
		}
	}
	inputs := make(map[string]any)
	for k, v := range step.With {
		// Prepare template data, flattening previous step outputs for direct access
		data := make(map[string]any)
		data["event"] = stepCtx.Event
		data["vars"] = stepCtx.Vars
		data["outputs"] = stepCtx.Outputs
		data["secrets"] = stepCtx.Secrets
		for id, out := range stepCtx.Outputs {
			data[id] = out
		}
		rendered, err := e.renderValue(v, data)
		if err != nil {
			return fmt.Errorf("template error in step %s: %w", stepID, err)
		}
		inputs[k] = rendered
	}
	// Auto-fill missing required parameters from manifest defaults (including $env)
	if manifest := adapterInst.Manifest(); manifest != nil {
		params, _ := manifest.Parameters["properties"].(map[string]any)
		required, _ := manifest.Parameters["required"].([]any)
		for _, req := range required {
			key, _ := req.(string)
			if _, present := inputs[key]; !present {
				if prop, ok := params[key].(map[string]any); ok {
					if def, ok := prop["default"].(map[string]any); ok {
						if envVar, ok := def["$env"].(string); ok {
							if val, ok := stepCtx.Secrets[envVar]; ok {
								inputs[key] = val
							}
						}
					}
				}
			}
		}
	}
	// Debug: log fully rendered payload for openai.chat
	if step.Use == "openai" {
		payload, _ := json.Marshal(inputs)
		debugLog("[debug] openai.chat payload: %s", payload)
	}
	if strings.HasPrefix(step.Use, "mcp://") {
		inputs["__use"] = step.Use
	}
	outputs, err := adapterInst.Execute(ctx, inputs)
	if err != nil {
		stepCtx.Outputs[stepID] = outputs
		return fmt.Errorf("step %s failed: %w", stepID, err)
	}
	debugLog("[DEBUG] Writing outputs for step %s: %+v", stepID, outputs)
	stepCtx.Outputs[stepID] = outputs
	debugLog("[DEBUG] Outputs map after step %s: %+v", stepID, stepCtx.Outputs)
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
	// Extend this struct to support cron-based triggers (see beemflow_spec.md for ideas).
}

func NewCronScheduler() *CronScheduler {
	return &CronScheduler{}
}

// debugLog prints debug logs only if BEEMFLOW_DEBUG is set.
func debugLog(format string, v ...any) {
	if os.Getenv("BEEMFLOW_DEBUG") != "" {
		log.Printf(format, v...)
	}
}

// Close cleans up all adapters and resources managed by the Engine.
func (e *Engine) Close() error {
	if e.Adapters != nil {
		return e.Adapters.CloseAll()
	}
	return nil
}
