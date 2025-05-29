package engine

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/awantoch/beemflow/adapter"
	"github.com/awantoch/beemflow/blob"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/dsl"
	"github.com/awantoch/beemflow/event"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/registry"
	"github.com/awantoch/beemflow/storage"
	"github.com/awantoch/beemflow/utils"
	"github.com/google/uuid"
)

// Define a custom type for context keys.
type runIDKeyType struct{}

var runIDKey = runIDKeyType{}

// Type aliases for better readability and type safety
type (
	StepInputs  = map[string]any
	StepOutputs = map[string]any
	EventData   = map[string]any
	SecretsData = map[string]any
)

// Result types for better error handling and type safety
type ExecutionResult struct {
	Outputs StepOutputs
	Error   error
}

type StepResult struct {
	StepID  string
	Outputs StepOutputs
	Error   error
}

// Template data structure for type safety
type TemplateData struct {
	Event   EventData
	Vars    map[string]any
	Outputs StepOutputs
	Secrets SecretsData
}

// Engine is the core runtime for executing BeemFlow flows. It manages adapters, templating, event bus, and in-memory state.
type Engine struct {
	Adapters  *adapter.Registry
	Templater *dsl.Templater
	EventBus  event.EventBus
	BlobStore blob.BlobStore
	Storage   storage.Storage
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
	RunID   uuid.UUID
}

// NewDefaultAdapterRegistry creates and returns a default adapter registry with core and registry tools.
//
// - Loads the curated registry (repo-managed, read-only) from registry/index.json.
// - Loads the local registry (user-writable) from config (registries[].path) or .beemflow/registry.json.
// - Merges both, with local entries taking precedence over curated ones.
// - Any tool installed via the CLI is written to the local registry file.
// - This is future-proofed for remote/community registries.
func NewDefaultAdapterRegistry(ctx context.Context) *adapter.Registry {
	reg := adapter.NewRegistry()

	// Register core adapters
	reg.Register(&adapter.CoreAdapter{})
	reg.Register(adapter.NewMCPAdapter())
	reg.Register(&adapter.HTTPAdapter{AdapterID: "http"}) // Unified HTTP adapter

	// Load and merge registry tools
	loadRegistryTools(ctx, reg)

	return reg
}

// loadRegistryTools loads tools from curated and local registries
func loadRegistryTools(ctx context.Context, reg *adapter.Registry) {
	localRegistryPath := config.DefaultLocalRegistryPath

	// Load config if available to get custom registry path
	if cfg, err := config.LoadConfig(config.DefaultConfigPath); err == nil {
		for _, regCfg := range cfg.Registries {
			if regCfg.Type == "local" && regCfg.Path != "" {
				localRegistryPath = regCfg.Path
				break
			}
		}
	}

	// Load and merge tools from both registries
	toolMap := mergeRegistryTools(ctx, localRegistryPath)

	// Register HTTP adapters for each tool
	for _, entry := range toolMap {
		manifest := &registry.ToolManifest{
			Name:        entry.Name,
			Description: entry.Description,
			Kind:        entry.Kind,
			Parameters:  entry.Parameters,
			Endpoint:    entry.Endpoint,
			Headers:     entry.Headers,
		}
		reg.Register(&adapter.HTTPAdapter{AdapterID: entry.Name, ToolManifest: manifest})
	}
}

// mergeRegistryTools loads and merges tools from curated and local registries
func mergeRegistryTools(ctx context.Context, localRegistryPath string) map[string]registry.RegistryEntry {
	toolMap := make(map[string]registry.RegistryEntry)

	// Load curated registry
	if curatedTools := loadRegistryFromPath(ctx, "registry/index.json"); curatedTools != nil {
		for _, entry := range curatedTools {
			toolMap[entry.Name] = entry
		}
	}

	// Load local registry (takes precedence)
	if localTools := loadRegistryFromPath(ctx, localRegistryPath); localTools != nil {
		for _, entry := range localTools {
			toolMap[entry.Name] = entry
		}
	}

	return toolMap
}

// loadRegistryFromPath loads tools from a registry file path
func loadRegistryFromPath(ctx context.Context, path string) []registry.RegistryEntry {
	reg := registry.NewLocalRegistry(path)
	mgr := registry.NewRegistryManager(reg)
	tools, err := mgr.ListAllServers(ctx, registry.ListOptions{})
	if err != nil {
		utils.Debug("Failed to load registry from %s: %v", path, err)
		return nil
	}
	return tools
}

