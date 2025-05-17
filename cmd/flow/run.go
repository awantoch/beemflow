package main

import (
	"encoding/json"
	"fmt"
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
			if debug {
				os.Setenv("BEEMFLOW_DEBUG", "1")
			}
			// Stub behavior when no file argument is provided
			if len(args) == 0 {
				fmt.Println("flow run (stub)")
				return
			}
			// Real execution when a file is provided
			file := args[0]
			flow, err := parser.ParseFlow(file)
			if err != nil {
				logger.Logger.Printf("YAML parse error: %v\n", err)
				exit(1)
			}
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				if os.IsNotExist(err) {
					logger.Logger.Printf("config file %s not found, using defaults\n", configPath)
					cfg = &config.Config{}
				} else {
					logger.Logger.Printf("Failed to load config: %v\n", err)
					exit(2)
				}
			}
			if debug {
				cfgJSON, _ := json.MarshalIndent(cfg.MCPServers, "", "  ")
				logger.Logger.Printf("Loaded MCPServers config:\n%s\n", cfgJSON)
			}
			if err := mcp.EnsureMCPServersWithTimeout(flow, cfg, mcpStartupTimeout); err != nil {
				logger.Logger.Printf("Failed to ensure MCP servers: %v\n", err)
				exit(3)
			}
			event, err := loadEvent(eventPath, eventJSON)
			if err != nil {
				logger.Logger.Printf("Failed to load event: %v\n", err)
				exit(4)
			}
			eng := engine.NewEngine()
			defer eng.Close()
			outputs, err := eng.Execute(cmd.Context(), flow, event)
			if err != nil {
				logger.Logger.Printf("Flow execution error: %v\n", err)
				exit(5)
			}

			if debug {
				// Print all outputs as JSON for debugging
				outJSONBytes, _ := json.MarshalIndent(outputs, "", "  ")
				fmt.Println(string(outJSONBytes))
				logger.Logger.Println("Flow executed successfully.")
				logger.Logger.Printf("Step outputs:\n%s\n", string(outJSONBytes))
			} else {
				// Only print the output of core.echo steps (by convention, steps with id 'print' or use 'core.echo')
				for _, stepOutput := range outputs {
					if outMap, ok := stepOutput.(map[string]any); ok {
						if text, ok := outMap["text"]; ok {
							fmt.Println(text)
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
