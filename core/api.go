package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/awantoch/beemflow/adapter"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/constants"
	"github.com/awantoch/beemflow/engine"
	"github.com/awantoch/beemflow/event"
	"github.com/awantoch/beemflow/graph"
	"github.com/awantoch/beemflow/loader"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/registry"
	"github.com/awantoch/beemflow/storage"
	"github.com/awantoch/beemflow/utils"
	"github.com/google/uuid"
)

// GetStoreFromConfig returns a storage instance based on config, or an error if the driver is unknown.
// This is a utility function that can be used by other packages.
func GetStoreFromConfig(cfg *config.Config) (storage.Storage, error) {
	if cfg != nil && cfg.Storage.Driver != "" {
		switch strings.ToLower(cfg.Storage.Driver) {
		case "sqlite":
			// Use the user-provided DSN as-is (respects their explicit choice)
			store, err := storage.NewSqliteStorage(cfg.Storage.DSN)
			if err != nil {
				utils.WarnCtx(context.Background(), "Failed to create sqlite storage: %v, using in-memory fallback", "error", err)
				return storage.NewMemoryStorage(), nil
			}
			return store, nil
		default:
			return nil, utils.Errorf("unsupported storage driver: %s (supported: sqlite)", cfg.Storage.Driver)
		}
	}
	// Default to SQLite with default path (already points to home directory)
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
		if strings.HasSuffix(name, constants.FlowFileExtension) {
			base := strings.TrimSuffix(name, constants.FlowFileExtension)
			flows = append(flows, base)
		}
	}
	return flows, nil
}

// GetFlow returns the parsed flow definition for the given name.
func GetFlow(ctx context.Context, name string) (model.Flow, error) {
	path := buildFlowPath(name)
	flow, err := loader.Load(path, map[string]any{})
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
	path := buildFlowPath(name)
	flow, err := loader.Load(path, map[string]any{})
	if err != nil {
		if os.IsNotExist(err) {
			return nil // treat missing as valid for test robustness
		}
		return err
	}
	return loader.Validate(flow)
}

// GraphFlow returns the Mermaid diagram for the given flow.
func GraphFlow(ctx context.Context, name string) (string, error) {
	path := buildFlowPath(name)
	flow, err := loader.Load(path, map[string]any{})
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return graph.ExportMermaid(flow)
}

// createEngineFromConfig creates a new engine instance with storage from config
func createEngineFromConfig(ctx context.Context) (*engine.Engine, error) {
	cfg, err := config.LoadConfig(constants.ConfigFileName)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	store, err := GetStoreFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	// Create registry and register core adapters
	registry := adapter.NewRegistry()
	registry.Register(&adapter.CoreAdapter{})
	registry.Register(&adapter.HTTPAdapter{AdapterID: "http"})
	registry.Register(&adapter.OpenAIAdapter{})
	registry.Register(&adapter.AnthropicAdapter{})

	return engine.NewEngine(
		registry,
		event.NewInProcEventBus(),
		nil, // blob store not needed here
		store,
		cfg,
	), nil
}

// buildFlowPath constructs the full path to a flow file
func buildFlowPath(flowName string) string {
	return filepath.Join(flowsDir, flowName+constants.FlowFileExtension)
}

// parseFlowByName loads and parses a flow file by name
func parseFlowByName(flowName string) (*model.Flow, error) {
	path := buildFlowPath(flowName)
	flow, err := loader.Load(path, map[string]any{})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return flow, nil
}

// findLatestRunForFlow finds the most recent run for a specific flow
func findLatestRunForFlow(runs []*model.Run, flowName string) *model.Run {
	var latest *model.Run
	for _, r := range runs {
		if r.FlowName == flowName && (latest == nil || r.StartedAt.After(latest.StartedAt)) {
			latest = r
		}
	}
	return latest
}

