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
	"github.com/awantoch/beemflow/constants"
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
	if curatedTools := loadRegistryFromPath(ctx, constants.RegistryIndexFile); curatedTools != nil {
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

	// Setup execution context
	stepCtx, runID := e.setupExecutionContext(ctx, flow, event)

	// Execute the flow steps
	outputs, err := e.executeStepsWithPersistence(ctx, flow, stepCtx, 0, runID)

	// Handle completion and error cases
	return e.finalizeExecution(ctx, flow, event, outputs, err, runID)
}

// setupExecutionContext prepares the execution environment
func (e *Engine) setupExecutionContext(ctx context.Context, flow *model.Flow, event map[string]any) (*StepContext, uuid.UUID) {
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

	return stepCtx, runID
}

// finalizeExecution handles completion, error cases, and catch blocks
func (e *Engine) finalizeExecution(ctx context.Context, flow *model.Flow, event map[string]any, outputs map[string]any, err error, runID uuid.UUID) (map[string]any, error) {
	// Determine final status
	status := model.RunSucceeded
	if err != nil {
		if strings.Contains(err.Error(), constants.ErrAwaitEventPause) {
			status = model.RunWaiting
		} else {
			status = model.RunFailed
		}
	}

	// Update final run status
	run := &model.Run{
		ID:        runID,
		FlowName:  flow.Name,
		Event:     event,
		Vars:      flow.Vars,
		Status:    status,
		StartedAt: time.Now(),
		EndedAt:   ptrTime(time.Now()),
	}
	if saveErr := e.Storage.SaveRun(ctx, run); saveErr != nil {
		utils.ErrorCtx(ctx, constants.ErrSaveRunFailed, "error", saveErr)
	}

	// Handle catch blocks if there was an error
	if err != nil && len(flow.Catch) > 0 {
		return e.executeCatchBlocks(ctx, flow, event, err)
	}

	return outputs, err
}

// executeCatchBlocks runs catch steps when an error occurs
func (e *Engine) executeCatchBlocks(ctx context.Context, flow *model.Flow, event map[string]any, originalErr error) (map[string]any, error) {
	// Recreate step context for catch blocks
	secretsMap := e.collectSecrets(event)
	stepCtx := NewStepContext(event, flow.Vars, secretsMap)

	// Run catch steps in defined order
	catchOutputs := map[string]any{}
	for _, step := range flow.Catch {
		if execErr := e.executeStep(ctx, &step, stepCtx, step.ID); execErr == nil {
			if output, ok := stepCtx.GetOutput(step.ID); ok {
				catchOutputs[step.ID] = output
			}
		}
	}
	return catchOutputs, originalErr
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

	// Execute steps sequentially from startIdx
	// Note: Future enhancement could add dependency mapping for more sophisticated
	// parallel execution patterns while maintaining backward compatibility
	for i := startIdx; i < len(flow.Steps); i++ {
		step := &flow.Steps[i]

		// Handle await_event steps
		if step.AwaitEvent != nil {
			return e.handleAwaitEventStep(ctx, step, flow, stepCtx, i, runID)
		}

		// Execute regular step
		err := e.executeStep(ctx, step, stepCtx, step.ID)

		// Persist the step after execution
		if persistErr := e.persistStepResult(ctx, step, stepCtx, err, runID); persistErr != nil {
			utils.Error(constants.ErrFailedToPersistStep, persistErr)
		}

		if err != nil {
			return stepCtx.Snapshot().Outputs, err
		}
	}

	return stepCtx.Snapshot().Outputs, nil
}

// handleAwaitEventStep processes await_event steps and sets up pause/resume logic
func (e *Engine) handleAwaitEventStep(ctx context.Context, step *model.Step, flow *model.Flow, stepCtx *StepContext, stepIdx int, runID uuid.UUID) (map[string]any, error) {
	// Render token from match (support template)
	match := step.AwaitEvent.Match
	tokenRaw, ok := safeStringAssert(match[constants.MatchKeyToken])
	if !ok || tokenRaw == "" {
		return nil, utils.Errorf(constants.ErrAwaitEventMissingToken)
	}

	// Prepare template data and render token
	data := e.prepareTemplateDataAsMap(stepCtx)
	utils.Debug("About to render template for step %s: data = %#v", step.ID, data)

	renderedToken, err := e.Templater.Render(tokenRaw, data)
	if err != nil {
		return nil, utils.Errorf(constants.ErrFailedToRenderToken, err)
	}
	token := renderedToken

	// Handle existing paused run with same token
	e.handleExistingPausedRun(ctx, token)

	// Register new paused run
	e.registerPausedRun(ctx, token, flow, stepCtx, stepIdx, runID)

	// Subscribe to resume events
	e.EventBus.Subscribe(ctx, constants.EventTopicResumePrefix+token, func(payload any) {
		resumeEvent, ok := payload.(map[string]any)
		if !ok {
			return
		}
		e.Resume(ctx, token, resumeEvent)
	})

	return nil, utils.Errorf(constants.ErrStepWaitingForEvent, step.ID)
}

