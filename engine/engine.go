package engine

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/awantoch/beemflow/adapter"
	"github.com/awantoch/beemflow/blob"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/event"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/storage"
	"github.com/google/uuid"
)

// Engine coordinates the execution of BeemFlow workflows.
type Engine struct {
	// Core dependencies
	AdapterRegistry *adapter.Registry
	EventBus        event.EventBus
	BlobStore       blob.BlobStore
	Storage         storage.Storage
	Config          *config.Config

	// Runtime state
	mu sync.RWMutex
}

// StepContext manages the execution context for workflow steps.
type StepContext struct {
	Event   map[string]any `json:"event"`
	Vars    map[string]any `json:"vars"`
	Secrets map[string]any `json:"secrets"`
	Outputs map[string]any `json:"outputs"`
	mu      sync.RWMutex
}

// NewEngine creates a new Engine with the provided dependencies.
func NewEngine(
	adapterRegistry *adapter.Registry,
	eventBus event.EventBus,
	blobStore blob.BlobStore,
	storage storage.Storage,
	config *config.Config,
) *Engine {
	return &Engine{
		AdapterRegistry: adapterRegistry,
		EventBus:        eventBus,
		BlobStore:       blobStore,
		Storage:         storage,
		Config:          config,
	}
}

// NewDefaultEngine creates a new Engine with default dependencies.
func NewDefaultEngine() *Engine {
	blobStore, _ := blob.NewDefaultBlobStore(context.Background(), nil)
	
	// Create registry and register core adapters
	registry := adapter.NewRegistry()
	registry.Register(&adapter.CoreAdapter{})
	registry.Register(&adapter.HTTPAdapter{AdapterID: "http"})
	registry.Register(&adapter.OpenAIAdapter{})
	registry.Register(&adapter.AnthropicAdapter{})
	
	return &Engine{
		AdapterRegistry: registry,
		EventBus:        event.NewInProcEventBus(),
		BlobStore:       blobStore,
		Storage:         storage.NewMemoryStorage(),
		Config:          &config.Config{},
	}
}

// Execute runs a flow with the given event data and returns the outputs.
func (e *Engine) Execute(ctx context.Context, flow *model.Flow, event map[string]any) (map[string]any, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow is nil")
	}
	if event == nil {
		event = make(map[string]any)
	}

	// Create a new run record
	run := &model.Run{
		ID:        uuid.New(),
		FlowName:  flow.Name,
		Status:    model.RunRunning,
		Event:     event,
		Vars:      flow.Vars,
		StartedAt: time.Now(),
	}

	// Save the run to storage
	if err := e.Storage.SaveRun(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save run: %w", err)
	}

	// Initialize outputs
	outputs := make(map[string]any)
	if len(flow.Steps) == 0 {
		// Update run status to completed
		run.Status = model.RunSucceeded
		endedAt := time.Now()
		run.EndedAt = &endedAt
		e.Storage.SaveRun(ctx, run)
		return outputs, nil
	}

	// Setup execution context
	stepCtx := e.setupExecutionContext(ctx, flow, event)

	// Execute the flow steps with parallel support
	var execErr error
	if err := e.executeSteps(ctx, flow.Steps, stepCtx); err != nil {
		execErr = err
	}

	// Update run with final status
	if execErr != nil {
		run.Status = model.RunFailed
	} else {
		run.Status = model.RunSucceeded
	}
	
	endedAt := time.Now()
	run.EndedAt = &endedAt
	
	// Save final run state
	e.Storage.SaveRun(ctx, run)

	if execErr != nil {
		return stepCtx.Outputs, execErr
	}

	return stepCtx.Outputs, nil
}

// setupExecutionContext prepares the execution environment
func (e *Engine) setupExecutionContext(ctx context.Context, flow *model.Flow, event map[string]any) *StepContext {
	// Collect secrets from event
	secretsMap := make(map[string]any)
	if eventSecrets, ok := event["secrets"].(map[string]any); ok {
		for k, v := range eventSecrets {
			secretsMap[k] = v
		}
	}

	// Create step context
	return NewStepContext(event, flow.Vars, secretsMap)
}

