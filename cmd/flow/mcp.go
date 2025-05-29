package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/awantoch/beemflow/api"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/constants"
	mcpserver "github.com/awantoch/beemflow/mcp"
	"github.com/awantoch/beemflow/registry"
	"github.com/awantoch/beemflow/utils"
	"github.com/spf13/cobra"
)

// newMCPCmd creates the 'mcp' subcommand and its subcommands.
func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   constants.CmdMCP,
		Short: constants.DescMCPCommands,
	}

	var configFile = &configPath

	cmd.AddCommand(
		newMCPServeCmd(),
		newMCPSearchCmd(),
		newMCPInstallCmd(configFile),
		newMCPListCmd(configFile),
	)
	return cmd
}

// newMCPSearchCmd creates the search subcommand for MCP servers
func newMCPSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   constants.CmdSearch + " [query]",
		Short: constants.DescSearchServers,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runMCPSearch,
	}
}

// runMCPSearch handles the search functionality for MCP servers
func runMCPSearch(cmd *cobra.Command, args []string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	ctx := context.Background()
	apiKey := os.Getenv(constants.EnvSmitheryKey)
	if apiKey == "" {
		return fmt.Errorf(constants.ErrEnvVarRequired, constants.EnvSmitheryKey)
	}

	client := registry.NewSmitheryRegistry(apiKey, "")
	entries, err := client.ListServers(ctx, registry.ListOptions{
		Query:    query,
		PageSize: constants.DefaultMCPPageSize,
	})
	if err != nil {
		return err
	}

	utils.User(constants.HeaderServers)
	for _, s := range entries {
		utils.User(constants.OutputFormatThree, s.Name, s.Description, s.Endpoint)
	}
	return nil
}

// newMCPInstallCmd creates the install subcommand for MCP servers
func newMCPInstallCmd(configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   constants.CmdInstall + " <serverName>",
		Short: constants.DescInstallServer,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPInstall(args[0], *configFile)
		},
	}
}

// runMCPInstall handles the installation of MCP servers
func runMCPInstall(serverName, configFile string) error {
	// Read existing config as raw JSON (preserve only user overrides)
	doc, err := loadConfigAsMap(configFile)
	if err != nil {
		return err
	}

	// Ensure mcpServers map exists
	mcpMap := ensureMCPServersMap(doc)

	// Fetch spec from Smithery
	spec, err := fetchServerSpec(serverName)
	if err != nil {
		return err
	}

	// Update configuration
	mcpMap[serverName] = spec
	doc[constants.MCPServersKey] = mcpMap

	// Write updated config
	if err := writeConfigMap(doc, configFile); err != nil {
		return err
	}

	utils.User(constants.MsgServerInstalled, serverName, configFile)
	return nil
}

// loadConfigAsMap loads configuration file as a generic map
func loadConfigAsMap(configFile string) (map[string]any, error) {
	var doc map[string]any
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf(constants.ErrConfigParseFailed, configFile, err)
	}
	return doc, nil
}

// ensureMCPServersMap ensures the mcpServers map exists in the config
func ensureMCPServersMap(doc map[string]any) map[string]any {
	mcpMap, ok := doc[constants.MCPServersKey].(map[string]any)
	if !ok {
		mcpMap = map[string]any{}
	}
	return mcpMap
}

// fetchServerSpec fetches server specification from Smithery registry
func fetchServerSpec(serverName string) (any, error) {
	ctx := context.Background()
	apiKey := os.Getenv(constants.EnvSmitheryKey)
	if apiKey == "" {
		return nil, fmt.Errorf(constants.ErrEnvVarRequired, constants.EnvSmitheryKey)
	}

	client := registry.NewSmitheryRegistry(apiKey, "")
	return client.GetServerSpec(ctx, serverName)
}

// writeConfigMap writes the configuration map to file
func writeConfigMap(doc map[string]any, configFile string) error {
	out, err := json.MarshalIndent(doc, "", constants.JSONIndent)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := os.WriteFile(configFile, out, constants.FilePermission); err != nil {
		return fmt.Errorf(constants.ErrConfigWriteFailed, configFile, err)
	}
	return nil
}

// newMCPListCmd creates the list subcommand for MCP servers
func newMCPListCmd(configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   constants.CmdList,
		Short: constants.DescListServers,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPList(*configFile)
		},
	}
}

// runMCPList handles listing all MCP servers
func runMCPList(configFile string) error {
	// Load config to get installed MCP servers
	cfg, err := config.LoadConfig(configFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	ctx := context.Background()
	utils.User(constants.HeaderMCPList)

	// List servers from config
	if cfg != nil && cfg.MCPServers != nil {
		for name, spec := range cfg.MCPServers {
			utils.User(constants.OutputFormatFive, "config", name, "", spec.Transport, spec.Endpoint)
		}
	}

	// List servers from local registry
	localMgr := registry.NewLocalRegistry("")
	servers, err := localMgr.ListMCPServers(ctx, registry.ListOptions{
		PageSize: constants.DefaultToolPageSize,
	})
	if err == nil {
		for _, s := range servers {
			utils.User(constants.OutputFormatFive, s.Registry, s.Name, s.Description, s.Kind, s.Endpoint)
		}
	}
	return nil
}

// newMCPServeCmd creates the serve subcommand for MCP
func newMCPServeCmd() *cobra.Command {
	var stdio bool
	var addr string
	cmd := &cobra.Command{
		Use:   constants.CmdServe,
		Short: constants.DescMCPServe,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := api.NewFlowService()
			tools := api.BuildMCPToolRegistrations(svc)
			return mcpserver.Serve(configPath, debug, stdio, addr, tools)
		},
	}
	cmd.Flags().BoolVar(&stdio, "stdio", true, "serve over stdin/stdout instead of HTTP (default)")
	cmd.Flags().StringVar(&addr, "addr", constants.DefaultMCPAddr, "listen address for HTTP mode")
	return cmd
}