// handleExistingPausedRun manages cleanup of existing paused runs with the same token
func (e *Engine) handleExistingPausedRun(ctx context.Context, token string) {
	e.mu.Lock()
	defer e.mu.Unlock()

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
				utils.ErrorCtx(ctx, constants.ErrFailedToDeletePausedRun, "error", err)
			}
		}
		delete(e.waiting, token)
	}
}

// registerPausedRun stores a new paused run for later resumption
func (e *Engine) registerPausedRun(ctx context.Context, token string, flow *model.Flow, stepCtx *StepContext, stepIdx int, runID uuid.UUID) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Register new paused run using snapshot
	snapshot := stepCtx.Snapshot()
	e.waiting[token] = &PausedRun{
		Flow:    flow,
		StepIdx: stepIdx,
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
}

// persistStepResult saves step execution results to storage
func (e *Engine) persistStepResult(ctx context.Context, step *model.Step, stepCtx *StepContext, execErr error, runID uuid.UUID) error {
	if e.Storage == nil {
		return nil
	}

	var stepOutputs map[string]any
	if output, ok := stepCtx.GetOutput(step.ID); ok {
		if out, ok := output.(map[string]any); ok {
			stepOutputs = out
		}
	}

	status := model.StepSucceeded
	var errorMsg string
	if execErr != nil {
		status = model.StepFailed
		errorMsg = execErr.Error()
	}

	srun := &model.StepRun{
		ID:        uuid.New(),
		RunID:     runID,
		StepName:  step.ID,
		Status:    status,
		StartedAt: time.Now(),
		EndedAt:   ptrTime(time.Now()),
		Outputs:   stepOutputs,
		Error:     errorMsg,
	}

	return e.Storage.SaveStep(ctx, srun)
}

// Resume resumes a paused run with the given token and event.
func (e *Engine) Resume(ctx context.Context, token string, resumeEvent map[string]any) {
	utils.Debug("Resume called for token %s with event: %+v", token, resumeEvent)

	// Retrieve and remove paused run
	paused := e.retrieveAndRemovePausedRun(ctx, token)
	if paused == nil {
		return
	}

	// Prepare context for resumption
	e.prepareResumeContext(paused, resumeEvent)

	// Continue execution and handle results
	e.continueExecutionAndStoreResults(ctx, token, paused)
}

// retrieveAndRemovePausedRun safely gets and removes a paused run
func (e *Engine) retrieveAndRemovePausedRun(ctx context.Context, token string) *PausedRun {
	e.mu.Lock()
	defer e.mu.Unlock()

	paused, ok := e.waiting[token]
	if !ok {
		return nil
	}

	delete(e.waiting, token)
	if e.Storage != nil {
		if err := e.Storage.DeletePausedRun(token); err != nil {
			utils.ErrorCtx(ctx, constants.ErrFailedToDeletePausedRun, "error", err)
		}
	}

	return paused
}

// prepareResumeContext updates the step context with resume event data
func (e *Engine) prepareResumeContext(paused *PausedRun, resumeEvent map[string]any) {
	// Update event context safely
	for k, v := range resumeEvent {
		paused.StepCtx.SetEvent(k, v)
	}

	// Log outputs map before resume (with safe access)
	utils.Debug("Outputs map before resume for token %s: %+v", paused.Token, paused.StepCtx.Snapshot().Outputs)
}

// continueExecutionAndStoreResults handles execution continuation and result storage
func (e *Engine) continueExecutionAndStoreResults(ctx context.Context, token string, paused *PausedRun) {
	// Continue execution from next step
	ctx = context.WithValue(ctx, runIDKey, paused.RunID)
	outputs, err := e.executeStepsWithPersistence(ctx, paused.Flow, paused.StepCtx, paused.StepIdx+1, paused.RunID)

	// Merge and store results
	allOutputs := e.mergeResumeOutputs(paused, outputs)
	e.storeCompletedOutputs(token, allOutputs)

	// Update storage with final run status
	e.updateRunStatusAfterResume(ctx, paused, err)
}

// mergeResumeOutputs combines outputs from before and after resume
func (e *Engine) mergeResumeOutputs(paused *PausedRun, newOutputs map[string]any) map[string]any {
	snapshot := paused.StepCtx.Snapshot()
	allOutputs := make(map[string]any)
	maps.Copy(allOutputs, snapshot.Outputs)

	// Add new outputs
	if newOutputs != nil {
		maps.Copy(allOutputs, newOutputs)
	} else if len(allOutputs) == 0 {
		// If both are nil/empty, ensure we store at least an empty map
		allOutputs = map[string]any{}
	}

	utils.Debug("Outputs map after resume for token %s: %+v", paused.Token, allOutputs)
	return allOutputs
}

