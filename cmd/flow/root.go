package main

import (
	"os"
	"time"

	// Load environment variables from .env file
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var (
	exit              = os.Exit
	configPath        string
	debug             bool
	mcpStartupTimeout time.Duration
)

// NewRootCmd creates the root 'flow' command with persistent flags and subcommands.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{Use: "flow"}
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "flow.config.json", "Path to flow config JSON")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logs")
	rootCmd.PersistentFlags().DurationVar(&mcpStartupTimeout, "mcp-timeout", 60*time.Second, "Timeout for MCP server startup")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// Load environment variables from .env file, if present
		_ = godotenv.Load()
		if debug {
			os.Setenv("BEEMFLOW_DEBUG", "1")
		}
	}
	rootCmd.AddCommand(
		newServeCmd(),
		newRunCmd(),
		newLintCmd(),
		newValidateCmd(),
		newGraphCmd(),
		newTestCmd(),
		newToolCmd(),
	)
	return rootCmd
}
