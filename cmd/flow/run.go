package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/constants"
	"github.com/awantoch/beemflow/dsl"
	"github.com/awantoch/beemflow/engine"
	"github.com/awantoch/beemflow/mcp"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/storage"
	"github.com/awantoch/beemflow/utils"
	"github.com/spf13/cobra"
)

// newRunCmd creates the 'run' subcommand.
func newRunCmd() *cobra.Command {
	var eventPath, eventJSON string
	cmd := &cobra.Command{
		Use:   constants.CmdRun + " [file]",
		Short: constants.DescRunFlow,
		Args:  cobra.RangeArgs(0, 1),
		Run: func(cmd *cobra.Command, args []string) {
			runFlowExecution(cmd, args, eventPath, eventJSON)
		},
	}
	cmd.Flags().StringVar(&eventPath, "event", "", "Path to event JSON file")
	cmd.Flags().StringVar(&eventJSON, "event-json", "", "Event as inline JSON string")
	return cmd
}

// runFlowExecution handles the main flow execution logic
func runFlowExecution(cmd *cobra.Command, args []string, eventPath, eventJSON string) {
	// Handle stub behavior when no file argument is provided
	if len(args) == 0 {
		utils.User(constants.StubFlowRun)
		return
	}

	// Parse the flow file
	flow, err := dsl.Parse(args[0])
	if err != nil {
		utils.Error("YAML parse error: %v", err)
		exit(1)
	}

	// Load configuration
	cfg, err := loadFlowConfig()
	if err != nil {
		utils.Error("Failed to load config: %v", err)
		exit(2)
	}

	// Debug config output
	if debug {
		debugMCPConfig(cfg)
	}

	// Ensure MCP servers are available
	if err := mcp.EnsureMCPServersWithTimeout(cmd.Context(), flow, cfg, mcpStartupTimeout); err != nil {
		utils.Error("Failed to ensure MCP servers: %v", err)
		exit(3)
	}

	// Load event data
	event, err := loadEvent(eventPath, eventJSON)
	if err != nil {
		utils.Error("Failed to load event: %v", err)
		exit(4)
	}

	// Initialize storage
	store, err := initializeStorage(cfg)
	if err != nil {
		exit(6) // Error already logged in initializeStorage
	}

	// Execute the flow
	outputs, err := executeFlow(cmd, flow, event, store)
	if err != nil {
		utils.Error(constants.ErrFlowExecutionFailed, err)
		exit(5)
	}

	// Output results
	outputFlowResults(outputs)
}

// loadFlowConfig loads the flow configuration with proper error handling
func loadFlowConfig() (*config.Config, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			utils.Warn("config file %s not found, using defaults", configPath)
			return &config.Config{}, nil
		}
		return nil, err
	}
	return cfg, nil
}

// debugMCPConfig outputs MCP configuration for debugging
func debugMCPConfig(cfg *config.Config) {
	cfgJSON, _ := json.MarshalIndent(cfg.MCPServers, "", constants.JSONIndent)
	utils.Debug("Loaded MCPServers config:\n%s\n", cfgJSON)
}

// initializeStorage sets up the storage backend based on configuration
func initializeStorage(cfg *config.Config) (storage.Storage, error) {
	if cfg.Storage.Driver != "" {
		return createConfiguredStorage(cfg)
	}
	return createDefaultStorage()
}

// createConfiguredStorage creates storage based on explicit configuration
func createConfiguredStorage(cfg *config.Config) (storage.Storage, error) {
	switch strings.ToLower(cfg.Storage.Driver) {
	case constants.StorageDriverSQLite:
		store, err := storage.NewSqliteStorage(cfg.Storage.DSN)
		if err != nil {
			utils.Error(constants.ErrStorageCreateFailed, err)
			return nil, err
		}
		return store, nil
	case constants.StorageDriverPostgres:
		store, err := storage.NewPostgresStorage(cfg.Storage.DSN)
		if err != nil {
			utils.Error(constants.ErrStorageCreateFailed, err)
			return nil, err
		}
		return store, nil
	default:
		utils.Error(constants.ErrStorageUnsupported, cfg.Storage.Driver)
		return nil, nil
	}
}

// createDefaultStorage creates default SQLite storage with fallback to memory
func createDefaultStorage() (storage.Storage, error) {
	sqliteStore, err := storage.NewSqliteStorage(config.DefaultSQLiteDSN)
	if err != nil {
		utils.Warn("Failed to create default sqlite storage: %v, using in-memory fallback", err)
		return storage.NewMemoryStorage(), nil
	}
	return sqliteStore, nil
}

// executeFlow runs the flow with the provided parameters
func executeFlow(cmd *cobra.Command, flow any, event any, store storage.Storage) (map[string]any, error) {
	// Type assert the flow to the correct type
	flowTyped, ok := flow.(*model.Flow)
	if !ok {
		return nil, fmt.Errorf("invalid flow type")
	}

	// Type assert the event to the correct type
	eventTyped, ok := event.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid event type")
	}

	eng := engine.NewDefaultEngine(cmd.Context())
	defer eng.Close()
	eng.Storage = store
	return eng.Execute(cmd.Context(), flowTyped, eventTyped)
}

// outputFlowResults handles the output of flow execution results
func outputFlowResults(outputs map[string]any) {
	if debug {
		outputDebugResults(outputs)
	} else {
		outputEchoResults(outputs)
	}
}

// outputDebugResults outputs all results as JSON for debugging
func outputDebugResults(outputs map[string]any) {
	outJSONBytes, _ := json.MarshalIndent(outputs, "", constants.JSONIndent)
	utils.User("%s", string(outJSONBytes))
	utils.Info(constants.MsgFlowExecuted)
	utils.Info(constants.MsgStepOutputs, string(outJSONBytes))
}

// outputEchoResults outputs only echo step results for normal operation
func outputEchoResults(outputs map[string]any) {
	// Only print the output of core.echo steps (by convention, steps with id 'print' or use 'core.echo')
	for _, stepOutput := range outputs {
		if outMap, ok := stepOutput.(map[string]any); ok {
			if text, ok := outMap["text"]; ok {
				utils.Info("%s", text)
			}
		}
	}
}
