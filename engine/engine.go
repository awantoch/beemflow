package engine

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/awantoch/beemflow/adapter"
	"github.com/awantoch/beemflow/blob"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/dsl"
	"github.com/awantoch/beemflow/event"
	"github.com/awantoch/beemflow/registry"
	pproto "github.com/awantoch/beemflow/spec/proto"
	"github.com/awantoch/beemflow/storage"
	"github.com/awantoch/beemflow/utils"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Define a custom type for context keys
type runIDKeyType struct{}

var runIDKey = runIDKeyType{}

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
	Flow    *pproto.Flow
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
	reg.Register(&adapter.CoreAdapter{})
	reg.Register(adapter.NewMCPAdapter())
	reg.Register(&adapter.HTTPFetchAdapter{})

	// Load config if available
	cfg, err := config.LoadConfig(config.DefaultConfigPath)
	localRegistryPath := config.DefaultLocalRegistryPath
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
	curatedTools, _ := curatedMgr.ListAllServers(ctx, registry.ListOptions{})

	// Load local registry
	localReg := registry.NewLocalRegistry(localRegistryPath)
	localMgr := registry.NewRegistryManager(localReg)
	localTools, _ := localMgr.ListAllServers(ctx, registry.ListOptions{})

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

// toProtoValue converts a Go value into a protobuf Value.
func toProtoValue(v any) *pproto.Value {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case string:
		return &pproto.Value{Value: &pproto.Value_S{S: x}}
	case bool:
		return &pproto.Value{Value: &pproto.Value_B{B: x}}
	case float64:
		return &pproto.Value{Value: &pproto.Value_N{N: x}}
	case int:
		return &pproto.Value{Value: &pproto.Value_N{N: float64(x)}}
	case int64:
		return &pproto.Value{Value: &pproto.Value_N{N: float64(x)}}
	case map[string]any:
		m := make(map[string]*pproto.Value)
		for k, v2 := range x {
			if pv := toProtoValue(v2); pv != nil {
				m[k] = pv
			}
		}
		return &pproto.Value{Value: &pproto.Value_M{M: &pproto.Struct{Fields: m}}}
	case []any:
		var lst []*pproto.Value
		for _, elem := range x {
			if pv := toProtoValue(elem); pv != nil {
				lst = append(lst, pv)
			}
		}
		return &pproto.Value{Value: &pproto.Value_L{L: &pproto.ListValue{Values: lst}}}
	default:
		return nil
	}
}

// toProtoMap converts a Go map[string]any into a map[string]*pproto.Value.
func toProtoMap(m map[string]any) map[string]*pproto.Value {
	out := make(map[string]*pproto.Value, len(m))
	for k, v := range m {
		if pv := toProtoValue(v); pv != nil {
			out[k] = pv
		}
	}
	return out
}

// protoValueToAny converts a protobuf Value into a Go any.
func protoValueToAny(v *pproto.Value) any {
	if v == nil {
		return nil
	}
	switch x := v.GetValue().(type) {
	case *pproto.Value_S:
		return x.S
	case *pproto.Value_N:
		return x.N
	case *pproto.Value_B:
		return x.B
	case *pproto.Value_M:
		m := map[string]any{}
		for k, vv := range x.M.GetFields() {
			m[k] = protoValueToAny(vv)
		}
		return m
	case *pproto.Value_L:
		var lst []any
		for _, vv := range x.L.GetValues() {
			lst = append(lst, protoValueToAny(vv))
		}
		return lst
	default:
		return nil
	}
}

// protoMapToGo converts a protobuf map[string]*Value to map[string]any for templating.
func protoMapToGo(m map[string]*pproto.Value) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = protoValueToAny(v)
	}
	return out
}