// executeSteps executes a list of steps, handling parallel execution and dependencies
func (e *Engine) executeSteps(ctx context.Context, steps []model.Step, stepCtx *StepContext) error {
	for _, step := range steps {
		if err := e.executeStepWithDependencies(ctx, &step, stepCtx); err != nil {
			return err
		}
	}
	return nil
}

// executeStepWithDependencies executes a step, handling parallel execution and dependencies
func (e *Engine) executeStepWithDependencies(ctx context.Context, step *model.Step, stepCtx *StepContext) error {
	// Handle parallel steps
	if step.Parallel && len(step.Steps) > 0 {
		return e.executeParallelSteps(ctx, step.Steps, stepCtx)
	}
	
	// Handle nested steps (sequential)
	if len(step.Steps) > 0 {
		return e.executeSteps(ctx, step.Steps, stepCtx)
	}
	
	// Execute single step
	return e.executeStep(ctx, step, stepCtx)
}

// executeParallelSteps executes multiple steps in parallel
func (e *Engine) executeParallelSteps(ctx context.Context, steps []model.Step, stepCtx *StepContext) error {
	// Create a context that can be cancelled if any step fails
	parallelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	
	// Channel to collect errors
	errChan := make(chan error, len(steps))
	
	// Execute all steps in parallel
	for i := range steps {
		go func(step *model.Step) {
			if err := e.executeStep(parallelCtx, step, stepCtx); err != nil {
				errChan <- fmt.Errorf("parallel step %s failed: %w", step.ID, err)
			} else {
				errChan <- nil
			}
		}(&steps[i])
	}
	
	// Wait for all steps to complete
	for i := 0; i < len(steps); i++ {
		if err := <-errChan; err != nil {
			cancel() // Cancel remaining steps
			return err
		}
	}
	
	return nil
}

// executeStep runs a single step and stores output.
func (e *Engine) executeStep(ctx context.Context, step *model.Step, stepCtx *StepContext) error {
	if step.Use == "" {
		return nil
	}

	// Determine adapter ID and tool from step.Use
	adapterID, toolName := parseStepUse(step.Use)
	
	// Get the adapter for this tool
	adapterInst, ok := e.AdapterRegistry.Get(adapterID)
	if !ok {
		return fmt.Errorf("adapter not found: %s", step.Use)
	}

	// Prepare inputs with basic template processing
	inputs := make(map[string]any)
	for k, v := range step.With {
		inputs[k] = e.processTemplates(v, stepCtx)
	}
	
	// Add the __use field to specify the specific tool
	if toolName != "" {
		inputs["__use"] = toolName
	}

	// Execute the tool
	outputs, err := adapterInst.Execute(ctx, inputs)
	if err != nil {
		stepCtx.SetOutput(step.ID, make(map[string]any))
		return fmt.Errorf("step %s failed: %w", step.ID, err)
	}

	// Store outputs
	stepCtx.SetOutput(step.ID, outputs)
	return nil
}

// parseStepUse extracts adapter ID and tool name from a step.Use value
// Examples:
//   "core.echo" -> ("core", "core.echo")
//   "http.fetch" -> ("http", "http.fetch") 
//   "openai.chat_completion" -> ("openai", "openai.chat_completion")
//   "anthropic.chat_completion" -> ("anthropic", "anthropic.chat_completion")
//   "mcp://server/tool" -> ("mcp", "mcp://server/tool")
//   "some-adapter" -> ("some-adapter", "")
func parseStepUse(stepUse string) (adapterID, toolName string) {
	// Handle MCP protocol
	if strings.HasPrefix(stepUse, "mcp://") {
		return "mcp", stepUse
	}
	
	// Handle adapter.tool format
	if dotIndex := strings.Index(stepUse, "."); dotIndex > 0 {
		adapterID = stepUse[:dotIndex]
		return adapterID, stepUse
	}
	
	// Default: assume the whole string is the adapter ID
	return stepUse, ""
}

// NewStepContext creates a new StepContext with the provided data
func NewStepContext(event map[string]any, vars map[string]any, secrets map[string]any) *StepContext {
	return &StepContext{
		Event:   copyMap(event),
		Vars:    copyMap(vars),
		Outputs: make(map[string]any),
		Secrets: copyMap(secrets),
	}
}

