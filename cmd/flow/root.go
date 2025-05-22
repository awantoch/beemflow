package main

import (
	"os"
	"time"

	// Load environment variables from .env file
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	_ "github.com/awantoch/beemflow/adapter"
	"github.com/awantoch/beemflow/api"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/registry"
)

var (
	exit              = os.Exit
	configPath        string
	debug             bool
	mcpStartupTimeout time.Duration
	flowsDir          string
)

// NewRootCmd creates the root 'flow' command with persistent flags and subcommands.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{Use: "flow"}
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", config.DefaultConfigPath, "Path to flow config JSON")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logs")
	rootCmd.PersistentFlags().DurationVar(&mcpStartupTimeout, "mcp-timeout", 60*time.Second, "Timeout for MCP server startup")
	rootCmd.PersistentFlags().StringVar(&flowsDir, "flows-dir", "", "Path to flows directory (overrides config file)")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// Load environment variables from .env file, if present
		_ = godotenv.Load()

		// Load config JSON to pick up default flowsDir
		cfg, err := config.LoadConfig(configPath)
		if err == nil && cfg.FlowsDir != "" {
			api.SetFlowsDir(cfg.FlowsDir)
		}

		// CLI flag overrides config file
		if flowsDir != "" {
			api.SetFlowsDir(flowsDir)
		}
	}

	// Create the service
	svc := api.NewFlowService()

	// Create command constructors
	constructors := api.CommandConstructors{
		NewServeCmd:    newServeCmd,
		NewRunCmd:      newRunCmd,
		NewLintCmd:     newLintCmd,
		NewValidateCmd: newValidateCmd,
		NewGraphCmd:    newGraphCmd,
		NewTestCmd:     newTestCmd,
		NewToolCmd:     newToolCmd,
		NewMCPCmd:      newMCPCmd,
		NewMetadataCmd: newMetadataCmd,
		NewSpecCmd:     newSpecCmd,
	}

	// Attach all CLI commands
	api.AttachCLICommands(rootCmd, svc, constructors)

	return rootCmd
}

// collectCobra recursively collects metadata for Cobra commands.
func collectCobra(cmd *cobra.Command) []registry.InterfaceMeta {
	metas := []registry.InterfaceMeta{{
		ID:          cmd.CommandPath(),
		Type:        registry.CLI,
		Use:         cmd.Use,
		Description: cmd.Short,
	}}
	for _, sub := range cmd.Commands() {
		metas = append(metas, collectCobra(sub)...)
	}
	return metas
}
