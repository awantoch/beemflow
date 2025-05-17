package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newToolCmd creates the 'tool' subcommand and its scaffolding commands.
func newToolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool",
		Short: "Tooling commands",
		Run:   func(cmd *cobra.Command, args []string) { fmt.Println("flow tool (stub)") },
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "scaffold",
			Short: "Scaffold a tool manifest",
			Run:   func(cmd *cobra.Command, args []string) { fmt.Println("flow tool scaffold (stub)") },
		},
	)
	return cmd
}
