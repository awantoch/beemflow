package mcp

import (
	"context"
	"io"
	"testing"

	mcp "github.com/metoro-io/mcp-golang"
	mcpstdio "github.com/metoro-io/mcp-golang/transport/stdio"
)

// Test argument types - kept local to avoid import cycles
type EmptyArgs struct{}

type GetFlowArgs struct {
	Name string `json:"name"`
}

type ValidateFlowArgs struct {
	Name string `json:"name"`
}

type StartRunArgs struct {
	FlowName string         `json:"flowName"`
	Event    map[string]any `json:"event"`
}

// TestServe_Basic tests that Serve can be called without panicking
func TestServe_Basic(t *testing.T) {
	// Create a simple server setup
	serverReader, clientWriter := io.Pipe()
	_, serverWriter := io.Pipe()
	server := mcp.NewServer(mcpstdio.NewStdioServerTransportWithIO(serverReader, serverWriter))

	// Close the client writer to simulate client disconnection
	clientWriter.Close()

	// Serve should handle the closed connection gracefully
	err := server.Serve()
	// We expect an error due to the closed connection, but it shouldn't panic
	if err == nil {
		t.Log("Serve completed without error (unexpected but not a failure)")
	} else {
		t.Logf("Serve completed with expected error: %v", err)
	}
}

func TestRegisterAllTools_Basic(t *testing.T) {
	serverReader, _ := io.Pipe()
	_, serverWriter := io.Pipe()
	server := mcp.NewServer(mcpstdio.NewStdioServerTransportWithIO(serverReader, serverWriter))

	// Test with empty tool list
	var emptyRegs []ToolRegistration
	RegisterAllTools(server, emptyRegs)
	// Should not panic

	// Test with single valid tool
	singleReg := []ToolRegistration{
		{
			Name:        "test_tool",
			Description: "A test tool",
			Handler: func(ctx context.Context, args EmptyArgs) (*mcp.ToolResponse, error) {
				return mcp.NewToolResponse(mcp.NewTextContent("test")), nil
			},
		},
	}
	RegisterAllTools(server, singleReg)
	// Should not panic

	// Test with multiple tools
	multiRegs := []ToolRegistration{
		{
			Name:        "tool1",
			Description: "First tool",
			Handler: func(ctx context.Context, args GetFlowArgs) (*mcp.ToolResponse, error) {
				return mcp.NewToolResponse(mcp.NewTextContent("tool1")), nil
			},
		},
		{
			Name:        "tool2",
			Description: "Second tool",
			Handler: func(ctx context.Context, args GetFlowArgs) (*mcp.ToolResponse, error) {
				return mcp.NewToolResponse(mcp.NewTextContent("tool2: " + args.Name)), nil
			},
		},
	}
	RegisterAllTools(server, multiRegs)
	// Should not panic
}

func TestRegisterAllTools_DifferentArgTypes(t *testing.T) {
	serverReader, _ := io.Pipe()
	_, serverWriter := io.Pipe()
	server := mcp.NewServer(mcpstdio.NewStdioServerTransportWithIO(serverReader, serverWriter))

	// Test with different argument types to ensure they all register
	regs := []ToolRegistration{
		{
			Name:        "empty_args",
			Description: "Tool with empty args",
			Handler: func(ctx context.Context, args EmptyArgs) (*mcp.ToolResponse, error) {
				return mcp.NewToolResponse(mcp.NewTextContent("empty")), nil
			},
		},
		{
			Name:        "flow_args",
			Description: "Tool with flow args",
			Handler: func(ctx context.Context, args GetFlowArgs) (*mcp.ToolResponse, error) {
				return mcp.NewToolResponse(mcp.NewTextContent("flow")), nil
			},
		},
		{
			Name:        "run_args",
			Description: "Tool with run args",
			Handler: func(ctx context.Context, args StartRunArgs) (*mcp.ToolResponse, error) {
				return mcp.NewToolResponse(mcp.NewTextContent("run")), nil
			},
		},
		{
			Name:        "validate_args",
			Description: "Tool with validate args",
			Handler: func(ctx context.Context, args ValidateFlowArgs) (*mcp.ToolResponse, error) {
				return mcp.NewToolResponse(mcp.NewTextContent("validate")), nil
			},
		},
	}

	// This should register all tools without panicking
	RegisterAllTools(server, regs)

	// If we get here without panicking, the test passes
	t.Log("Successfully registered tools with different argument types")
}

func TestRegisterAllTools_EdgeCases(t *testing.T) {
	serverReader, _ := io.Pipe()
	_, serverWriter := io.Pipe()
	server := mcp.NewServer(mcpstdio.NewStdioServerTransportWithIO(serverReader, serverWriter))

	// Test with tools that have various edge case configurations
	edgeCaseRegs := []ToolRegistration{
		{
			Name:        "minimal",
			Description: "", // Empty description
			Handler: func(ctx context.Context, args EmptyArgs) (*mcp.ToolResponse, error) {
				return mcp.NewToolResponse(mcp.NewTextContent("minimal")), nil
			},
		},
		{
			Name:        "long_name_tool_with_underscores_and_numbers_123",
			Description: "Tool with a very long name to test name handling",
			Handler: func(ctx context.Context, args EmptyArgs) (*mcp.ToolResponse, error) {
				return mcp.NewToolResponse(mcp.NewTextContent("long_name")), nil
			},
		},
		{
			Name:        "special_chars",
			Description: "Tool with special characters: !@#$%^&*()",
			Handler: func(ctx context.Context, args EmptyArgs) (*mcp.ToolResponse, error) {
				return mcp.NewToolResponse(mcp.NewTextContent("special")), nil
			},
		},
	}

	// This should register all tools without issues
	RegisterAllTools(server, edgeCaseRegs)

	t.Log("Successfully registered tools with edge case configurations")
}