// NewEngineWithBlobStore creates a new Engine with a custom BlobStore.
func NewEngineWithBlobStore(ctx context.Context, blobStore blob.BlobStore) *Engine {
	return &Engine{
		Adapters:         NewDefaultAdapterRegistry(ctx),
		Templater:        dsl.NewTemplater(),
		EventBus:         event.NewInProcEventBus(),
		BlobStore:        blobStore,
		waiting:          make(map[string]*PausedRun),
		completedOutputs: make(map[string]map[string]any),
		Storage:          storage.NewMemoryStorage(),
	}
}

// NewEngine creates a new Engine with all dependencies injected.
func NewEngine(
	adapters *adapter.Registry,
	templater *dsl.Templater,
	eventBus event.EventBus,
	blobStore blob.BlobStore,
	storage storage.Storage,
) *Engine {
	return &Engine{
		Adapters:         adapters,
		Templater:        templater,
		EventBus:         eventBus,
		BlobStore:        blobStore,
		Storage:          storage,
		waiting:          make(map[string]*PausedRun),
		completedOutputs: make(map[string]map[string]any),
	}
}

// Execute now supports pausing and resuming at await_event.
func (e *Engine) Execute(ctx context.Context, flow *model.Flow, event map[string]any) (map[string]any, error) {
	if flow == nil {
		return nil, nil
	}

	// Initialize outputs and handle empty flow as no-op
	outputs := make(map[string]any)
	if len(flow.Steps) == 0 {
		return outputs, nil
	}

	// Collect env secrets and merge with event-supplied secrets
	secretsMap := e.collectSecrets(event)

	// Create step context using the new constructor
	stepCtx := NewStepContext(event, flow.Vars, secretsMap)

	// Create and persist the run
	runID := uuid.New()
	run := &model.Run{
		ID:        runID,
		FlowName:  flow.Name,
		Event:     event,
		Vars:      flow.Vars,
		Status:    model.RunRunning,
		StartedAt: time.Now(),
	}
	if err := e.Storage.SaveRun(ctx, run); err != nil {
		utils.ErrorCtx(ctx, "SaveRun failed: %v", "error", err)
	}

	outputs, err := e.executeStepsWithPersistence(ctx, flow, stepCtx, 0, runID)

	// On completion, update run status (treat pause as waiting)
	status := model.RunSucceeded
	if err != nil {
		if strings.Contains(err.Error(), "await_event pause") {
			status = model.RunWaiting
		} else {
			status = model.RunFailed
		}
	}
	run = &model.Run{
		ID:        runID,
		FlowName:  flow.Name,
		Event:     event,
		Vars:      flow.Vars,
		Status:    status,
		StartedAt: time.Now(),
		EndedAt:   ptrTime(time.Now()),
	}
	if err := e.Storage.SaveRun(ctx, run); err != nil {
		utils.ErrorCtx(ctx, "SaveRun failed: %v", "error", err)
	}

	if err != nil && len(flow.Catch) > 0 {
		// Run catch steps in defined order if error
		catchOutputs := map[string]any{}
		for _, step := range flow.Catch {
			err2 := e.executeStep(ctx, &step, stepCtx, step.ID)
			if err2 == nil {
				if output, ok := stepCtx.GetOutput(step.ID); ok {
					catchOutputs[step.ID] = output
				}
			}
		}
		return catchOutputs, err
	}
	return outputs, err
}

// collectSecrets gathers secrets from environment and event
func (e *Engine) collectSecrets(event map[string]any) SecretsData {
	secretsMap := make(SecretsData)

	// Collect env secrets
	for _, envKV := range os.Environ() {
		if eq := strings.Index(envKV, "="); eq != -1 {
			k := envKV[:eq]
			v := envKV[eq+1:]
			secretsMap[k] = v
		}
	}

	// Merge with event-supplied secrets
	if event != nil {
		if s, ok := safeMapAssert(event["secrets"]); ok {
			for k, v := range s {
				if secretVal, ok := safeStringAssert(v); ok {
					secretsMap[k] = secretVal
				}
			}
		}
	}

	return secretsMap
}

