package main

import (
	"github.com/awantoch/beemflow/logger"
	"github.com/spf13/cobra"
)

// newTestCmd creates the 'test' subcommand.
func newTestCmd() *cobra.Command {
	// Not implemented yet. Planned for a future release.
	return &cobra.Command{
		Use:   "test",
		Short: "Test a flow file",
		Run: func(cmd *cobra.Command, args []string) {
			logger.User("flow test (not yet implemented)")
		},
	}
}