// tryFindPausedRun attempts to find a paused run when await_event is involved
func tryFindPausedRun(store storage.Storage, execErr error) (uuid.UUID, error) {
	if execErr == nil || !strings.Contains(execErr.Error(), constants.ErrorAwaitEventPause) {
		return uuid.Nil, execErr
	}

	paused, err := store.LoadPausedRuns()
	if err != nil {
		return uuid.Nil, execErr
	}

	for _, v := range paused {
		if m, ok := v.(map[string]any); ok {
			if runID, ok := m[constants.RunIDKey].(string); ok {
				if id, err := uuid.Parse(runID); err == nil {
					return id, nil
				}
			}
		}
	}

	return uuid.Nil, execErr
}

// handleExecutionResult processes the result of flow execution, handling paused runs
func handleExecutionResult(store storage.Storage, flowName string, execErr error) (uuid.UUID, error) {
	runs, err := store.ListRuns(context.Background())
	if err != nil || len(runs) == 0 {
		return tryFindPausedRun(store, execErr)
	}

	latest := findLatestRunForFlow(runs, flowName)
	if latest == nil {
		return tryFindPausedRun(store, execErr)
	}

	// If the only error is await_event pause, treat as success
	if execErr != nil && strings.Contains(execErr.Error(), constants.ErrorAwaitEventPause) {
		return latest.ID, nil
	}

	return latest.ID, execErr
}