// Helper to get pointer to time.Time.
func ptrTime(t time.Time) *time.Time {
	return &t
}

// executeStepsWithPersistence executes steps, persisting each step after execution.
func (e *Engine) executeStepsWithPersistence(ctx context.Context, flow *model.Flow, stepCtx *StepContext, startIdx int, runID uuid.UUID) (map[string]any, error) {
	if runID == uuid.Nil {
		runID = runIDFromContext(ctx)
	}
	// Add a helper to build a dependency map and execution order supporting block-parallel barriers
	// This should be called at the start of executeStepsWithPersistence
	// Pseudocode:
	// 1. Build a map of stepID -> step
	// 2. For each step with ParallelSteps, for each id in ParallelSteps, add an edge from that id to this step (barrier)
	// 3. For each step, collect its dependencies (depends_on + implicit barrier edges)
	// 4. Topologically sort steps for execution order
	// 5. During execution, only run a step when all its dependencies are complete
	//
	// This will allow block-parallel barriers and keep backward compatibility for parallel: true

	for i := startIdx; i < len(flow.Steps); i++ {
		step := &flow.Steps[i]
		if step.AwaitEvent != nil {
			// Render token from match (support template)
			match := step.AwaitEvent.Match
			tokenRaw, ok := safeStringAssert(match["token"])
			if !ok || tokenRaw == "" {
				return nil, utils.Errorf("await_event step missing or invalid token in match")
			}

			// Use the simplified template data preparation
			data := e.prepareTemplateDataAsMap(stepCtx)

			utils.Debug("About to render template for step %s: data = %#v", step.ID, data)
			renderedToken, err := e.Templater.Render(tokenRaw, data)
			if err != nil {
				return nil, utils.Errorf("failed to render token template: %w", err)
			}
			token := renderedToken

			// Pause: store state and subscribe for resume
			e.mu.Lock()
			// If an existing paused run uses this token, mark it skipped and remove it
			if old, exists := e.waiting[token]; exists {
				if e.Storage != nil {
					if existingRun, err := e.Storage.GetRun(ctx, old.RunID); err == nil {
						existingRun.Status = model.RunSkipped
						existingRun.EndedAt = ptrTime(time.Now())
						if err := e.Storage.SaveRun(ctx, existingRun); err != nil {
							utils.ErrorCtx(ctx, "Failed to mark existing run as skipped: %v", "error", err)
						}
					}
					if err := e.Storage.DeletePausedRun(token); err != nil {
						utils.ErrorCtx(ctx, "Failed to delete existing paused run: %v", "error", err)
					}
				}
				delete(e.waiting, token)
			}

			// Register new paused run using snapshot
			snapshot := stepCtx.Snapshot()
			e.waiting[token] = &PausedRun{
				Flow:    flow,
				StepIdx: i,
				StepCtx: stepCtx,
				Outputs: snapshot.Outputs,
				Token:   token,
				RunID:   runID,
			}
			if e.Storage != nil {
				if err := e.Storage.SavePausedRun(token, pausedRunToMap(e.waiting[token])); err != nil {
					utils.ErrorCtx(ctx, "Failed to save paused run: %v", "error", err)
				}
			}
			e.mu.Unlock()

			e.EventBus.Subscribe(ctx, "resume:"+token, func(payload any) {
				resumeEvent, ok := payload.(map[string]any)
				if !ok {
					return
				}
				e.Resume(ctx, token, resumeEvent)
			})
			return nil, utils.Errorf("step %s is waiting for event (await_event pause)", step.ID)
		}

		err := e.executeStep(ctx, step, stepCtx, step.ID)

		// Persist the step after execution
		if e.Storage != nil {
			var stepOutputs map[string]any
			if output, ok := stepCtx.GetOutput(step.ID); ok {
				if out, ok := output.(map[string]any); ok {
					stepOutputs = out
				}
			}

			srun := &model.StepRun{
				ID:        uuid.New(),
				RunID:     runID,
				StepName:  step.ID,
				Status:    model.StepSucceeded,
				StartedAt: time.Now(),
				EndedAt:   ptrTime(time.Now()),
				Outputs:   stepOutputs,
			}
			if err != nil {
				srun.Status = model.StepFailed
				srun.Error = err.Error()
			}
			if err := e.Storage.SaveStep(ctx, srun); err != nil {
				utils.Error("SaveStep failed: %v", err)
			}
		}
		if err != nil {
			return stepCtx.Snapshot().Outputs, err
		}
	}

	return stepCtx.Snapshot().Outputs, nil
}