// GetOutput retrieves a stored step output in a thread-safe manner.
func (sc *StepContext) GetOutput(key string) (any, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	val, ok := sc.Outputs[key]
	return val, ok
}

// SetOutput stores a step output in a thread-safe manner.
func (sc *StepContext) SetOutput(key string, val any) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Outputs[key] = val
}

// Close cleans up all adapters and resources managed by the Engine.
func (e *Engine) Close() error {
	if e.AdapterRegistry != nil {
		return e.AdapterRegistry.CloseAll()
	}
	return nil
}

// copyMap creates a shallow copy of a map[string]any.
func copyMap(in map[string]any) map[string]any {
	if in == nil {
		return make(map[string]any)
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// ListRuns returns all runs from storage
func (e *Engine) ListRuns(ctx context.Context) ([]*model.Run, error) {
	return e.Storage.ListRuns(ctx)
}

// GetRunByID returns a run by ID from storage
func (e *Engine) GetRunByID(ctx context.Context, id uuid.UUID) (*model.Run, error) {
	return e.Storage.GetRun(ctx, id)
}

// processTemplates applies basic template processing to a value
func (e *Engine) processTemplates(value any, stepCtx *StepContext) any {
	switch v := value.(type) {
	case string:
		return e.processStringTemplate(v, stepCtx)
	case map[string]any:
		return e.processMapTemplate(v, stepCtx)
	case []any:
		return e.processSliceTemplate(v, stepCtx)
	default:
		return value
	}
}

// processStringTemplate applies basic template processing to a string
func (e *Engine) processStringTemplate(value string, stepCtx *StepContext) string {
	re := regexp.MustCompile(`\{\{\s*([^}]+)\s*\}\}`)
	return re.ReplaceAllStringFunc(value, func(match string) string {
		expr := strings.TrimSpace(re.FindStringSubmatch(match)[1])
		resolved := e.resolveExpression(expr, stepCtx)
		return fmt.Sprintf("%v", resolved)
	})
}

// processMapTemplate applies basic template processing to a map[string]any
func (e *Engine) processMapTemplate(value map[string]any, stepCtx *StepContext) map[string]any {
	out := make(map[string]any)
	for k, v := range value {
		out[k] = e.processTemplates(v, stepCtx)
	}
	return out
}

// processSliceTemplate applies basic template processing to a slice
func (e *Engine) processSliceTemplate(value []any, stepCtx *StepContext) []any {
	out := make([]any, len(value))
	for i, v := range value {
		out[i] = e.processTemplates(v, stepCtx)
	}
	return out
}

// resolveExpression resolves template expressions like vars.URL or step.field
func (e *Engine) resolveExpression(expr string, stepCtx *StepContext) any {
	parts := strings.Split(expr, ".")

	switch parts[0] {
	case "vars":
		// Explicit vars.field reference
		if len(parts) < 2 {
			return expr
		}
		return e.lookupNested(stepCtx.Vars, parts[1:])
	case "event":
		// Explicit event.field reference
		if len(parts) < 2 {
			return expr
		}
		return e.lookupNested(stepCtx.Event, parts[1:])
	default:
		// Could be a step output reference or a vars field
		if len(parts) == 1 {
			// Single field - check vars first, then step outputs
			if stepCtx.Vars != nil {
				if value, exists := stepCtx.Vars[parts[0]]; exists {
					return value
				}
			}
			// Check step outputs
			if stepOutput, ok := stepCtx.GetOutput(parts[0]); ok {
				return stepOutput
			}
			return expr // Return as-is if not found
		} else {
			// Multi-part - assume it's a step output reference
			stepOutput, ok := stepCtx.GetOutput(parts[0])
			if !ok {
				return expr // Return as-is if step not found
			}
			return e.lookupNested(stepOutput, parts[1:])
		}
	}
}

// lookupNested performs nested field lookup in a map or value
func (e *Engine) lookupNested(value any, path []string) any {
	current := value
	for _, key := range path {
		switch v := current.(type) {
		case map[string]any:
			current = v[key]
		default:
			return fmt.Sprintf("{{%s}}", strings.Join(path, ".")) // Return template if can't resolve
		}
	}
	return current
}