package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/engine"
	"github.com/awantoch/beemflow/mcp"
	"github.com/awantoch/beemflow/parser"
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
				fmt.Fprintf(os.Stderr, "YAML parse error: %v\n", err)
				exit(1)
			}
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "[beemflow] config file %s not found, using defaults\n", configPath)
					cfg = &config.Config{}
				} else {
					fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
					exit(2)
				}
			}
			if debug {
				cfgJSON, _ := json.MarshalIndent(cfg.MCPServers, "", "  ")
				fmt.Fprintf(os.Stderr, "[beemflow] Loaded MCPServers config:\n%s\n", cfgJSON)
			}
			if err := mcp.EnsureMCPServersWithTimeout(flow, cfg, mcpStartupTimeout); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to ensure MCP servers: %v\n", err)
				exit(3)
			}
			event, err := loadEvent(eventPath, eventJSON)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load event: %v\n", err)
				exit(4)
			}
			eng := engine.NewEngine()
			outputs, err := eng.Execute(cmd.Context(), flow, event)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Flow execution error: %v\n", err)
				exit(5)
			}
			// Print outputs as JSON to stdout for scripting
			outJSONBytes, _ := json.Marshal(outputs)
			fmt.Println(string(outJSONBytes))
			if debug {
				fmt.Fprintln(os.Stderr, "[beemflow] Flow executed successfully.")
				outJSON, _ := json.MarshalIndent(outputs, "", "  ")
				fmt.Fprintf(os.Stderr, "[beemflow] Step outputs:\n%s\n", outJSON)
			}
		},
	}
	cmd.Flags().StringVar(&eventPath, "event", "", "Path to event JSON file")
	cmd.Flags().StringVar(&eventJSON, "event-json", "", "Event as inline JSON string")
	return cmd
}