// Resume resumes a paused run with the given token and event.
func (e *Engine) Resume(ctx context.Context, token string, resumeEvent map[string]any) {
	utils.Debug("Resume called for token %s with event: %+v", token, resumeEvent)
	e.mu.Lock()
	paused, ok := e.waiting[token]
	if !ok {
		e.mu.Unlock()
		return
	}
	delete(e.waiting, token)
	if e.Storage != nil {
		if err := e.Storage.DeletePausedRun(token); err != nil {
			utils.ErrorCtx(ctx, "Failed to delete paused run during resume: %v", "error", err)
		}
	}
	e.mu.Unlock()

	// Update event context safely
	for k, v := range resumeEvent {
		paused.StepCtx.SetEvent(k, v)
	}

	// Log outputs map before resume (with safe access)
	utils.Debug("Outputs map before resume for token %s: %+v", token, paused.StepCtx.Snapshot().Outputs)

	// Continue execution from next step
	ctx = context.WithValue(ctx, runIDKey, paused.RunID)
	outputs, err := e.executeStepsWithPersistence(ctx, paused.Flow, paused.StepCtx, paused.StepIdx+1, paused.RunID)

	// Merge outputs from before and after resume
	snapshot := paused.StepCtx.Snapshot()
	allOutputs := make(map[string]any)
	maps.Copy(allOutputs, snapshot.Outputs)

	// Add new outputs
	if outputs != nil {
		maps.Copy(allOutputs, outputs)
	} else if len(allOutputs) == 0 {
		// If both are nil/empty, ensure we store at least an empty map
		allOutputs = map[string]any{}
	}

	utils.Debug("Outputs map after resume for token %s: %+v", token, allOutputs)

	e.mu.Lock()
	e.completedOutputs[token] = allOutputs
	e.mu.Unlock()

	// Update the run in storage after resume
	if e.Storage != nil {
		status := model.RunSucceeded
		if err != nil {
			status = model.RunFailed
		}

		run := &model.Run{
			ID:        paused.RunID,
			FlowName:  paused.Flow.Name,
			Event:     snapshot.Event,
			Vars:      snapshot.Vars,
			Status:    status,
			StartedAt: time.Now(),
			EndedAt:   ptrTime(time.Now()),
		}
		if err := e.Storage.SaveRun(ctx, run); err != nil {
			utils.ErrorCtx(ctx, "SaveRun failed: %v", "error", err)
		}
	}
}

// GetCompletedOutputs returns and clears the outputs for a completed resumed run.
func (e *Engine) GetCompletedOutputs(token string) map[string]any {
	utils.Debug("GetCompletedOutputs called for token %s", token)
	e.mu.Lock()
	defer e.mu.Unlock()
	outputs := e.completedOutputs[token]
	utils.Debug("GetCompletedOutputs for token %s returns: %+v", token, outputs)
	delete(e.completedOutputs, token)
	return outputs
}

// executeStep runs a single step (use/with) and stores output.
func (e *Engine) executeStep(ctx context.Context, step *model.Step, stepCtx *StepContext, stepID string) error {
	// Nested parallel block logic
	if step.Parallel && len(step.Steps) > 0 {
		return e.executeParallelBlock(ctx, step, stepCtx, stepID)
	}

	// Sequential block (non-parallel) for steps
	if !step.Parallel && len(step.Steps) > 0 {
		return e.executeSequentialBlock(ctx, step, stepCtx, stepID)
	}

	// Foreach logic: handle steps with Foreach and Do
	if step.Foreach != "" {
		return e.executeForeachBlock(ctx, step, stepCtx, stepID)
	}

	// Tool execution
	return e.executeToolCall(ctx, step, stepCtx, stepID)
}

