package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/awantoch/beemflow/blob"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/engine"
	"github.com/awantoch/beemflow/event"
	"github.com/awantoch/beemflow/storage"
	mcp "github.com/metoro-io/mcp-golang"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"
	mcpstdio "github.com/metoro-io/mcp-golang/transport/stdio"
	"github.com/spf13/cobra"
)

func newMCPServeCmd() *cobra.Command {
	var stdio bool
	var addr string
	cmd := &cobra.Command{
		Use:   "mcp serve",
		Short: "Serve BeemFlow as an MCP server (HTTP or stdio)",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("[DEBUG] Entered MCP serve Run function")
			// Load runtime config
			cfg, err := config.LoadConfig(configPath)
			if err != nil && !strings.Contains(err.Error(), "no such file") {
				log.Fatalf("failed to load config %s: %v", configPath, err)
			}
			// Setup storage
			var store storage.Storage
			if cfg != nil && cfg.Storage.Driver != "" {
				switch strings.ToLower(cfg.Storage.Driver) {
				case "sqlite":
					store, err = storage.NewSqliteStorage(cfg.Storage.DSN)
				default:
					log.Fatalf("unsupported storage driver: %s", cfg.Storage.Driver)
				}
				if err != nil {
					log.Fatalf("failed to init storage: %v", err)
				}
			} else {
				store = storage.NewMemoryStorage()
			}
			// Setup blob store
			var bs blob.BlobStore
			if cfg != nil && cfg.Blob.Driver != "" {
				// Map config.BlobConfig to blob.BlobConfig
				bc := &blob.BlobConfig{
					Driver:    cfg.Blob.Driver,
					Bucket:    cfg.Blob.Bucket,
					Directory: "",
					Region:    "",
				}
				bs, err = blob.NewDefaultBlobStore(bc)
				if err != nil {
					log.Fatalf("failed to init blob store: %v", err)
				}
			} else {
				bs, _ = blob.NewDefaultBlobStore(nil)
			}
			// Setup event bus (in-process only)
			bus := event.NewInProcEventBus()
			fmt.Println("[DEBUG] Creating engine with config...")
			eng := engine.NewEngineWithStorage(store)
			eng.BlobStore = bs
			eng.EventBus = bus
			fmt.Println("[DEBUG] Engine created with storage, blob, and event bus")
			var server *mcp.Server
			if stdio {
				fmt.Println("[MCP] Starting MCP server on stdio...")
				transport := mcpstdio.NewStdioServerTransport()
				server = mcp.NewServer(transport)
			} else {
				fmt.Printf("[MCP] Starting MCP server on HTTP at %s...\n", addr)
				transport := mcphttp.NewHTTPTransport("/mcp")
				transport.WithAddr(addr)
				server = mcp.NewServer(transport)
			}
			fmt.Println("[DEBUG] Registering all MCP tools...")
			registerAllMCPTools(server, eng)
			fmt.Println("[DEBUG] All MCP tools registered. Calling server.Serve()...")
			if err := server.Serve(); err != nil {
				log.Fatalf("MCP server failed: %v", err)
			}
		},
	}
	cmd.Flags().BoolVar(&stdio, "stdio", true, "serve over stdin/stdout instead of HTTP (default)")
	cmd.Flags().StringVar(&addr, "addr", ":9090", "listen address for HTTP mode")
	return cmd
}
