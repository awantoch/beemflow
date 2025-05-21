package main

import (
	"github.com/awantoch/beemflow/dsl"
	"github.com/awantoch/beemflow/utils/logger"
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
			flow, err := dsl.Parse(file)
			if err != nil {
				logger.Error("YAML parse error: %v\n", err)
				exit(1)
			}
			err = dsl.Validate(flow)
			if err != nil {
				logger.Error("Schema validation error: %v\n", err)
				exit(2)
			}
			logger.User("Lint OK: flow is valid!")
		},
	}
}
