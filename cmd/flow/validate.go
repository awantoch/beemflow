package main

import (
	"github.com/awantoch/beemflow/logger"
	"github.com/awantoch/beemflow/parser"
	"github.com/spf13/cobra"
)

// newValidateCmd creates the 'validate' subcommand.
func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [file]",
		Short: "Validate a flow file (YAML parse + schema validate)",
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
			logger.User("Validation OK: flow is valid!")
		},
	}
}