// executeParallelBlock handles parallel execution of nested steps
func (e *Engine) executeParallelBlock(ctx context.Context, step *model.Step, stepCtx *StepContext, stepID string) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(step.Steps))
	outputs := make(map[string]any)
	var outputsMu sync.Mutex // Protect concurrent access to outputs map

	for i := range step.Steps {
		child := &step.Steps[i]
		wg.Add(1)
		go func(child *model.Step) {
			defer wg.Done()
			if err := e.executeStep(ctx, child, stepCtx, child.ID); err != nil {
				errChan <- err
				return
			}
			// Safely get the output using StepContext
			if childOutput, ok := stepCtx.GetOutput(child.ID); ok {
				outputsMu.Lock()
				outputs[child.ID] = childOutput
				outputsMu.Unlock()
			}
		}(child)
	}
	wg.Wait()
	close(errChan)
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	// Store the combined outputs
	stepCtx.SetOutput(stepID, outputs)
	return nil
}

// executeSequentialBlock handles sequential execution of nested steps
func (e *Engine) executeSequentialBlock(ctx context.Context, step *model.Step, stepCtx *StepContext, stepID string) error {
	outputs := make(map[string]any)
	for i := range step.Steps {
		child := &step.Steps[i]
		if err := e.executeStep(ctx, child, stepCtx, child.ID); err != nil {
			return err
		}
		childOutput, ok := stepCtx.GetOutput(child.ID)
		if ok {
			outputs[child.ID] = childOutput
		}
	}
	stepCtx.SetOutput(stepID, outputs)
	return nil
}

// executeForeachBlock handles foreach loop execution
func (e *Engine) executeForeachBlock(ctx context.Context, step *model.Step, stepCtx *StepContext, stepID string) error {
	// Prepare template data for foreach evaluation
	data := e.prepareTemplateDataAsMap(stepCtx)

	// Evaluate the foreach expression to get the actual value (not rendered as string)
	rendered, err := e.Templater.EvaluateExpression(step.Foreach, data)
	if err != nil {
		return utils.Errorf("template error in foreach expression: %w", err)
	}

	// The rendered result should be a list
	list, ok := rendered.([]any)
	if !ok {
		return utils.Errorf("foreach expression did not evaluate to a list, got: %T", rendered)
	}

	if len(list) == 0 {
		stepCtx.SetOutput(stepID, make(map[string]any))
		return nil
	}

	if step.Parallel {
		return e.executeForeachParallel(ctx, step, stepCtx, stepID, list)
	}
	return e.executeForeachSequential(ctx, step, stepCtx, stepID, list)
}

// executeForeachParallel handles parallel foreach execution
func (e *Engine) executeForeachParallel(ctx context.Context, step *model.Step, stepCtx *StepContext, stepID string, list []any) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(list))

	for _, item := range list {
		wg.Add(1)
		go func(item any) {
			defer wg.Done()

			// Create a copy of the step context for this iteration
			iterStepCtx := e.createIterationContext(stepCtx, step.As, item)

			for _, inner := range step.Do {
				// Create a copy of inner to avoid race conditions
				innerCopy := inner

				// Render the step ID as a template
				iterData := e.prepareTemplateDataAsMap(iterStepCtx)
				renderedStepID, err := e.Templater.Render(inner.ID, iterData)
				if err != nil {
					errChan <- utils.Errorf("template error in step ID %s: %w", inner.ID, err)
					return
				}

				// Use the iteration-specific context
				if err := e.executeStep(ctx, &innerCopy, iterStepCtx, renderedStepID); err != nil {
					errChan <- err
					return
				}

				// Copy outputs back to main context
				if output, ok := iterStepCtx.GetOutput(renderedStepID); ok {
					stepCtx.SetOutput(renderedStepID, output)
				}
			}
		}(item)
	}
	wg.Wait()
	close(errChan)
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	// Only set output if stepID is non-empty
	if stepID != "" {
		stepCtx.SetOutput(stepID, make(map[string]any))
	}
	return nil
}

// executeForeachSequential handles sequential foreach execution
func (e *Engine) executeForeachSequential(ctx context.Context, step *model.Step, stepCtx *StepContext, stepID string, list []any) error {
	for _, item := range list {
		// Set the loop variable (step.As) to the current item
		if step.As != "" {
			stepCtx.SetVar(step.As, item)
		}

		for _, inner := range step.Do {
			// Render the step ID as a template
			data := e.prepareTemplateDataAsMap(stepCtx)
			renderedStepID, err := e.Templater.Render(inner.ID, data)
			if err != nil {
				return utils.Errorf("template error in step ID %s: %w", inner.ID, err)
			}

			if err := e.executeStep(ctx, &inner, stepCtx, renderedStepID); err != nil {
				return err
			}
		}
	}
	// Only set output if stepID is non-empty
	if stepID != "" {
		stepCtx.SetOutput(stepID, make(map[string]any))
	}
	return nil
}

