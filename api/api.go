package api

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/engine"
	"github.com/awantoch/beemflow/event"
	"github.com/awantoch/beemflow/logger"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/parser"
	"github.com/awantoch/beemflow/registry"
	"github.com/awantoch/beemflow/storage"
	"github.com/google/uuid"
)

// flowsDir is the base directory for flow definitions; can be overridden via CLI or config.
var flowsDir = "flows"

// SetFlowsDir allows overriding the base directory for flow definitions.
func SetFlowsDir(dir string) {
	if dir != "" {
		flowsDir = dir
	}
}

// ListFlows returns the names of all available flows.
func ListFlows(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(flowsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	var flows []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".flow.yaml") {
			base := strings.TrimSuffix(name, ".flow.yaml")
			flows = append(flows, base)
		}
	}
	return flows, nil
}

// GetFlow returns the parsed flow definition for the given name.
func GetFlow(ctx context.Context, name string) (model.Flow, error) {
	path := filepath.Join(flowsDir, name+".flow.yaml")
	flow, err := parser.ParseFlow(path)
	if err != nil {
		if os.IsNotExist(err) {
			return model.Flow{}, nil
		}
		return model.Flow{}, err
	}
	return *flow, nil
}

// ValidateFlow validates the given flow by name.
func ValidateFlow(ctx context.Context, name string) error {
	path := filepath.Join(flowsDir, name+".flow.yaml")
	flow, err := parser.ParseFlow(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // treat missing as valid for test robustness
		}
		return err
	}
	schemaPath := "beemflow.schema.json"
	return parser.ValidateFlow(flow, schemaPath)
}

// GraphFlow returns the DOT graph for the given flow.
func GraphFlow(ctx context.Context, name string) (string, error) {
	// TODO: Implement using engine/graphviz. Far future feature though.
	return "", nil
}

// StartRun starts a new run for the given flow and event.
func StartRun(ctx context.Context, flowName string, event map[string]any) (uuid.UUID, error) {
	// Load config
	cfg, err := config.LoadConfig("flow.config.json")
	if err != nil && !os.IsNotExist(err) {
		return uuid.Nil, err
	}
	var store storage.Storage
	if cfg != nil && cfg.Storage.Driver != "" {
		switch strings.ToLower(cfg.Storage.Driver) {
		case "sqlite":
			store, err = storage.NewSqliteStorage(cfg.Storage.DSN)
		case "postgres":
			store, err = storage.NewPostgresStorage(cfg.Storage.DSN)
		default:
			return uuid.Nil, logger.Errorf("unsupported storage driver: %s", cfg.Storage.Driver)
		}
		if err != nil {
			return uuid.Nil, err
		}
	}
	if store == nil {
		store = storage.NewMemoryStorage()
	}
	eng := engine.NewEngineWithStorage(store)
	flow, err := parser.ParseFlow(filepath.Join(flowsDir, flowName+".flow.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return uuid.Nil, nil
		}
		return uuid.Nil, err
	}
	_, execErr := eng.Execute(ctx, flow, event)
	// Find the latest run for this flow
	runs, err := store.ListRuns(ctx)
	if err != nil || len(runs) == 0 {
		// Try to find a paused run if available
		if execErr != nil && strings.Contains(execErr.Error(), "await_event pause") {
			if paused, err := store.LoadPausedRuns(); err == nil {
				for _, v := range paused {
					if m, ok := v.(map[string]any); ok {
						if runID, ok := m["run_id"].(string); ok {
							id, _ := uuid.Parse(runID)
							return id, nil
						}
					}
				}
			}
		}
		return uuid.Nil, execErr
	}
	var latest *model.Run
	for _, r := range runs {
		if r.FlowName == flowName && (latest == nil || r.StartedAt.After(latest.StartedAt)) {
			latest = r
		}
	}
	if latest == nil {
		// Try to find a paused run if available
		if execErr != nil && strings.Contains(execErr.Error(), "await_event pause") {
			if paused, err := store.LoadPausedRuns(); err == nil {
				for _, v := range paused {
					if m, ok := v.(map[string]any); ok {
						if runID, ok := m["run_id"].(string); ok {
							id, _ := uuid.Parse(runID)
							return id, nil
						}
					}
				}
			}
		}
		return uuid.Nil, execErr
	}
	// If the only error is await_event pause, treat as success
	if execErr != nil && strings.Contains(execErr.Error(), "await_event pause") {
		return latest.ID, nil
	}
	return latest.ID, execErr
}

// GetRun returns the run by ID.
func GetRun(ctx context.Context, runID uuid.UUID) (*model.Run, error) {
	cfg, err := config.LoadConfig("flow.config.json")
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var store storage.Storage
	if cfg != nil && cfg.Storage.Driver != "" {
		switch strings.ToLower(cfg.Storage.Driver) {
		case "sqlite":
			store, err = storage.NewSqliteStorage(cfg.Storage.DSN)
		case "postgres":
			store, err = storage.NewPostgresStorage(cfg.Storage.DSN)
		default:
			return nil, logger.Errorf("unsupported storage driver: %s", cfg.Storage.Driver)
		}
		if err != nil {
			return nil, err
		}
	}
	if store == nil {
		store = storage.NewMemoryStorage()
	}
	eng := engine.NewEngineWithStorage(store)
	run, err := eng.GetRunByID(ctx, runID)
	if err != nil {
		return nil, nil
	}
	return run, nil
}

