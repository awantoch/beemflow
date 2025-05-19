package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/awantoch/beemflow/registry"
	"github.com/spf13/cobra"
)

// newMCPCmd creates the 'mcp' subcommand and its subcommands.
func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
	}

	// Registry manager setup: support both Smithery and local registry
	apiKey := os.Getenv("SMITHERY_API_KEY")
	localPath := os.Getenv("BEEMFLOW_REGISTRY")
	mgr := registry.NewRegistryManager(
		registry.NewSmitheryRegistry(apiKey, ""),
		registry.NewLocalRegistry(localPath),
	)

	cmd.AddCommand(
		newMCPServeCmd(),
		&cobra.Command{
			Use:   "search [query]",
			Short: "Search for MCP servers in all registries",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				query := ""
				if len(args) > 0 {
					query = args[0]
				}
				servers, err := mgr.ListAllServers(ctx, registry.ListOptions{Query: query, PageSize: 50})
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
		&cobra.Command{
			Use:   "install <serverName>",
			Short: "Install an MCP server from any registry (use registry:name for qualified lookup)",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				input := args[0]
				var regName, name string
				if strings.Contains(input, ":") {
					parts := strings.SplitN(input, ":", 2)
					regName, name = parts[0], parts[1]
				} else {
					name = input
				}
				servers, err := mgr.ListAllServers(ctx, registry.ListOptions{})
				if err != nil {
					return err
				}
				var matches []registry.RegistryEntry
				for _, s := range servers {
					if regName != "" {
						if s.Registry == regName && s.Name == name {
							matches = append(matches, s)
						}
					} else if s.Name == name {
						matches = append(matches, s)
					}
				}
				if len(matches) == 0 {
					return fmt.Errorf("server %s not found in any registry", input)
				}
				if len(matches) > 1 && regName == "" {
					fmt.Fprintf(os.Stderr, "Ambiguous server name '%s'. Please specify one of:\n", name)
					for _, m := range matches {
						fmt.Fprintf(os.Stderr, "  %s:%s\n", m.Registry, m.Name)
					}
					return nil
				}
				entry := matches[0]
				fmt.Fprintf(os.Stdout, "Found server: %s:%s\nEndpoint: %s\nKind: %s\n", entry.Registry, entry.Name, entry.Endpoint, entry.Kind)
				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all MCP servers",
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				servers, err := mgr.ListAllServers(ctx, registry.ListOptions{PageSize: 100})
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
