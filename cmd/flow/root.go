package main

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	exit       = os.Exit
	configPath string
	debug      bool
)

// NewRootCmd creates the root 'flow' command with persistent flags and subcommands.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{Use: "flow"}
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "runtime.config.json", "Path to runtime config JSON")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logs")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
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
