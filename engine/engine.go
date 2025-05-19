package engine

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/awantoch/beemflow/adapter"
	"github.com/awantoch/beemflow/blob"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/event"
	"github.com/awantoch/beemflow/logger"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/registry"
	"github.com/awantoch/beemflow/storage"
	"github.com/awantoch/beemflow/templater"
	"github.com/google/uuid"
)

// Define a custom type for context keys
type runIDKeyType struct{}

var runIDKey = runIDKeyType{}

// Engine is the core runtime for executing BeemFlow flows. It manages adapters, templating, event bus, and in-memory state.
type Engine struct {
	Adapters  *adapter.Registry
	Templater *templater.Templater
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

// newDefaultAdapterRegistry creates and returns a default adapter registry with core and registry tools.
//
// - Loads the curated registry (repo-managed, read-only) from registry/index.json.
// - Loads the local registry (user-writable) from config (registries[].path) or .beemflow/local_registry.json.
// - Merges both, with local entries taking precedence over curated ones.
// - Any tool installed via the CLI is written to the local registry file.
// - This is future-proofed for remote/community registries.
func newDefaultAdapterRegistry() *adapter.Registry {
	reg := adapter.NewRegistry()
	reg.Register(&adapter.CoreAdapter{})
	reg.Register(adapter.NewMCPAdapter())
	reg.Register(&adapter.HTTPFetchAdapter{})

	// Load config if available
	cfg, err := config.LoadConfig("flow.config.json")
	localRegistryPath := ".beemflow/local_registry.json"
	if err == nil && len(cfg.Registries) > 0 {
		for _, regCfg := range cfg.Registries {
			if regCfg.Type == "local" && regCfg.Path != "" {
				localRegistryPath = regCfg.Path
			}
		}
	}

	// Load curated registry
	curatedReg := registry.NewLocalRegistry("registry/index.json")
	curatedMgr := registry.NewRegistryManager(curatedReg)
	curatedTools, _ := curatedMgr.ListAllServers(context.Background(), registry.ListOptions{})

	// Load local registry
	localReg := registry.NewLocalRegistry(localRegistryPath)
	localMgr := registry.NewRegistryManager(localReg)
	localTools, _ := localMgr.ListAllServers(context.Background(), registry.ListOptions{})

	// Merge: local takes precedence
	toolMap := map[string]registry.RegistryEntry{}
	for _, entry := range curatedTools {
		toolMap[entry.Name] = entry
	}
	for _, entry := range localTools {
		toolMap[entry.Name] = entry
	}
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
	return reg
}

// NewEngineWithBlobStore creates a new Engine with a custom BlobStore.
func NewEngineWithBlobStore(blobStore blob.BlobStore) *Engine {
	return &Engine{
		Adapters:         newDefaultAdapterRegistry(),
		Templater:        templater.NewTemplater(),
		EventBus:         event.NewInProcEventBus(),
		BlobStore:        blobStore,
		waiting:          make(map[string]*PausedRun),
		completedOutputs: make(map[string]map[string]any),
		Storage:          storage.NewMemoryStorage(),
	}
}

// NewEngine creates a new Engine with the default BlobStore (filesystem, zero config).
func NewEngine() *Engine {
	bs, err := blob.NewDefaultBlobStore(nil)
	if err != nil {
		logger.Warn("Failed to create default blob store: %v, using in-memory fallback", err)
		bs = nil // or fallback to a stub if you want
	}
	return NewEngineWithBlobStore(bs)
}

// NewEngineWithStorage creates a new Engine with a custom Storage backend.
func NewEngineWithStorage(store storage.Storage) *Engine {
	if store == nil {
		store = storage.NewMemoryStorage()
	}
	eng := &Engine{
		Adapters:         newDefaultAdapterRegistry(),
		Templater:        templater.NewTemplater(),
		EventBus:         event.NewInProcEventBus(),
		BlobStore:        nil, // or set to a default if needed
		Storage:          store,
		waiting:          make(map[string]*PausedRun),
		completedOutputs: make(map[string]map[string]any),
	}
	// Load paused runs from storage (generic)
	if store != nil {
		paused, err := store.LoadPausedRuns()
		if err == nil {
			for token, v := range paused {
				// Try to coerce to the expected struct (PausedRunPersist)
				var persist struct {
					Flow    *model.Flow    `json:"flow"`
					StepIdx int            `json:"step_idx"`
					StepCtx map[string]any `json:"step_ctx"`
					Outputs map[string]any `json:"outputs"`
					Token   string         `json:"token"`
					RunID   string         `json:"run_id"`
				}
				b, _ := json.Marshal(v)
				_ = json.Unmarshal(b, &persist)
				pr := &PausedRun{
					Flow:    persist.Flow,
					StepIdx: persist.StepIdx,
					StepCtx: &StepContext{},
					Outputs: persist.Outputs,
					Token:   token,
				}
				b2, _ := json.Marshal(persist.StepCtx)
				_ = json.Unmarshal(b2, pr.StepCtx)
				if persist.RunID != "" {
					pr.RunID, _ = uuid.Parse(persist.RunID)
				}
				eng.waiting[token] = pr
				// Re-subscribe to event bus
				eng.EventBus.Subscribe("resume:"+token, func(payload any) {
					resumeEvent, ok := payload.(map[string]any)
					if !ok {
						return
					}
					eng.Resume(token, resumeEvent)
				})
			}
		}
	}
	return eng
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

	// Create and persist the run
	var runID uuid.UUID = uuid.New()
	run := &model.Run{
		ID:        runID,
		FlowName:  flow.Name,
		Event:     event,
		Vars:      flow.Vars,
		Status:    model.RunRunning,
		StartedAt: time.Now(),
	}
	if err := e.Storage.SaveRun(ctx, run); err != nil {
		logger.Error("SaveRun failed: %v", err)
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
		logger.Error("SaveRun failed: %v", err)
	}

	if err != nil && len(flow.Catch) > 0 {
		// Run catch steps in defined order if error
		catchOutputs := map[string]any{}
		for _, step := range flow.Catch {
			err2 := e.executeStep(ctx, &step, stepCtx, step.ID)
			if err2 == nil {
				catchOutputs[step.ID] = stepCtx.Outputs[step.ID]
			}
		}
		return catchOutputs, err
	}
	return outputs, err
}

// Helper to get pointer to time.Time
func ptrTime(t time.Time) *time.Time {
	return &t
}

// executeStepsWithPersistence executes steps, persisting each step after execution
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
			tokenRaw, _ := match["token"].(string)
			if tokenRaw == "" {
				return nil, logger.Errorf("await_event step missing token in match")
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
				return nil, logger.Errorf("failed to render token template: %w", err)
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
						_ = e.Storage.SaveRun(ctx, existingRun)
					}
					_ = e.Storage.DeletePausedRun(token)
				}
				delete(e.waiting, token)
			}
			// Register new paused run
			e.waiting[token] = &PausedRun{
				Flow:    flow,
				StepIdx: i,
				StepCtx: stepCtx,
				Outputs: stepCtx.Outputs,
				Token:   token,
				RunID:   runID,
			}
			if e.Storage != nil {
				_ = e.Storage.SavePausedRun(token, pausedRunToMap(e.waiting[token]))
			}
			e.mu.Unlock()
			e.EventBus.Subscribe("resume:"+token, func(payload any) {
				resumeEvent, ok := payload.(map[string]any)
				if !ok {
					return
				}
				e.Resume(token, resumeEvent)
			})
			return nil, logger.Errorf("step %s is waiting for event (await_event pause)", step.ID)
		}
		err := e.executeStep(ctx, step, stepCtx, step.ID)
		// Persist the step after execution
		if e.Storage != nil {
			var stepOutputs map[string]any
			if out, ok := stepCtx.Outputs[step.ID].(map[string]any); ok {
				stepOutputs = out
			} else {
				stepOutputs = nil
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
				logger.Error("SaveStep failed: %v", err)
			}
		}
		if err != nil {
			return stepCtx.Outputs, err
		}
	}
	return stepCtx.Outputs, nil
}