// storeCompletedOutputs safely stores the completed outputs for retrieval
func (e *Engine) storeCompletedOutputs(token string, allOutputs map[string]any) {
	e.mu.Lock()
	e.completedOutputs[token] = allOutputs
	e.mu.Unlock()
}

// updateRunStatusAfterResume updates the run status in storage after resumption
func (e *Engine) updateRunStatusAfterResume(ctx context.Context, paused *PausedRun, err error) {
	if e.Storage == nil {
		return
	}

	status := model.RunSucceeded
	if err != nil {
		status = model.RunFailed
	}

	snapshot := paused.StepCtx.Snapshot()
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

	// Process each item in parallel
	for _, item := range list {
		wg.Add(1)
		go e.processParallelForeachItem(ctx, step, stepCtx, item, &wg, errChan)
	}

	// Wait for all goroutines and collect errors
	return e.collectParallelErrors(&wg, errChan, stepCtx, stepID)
}

// processParallelForeachItem processes a single item in a parallel foreach loop
func (e *Engine) processParallelForeachItem(ctx context.Context, step *model.Step, stepCtx *StepContext, item any, wg *sync.WaitGroup, errChan chan<- error) {
	defer wg.Done()

	// Create iteration context for this item
	iterStepCtx := e.createIterationContext(stepCtx, step.As, item)

	// Execute all steps for this iteration
	if err := e.executeIterationSteps(ctx, step.Do, iterStepCtx, stepCtx); err != nil {
		errChan <- err
	}
}

// executeIterationSteps executes all steps for a single foreach iteration
func (e *Engine) executeIterationSteps(ctx context.Context, steps []model.Step, iterStepCtx, mainStepCtx *StepContext) error {
	for _, inner := range steps {
		// Create a copy to avoid race conditions
		innerCopy := inner

		// Render the step ID as a template
		renderedStepID, err := e.renderStepID(inner.ID, iterStepCtx)
		if err != nil {
			return err
		}

		// Execute the step with iteration context
		if err := e.executeStep(ctx, &innerCopy, iterStepCtx, renderedStepID); err != nil {
			return err
		}

		// Copy outputs back to main context
		e.copyIterationOutput(iterStepCtx, mainStepCtx, renderedStepID)
	}
	return nil
}

// renderStepID renders a step ID as a template
func (e *Engine) renderStepID(stepID string, stepCtx *StepContext) (string, error) {
	iterData := e.prepareTemplateDataAsMap(stepCtx)
	renderedStepID, err := e.Templater.Render(stepID, iterData)
	if err != nil {
		return "", utils.Errorf("template error in step ID %s: %w", stepID, err)
	}
	return renderedStepID, nil
}

// copyIterationOutput safely copies output from iteration context to main context
func (e *Engine) copyIterationOutput(iterStepCtx, mainStepCtx *StepContext, renderedStepID string) {
	if output, ok := iterStepCtx.GetOutput(renderedStepID); ok {
		mainStepCtx.SetOutput(renderedStepID, output)
	}
}

// collectParallelErrors waits for parallel operations and collects any errors
func (e *Engine) collectParallelErrors(wg *sync.WaitGroup, errChan chan error, stepCtx *StepContext, stepID string) error {
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	// Set output if stepID is non-empty
	if stepID != "" {
		stepCtx.SetOutput(stepID, make(map[string]any))
	}
	return nil
}

// executeForeachSequential handles sequential foreach execution
func (e *Engine) executeForeachSequential(ctx context.Context, step *model.Step, stepCtx *StepContext, stepID string, list []any) error {
	for _, item := range list {
		// Set the loop variable for this iteration
		if step.As != "" {
			stepCtx.SetVar(step.As, item)
		}

		// Execute all steps for this iteration
		if err := e.executeSequentialIterationSteps(ctx, step.Do, stepCtx); err != nil {
			return err
		}
	}

	// Set output if stepID is non-empty
	if stepID != "" {
		stepCtx.SetOutput(stepID, make(map[string]any))
	}
	return nil
}

// executeSequentialIterationSteps executes all steps for a single sequential foreach iteration
func (e *Engine) executeSequentialIterationSteps(ctx context.Context, steps []model.Step, stepCtx *StepContext) error {
	for _, inner := range steps {
		// Render the step ID as a template
		renderedStepID, err := e.renderStepID(inner.ID, stepCtx)
		if err != nil {
			return err
		}

		// Execute the step
		if err := e.executeStep(ctx, &inner, stepCtx, renderedStepID); err != nil {
			return err
		}
	}
	return nil
}