// Execute now supports pausing and resuming at await_event.
func (e *Engine) Execute(ctx context.Context, flow *pproto.Flow, event map[string]any) (map[string]any, error) {
	if flow == nil {
		return nil, nil
	}
	// Initialize outputs and handle empty flow as no-op
	outputs := make(map[string]any)
	if len(flow.GetSteps()) == 0 {
		return outputs, nil
	}
	// Collect env secrets and merge with event-supplied secrets
	secretsMap := map[string]any{}
	for _, envKV := range os.Environ() {
		if eq := strings.Index(envKV, "="); eq != -1 {
			k := envKV[:eq]
			v := envKV[eq+1:]
			secretsMap[k] = v
		}
	}
	if event != nil {
		if s, ok := event["secrets"].(map[string]any); ok {
			for k, v := range s {
				if _, ok2 := v.(string); ok2 {
					secretsMap[k] = v
				}
			}
		}
	}
	// Register a 'secrets' helper for this execution
	// With pongo2, secrets are available as a map in the context: {{ secrets.MY_SECRET }}
	// No need to register helpers; context flattening will expose secrets as top-level keys if needed.
	stepCtx := &StepContext{
		Event:   event,
		Vars:    protoMapToGo(flow.GetVars()),
		Outputs: outputs,
		Secrets: secretsMap,
	}

	// Create and persist the run
	runID := uuid.New()
	run := &pproto.Run{
		Id:        runID.String(),
		FlowName:  flow.Name,
		Event:     toProtoMap(event),
		Vars:      flow.GetVars(),
		Status:    pproto.RunStatus_RUN_STATUS_RUNNING,
		StartedAt: timestamppb.New(time.Now()),
	}
	if err := e.Storage.SaveRun(ctx, run); err != nil {
		utils.ErrorCtx(ctx, "SaveRun failed: %v", "error", err)
	}

	outputs, err := e.executeStepsWithPersistence(ctx, flow, stepCtx, 0, runID)

	// On completion, update run status (treat pause as waiting)
	status := pproto.RunStatus_RUN_STATUS_SUCCEEDED
	if err != nil {
		if strings.Contains(err.Error(), "await_event pause") {
			status = pproto.RunStatus_RUN_STATUS_WAITING
		} else {
			status = pproto.RunStatus_RUN_STATUS_FAILED
		}
	}
	run = &pproto.Run{
		Id:        runID.String(),
		FlowName:  flow.Name,
		Event:     toProtoMap(event),
		Vars:      flow.GetVars(),
		Status:    status,
		StartedAt: timestamppb.New(time.Now()),
		EndedAt:   timestamppb.New(time.Now()),
	}
	if err := e.Storage.SaveRun(ctx, run); err != nil {
		utils.ErrorCtx(ctx, "SaveRun failed: %v", "error", err)
	}

	if err != nil && len(flow.GetCatch()) > 0 {
		// Run catch steps in defined order if error
		catchOutputs := map[string]any{}
		for _, step := range flow.GetCatch() {
			err2 := e.executeStep(ctx, step, stepCtx, step.GetId())
			if err2 == nil {
				catchOutputs[step.GetId()] = stepCtx.Outputs[step.GetId()]
			}
		}
		return catchOutputs, err
	}
	return outputs, err
}

