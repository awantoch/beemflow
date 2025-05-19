package main

import (
	"log"

	mcpserver "github.com/awantoch/beemflow/mcp"
	"github.com/spf13/cobra"
)

func newMCPServeCmd() *cobra.Command {
	var stdio bool
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve BeemFlow as an MCP server (HTTP or stdio)",
		Run: func(cmd *cobra.Command, args []string) {
			tools := buildMCPToolRegistrations()
			if err := mcpserver.Serve(configPath, debug, stdio, addr, tools); err != nil {
				log.Fatalf("MCP server failed: %v", err)
			}
		},
	}
	cmd.Flags().BoolVar(&stdio, "stdio", true, "serve over stdin/stdout instead of HTTP (default)")
	cmd.Flags().StringVar(&addr, "addr", ":9090", "listen address for HTTP mode")
	return cmd
}
