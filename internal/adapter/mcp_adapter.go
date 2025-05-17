package adapter

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/awantoch/beemflow/internal/mcp"
)

// MCPAdapter implements Adapter for mcp://server/tool URIs.
type MCPAdapter struct {
	clients map[string]mcp.MCPClient
}

// NewMCPAdapter creates a new MCPAdapter with an empty client cache.
func NewMCPAdapter() *MCPAdapter {
	return &MCPAdapter{clients: make(map[string]mcp.MCPClient)}
}

// ID returns the adapter ID.
func (a *MCPAdapter) ID() string {
	return "mcp"
}

var mcpRe = regexp.MustCompile(`^mcp://([^/]+)/([\w.-]+)$`)

// Execute calls a tool on the specified MCP server.
func (a *MCPAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	use, ok := inputs["__use"].(string)
	if !ok {
		return nil, fmt.Errorf("missing __use for MCPAdapter")
	}
	matches := mcpRe.FindStringSubmatch(use)
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid mcp:// identifier: %s", use)
	}
	host, tool := matches[1], matches[2]
	var baseURL string
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
		baseURL = "http://" + host
	} else {
		baseURL = "https://" + host
	}
	client, ok := a.clients[host]
	if !ok {
		client = mcp.NewHTTPMCPClient(baseURL)
		a.clients[host] = client
	}
	tools, err := client.ListTools()
	if err != nil {
		return nil, fmt.Errorf("MCP ListTools failed: %w", err)
	}
	found := false
	for _, t := range tools {
		if t.Name == tool {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("tool %s not found on MCP server %s", tool, host)
	}
	args := make(map[string]any, len(inputs))
	for k, v := range inputs {
		if k != "__use" {
			args[k] = v
		}
	}
	return client.CallTool(tool, args)
}
