package mcp

import (
	"context"
	"io"
	"testing"

	mcp "github.com/metoro-io/mcp-golang"
	mcpstdio "github.com/metoro-io/mcp-golang/transport/stdio"
)

// startTestServer launches an in-memory stdio MCP server with the given tool registrations and returns a client.
func startTestServer(t *testing.T, regs []ToolRegistration) *mcp.Client {
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	server := mcp.NewServer(mcpstdio.NewStdioServerTransportWithIO(serverReader, serverWriter))
	RegisterAllTools(server, regs)
	go func() {
		if err := server.Serve(); err != nil {
			t.Errorf("MCP server Serve failed: %v", err)
		}
	}()
	client := mcp.NewClient(mcpstdio.NewStdioServerTransportWithIO(clientReader, clientWriter))
	ctx := context.Background()
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize MCP client: %v", err)
	}
	return client
}

// TestListTools ensures that registered tools are returned by ListTools.
func TestListTools(t *testing.T) {
	regs := []ToolRegistration{
		{
			Name:        "foo",
			Description: "foo tool",
			Handler: func(ctx context.Context, args EmptyArgs) (*mcp.ToolResponse, error) {
				return mcp.NewToolResponse(mcp.NewTextContent("foo")), nil
			},
		},
		{
			Name:        "bar",
			Description: "bar tool",
			Handler: func(ctx context.Context, args EmptyArgs) (*mcp.ToolResponse, error) {
				return mcp.NewToolResponse(mcp.NewTextContent("bar")), nil
			},
		},
	}
	client := startTestServer(t, regs)
	resp, err := client.ListTools(context.Background(), new(string))
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(resp.Tools) != len(regs) {
		t.Fatalf("Expected %d tools, got %d", len(regs), len(resp.Tools))
	}
}

// TestCallTool verifies that CallTool invokes the correct handler and returns content.
func TestCallTool(t *testing.T) {
	regs := []ToolRegistration{
		{
			Name:        "hello",
			Description: "hello tool",
			Handler: func(ctx context.Context, args EmptyArgs) (*mcp.ToolResponse, error) {
				return mcp.NewToolResponse(mcp.NewTextContent("world")), nil
			},
		},
	}
	client := startTestServer(t, regs)
	resp, err := client.CallTool(context.Background(), "hello", EmptyArgs{})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if len(resp.Content) == 0 {
		t.Fatalf("Expected non-empty Content, got none")
	}
}