// executeStepsWithPersistence executes steps, persisting each step after execution
func (e *Engine) executeStepsWithPersistence(ctx context.Context, flow *pproto.Flow, stepCtx *StepContext, startIdx int, runID uuid.UUID) (map[string]any, error) {
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

	for i := startIdx; i < len(flow.GetSteps()); i++ {
		step := flow.GetSteps()[i]
		// Conditional execution: skip if condition is defined and evaluates to false
		if cond := step.GetCondition(); cond != "" {
			// Prepare template data
			data := make(map[string]any)
			data["event"] = stepCtx.Event
			data["vars"] = stepCtx.Vars
			data["outputs"] = stepCtx.Outputs
			data["secrets"] = stepCtx.Secrets
			for id, out := range stepCtx.Outputs {
				data[id] = out
			}
			for key, val := range stepCtx.Vars {
				data[key] = val
			}
			rendered, err := e.Templater.Render(cond, data)
			if err != nil {
				return nil, utils.Errorf("failed to render condition for step %s: %w", step.GetId(), err)
			}
			b, err := strconv.ParseBool(strings.TrimSpace(rendered))
			if err != nil || !b {
				// Skip this step
				continue
			}
		}
		if step.GetAwaitEvent() != nil {
			// Render token from match (support template)
			eqMap := step.GetAwaitEvent().GetMatch().GetEquals()
			tokenRaw := eqMap["token"]
			if tokenRaw == "" {
				return nil, utils.Errorf("await_event step missing token in match")
			}
			// Render the token template
			data := make(map[string]any)
			data["event"] = stepCtx.Event
			data["vars"] = stepCtx.Vars
			data["outputs"] = stepCtx.Outputs
			data["secrets"] = stepCtx.Secrets
			// Flatten outputs into context for template rendering
			for id, out := range stepCtx.Outputs {
				data[id] = out
			}
			// Flatten vars into context for template rendering
			if stepCtx.Vars != nil {
				for key, val := range stepCtx.Vars {
					data[key] = val
				}
			}
			// DEBUG: Log full context before rendering
			utils.Debug("About to render template for step %s: data = %#v", step.GetId(), data)
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
						existingRun.Status = pproto.RunStatus_RUN_STATUS_SKIPPED
						existingRun.EndedAt = timestamppb.New(time.Now())
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
			e.EventBus.Subscribe(ctx, "resume:"+token, func(payload any) {
				resumeEvent, ok := payload.(map[string]any)
				if !ok {
					return
				}
				e.Resume(ctx, token, resumeEvent)
			})
			return nil, utils.Errorf("step %s is waiting for event (await_event pause)", step.GetId())
		}
		err := e.executeStep(ctx, step, stepCtx, step.GetId())
		// Persist the step after execution
		if e.Storage != nil {
			srun := &pproto.StepRun{
				Id:        uuid.New().String(),
				RunId:     runID.String(),
				StepName:  step.GetId(),
				Status:    pproto.StepStatus_STEP_STATUS_SUCCEEDED,
				StartedAt: timestamppb.New(time.Now()),
				EndedAt:   timestamppb.New(time.Now()),
			}
			if err != nil {
				srun.Status = pproto.StepStatus_STEP_STATUS_FAILED
				srun.Error = err.Error()
			}
			if err := e.Storage.SaveStep(ctx, srun); err != nil {
				utils.Error("SaveStep failed: %v", err)
			}
		}
		if err != nil {
			return stepCtx.Outputs, err
		}
	}
	return stepCtx.Outputs, nil
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
		_ = e.Storage.DeletePausedRun(token)
	}
	e.mu.Unlock()
	// Update event context
	for k, v := range resumeEvent {
		paused.StepCtx.Event[k] = v
	}
	utils.Debug("Outputs map before resume for token %s: %+v", token, paused.StepCtx.Outputs)
	// Continue execution from next step
	ctx = context.WithValue(ctx, runIDKey, paused.RunID)
	outputs, err := e.executeStepsWithPersistence(ctx, paused.Flow, paused.StepCtx, paused.StepIdx+1, paused.RunID)
	// Merge outputs from before and after resume
	allOutputs := make(map[string]any)
	for k, v := range paused.StepCtx.Outputs {
		allOutputs[k] = v
	}
	if outputs != nil {
		for k, v := range outputs {
			allOutputs[k] = v
		}
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
		status := pproto.RunStatus_RUN_STATUS_SUCCEEDED
		if err != nil {
			status = pproto.RunStatus_RUN_STATUS_FAILED
		}
		run := &pproto.Run{
			Id:        paused.RunID.String(),
			FlowName:  paused.Flow.Name,
			Event:     toProtoMap(paused.StepCtx.Event),
			Vars:      toProtoMap(paused.StepCtx.Vars),
			Status:    status,
			StartedAt: timestamppb.New(time.Now()),
			EndedAt:   timestamppb.New(time.Now()),
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

// executeStep runs a single step (use/with) and stores output
func (e *Engine) executeStep(ctx context.Context, step *pproto.Step, stepCtx *StepContext, stepID string) error {
	// Nested parallel block logic
	if step.GetParallel() != nil && len(step.GetParallel().GetSteps()) > 0 {
		var wg sync.WaitGroup
		errChan := make(chan error, len(step.GetParallel().GetSteps()))
		outputs := make(map[string]any)
		mu := sync.Mutex{}
		for _, child := range step.GetParallel().GetSteps() {
			wg.Add(1)
			go func(child *pproto.Step) {
				defer wg.Done()
				if err := e.executeStep(ctx, child, stepCtx, child.GetId()); err != nil {
					errChan <- err
					return
				}
				mu.Lock()
				outputs[child.GetId()] = stepCtx.Outputs[child.GetId()]
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
	// Foreach logic: handle steps with Foreach and Do
	if f := step.GetForeach(); f != nil {
		// Evaluate list expression as variable name
		listKey := f.GetListExpr()
		itemsRaw, ok := stepCtx.Vars[listKey]
		if !ok {
			return utils.Errorf("foreach list variable not found: %s", listKey)
		}
		items, ok := itemsRaw.([]any)
		if !ok {
			return utils.Errorf("foreach variable %s is not a list", listKey)
		}
		// Determine alias for each item
		alias := f.GetAlias()
		if alias == "" {
			alias = listKey
		}
		// Collect aggregated outputs for child steps
		aggregated := make(map[string][]any)
		for _, item := range items {
			// Set alias in vars for nested templating
			stepCtx.Vars[alias] = item
			for _, child := range f.GetSteps() {
				if err := e.executeStep(ctx, child, stepCtx, child.GetId()); err != nil {
					return err
				}
				aggregated[child.GetId()] = append(aggregated[child.GetId()], stepCtx.Outputs[child.GetId()])
			}
		}
		// Cleanup alias variable
		delete(stepCtx.Vars, alias)
		// Write aggregated outputs back to context
		for childID, arr := range aggregated {
			stepCtx.Outputs[childID] = arr
		}
		return nil
	}
	if step.GetExec().GetUse() == "" {
		return nil
	}
	execUse := step.GetExec().GetUse()
	adapterInst, ok := e.Adapters.Get(execUse)
	if !ok {
		if strings.HasPrefix(execUse, "mcp://") {
			adapterInst, ok = e.Adapters.Get("mcp")
			if !ok {
				stepCtx.Outputs[stepID] = make(map[string]any)
				return utils.Errorf("MCPAdapter not registered")
			}
		} else {
			stepCtx.Outputs[stepID] = make(map[string]any)
			return utils.Errorf("adapter not found: %s", execUse)
		}
	}
	inputs := make(map[string]any)
	for k, v := range step.GetExec().GetWith() {
		// Prepare template data, flattening previous step outputs for direct access
		data := make(map[string]any)
		data["event"] = stepCtx.Event
		data["vars"] = stepCtx.Vars
		data["outputs"] = stepCtx.Outputs
		data["secrets"] = stepCtx.Secrets
		for id, out := range stepCtx.Outputs {
			data[id] = out
		}
		// --- FLATTEN VARS INTO TOP-LEVEL CONTEXT ---
		if stepCtx.Vars != nil {
			for key, val := range stepCtx.Vars {
				data[key] = val
			}
		}
		// DEBUG: Log context keys and important values
		varsKeys := []string{}
		if vars, ok := data["vars"].(map[string]any); ok {
			for key := range vars {
				varsKeys = append(varsKeys, key)
			}
		}
		utils.Debug("Template context keys: %v, vars keys: %v, vars: %+v", keys(data), varsKeys, data["vars"])
		// DEBUG: Log full context before rendering
		utils.Debug("About to render template for step %s: data = %#v", stepID, data)
		rendered, err := e.renderValue(v, data)
		if err != nil {
			return utils.Errorf("template error in step %s: %w", stepID, err)
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
	utils.Debug("tool %s payload: %s", execUse, payload)
	if strings.HasPrefix(execUse, "mcp://") {
		inputs["__use"] = execUse
	}
	outputs, err := adapterInst.Execute(ctx, inputs)
	if err != nil {
		stepCtx.Outputs[stepID] = outputs
		return utils.Errorf("step %s failed: %w", stepID, err)
	}
	utils.Debug("Writing outputs for step %s: %+v", stepID, outputs)
	stepCtx.Outputs[stepID] = outputs
	utils.Debug("Outputs map after step %s: %+v", stepID, stepCtx.Outputs)
	return nil
}

// StepContext holds context for step execution (event, vars, outputs, secrets)
type StepContext struct {
	Event   map[string]any
	Vars    map[string]any
	Outputs map[string]any
	Secrets map[string]any
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
func (e *Engine) ListRuns(ctx context.Context) ([]*pproto.Run, error) {
	return e.Storage.ListRuns(ctx)
}

// GetRunByID returns a run by ID, using storage if available
func (e *Engine) GetRunByID(ctx context.Context, id uuid.UUID) (*pproto.Run, error) {
	run, err := e.Storage.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	steps, err := e.Storage.GetSteps(ctx, id)
	if err == nil {
		var persisted []*pproto.StepRun
		for _, s := range steps {
			persisted = append(persisted, s)
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

// Helper to get map keys for debug logging
func keys(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
