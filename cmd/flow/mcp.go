package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/logger"
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
				logger.User("NAME\tDESCRIPTION\tENDPOINT")
				for _, s := range entries {
					logger.User("%s\t%s\t%s", s.Name, s.Description, s.Endpoint)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "install <serverName>",
			Short: "Install an MCP server from the Smithery registry",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				qn := args[0]
				// Read existing config as raw JSON (preserve only user overrides)
				var doc map[string]any
				data, err := os.ReadFile(*configFile)
				if err != nil {
					if os.IsNotExist(err) {
						doc = map[string]any{}
					} else {
						return err
					}
				} else {
					if err := json.Unmarshal(data, &doc); err != nil {
						return fmt.Errorf("failed to parse %s: %w", *configFile, err)
					}
				}
				// Ensure mcpServers map exists
				mcpMap, ok := doc["mcpServers"].(map[string]any)
				if !ok {
					mcpMap = map[string]any{}
				}
				// Fetch spec from Smithery
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
				// Patch mcpServers
				mcpMap[qn] = spec
				doc["mcpServers"] = mcpMap
				// Write updated config
				out, err := json.MarshalIndent(doc, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to serialize config: %w", err)
				}
				if err := os.WriteFile(*configFile, out, 0644); err != nil {
					return fmt.Errorf("failed to write %s: %w", *configFile, err)
				}
				logger.User("Installed MCP server %s to %s (mcpServers)", qn, *configFile)
				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all MCP servers",
			RunE: func(cmd *cobra.Command, args []string) error {
				// Load config to get installed MCP servers
				cfg, err := config.LoadConfig(*configFile)
				if err != nil && !os.IsNotExist(err) {
					return err
				}
				ctx := context.Background()
				logger.User("REGISTRY\tNAME\tDESCRIPTION\tKIND\tENDPOINT")
				if cfg != nil && cfg.MCPServers != nil {
					for name, spec := range cfg.MCPServers {
						logger.User("config\t%s\t%s\t%s\t%s", name, "", spec.Transport, spec.Endpoint)
					}
				}
				localMgr := registry.NewLocalRegistry("")
				servers, err := localMgr.ListMCPServers(ctx, registry.ListOptions{PageSize: 100})
				if err == nil {
					for _, s := range servers {
						logger.User("%s\t%s\t%s\t%s\t%s", s.Registry, s.Name, s.Description, s.Kind, s.Endpoint)
					}
				}
				return nil
			},
		},
	)
	return cmd
}
