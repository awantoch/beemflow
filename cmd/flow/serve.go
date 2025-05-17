package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newServeCmd creates the 'serve' subcommand.
func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the BeemFlow runtime",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("flow serve (stub)")
		},
	}
}
