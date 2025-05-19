package main

import (
	"context"
	"os"
	"text/tabwriter"

	"github.com/awantoch/beemflow/api"
	"github.com/awantoch/beemflow/logger"
	"github.com/spf13/cobra"
)

// newMCPCmd creates the 'mcp' subcommand and its subcommands.
func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List all MCP servers",
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				mcps, err := api.ListMCPServers(ctx)
				if err != nil {
					return err
				}
				w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
				defer w.Flush()
				logger.User("NAME\tDESCRIPTION\tENDPOINT\tTRANSPORT")
				for _, m := range mcps {
					name, _ := m["name"].(string)
					desc, _ := m["description"].(string)
					endpoint, _ := m["endpoint"].(string)
					transport, _ := m["transport"].(string)
					logger.User("%s\t%s\t%s\t%s", name, desc, endpoint, transport)
				}
				return nil
			},
		},
	)
	return cmd
}
