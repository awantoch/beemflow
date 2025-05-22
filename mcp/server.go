package mcp

import (
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/utils"
	mcp "github.com/metoro-io/mcp-golang"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"
	mcpstdio "github.com/metoro-io/mcp-golang/transport/stdio"
)

// ToolRegistration holds a tool's registration info for the MCP server.
type ToolRegistration struct {
	Name        string
	Description string
	Handler     any // must be a func(ctx, args) (*mcp.ToolResponse, error)
}

// Serve starts the BeemFlow MCP server with the given configuration and tool registrations.
func Serve(configPath string, debug bool, stdio bool, addr string, tools []ToolRegistration) error {
	// If using stdio transport and debug is disabled, silence user-facing logs on stdout; keep internal logs on stderr
	if stdio && !debug {
		utils.SetUserOutput(io.Discard)
	}

	// Load runtime config
	_, err := config.LoadConfig(configPath)
	if err != nil && !strings.Contains(err.Error(), "no such file") {
		return utils.Errorf("failed to load config %s: %w", configPath, err)
	}
	// Create MCP server transport
	var server *mcp.Server
	if stdio {
		utils.Info("Starting MCP server on stdio...")
		transport := mcpstdio.NewStdioServerTransport()
		server = mcp.NewServer(transport)
	} else {
		utils.Info("Starting MCP server on HTTP at %s...", addr)
		transport := mcphttp.NewHTTPTransport("/mcp").WithAddr(addr)
		server = mcp.NewServer(transport)
	}
	// Register all tools
	RegisterAllTools(server, tools)
	// Start serving
	if err := server.Serve(); err != nil {
		return err
	}
	// For stdio transport, wait for termination signals and exit gracefully
	if stdio {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		sig := <-sigCh
		utils.Info("Received signal %v, shutting down MCP stdio server", sig)
	}
	return nil
}

// RegisterAllTools registers all provided tools with the MCP server.
// This function is generic and does not import any business logic.
func RegisterAllTools(server *mcp.Server, tools []ToolRegistration) {
	for _, t := range tools {
		_ = server.RegisterTool(t.Name, t.Description, t.Handler)
	}
}

// Argument types for MCP handlers (can be re-used by CLI)

type EmptyArgs struct{}

type GetFlowArgs struct {
	Name string `json:"name"`
}

type ValidateFlowArgs struct {
	Name string `json:"name"`
}

type GraphFlowArgs struct {
	Name string `json:"name"`
}

type StartRunArgs struct {
	FlowName string         `json:"flowName"`
	Event    map[string]any `json:"event"`
}

type GetRunArgs struct {
	RunID string `json:"runID"`
}

type PublishEventArgs struct {
	Topic   string         `json:"topic"`
	Payload map[string]any `json:"payload"`
}

// ResumeRunArgs are the arguments for the resumeRun MCP tool.
type ResumeRunArgs struct {
	Token string         `json:"token"`
	Event map[string]any `json:"event"`
}