// Resume resumes a paused run with the given token and event.
func (e *Engine) Resume(token string, resumeEvent map[string]any) {
	logger.Debug("Resume called for token %s with event: %+v", token, resumeEvent)
	e.mu.Lock()
	paused, ok := e.waiting[token]
	if !ok {
		e.mu.Unlock()
		return
	}
	delete(e.waiting, token)
	if e.Storage != nil {
		_ = e.Storage.DeletePausedRun(token)
	}
	e.mu.Unlock()
	// Update event context
	for k, v := range resumeEvent {
		paused.StepCtx.Event[k] = v
	}
	logger.Debug("Outputs map before resume for token %s: %+v", token, paused.StepCtx.Outputs)
	// Continue execution from next step
	ctx := context.WithValue(context.Background(), runIDKey, paused.RunID)
	outputs, err := e.executeStepsWithPersistence(ctx, paused.Flow, paused.StepCtx, paused.StepIdx+1, paused.RunID)
	// Merge outputs from before and after resume
	allOutputs := make(map[string]any)
	for k, v := range paused.StepCtx.Outputs {
		allOutputs[k] = v
	}
	for k, v := range outputs {
		allOutputs[k] = v
	}
	logger.Debug("Outputs map after resume for token %s: %+v", token, allOutputs)
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
			Event:     paused.StepCtx.Event,
			Vars:      paused.StepCtx.Vars,
			Status:    status,
			StartedAt: time.Now(),
			EndedAt:   ptrTime(time.Now()),
		}
		if err := e.Storage.SaveRun(context.Background(), run); err != nil {
			logger.Error("SaveRun failed: %v", err)
		}
	}
}

