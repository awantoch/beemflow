package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/registry"
	"github.com/spf13/cobra"
)

// newMCPCmd creates the 'mcp' subcommand and its subcommands.
func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
	}

	var configFile = &configPath

	cmd.AddCommand(
		newMCPServeCmd(),
		&cobra.Command{
			Use:   "search [query]",
			Short: "Search for MCP servers in the Smithery registry",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				query := ""
				if len(args) > 0 {
					query = args[0]
				}
				ctx := context.Background()
				apiKey := os.Getenv("SMITHERY_API_KEY")
				if apiKey == "" {
					return fmt.Errorf("environment variable SMITHERY_API_KEY must be set")
				}
				client := registry.NewSmitheryRegistry(apiKey, "")
				entries, err := client.ListServers(ctx, registry.ListOptions{Query: query, PageSize: 50})
				if err != nil {
					return err
				}
				w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
				fmt.Fprintln(w, "NAME\tDESCRIPTION\tENDPOINT")
				for _, s := range entries {
					fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.Description, s.Endpoint)
				}
				w.Flush()
				return nil
			},
		},
		&cobra.Command{
			Use:   "install <serverName>",
			Short: "Install an MCP server from the Smithery registry",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				qn := args[0]
				cfg, err := config.LoadAndInjectRegistries(*configFile)
				if err != nil {
					return err
				}
				ctx := context.Background()
				apiKey := os.Getenv("SMITHERY_API_KEY")
				if apiKey == "" {
					return fmt.Errorf("environment variable SMITHERY_API_KEY must be set")
				}
				client := registry.NewSmitheryRegistry(apiKey, "")
				spec, err := client.GetServerSpec(ctx, qn)
				if err != nil {
					return err
				}
				config.UpsertMCPServer(cfg, qn, spec)
				if err := config.SaveConfig(*configFile, cfg); err != nil {
					return err
				}
				fmt.Fprintf(os.Stdout, "Installed MCP server %s to %s (mcpServers)\n", qn, *configFile)
				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all MCP servers",
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := config.LoadAndInjectRegistries(*configFile)
				if err != nil {
					return err
				}
				localPath := ""
				for _, reg := range cfg.Registries {
					if reg.Type == "local" && reg.Path != "" {
						localPath = reg.Path
					}
				}
				localMgr := registry.NewLocalRegistry(localPath)
				ctx := context.Background()
				servers, err := localMgr.ListMCPServers(ctx, registry.ListOptions{PageSize: 100})
				if err != nil {
					return err
				}
				w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
				fmt.Fprintln(w, "REGISTRY\tNAME\tDESCRIPTION\tKIND\tENDPOINT")
				for _, s := range servers {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.Registry, s.Name, s.Description, s.Kind, s.Endpoint)
				}
				w.Flush()
				return nil
			},
		},
	)
	return cmd
}