// executeToolCall handles individual tool execution
func (e *Engine) executeToolCall(ctx context.Context, step *model.Step, stepCtx *StepContext, stepID string) error {
	if step.Use == "" {
		return nil
	}

	adapterInst, ok := e.Adapters.Get(step.Use)
	if !ok {
		switch {
		case strings.HasPrefix(step.Use, "mcp://"):
			adapterInst, ok = e.Adapters.Get("mcp")
			if !ok {
				stepCtx.SetOutput(stepID, make(map[string]any))
				return utils.Errorf("MCPAdapter not registered")
			}
		case strings.HasPrefix(step.Use, "core."):
			// Handle core tools by routing to the core adapter
			adapterInst, ok = e.Adapters.Get("core")
			if !ok {
				stepCtx.SetOutput(stepID, make(map[string]any))
				return utils.Errorf("CoreAdapter not registered")
			}
		default:
			stepCtx.SetOutput(stepID, make(map[string]any))
			return utils.Errorf("adapter not found: %s", step.Use)
		}
	}

	// Prepare inputs for the tool
	inputs, err := e.prepareToolInputs(step, stepCtx, stepID)
	if err != nil {
		return err
	}

	// Auto-fill missing required parameters from manifest defaults
	e.autoFillRequiredParams(adapterInst, inputs, stepCtx)

	// Execute the tool
	payload, err := json.Marshal(inputs)
	if err != nil {
		utils.ErrorCtx(ctx, "Failed to marshal tool inputs: %v", "error", err)
		payload = []byte("{}")
	}
	utils.Debug("tool %s payload: %s", step.Use, payload)

	if strings.HasPrefix(step.Use, "mcp://") {
		inputs["__use"] = step.Use
	} else if strings.HasPrefix(step.Use, "core.") {
		inputs["__use"] = step.Use
	}

	outputs, err := adapterInst.Execute(ctx, inputs)
	if err != nil {
		stepCtx.SetOutput(stepID, outputs)
		return utils.Errorf("step %s failed: %w", stepID, err)
	}

	utils.Debug("Writing outputs for step %s: %+v", stepID, outputs)
	stepCtx.SetOutput(stepID, outputs)
	utils.Debug("Outputs map after step %s: %+v", stepID, stepCtx.Snapshot().Outputs)
	return nil
}

// prepareTemplateData creates template data from step context
func (e *Engine) prepareTemplateData(stepCtx *StepContext) TemplateData {
	snapshot := stepCtx.Snapshot()

	return TemplateData(snapshot)
}

// prepareTemplateDataAsMap creates template data as map for templating system
func (e *Engine) prepareTemplateDataAsMap(stepCtx *StepContext) map[string]any {
	templateData := e.prepareTemplateData(stepCtx)

	data := make(map[string]any)
	data["event"] = templateData.Event
	data["vars"] = templateData.Vars
	data["outputs"] = templateData.Outputs
	data["secrets"] = templateData.Secrets
	data["steps"] = templateData.Outputs // Add steps namespace for step output access

	// Flatten vars and event into context for template rendering, but be careful with outputs
	maps.Copy(data, templateData.Vars)
	maps.Copy(data, templateData.Event) // For foreach expressions like {{list}}

	// Only flatten outputs that have valid identifier names (no template syntax)
	for k, v := range templateData.Outputs {
		if isValidIdentifier(k) {
			data[k] = v
		}
	}

	return data
}

// isValidIdentifier checks if a string is a valid template identifier
// Valid identifiers contain only letters, numbers, and underscores, and don't contain template syntax
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}

	// Check for template syntax that would make this an invalid identifier
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") || strings.Contains(s, "{%") || strings.Contains(s, "%}") {
		return false
	}

	// Check that it starts with a letter or underscore
	if (s[0] < 'a' || s[0] > 'z') && (s[0] < 'A' || s[0] > 'Z') && s[0] != '_' {
		return false
	}

	// Check that all characters are valid identifier characters
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}

	return true
}