// GetCompletedOutputs returns and clears the outputs for a completed resumed run.
func (e *Engine) GetCompletedOutputs(token string) map[string]any {
	logger.Debug("GetCompletedOutputs called for token %s", token)
	e.mu.Lock()
	defer e.mu.Unlock()
	outputs := e.completedOutputs[token]
	logger.Debug("GetCompletedOutputs for token %s returns: %+v", token, outputs)
	delete(e.completedOutputs, token)
	return outputs
}

// executeStep runs a single step (use/with) and stores output
func (e *Engine) executeStep(ctx context.Context, step *model.Step, stepCtx *StepContext, stepID string) error {
	// Nested parallel block logic
	if step.Parallel && len(step.Steps) > 0 {
		var wg sync.WaitGroup
		errChan := make(chan error, len(step.Steps))
		outputs := make(map[string]any)
		mu := sync.Mutex{}
		for i := range step.Steps {
			child := &step.Steps[i]
			wg.Add(1)
			go func(child *model.Step) {
				defer wg.Done()
				if err := e.executeStep(ctx, child, stepCtx, child.ID); err != nil {
					errChan <- err
					return
				}
				mu.Lock()
				outputs[child.ID] = stepCtx.Outputs[child.ID]
				mu.Unlock()
			}(child)
		}
		wg.Wait()
		close(errChan)
		for err := range errChan {
			if err != nil {
				return err
			}
		}
		stepCtx.Outputs[stepID] = outputs
		return nil
	}
	// Sequential block (non-parallel) for steps
	if !step.Parallel && len(step.Steps) > 0 {
		outputs := make(map[string]any)
		for i := range step.Steps {
			child := &step.Steps[i]
			if err := e.executeStep(ctx, child, stepCtx, child.ID); err != nil {
				return err
			}
			outputs[child.ID] = stepCtx.Outputs[child.ID]
		}
		stepCtx.Outputs[stepID] = outputs
		return nil
	}
	// Foreach logic: handle steps with Foreach and Do
	if step.Foreach != "" {
		s := strings.TrimSpace(step.Foreach)
		if strings.HasPrefix(s, "{{") && strings.HasSuffix(s, "}}") {
			key := strings.TrimSpace(s[2 : len(s)-2])
			val, ok := stepCtx.Event[key]
			if !ok {
				return logger.Errorf("foreach variable not found: %s", key)
			}
			list, ok := val.([]any)
			if !ok {
				return logger.Errorf("foreach variable %s is not a list", key)
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
		return logger.Errorf("unsupported foreach expression: %s", step.Foreach)
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
				return logger.Errorf("MCPAdapter not registered")
			}
		} else {
			stepCtx.Outputs[stepID] = make(map[string]any)
			return logger.Errorf("adapter not found: %s", step.Use)
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
			return logger.Errorf("template error in step %s: %w", stepID, err)
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
	// Optionally, add a generic debug log for all tool payloads if desired:
	payload, _ := json.Marshal(inputs)
	logger.Debug("tool %s payload: %s", step.Use, payload)
	if strings.HasPrefix(step.Use, "mcp://") {
		inputs["__use"] = step.Use
	}
	outputs, err := adapterInst.Execute(ctx, inputs)
	if err != nil {
		stepCtx.Outputs[stepID] = outputs
		return logger.Errorf("step %s failed: %w", stepID, err)
	}
	logger.Debug("Writing outputs for step %s: %+v", stepID, outputs)
	stepCtx.Outputs[stepID] = outputs
	logger.Debug("Outputs map after step %s: %+v", stepID, stepCtx.Outputs)
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

// Close cleans up all adapters and resources managed by the Engine.
func (e *Engine) Close() error {
	if e.Adapters != nil {
		return e.Adapters.CloseAll()
	}
	return nil
}

// Helper to convert PausedRun to map[string]any for storage
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

// Add a helper to extract runID from context (or use a global if needed)
func runIDFromContext(ctx context.Context) uuid.UUID {
	if v := ctx.Value(runIDKey); v != nil {
		if id, ok := v.(uuid.UUID); ok {
			return id
		}
	}
	return uuid.Nil
}

// ListRuns returns all runs, using storage if available, otherwise in-memory
func (e *Engine) ListRuns(ctx context.Context) ([]*model.Run, error) {
	return e.Storage.ListRuns(ctx)
}

// GetRunByID returns a run by ID, using storage if available
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

// ListMCPServers returns all MCP servers from the registry, including their names.
type MCPServerWithName struct {
	Name   string
	Config *config.MCPServerConfig
}

func (e *Engine) ListMCPServers() ([]*MCPServerWithName, error) {
	localReg := registry.NewLocalRegistry("registry/index.json")
	regMgr := registry.NewRegistryManager(localReg)
	tools, err := regMgr.ListAllServers(context.Background(), registry.ListOptions{})
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
