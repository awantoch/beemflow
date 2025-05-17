package main

import (
	"fmt"

	"github.com/awantoch/beemflow/parser"
	"github.com/awantoch/beemflow/pkg/logger"
	"github.com/spf13/cobra"
)

// newLintCmd creates the 'lint' subcommand.
func newLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint [file]",
		Short: "Lint a flow file (YAML parse + schema validate)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			file := args[0]
			flow, err := parser.ParseFlow(file)
			if err != nil {
				logger.Error("YAML parse error: %v\n", err)
				exit(1)
			}
			err = parser.ValidateFlow(flow, "../../beemflow.schema.json")
			if err != nil {
				logger.Error("Schema validation error: %v\n", err)
				exit(2)
			}
			fmt.Println("Lint OK: flow is valid!")
		},
	}
}