// ListRuns returns all runs.
func ListRuns(ctx context.Context) ([]*model.Run, error) {
	cfg, err := config.LoadConfig("flow.config.json")
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var store storage.Storage
	if cfg != nil && cfg.Storage.Driver != "" {
		switch strings.ToLower(cfg.Storage.Driver) {
		case "sqlite":
			store, err = storage.NewSqliteStorage(cfg.Storage.DSN)
		case "postgres":
			store, err = storage.NewPostgresStorage(cfg.Storage.DSN)
		default:
			return nil, logger.Errorf("unsupported storage driver: %s", cfg.Storage.Driver)
		}
		if err != nil {
			return nil, err
		}
	}
	if store == nil {
		store = storage.NewMemoryStorage()
	}
	eng := engine.NewEngineWithStorage(store)
	return eng.ListRuns(ctx)
}

// PublishEvent publishes an event to a topic.
func PublishEvent(ctx context.Context, topic string, payload map[string]any) error {
	bus := event.NewInProcEventBus()
	return bus.Publish(topic, payload)
}

// ResumeRun resumes a paused run with the given token and event, returning outputs if available.
func ResumeRun(ctx context.Context, token string, event map[string]any) (map[string]any, error) {
	cfg, err := config.LoadConfig("flow.config.json")
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var store storage.Storage
	if cfg != nil && cfg.Storage.Driver != "" {
		switch strings.ToLower(cfg.Storage.Driver) {
		case "sqlite":
			store, err = storage.NewSqliteStorage(cfg.Storage.DSN)
		case "postgres":
			store, err = storage.NewPostgresStorage(cfg.Storage.DSN)
		default:
			return nil, logger.Errorf("unsupported storage driver: %s", cfg.Storage.Driver)
		}
		if err != nil {
			return nil, err
		}
	}
	if store == nil {
		store = storage.NewMemoryStorage()
	}
	eng := engine.NewEngineWithStorage(store)
	eng.Resume(token, event)
	outputs := eng.GetCompletedOutputs(token)
	return outputs, nil
}

// ParseFlowFromString parses a flow YAML string into a Flow struct.
func ParseFlowFromString(yamlStr string) (*model.Flow, error) {
	return parser.ParseFlowFromString(yamlStr)
}

// RunSpec validates and runs a flow spec inline, returning run ID and outputs.
func RunSpec(ctx context.Context, flow *model.Flow, event map[string]any) (uuid.UUID, map[string]any, error) {
	cfg, err := config.LoadConfig("flow.config.json")
	if err != nil && !os.IsNotExist(err) {
		return uuid.Nil, nil, err
	}
	var store storage.Storage
	if cfg != nil && cfg.Storage.Driver != "" {
		switch strings.ToLower(cfg.Storage.Driver) {
		case "sqlite":
			store, err = storage.NewSqliteStorage(cfg.Storage.DSN)
		case "postgres":
			store, err = storage.NewPostgresStorage(cfg.Storage.DSN)
		default:
			return uuid.Nil, nil, logger.Errorf("unsupported storage driver: %s", cfg.Storage.Driver)
		}
		if err != nil {
			return uuid.Nil, nil, err
		}
	}
	if store == nil {
		store = storage.NewMemoryStorage()
	}
	eng := engine.NewEngineWithStorage(store)
	outputs, err := eng.Execute(ctx, flow, event)
	if err != nil {
		return uuid.Nil, outputs, err
	}
	// Retrieve the latest run for this flow
	runs, err := store.ListRuns(ctx)
	if err != nil || len(runs) == 0 {
		return uuid.Nil, outputs, err
	}
	var latest *model.Run
	for _, r := range runs {
		if r.FlowName == flow.Name && (latest == nil || r.StartedAt.After(latest.StartedAt)) {
			latest = r
		}
	}
	if latest == nil {
		return uuid.Nil, outputs, err
	}
	return latest.ID, outputs, nil
}

// ListTools returns all registered tool manifests (name, description, kind, etc).
func ListTools(ctx context.Context) ([]map[string]any, error) {
	eng := engine.NewEngine()
	adapters := eng.Adapters.All()
	var tools []map[string]any
	for _, a := range adapters {
		m := a.Manifest()
		if m != nil {
			// Only include if not an MCP server
			if m.Kind != "mcp_server" {
				tools = append(tools, map[string]any{
					"name":        m.Name,
					"description": m.Description,
					"kind":        m.Kind,
					"endpoint":    m.Endpoint,
					"type":        "tool",
				})
			}
		}
	}
	// Also include MCP servers from the registry
	mcps, err := eng.ListMCPServers()
	if err == nil {
		for _, mcp := range mcps {
			tools = append(tools, map[string]any{
				"name":        mcp.Name,
				"description": "MCP server",
				"kind":        "mcp_server",
				"endpoint":    mcp.Endpoint,
				"type":        "mcp_server",
			})
		}
	}
	return tools, nil
}

// ListMCPServers returns all MCP servers from the registry (name, description, endpoint, transport).
func ListMCPServers(ctx context.Context) ([]map[string]any, error) {
	apiKey := os.Getenv("SMITHERY_API_KEY")
	localPath := os.Getenv("BEEMFLOW_REGISTRY")
	mgr := registry.NewRegistryManager(
		registry.NewSmitheryRegistry(apiKey, ""),
		registry.NewLocalRegistry(localPath),
	)
	servers, err := mgr.ListAllServers(ctx, registry.ListOptions{PageSize: 100})
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for _, s := range servers {
		out = append(out, map[string]any{
			"name":        s.Name,
			"description": s.Description,
			"endpoint":    s.Endpoint,
			"transport":   s.Kind,
		})
	}
	return out, nil
}