// executeToolCall handles individual tool execution
func (e *Engine) executeToolCall(ctx context.Context, step *model.Step, stepCtx *StepContext, stepID string) error {
	if step.Use == "" {
		return nil
	}

	// Resolve the appropriate adapter for this tool
	adapterInst, err := e.resolveAdapter(step.Use, stepCtx, stepID)
	if err != nil {
		return err
	}

	// Prepare inputs and execute the tool
	return e.executeToolWithInputs(ctx, step, stepCtx, stepID, adapterInst)
}

// resolveAdapter finds and returns the appropriate adapter for a tool
func (e *Engine) resolveAdapter(toolName string, stepCtx *StepContext, stepID string) (adapter.Adapter, error) {
	adapterInst, ok := e.Adapters.Get(toolName)
	if !ok {
		switch {
		case strings.HasPrefix(toolName, "mcp://"):
			adapterInst, ok = e.Adapters.Get("mcp")
			if !ok {
				stepCtx.SetOutput(stepID, make(map[string]any))
				return nil, utils.Errorf("MCPAdapter not registered")
			}
		case strings.HasPrefix(toolName, "core."):
			adapterInst, ok = e.Adapters.Get("core")
			if !ok {
				stepCtx.SetOutput(stepID, make(map[string]any))
				return nil, utils.Errorf("CoreAdapter not registered")
			}
		default:
			stepCtx.SetOutput(stepID, make(map[string]any))
			return nil, utils.Errorf("adapter not found: %s", toolName)
		}
	}
	return adapterInst, nil
}

// executeToolWithInputs prepares inputs and executes the tool
func (e *Engine) executeToolWithInputs(ctx context.Context, step *model.Step, stepCtx *StepContext, stepID string, adapterInst adapter.Adapter) error {
	// Prepare inputs for the tool
	inputs, err := e.prepareToolInputs(step, stepCtx, stepID)
	if err != nil {
		return err
	}

	// Auto-fill missing required parameters from manifest defaults
	e.autoFillRequiredParams(adapterInst, inputs, stepCtx)

	// Add special use parameter for specific tool types
	e.addSpecialUseParameter(step.Use, inputs)

	// Execute the tool and handle results
	return e.handleToolExecution(ctx, step.Use, stepID, stepCtx, adapterInst, inputs)
}

// addSpecialUseParameter adds the __use parameter for MCP and core tools
func (e *Engine) addSpecialUseParameter(toolName string, inputs map[string]any) {
	if strings.HasPrefix(toolName, "mcp://") || strings.HasPrefix(toolName, "core.") {
		inputs["__use"] = toolName
	}
}

// handleToolExecution executes the tool and processes outputs
func (e *Engine) handleToolExecution(ctx context.Context, toolName, stepID string, stepCtx *StepContext, adapterInst adapter.Adapter, inputs map[string]any) error {
	// Log payload for debugging
	if payload, err := json.Marshal(inputs); err == nil {
		utils.Debug("tool %s payload: %s", toolName, payload)
	} else {
		utils.ErrorCtx(ctx, "Failed to marshal tool inputs: %v", "error", err)
	}

	// Execute the tool
	outputs, err := adapterInst.Execute(ctx, inputs)
	if err != nil {
		stepCtx.SetOutput(stepID, outputs)
		return utils.Errorf("step %s failed: %w", stepID, err)
	}

	// Store outputs and log success
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
	// Load tools from registry
	tools, err := e.loadRegistryTools(ctx)
	if err != nil {
		return nil, err
	}

	// Filter and convert MCP tools to server configs
	return e.convertToMCPServers(tools), nil
}

// loadRegistryTools loads all tools from the registry
func (e *Engine) loadRegistryTools(ctx context.Context) ([]registry.RegistryEntry, error) {
	localReg := registry.NewLocalRegistry(constants.RegistryIndexFile)
	regMgr := registry.NewRegistryManager(localReg)
	return regMgr.ListAllServers(ctx, registry.ListOptions{})
}

// convertToMCPServers filters and converts registry entries to MCP server configs
func (e *Engine) convertToMCPServers(tools []registry.RegistryEntry) []*MCPServerWithName {
	var mcps []*MCPServerWithName
	for _, entry := range tools {
		if strings.HasPrefix(entry.Name, "mcp://") {
			mcps = append(mcps, e.createMCPServerConfig(entry))
		}
	}
	return mcps
}

// createMCPServerConfig creates an MCP server configuration from a registry entry
func (e *Engine) createMCPServerConfig(entry registry.RegistryEntry) *MCPServerWithName {
	return &MCPServerWithName{
		Name: entry.Name,
		Config: &config.MCPServerConfig{
			Command:   entry.Command,
			Args:      entry.Args,
			Env:       entry.Env,
			Port:      entry.Port,
			Transport: entry.Transport,
			Endpoint:  entry.Endpoint,
		},
	}
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
