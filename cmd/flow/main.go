package main

import (
	"fmt"
	"os"

	"github.com/awantoch/beemflow/internal/config"
	"github.com/awantoch/beemflow/internal/mcp"
	"github.com/awantoch/beemflow/internal/parser"
	"github.com/spf13/cobra"
)

var (
	exit       = os.Exit
	configPath string
)

func main() {
	rootCmd := &cobra.Command{Use: "flow"}
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "runtime.config.json", "Path to runtime config JSON")

	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "serve",
			Short: "Start the BeemFlow runtime",
			Run:   func(cmd *cobra.Command, args []string) { fmt.Println("flow serve (stub)") },
		},
		&cobra.Command{
			Use:   "run [file]",
			Short: "Run a flow",
			Args:  cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
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
				// MCP server auto-discovery/installation
				if err := mcp.EnsureMCPServers(flow, cfg); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to ensure MCP servers: %v\n", err)
					exit(3)
				}
				fmt.Println("[beemflow] All required MCP servers are running. (Flow execution stub)")
			},
		},
		&cobra.Command{
			Use:   "lint [file]",
			Short: "Lint a flow file (YAML parse + schema validate)",
			Args:  cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				file := args[0]
				flow, err := parser.ParseFlow(file)
				if err != nil {
					fmt.Fprintf(os.Stderr, "YAML parse error: %v\n", err)
					exit(1)
				}
				schemaPath := "../../beemflow.schema.json"
				err = parser.ValidateFlow(flow, schemaPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Schema validation error: %v\n", err)
					exit(2)
				}
				fmt.Println("Lint OK: flow is valid!")
			},
		},
		&cobra.Command{
			Use:   "graph",
			Short: "Visualize a flow as a DAG",
			Run:   func(cmd *cobra.Command, args []string) { fmt.Println("flow graph (stub)") },
		},
		&cobra.Command{
			Use:   "validate [file]",
			Short: "Validate a flow file (YAML parse + schema validate)",
			Args:  cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				file := args[0]
				flow, err := parser.ParseFlow(file)
				if err != nil {
					fmt.Fprintf(os.Stderr, "YAML parse error: %v\n", err)
					exit(1)
				}
				schemaPath := "../../beemflow.schema.json"
				err = parser.ValidateFlow(flow, schemaPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Schema validation error: %v\n", err)
					exit(2)
				}
				fmt.Println("Validation OK: flow is valid!")
			},
		},
		&cobra.Command{
			Use:   "test",
			Short: "Test a flow file",
			Run:   func(cmd *cobra.Command, args []string) { fmt.Println("flow test (stub)") },
		},
	)

	toolCmd := &cobra.Command{
		Use:   "tool",
		Short: "Tooling commands",
		Run:   func(cmd *cobra.Command, args []string) { fmt.Println("flow tool (stub)") },
	}
	toolCmd.AddCommand(&cobra.Command{
		Use:   "scaffold",
		Short: "Scaffold a tool manifest",
		Run:   func(cmd *cobra.Command, args []string) { fmt.Println("flow tool scaffold (stub)") },
	})
	rootCmd.AddCommand(toolCmd)

	if err := rootCmd.Execute(); err != nil {
		exit(1)
	}
}
