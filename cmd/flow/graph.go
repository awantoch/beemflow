package main

import (
	"os"

	"github.com/awantoch/beemflow/graphviz"
	"github.com/awantoch/beemflow/parser"
	"github.com/awantoch/beemflow/pkg/logger"
	"github.com/spf13/cobra"
)

// newGraphCmd creates the 'graph' subcommand.
func newGraphCmd() *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:   "graph [flow_file]",
		Short: "Visualize a flow as a DAG (Mermaid syntax)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			file := args[0]
			flow, err := parser.ParseFlow(file)
			if err != nil {
				logger.Error("YAML parse error: %v\n", err)
				os.Exit(1)
			}
			diagram, err := graphviz.ExportMermaid(flow)
			if err != nil {
				logger.Error("Graph export error: %v\n", err)
				os.Exit(2)
			}
			if outPath != "" {
				if err := os.WriteFile(outPath, []byte(diagram), 0644); err != nil {
					logger.Error("Failed to write graph to %s: %v\n", outPath, err)
					os.Exit(3)
				}
			} else {
				logger.Info("%s", diagram)
			}
		},
	}
	cmd.Flags().StringVarP(&outPath, "output", "o", "", "Path to write graph output (defaults to stdout)")
	return cmd
}
