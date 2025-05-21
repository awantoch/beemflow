package main

import (
	"context"

	"github.com/awantoch/beemflow/api"
	"github.com/awantoch/beemflow/utils/logger"
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
				logger.User("flow tool (stub)")
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all available tools",
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				tools, err := api.ListTools(ctx)
				if err != nil {
					return err
				}
				// Header
				logger.User("NAME\tKIND\tDESCRIPTION\tENDPOINT")
				for _, t := range tools {
					name, _ := t["name"].(string)
					kind, _ := t["kind"].(string)
					desc, _ := t["description"].(string)
					endpoint, _ := t["endpoint"].(string)
					logger.User("%s\t%s\t%s\t%s", name, kind, desc, endpoint)
				}
				return nil
			},
		},
	)
	return cmd
}