// createIterationContext creates a new context for foreach iterations
func (e *Engine) createIterationContext(stepCtx *StepContext, asVar string, item any) *StepContext {
	snapshot := stepCtx.Snapshot()
	iterStepCtx := NewStepContext(snapshot.Event, snapshot.Vars, snapshot.Secrets)

	// Copy existing outputs
	for k, v := range snapshot.Outputs {
		iterStepCtx.SetOutput(k, v)
	}

	// Set the loop variable to the current item
	if asVar != "" {
		iterStepCtx.SetVar(asVar, item)
	}

	return iterStepCtx
}

// prepareToolInputs prepares inputs for tool execution
func (e *Engine) prepareToolInputs(step *model.Step, stepCtx *StepContext, stepID string) (map[string]any, error) {
	data := e.prepareTemplateDataAsMap(stepCtx)
	inputs := make(map[string]any)

	for k, v := range step.With {
		// DEBUG: Log context keys and important values
		varsKeys := []string{}
		if vars, ok := data["vars"].(map[string]any); ok {
			for key := range vars {
				varsKeys = append(varsKeys, key)
			}
		}
		utils.Debug("Template context keys: %v, vars keys: %v, vars: %+v", keys(data), varsKeys, data["vars"])
		utils.Debug("About to render template for step %s: data = %#v", stepID, data)

		rendered, err := e.renderValue(v, data)
		if err != nil {
			return nil, utils.Errorf("template error in step %s: %w", stepID, err)
		}
		inputs[k] = rendered
	}

	return inputs, nil
}

// autoFillRequiredParams fills missing required parameters from manifest defaults
func (e *Engine) autoFillRequiredParams(adapterInst adapter.Adapter, inputs map[string]any, stepCtx *StepContext) {
	if manifest := adapterInst.Manifest(); manifest != nil {
		params, ok := safeMapAssert(manifest.Parameters["properties"])
		if !ok {
			return
		}

		required, ok := safeSliceAssert(manifest.Parameters["required"])
		if !ok {
			return
		}

		secrets := stepCtx.Snapshot().Secrets

		for _, req := range required {
			key, ok := safeStringAssert(req)
			if !ok {
				continue
			}

			if _, present := inputs[key]; !present {
				if prop, ok := safeMapAssert(params[key]); ok {
					if def, ok := safeMapAssert(prop["default"]); ok {
						if envVar, ok := safeStringAssert(def["$env"]); ok {
							if val, ok := secrets[envVar]; ok {
								inputs[key] = val
							}
						}
					}
				}
			}
		}
	}
}

// StepContext holds context for step execution (event, vars, outputs, secrets).
type StepContext struct {
	mu      sync.RWMutex
	Event   EventData
	Vars    map[string]any
	Outputs StepOutputs
	Secrets SecretsData
}

// ContextSnapshot returns immutable copies of all context data
type ContextSnapshot struct {
	Event   EventData
	Vars    map[string]any
	Outputs StepOutputs
	Secrets SecretsData
}

// NewStepContext creates a new StepContext with the provided data
func NewStepContext(event EventData, vars map[string]any, secrets SecretsData) *StepContext {
	return &StepContext{
		Event:   copyMap(event),
		Vars:    copyMap(vars),
		Outputs: make(StepOutputs),
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

// SetEvent stores a value in the Event map in a thread-safe manner.
func (sc *StepContext) SetEvent(key string, val any) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Event[key] = val
}

// SetVar stores a value in the Vars map in a thread-safe manner.
func (sc *StepContext) SetVar(key string, val any) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Vars[key] = val
}

// SetSecret stores a value in the Secrets map in a thread-safe manner.
func (sc *StepContext) SetSecret(key string, val any) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Secrets[key] = val
}

// Snapshot returns a complete snapshot of the context in a thread-safe manner
func (sc *StepContext) Snapshot() ContextSnapshot {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return ContextSnapshot{
		Event:   copyMap(sc.Event),
		Vars:    copyMap(sc.Vars),
		Outputs: copyMap(sc.Outputs),
		Secrets: copyMap(sc.Secrets),
	}
}

// CronScheduler is a stub for cron-based triggers.
type CronScheduler struct {
	// Extend this struct to support cron-based triggers (see SPEC.md for ideas).
}

func NewCronScheduler() *CronScheduler {
	return &CronScheduler{}
}

