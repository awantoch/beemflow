package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sync"

	"github.com/awantoch/beemflow/config"
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
	mu sync.Mutex // protects clients, processes, pipes
	// TODO: handle process cleanup for stdio MCP servers (e.g., on shutdown or error)
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
	cfg, err := config.LoadConfig("flow.config.json")
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
	client, ok := a.clients[host]
	a.mu.Unlock()
	if !ok {
		cfg, err := getMCPServerConfig(host)
		if err != nil {
			return nil, err
		}
		if cfg.Command != "" {
			a.mu.Lock()
			cmd, ok := a.processes[host]
			a.mu.Unlock()
			if !ok {
				cmd = exec.Command(cfg.Command, cfg.Args...)
				cmd.Env = os.Environ()
				for k, v := range cfg.Env {
					if v == "$env" {
						if val := os.Getenv(k); val != "" {
							cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, val))
						}
					} else {
						cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
					}
				}
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
			return nil, fmt.Errorf("MCP server '%s' config is missing 'command' (stdio only supported; HTTP fallback is disabled)", host)
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

func (a *MCPAdapter) Manifest() *ToolManifest {
	return nil
}
