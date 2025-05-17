package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newServeCmd creates the 'serve' subcommand.
func newServeCmd() *cobra.Command {
	// Not implemented yet. Planned for a future release.
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the BeemFlow runtime",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("flow serve (stub)")
		},
	}
}