// Close cleans up all adapters and resources managed by the Engine.
func (e *Engine) Close() error {
	if e.Adapters != nil {
		return e.Adapters.CloseAll()
	}
	return nil
}

// Helper to convert PausedRun to map[string]any for storage.
func pausedRunToMap(pr *PausedRun) map[string]any {
	return map[string]any{
		"flow":     pr.Flow,
		"step_idx": pr.StepIdx,
		"step_ctx": pr.StepCtx,
		"outputs":  pr.Outputs,
		"token":    pr.Token,
		"run_id":   pr.RunID.String(),
	}
}

// Add a helper to extract runID from context (or use a global if needed).
func runIDFromContext(ctx context.Context) uuid.UUID {
	if v := ctx.Value(runIDKey); v != nil {
		if id, ok := v.(uuid.UUID); ok {
			return id
		}
	}
	return uuid.Nil
}

// ListRuns returns all runs, using storage if available, otherwise in-memory.
func (e *Engine) ListRuns(ctx context.Context) ([]*model.Run, error) {
	return e.Storage.ListRuns(ctx)
}

// GetRunByID returns a run by ID, using storage if available.
func (e *Engine) GetRunByID(ctx context.Context, id uuid.UUID) (*model.Run, error) {
	run, err := e.Storage.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	steps, err := e.Storage.GetSteps(ctx, id)
	if err == nil {
		var persisted []model.StepRun
		for _, s := range steps {
			persisted = append(persisted, *s)
		}
		run.Steps = persisted
	}
	return run, nil
}

// ListMCPServers returns all MCP servers from the registry, using the provided context.
type MCPServerWithName struct {
	Name   string
	Config *config.MCPServerConfig
}

func (e *Engine) ListMCPServers(ctx context.Context) ([]*MCPServerWithName, error) {
	localReg := registry.NewLocalRegistry("registry/index.json")
	regMgr := registry.NewRegistryManager(localReg)
	tools, err := regMgr.ListAllServers(ctx, registry.ListOptions{})
	if err != nil {
		return nil, err
	}
	var mcps []*MCPServerWithName
	for _, entry := range tools {
		if strings.HasPrefix(entry.Name, "mcp://") {
			mcps = append(mcps, &MCPServerWithName{
				Name: entry.Name,
				Config: &config.MCPServerConfig{
					Command:   entry.Command,
					Args:      entry.Args,
					Env:       entry.Env,
					Port:      entry.Port,
					Transport: entry.Transport,
					Endpoint:  entry.Endpoint,
				},
			})
		}
	}
	return mcps, nil
}

// renderValue recursively renders template strings in nested values.
func (e *Engine) renderValue(val any, data map[string]any) (any, error) {
	switch x := val.(type) {
	case string:
		return e.Templater.Render(x, data)
	case []any:
		// Create a copy to avoid race conditions
		result := make([]any, len(x))
		for i, elem := range x {
			rendered, err := e.renderValue(elem, data)
			if err != nil {
				return nil, err
			}
			result[i] = rendered
		}
		return result, nil
	case map[string]any:
		// Create a copy to avoid race conditions
		result := make(map[string]any, len(x))
		for k, elem := range x {
			rendered, err := e.renderValue(elem, data)
			if err != nil {
				return nil, err
			}
			result[k] = rendered
		}
		return result, nil
	default:
		return val, nil
	}
}

// NewDefaultEngine creates a new Engine with default dependencies (adapter registry, templater, in-process event bus, default blob store, in-memory storage).
func NewDefaultEngine(ctx context.Context) *Engine {
	// Default BlobStore
	bs, err := blob.NewDefaultBlobStore(ctx, nil)
	if err != nil {
		utils.WarnCtx(ctx, "Failed to create default blob store: %v, using nil fallback", "error", err)
		bs = nil
	}
	return NewEngine(
		NewDefaultAdapterRegistry(ctx),
		dsl.NewTemplater(),
		event.NewInProcEventBus(),
		bs,
		storage.NewMemoryStorage(),
	)
}

// Helper to get map keys for debug logging.
func keys(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// copyMap creates a shallow copy of a map[string]any.
func copyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	maps.Copy(out, in)
	return out
}

// Safe type assertion helpers to prevent panics
func safeStringAssert(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func safeMapAssert(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func safeSliceAssert(v any) ([]any, bool) {
	s, ok := v.([]any)
	return s, ok
}
