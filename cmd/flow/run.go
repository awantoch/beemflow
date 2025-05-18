package main

import (
	"encoding/json"
	"os"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/engine"
	"github.com/awantoch/beemflow/mcp"
	"github.com/awantoch/beemflow/parser"
	"github.com/awantoch/beemflow/pkg/logger"
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
				logger.User("flow run (stub)")
				return
			}
			// Real execution when a file is provided
			file := args[0]
			flow, err := parser.ParseFlow(file)
			if err != nil {
				logger.Error("YAML parse error: %v", err)
				exit(1)
			}
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				if os.IsNotExist(err) {
					logger.Warn("config file %s not found, using defaults", configPath)
					cfg = &config.Config{}
				} else {
					logger.Error("Failed to load config: %v", err)
					exit(2)
				}
			}
			if debug {
				cfgJSON, _ := json.MarshalIndent(cfg.MCPServers, "", "  ")
				logger.Debug("Loaded MCPServers config:\n%s\n", cfgJSON)
			}
			if err := mcp.EnsureMCPServersWithTimeout(flow, cfg, mcpStartupTimeout); err != nil {
				logger.Error("Failed to ensure MCP servers: %v", err)
				exit(3)
			}
			event, err := loadEvent(eventPath, eventJSON)
			if err != nil {
				logger.Error("Failed to load event: %v", err)
				exit(4)
			}
			eng := engine.NewEngine()
			defer eng.Close()
			outputs, err := eng.Execute(cmd.Context(), flow, event)
			if err != nil {
				logger.Error("Flow execution error: %v", err)
				exit(5)
			}

			if debug {
				// Print all outputs as JSON for debugging
				outJSONBytes, _ := json.MarshalIndent(outputs, "", "  ")
				logger.User("%s", string(outJSONBytes))
				logger.Info("Flow executed successfully.")
				logger.Info("Step outputs:\n%s\n", string(outJSONBytes))
			} else {
				// Only print the output of core.echo steps (by convention, steps with id 'print' or use 'core.echo')
				for _, stepOutput := range outputs {
					if outMap, ok := stepOutput.(map[string]any); ok {
						if text, ok := outMap["text"]; ok {
							logger.Info("%s", text)
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
