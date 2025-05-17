package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newGraphCmd creates the 'graph' subcommand.
func newGraphCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "graph",
		Short: "Visualize a flow as a DAG",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("flow graph (stub)")
		},
	}
}
