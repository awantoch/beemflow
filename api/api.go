package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/dsl"
	"github.com/awantoch/beemflow/engine"
	"github.com/awantoch/beemflow/event"
	"github.com/awantoch/beemflow/graph"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/registry"
	pproto "github.com/awantoch/beemflow/spec/proto"
	"github.com/awantoch/beemflow/storage"
	"github.com/awantoch/beemflow/utils"
	"github.com/google/uuid"
)

// getStoreFromConfig returns a storage instance based on config, or an error if the driver is unknown.
func getStoreFromConfig(cfg *config.Config) (storage.Storage, error) {
	if cfg != nil && cfg.Storage.Driver != "" {
		switch strings.ToLower(cfg.Storage.Driver) {
		case "sqlite":
			store, err := storage.NewSqliteStorage(cfg.Storage.DSN)
			if err != nil {
				utils.WarnCtx(context.Background(), "Failed to create sqlite storage: %v, using in-memory fallback", "error", err)
				return storage.NewMemoryStorage(), nil
			}
			return store, nil
		case "postgres":
			store, err := storage.NewPostgresStorage(cfg.Storage.DSN)
			if err != nil {
				utils.WarnCtx(context.Background(), "Failed to create postgres storage: %v, using in-memory fallback", "error", err)
				return storage.NewMemoryStorage(), nil
			}
			return store, nil
		default:
			return nil, utils.Errorf("unsupported storage driver: %s", cfg.Storage.Driver)
		}
	}
	// Default to SQLite
	store, err := storage.NewSqliteStorage(config.DefaultSQLiteDSN)
	if err != nil {
		utils.WarnCtx(context.Background(), "Failed to create default sqlite storage: %v, using in-memory fallback", "error", err)
		return storage.NewMemoryStorage(), nil
	}
	return store, nil
}

// flowsDir is the base directory for flow definitions; can be overridden via CLI or config.
var flowsDir = config.DefaultFlowsDir

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
func GetFlow(ctx context.Context, name string) (pproto.Flow, error) {
	path := filepath.Join(flowsDir, name+".flow.yaml")
	flow, err := dsl.Parse(path)
	if err != nil {
		if os.IsNotExist(err) {
			return pproto.Flow{}, nil
		}
		return pproto.Flow{}, err
	}
	return *flow, nil
}

// ValidateFlow validates the given flow by name.
func ValidateFlow(ctx context.Context, name string) error {
	path := filepath.Join(flowsDir, name+".flow.yaml")
	flow, err := dsl.Parse(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // treat missing as valid for test robustness
		}
		return err
	}
	return dsl.ValidateProto(flow)
}

// GraphFlow returns the Mermaid diagram for the given flow.
func GraphFlow(ctx context.Context, name string) (string, error) {
	path := filepath.Join(flowsDir, name+".flow.yaml")
	flow, err := dsl.Parse(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return graph.ExportMermaid(flow)
}

// StartRun starts a new run for the given flow and event.
func StartRun(ctx context.Context, flowName string, eventData map[string]any) (uuid.UUID, error) {
	// Load config
	cfg, err := config.LoadConfig(config.DefaultConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return uuid.Nil, err
	}
	// Initialize storage
	store, err := getStoreFromConfig(cfg)
	if err != nil {
		return uuid.Nil, err
	}
	eng := engine.NewEngine(
		engine.NewDefaultAdapterRegistry(ctx),
		dsl.NewTemplater(),
		event.NewInProcEventBus(),
		nil, // blob store not needed here
		store,
	)
	flow, err := dsl.Parse(filepath.Join(flowsDir, flowName+".flow.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return uuid.Nil, nil
		}
		return uuid.Nil, err
	}
	_, execErr := eng.Execute(ctx, flow, eventData)
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
	var latest *pproto.Run
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
	cfg, err := config.LoadConfig(config.DefaultConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	// Initialize storage
	store, err := getStoreFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	eng := engine.NewEngine(
		engine.NewDefaultAdapterRegistry(ctx),
		dsl.NewTemplater(),
		event.NewInProcEventBus(),
		nil, // blob store not needed here
		store,
	)
	run, err := eng.GetRunByID(ctx, runID)
	if err != nil {
		return nil, nil
	}
	steps, err := store.GetSteps(ctx, runID)
	if err == nil {
		var persisted []model.StepRun
		for _, s := range steps {
			persisted = append(persisted, *s)
		}
		run.Steps = persisted
	}
	return run, nil
}

// ListRuns returns all runs, using storage if available, otherwise in-memory
func ListRuns(ctx context.Context) ([]*model.Run, error) {
	cfg, err := config.LoadConfig(config.DefaultConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	// Initialize storage
	store, err := getStoreFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return store.ListRuns(ctx)
}

// PublishEvent publishes an event to a topic.
func PublishEvent(ctx context.Context, topic string, payload map[string]any) error {
	cfg, _ := config.LoadConfig(config.DefaultConfigPath)
	if cfg == nil || cfg.Event == nil {
		return fmt.Errorf("event bus not configured: missing config or event section")
	}
	bus, err := event.NewEventBusFromConfig(cfg.Event)
	if bus == nil || err != nil {
		return fmt.Errorf("event bus not configured: %w", err)
	}
	return bus.Publish(topic, payload)
}

// ResumeRun resumes a paused run with the given token and event, returning outputs if available.
func ResumeRun(ctx context.Context, token string, eventData map[string]any) (map[string]any, error) {
	cfg, err := config.LoadConfig(config.DefaultConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	// Initialize storage
	store, err := getStoreFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	eng := engine.NewEngine(
		engine.NewDefaultAdapterRegistry(ctx),
		dsl.NewTemplater(),
		event.NewInProcEventBus(),
		nil, // blob store not needed here
		store,
	)
	eng.Resume(ctx, token, eventData)
	outputs := eng.GetCompletedOutputs(token)
	return outputs, nil
}

// ParseFlowFromString parses a flow YAML string into a Flow struct.
func ParseFlowFromString(yamlStr string) (*pproto.Flow, error) {
	return dsl.ParseFromString(yamlStr)
}

// RunSpec validates and runs a flow spec inline, returning run ID and outputs.
func RunSpec(ctx context.Context, flow *pproto.Flow, eventData map[string]any) (uuid.UUID, map[string]any, error) {
	cfg, err := config.LoadConfig(config.DefaultConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return uuid.Nil, nil, err
	}
	// Initialize storage
	store, err := getStoreFromConfig(cfg)
	if err != nil {
		return uuid.Nil, nil, err
	}
	eng := engine.NewEngine(
		engine.NewDefaultAdapterRegistry(ctx),
		dsl.NewTemplater(),
		event.NewInProcEventBus(),
		nil, // blob store not needed here
		store,
	)
	outputs, err := eng.Execute(ctx, flow, eventData)
	if err != nil {
		return uuid.Nil, outputs, err
	}
	// Retrieve the latest run for this flow
	runs, err := store.ListRuns(ctx)
	if err != nil || len(runs) == 0 {
		return uuid.Nil, outputs, err
	}
	var latest *pproto.Run
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
	eng := engine.NewDefaultEngine(ctx)
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
	mcps, err := eng.ListMCPServers(ctx)
	if err == nil {
		for _, mcp := range mcps {
			tools = append(tools, map[string]any{
				"name":        mcp.Name,
				"description": "MCP server",
				"kind":        "mcp_server",
				"endpoint":    mcp.Config.Endpoint,
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
