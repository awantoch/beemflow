package main

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/dsl"
	"github.com/awantoch/beemflow/engine"
	"github.com/awantoch/beemflow/mcp"
	"github.com/awantoch/beemflow/storage"
	"github.com/awantoch/beemflow/utils"
	"github.com/spf13/cobra"
)

// newRunCmd creates the 'run' subcommand.
func newRunCmd() *cobra.Command {
	var eventPath, eventJSON string
	cmd := &cobra.Command{
		Use:   "run [file]",
		Short: "Run a flow",
		Args:  cobra.RangeArgs(0, 1),
		Run: func(cmd *cobra.Command, args []string) {
			// Stub behavior when no file argument is provided
			if len(args) == 0 {
				utils.User("flow run (stub)")
				return
			}
			// Real execution when a file is provided
			file := args[0]
			flow, err := dsl.Parse(file)
			if err != nil {
				utils.Error("YAML parse error: %v", err)
				exit(1)
			}
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				if os.IsNotExist(err) {
					utils.Warn("config file %s not found, using defaults", configPath)
					cfg = &config.Config{}
				} else {
					utils.Error("Failed to load config: %v", err)
					exit(2)
				}
			}
			if debug {
				cfgJSON, _ := json.MarshalIndent(cfg.MCPServers, "", "  ")
				utils.Debug("Loaded MCPServers config:\n%s\n", cfgJSON)
			}
			if err := mcp.EnsureMCPServersWithTimeout(cmd.Context(), flow, cfg, mcpStartupTimeout); err != nil {
				utils.Error("Failed to ensure MCP servers: %v", err)
				exit(3)
			}
			event, err := loadEvent(eventPath, eventJSON)
			if err != nil {
				utils.Error("Failed to load event: %v", err)
				exit(4)
			}
			// Determine storage based on config or default to SQLite
			var store storage.Storage
			if cfg.Storage.Driver != "" {
				switch strings.ToLower(cfg.Storage.Driver) {
				case "sqlite":
					store, err = storage.NewSqliteStorage(cfg.Storage.DSN)
				case "postgres":
					store, err = storage.NewPostgresStorage(cfg.Storage.DSN)
				default:
					utils.Error("unsupported storage driver: %s", cfg.Storage.Driver)
					exit(6)
				}
				if err != nil {
					utils.Error("Failed to create storage: %v", err)
					exit(7)
				}
			} else {
				// Default to SQLite
				sqliteStore, err := storage.NewSqliteStorage(config.DefaultSQLiteDSN)
				if err != nil {
					utils.Warn("Failed to create default sqlite storage: %v, using in-memory fallback", err)
					store = storage.NewMemoryStorage()
				} else {
					store = sqliteStore
				}
			}
			eng := engine.NewDefaultEngine(cmd.Context())
			defer eng.Close()
			eng.Storage = store
			outputs, err := eng.Execute(cmd.Context(), flow, event)
			if err != nil {
				utils.Error("Flow execution error: %v", err)
				exit(5)
			}

			if debug {
				// Print all outputs as JSON for debugging
				outJSONBytes, _ := json.MarshalIndent(outputs, "", "  ")
				utils.User("%s", string(outJSONBytes))
				utils.Info("Flow executed successfully.")
				utils.Info("Step outputs:\n%s\n", string(outJSONBytes))
			} else {
				// Only print the output of core.echo steps (by convention, steps with id 'print' or use 'core.echo')
				for _, stepOutput := range outputs {
					if outMap, ok := stepOutput.(map[string]any); ok {
						if text, ok := outMap["text"]; ok {
							utils.Info("%s", text)
						}
					}
				}
			}
		},
	}
	cmd.Flags().StringVar(&eventPath, "event", "", "Path to event JSON file")
	cmd.Flags().StringVar(&eventJSON, "event-json", "", "Event as inline JSON string")
	return cmd
}
