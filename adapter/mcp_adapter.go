package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sync"

	"github.com/awantoch/beemflow/config"
	mcpmanager "github.com/awantoch/beemflow/mcp"
	"github.com/awantoch/beemflow/registry"
	mcp "github.com/metoro-io/mcp-golang"
	mcpstdio "github.com/metoro-io/mcp-golang/transport/stdio"
)

// MCPAdapter implements Adapter for mcp://server/tool URIs.
type MCPAdapter struct {
	clients map[string]*mcp.Client
	// For stdio support:
	processes map[string]*exec.Cmd // MCP server processes per host
	pipes     map[string]struct {
		stdin  io.WriteCloser
		stdout io.ReadCloser
	}
	mu     sync.Mutex // protects clients, processes, pipes
	closed bool       // tracks if Close has been called
}

// NewMCPAdapter creates a new MCPAdapter with an empty client cache.
func NewMCPAdapter() *MCPAdapter {
	return &MCPAdapter{
		clients:   make(map[string]*mcp.Client),
		processes: make(map[string]*exec.Cmd),
		pipes: make(map[string]struct {
			stdin  io.WriteCloser
			stdout io.ReadCloser
		}),
	}
}

// ID returns the adapter ID.
func (a *MCPAdapter) ID() string {
	return "mcp"
}

var mcpRe = regexp.MustCompile(`^mcp://([^/]+)/([\w.-]+)$`)

// Helper to resolve MCP server config from environment/config file
func getMCPServerConfig(host string) (config.MCPServerConfig, error) {
	cfgPath := os.Getenv("BEEMFLOW_CONFIG")
	if cfgPath == "" {
		cfgPath = "flow.config.json"
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		return config.MCPServerConfig{}, err
	}
	return config.GetMergedMCPServerConfig(cfg, host)
}

// Execute calls a tool on the specified MCP server.
// Supports both HTTP and stdio transports. For stdio, starts the process if needed and connects via pipes.
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

	a.mu.Lock()
	client, exists := a.clients[host]
	a.mu.Unlock()
	if !exists {
		cfg, err := getMCPServerConfig(host)
		if err != nil {
			return nil, err
		}
		// HTTP transport fallback if configured
		if cfg.Transport == "http" && cfg.Endpoint != "" {
			// Minimal HTTP JSON-RPC fallback (tools/list and tools/call)
			// List tools
			listReq := map[string]any{"method": "tools/list", "params": []any{}, "id": 1}
			bodyBytes, _ := json.Marshal(listReq)
			resp, err := http.Post(cfg.Endpoint, "application/json", bytes.NewReader(bodyBytes))
			if err != nil {
				return nil, fmt.Errorf("failed to list tools: %w", err)
			}
			defer resp.Body.Close()
			var listResp struct{ Tools []struct{ Name string } }
			if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
				return nil, fmt.Errorf("failed to decode tools list: %w", err)
			}
			found := false
			for _, t := range listResp.Tools {
				if t.Name == tool {
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("tool %s not found on MCP server %s", tool, host)
			}
			// Call tool
			callReq := map[string]any{"method": "tools/call", "params": map[string]any{"name": tool, "arguments": inputs}, "id": 1}
			callBytes, _ := json.Marshal(callReq)
			resp2, err := http.Post(cfg.Endpoint, "application/json", bytes.NewReader(callBytes))
			if err != nil {
				return nil, fmt.Errorf("MCP CallTool failed: %w", err)
			}
			defer resp2.Body.Close()
			var callResp struct{ Result map[string]any }
			if err := json.NewDecoder(resp2.Body).Decode(&callResp); err != nil {
				return nil, fmt.Errorf("failed to decode call response: %w", err)
			}
			return callResp.Result, nil
		} else if cfg.Command != "" {
			// stdio transport
			a.mu.Lock()
			cmd := a.processes[host]
			a.mu.Unlock()
			if cmd == nil {
				// use centralized command builder to merge env
				cmd = mcpmanager.NewMCPCommand(cfg)
				stdin, err := cmd.StdinPipe()
				if err != nil {
					return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
				}
				stdout, err := cmd.StdoutPipe()
				if err != nil {
					return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
				}
				cmd.Stderr = os.Stderr
				if err := cmd.Start(); err != nil {
					return nil, fmt.Errorf("failed to start MCP server %s: %w", host, err)
				}
				a.mu.Lock()
				a.processes[host] = cmd
				a.pipes[host] = struct {
					stdin  io.WriteCloser
					stdout io.ReadCloser
				}{stdin, stdout}
				a.mu.Unlock()
			}
			a.mu.Lock()
			pipes := a.pipes[host]
			a.mu.Unlock()
			transport := mcpstdio.NewStdioServerTransportWithIO(pipes.stdout, pipes.stdin)
			client = mcp.NewClient(transport)
			if _, err := client.Initialize(ctx); err != nil {
				return nil, fmt.Errorf("failed to initialize MCP stdio client: %w", err)
			}
			a.mu.Lock()
			a.clients[host] = client
			a.mu.Unlock()
		} else {
			return nil, fmt.Errorf("MCP server '%s' config is missing 'command' or 'http' transport config", host)
		}
	}
	// List tools to check if the tool exists
	toolsResp, err := client.ListTools(ctx, new(string))
	if err != nil {
		return nil, fmt.Errorf("MCP ListTools failed: %w", err)
	}
	found := false
	for _, t := range toolsResp.Tools {
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
	resp, err := client.CallTool(ctx, tool, args)
	if err != nil {
		return nil, fmt.Errorf("MCP CallTool failed: %w", err)
	}
	// Flatten the response content if possible
	if resp != nil && len(resp.Content) > 0 && resp.Content[0].TextContent != nil {
		return map[string]any{"text": resp.Content[0].TextContent.Text}, nil
	}
	// If the response is more complex, return as-is
	b, _ := json.Marshal(resp)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out, nil
}

func (a *MCPAdapter) Manifest() *registry.ToolManifest {
	return nil
}

// Close terminates all started stdio MCP server processes and closes their pipes.
func (a *MCPAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	a.closed = true
	var firstErr error
	for host, cmd := range a.processes {
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if pipes, ok := a.pipes[host]; ok {
			_ = pipes.stdin.Close()
			_ = pipes.stdout.Close()
		}
		delete(a.processes, host)
		delete(a.pipes, host)
		delete(a.clients, host)
	}
	return firstErr
}
