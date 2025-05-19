package main

import (
	"github.com/awantoch/beemflow/logger"
	"github.com/spf13/cobra"
)

// newToolCmd creates the 'tool' subcommand and its scaffolding commands.
func newToolCmd() *cobra.Command {
	// Not implemented yet. Planned for a future release.
	cmd := &cobra.Command{
		Use:   "tool",
		Short: "Tooling commands",
		Run: func(cmd *cobra.Command, args []string) {
			logger.User("flow tool (stub)")
		},
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "scaffold",
			Short: "Scaffold a tool manifest",
			// Not implemented yet. Planned for a future release.
			Run: func(cmd *cobra.Command, args []string) {
				logger.User("flow tool scaffold (stub)")
			},
		},
	)
	return cmd
}
