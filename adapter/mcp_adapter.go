package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/awantoch/beemflow/config"
	mcp "github.com/metoro-io/mcp-golang"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"
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
	// Load main runtime config
	cfgPath := os.Getenv("BEEMFLOW_CONFIG")
	if cfgPath == "" {
		cfgPath = "flow.config.json"
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		return config.MCPServerConfig{}, err
	}
	// Get stub from flow.config.json
	info, ok := cfg.MCPServers[host]
	if !ok {
		return config.MCPServerConfig{}, fmt.Errorf("MCP server '%s' not found in config", host)
	}
	// Merge curated config from mcp_servers/<host>.json if available
	curatedPath := filepath.Join("mcp_servers", host+".json")
	if data, err := os.ReadFile(curatedPath); err == nil {
		var m map[string]config.MCPServerConfig
		if err := json.Unmarshal(data, &m); err == nil {
			if ci, ok2 := m[host]; ok2 {
				info = ci
			}
		}
	}
	return info, nil
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
		// Default to stdio transport if InstallCmd is provided and no transport is set
		if cfg.Transport == "" && len(cfg.InstallCmd) > 0 {
			cfg.Transport = "stdio"
		}
		if len(cfg.InstallCmd) > 0 {
			a.mu.Lock()
			cmd, ok := a.processes[host]
			a.mu.Unlock()
			if !ok {
				if len(cfg.InstallCmd) == 0 {
					return nil, fmt.Errorf("MCP server '%s' config is missing 'install_cmd'", host)
				}
				cmd = exec.Command(cfg.InstallCmd[0], cfg.InstallCmd[1:]...)
				cmd.Env = os.Environ()
				for _, key := range cfg.RequiredEnv {
					cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, os.Getenv(key)))
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
			// pipes.stdout is the process's stdout (io.Reader), pipes.stdin is the process's stdin (io.Writer)
			transport := mcpstdio.NewStdioServerTransportWithIO(pipes.stdout, pipes.stdin)
			client = mcp.NewClient(transport)
			// Initialize the client
			if _, err := client.Initialize(ctx); err != nil {
				return nil, fmt.Errorf("failed to initialize MCP stdio client: %w", err)
			}
			a.mu.Lock()
			a.clients[host] = client
			a.mu.Unlock()
		} else {
			endpoint := cfg.Endpoint
			if endpoint == "" && cfg.Port > 0 {
				endpoint = fmt.Sprintf("http://localhost:%d", cfg.Port)
			}
			if endpoint == "" {
				if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
					endpoint = "http://" + host
				} else {
					endpoint = "https://" + host
				}
			}
			// HTTP transport is stateless and does not support notifications
			httpTransport := mcphttp.NewHTTPClientTransport("/mcp").WithBaseURL(endpoint)
			client = mcp.NewClient(httpTransport)
			a.mu.Lock()
			a.clients[host] = client
			a.mu.Unlock()
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