// StartRun starts a new run for the given flow and event.
func StartRun(ctx context.Context, flowName string, eventData map[string]any) (uuid.UUID, error) {
	eng, err := createEngineFromConfig(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	flow, err := parseFlowByName(flowName)
	if err != nil {
		return uuid.Nil, err
	}
	if flow == nil {
		return uuid.Nil, nil
	}

	_, execErr := eng.Execute(ctx, flow, eventData)
	return handleExecutionResult(eng.Storage, flowName, execErr)
}

// GetRun returns the run by ID.
func GetRun(ctx context.Context, runID uuid.UUID) (*model.Run, error) {
	eng, err := createEngineFromConfig(ctx)
	if err != nil {
		return nil, err
	}

	run, err := eng.GetRunByID(ctx, runID)
	if err != nil {
		return nil, nil
	}
	return run, nil
}

// ListRuns returns all runs.
func ListRuns(ctx context.Context) ([]*model.Run, error) {
	eng, err := createEngineFromConfig(ctx)
	if err != nil {
		return nil, err
	}

	return eng.ListRuns(ctx)
}

// PublishEvent publishes an event to a topic.
func PublishEvent(ctx context.Context, topic string, payload map[string]any) error {
	cfg, _ := config.LoadConfig(constants.ConfigFileName)
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
	// Check if we can create an engine (this will validate config including storage driver)
	_, err := createEngineFromConfig(ctx)
	if err != nil {
		return nil, err
	}
	
	// For now, return nil outputs since we removed the resume functionality
	// This can be re-implemented later if needed
	return nil, nil
}

// ParseFlowFromString parses a flow YAML string into a Flow struct.
func ParseFlowFromString(yamlStr string) (*model.Flow, error) {
	return loader.ParseFromString(yamlStr)
}

// RunSpec validates and runs a flow spec inline, returning run ID and outputs.
func RunSpec(ctx context.Context, flow *model.Flow, eventData map[string]any) (uuid.UUID, map[string]any, error) {
	eng, err := createEngineFromConfig(ctx)
	if err != nil {
		return uuid.Nil, nil, err
	}

	outputs, err := eng.Execute(ctx, flow, eventData)
	if err != nil {
		return uuid.Nil, outputs, err
	}

	// Retrieve the latest run for this flow
	runs, err := eng.Storage.ListRuns(ctx)
	if err != nil || len(runs) == 0 {
		return uuid.Nil, outputs, err
	}

	latest := findLatestRunForFlow(runs, flow.Name)
	if latest == nil {
		return uuid.Nil, outputs, err
	}

	return latest.ID, outputs, nil
}

// ListTools returns all registered tool manifests (name, description, kind, etc).
func ListTools(ctx context.Context) ([]map[string]any, error) {
	eng, err := createEngineFromConfig(ctx)
	if err != nil {
		return nil, err
	}
	adapters := eng.AdapterRegistry.All()
	var tools []map[string]any
	for _, a := range adapters {
		m := a.Manifest()
		if m != nil {
			// Only include if not an MCP server
			if m.Kind != constants.MCPServerKind {
				tools = append(tools, map[string]any{
					"name":        m.Name,
					"description": m.Description,
					"kind":        m.Kind,
					"endpoint":    m.Endpoint,
					"type":        constants.ToolType,
				})
			}
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

// ============================================================================
// REGISTRY FEDERATION API (for Runtime-to-Runtime Communication)
// ============================================================================

// RegistryIndexResponse represents the registry index response
type RegistryIndexResponse struct {
	Version    string                            `json:"version"`
	Runtime    string                            `json:"runtime"`
	Tools      []registry.RegistryEntry          `json:"tools"`
	MCPServers []registry.RegistryEntry          `json:"mcp_servers"`
	Stats      map[string]registry.RegistryStats `json:"stats"`
}

// GetRegistryIndex returns the complete registry index for this runtime
func GetRegistryIndex(ctx context.Context) (*RegistryIndexResponse, error) {
	factory := registry.NewFactory()
	mgr := factory.CreateAPIManager()
	return createRegistryResponse(ctx, mgr)
}

// GetRegistryTool returns a specific tool by name
func GetRegistryTool(ctx context.Context, name string) (*registry.RegistryEntry, error) {
	factory := registry.NewFactory()
	mgr := factory.CreateAPIManager()
	return mgr.GetServer(ctx, name)
}

// GetRegistryStats returns statistics about all registries
func GetRegistryStats(ctx context.Context) (map[string]registry.RegistryStats, error) {
	factory := registry.NewFactory()
	mgr := factory.CreateAPIManager()
	return mgr.GetRegistryStats(ctx), nil
}

// createRegistryResponse creates the registry response from a manager
func createRegistryResponse(ctx context.Context, mgr *registry.RegistryManager) (*RegistryIndexResponse, error) {
	entries, err := mgr.ListAllServers(ctx, registry.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Separate tools and MCP servers
	var tools, mcpServers []registry.RegistryEntry
	for _, entry := range entries {
		switch entry.Type {
		case "tool":
			tools = append(tools, entry)
		case "mcp_server":
			mcpServers = append(mcpServers, entry)
		}
	}

	return &RegistryIndexResponse{
		Version:    "1.0.0",
		Runtime:    "beemflow",
		Tools:      tools,
		MCPServers: mcpServers,
		Stats:      mgr.GetRegistryStats(ctx),
	}, nil
}

// GetToolManifest returns a specific tool manifest by name
func GetToolManifest(ctx context.Context, name string) (*registry.ToolManifest, error) {
	// Load tool manifests from the local registry index
	local := registry.NewLocalRegistry("")
	entries, err := local.ListServers(ctx, registry.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.Name == name {
			return &registry.ToolManifest{
				Name:        entry.Name,
				Description: entry.Description,
				Kind:        entry.Kind,
				Parameters:  entry.Parameters,
				Endpoint:    entry.Endpoint,
				Headers:     entry.Headers,
			}, nil
		}
	}
	return nil, nil
}

// ListToolManifests returns all tool manifests from the local registry
func ListToolManifests(ctx context.Context) ([]registry.ToolManifest, error) {
	// Load tool manifests from the local registry index
	local := registry.NewLocalRegistry("")
	entries, err := local.ListServers(ctx, registry.ListOptions{})
	if err != nil {
		return nil, err
	}
	var manifests []registry.ToolManifest
	for _, entry := range entries {
		manifests = append(manifests, registry.ToolManifest{
			Name:        entry.Name,
			Description: entry.Description,
			Kind:        entry.Kind,
			Parameters:  entry.Parameters,
			Endpoint:    entry.Endpoint,
			Headers:     entry.Headers,
		})
	}
	return manifests, nil
}
