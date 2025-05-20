package main

import (
	"encoding/json"
	"fmt"

	"github.com/awantoch/beemflow/registry"
	"github.com/spf13/cobra"
)

// newMetadataCmd creates the 'metadata' subcommand to list interface metadata.
func newMetadataCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "metadata",
		Short: "List all registered CLI, HTTP, and MCP interfaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			metas := registry.AllInterfaces()
			b, err := json.MarshalIndent(metas, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(b))
			return nil
		},
	}
}
