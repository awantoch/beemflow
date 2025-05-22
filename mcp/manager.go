package mcp

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/awantoch/beemflow/config"
	pproto "github.com/awantoch/beemflow/spec/proto"
	"github.com/awantoch/beemflow/utils"
	mcp "github.com/metoro-io/mcp-golang"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"
)

// FindMCPServersInFlow scans a Flow for MCP tool usage and returns a set of required MCP server addresses.
func FindMCPServersInFlow(flow *pproto.Flow) map[string]bool {
	servers := map[string]bool{}
	// scan a single step for mcp:// URIs
	var scanStep func(step *pproto.Step)
	scanExec := func(use string) {
		const prefix = "mcp://"
		if strings.HasPrefix(use, prefix) {
			rest := use[len(prefix):]
			parts := strings.SplitN(rest, "/", 2)
			if len(parts) > 0 && parts[0] != "" {
				servers[parts[0]] = true
			}
		}
	}
	scanStep = func(s *pproto.Step) {
		if s.GetExec() != nil {
			scanExec(s.GetExec().GetUse())
		}
		if p := s.GetParallel(); p != nil {
			for _, child := range p.GetSteps() {
				scanStep(child)
			}
		}
		if f := s.GetForeach(); f != nil {
			for _, child := range f.GetSteps() {
				scanStep(child)
			}
		}
		// catch blocks inside Execute.CatchBlock skip; not relevant here
	}
	if flow == nil {
		return servers
	}
	for _, s := range flow.GetSteps() {
		scanStep(s)
	}
	for _, s := range flow.GetCatch() {
		scanStep(s)
	}
	return servers
}

// NewMCPCommand creates an *exec.Cmd for the given MCP server config, merging environment variables.
func NewMCPCommand(info config.MCPServerConfig) *exec.Cmd {
	cmd := exec.Command(info.Command, info.Args...)
	cmd.Env = os.Environ()
	for k, v := range info.Env {
		// Support "$env:VARNAME" placeholders
		if strings.HasPrefix(v, "$env:") {
			envKey := strings.TrimPrefix(v, "$env:")
			if val := os.Getenv(envKey); val != "" {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, val))
			}
		} else if v == "$env" {
			// Legacy behavior: use the key as the env var name
			if val := os.Getenv(k); val != "" {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, val))
			}
		} else {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return cmd
}

// EnsureMCPServersWithTimeout uses runtime configuration to check and run all MCP servers referenced in the flow, with a configurable timeout.
func EnsureMCPServersWithTimeout(ctx context.Context, flow *pproto.Flow, cfg *config.Config, timeout time.Duration) error {
	servers := FindMCPServersInFlow(flow)
	for server := range servers {
		info, err := config.GetMergedMCPServerConfig(cfg, server)
		if err != nil {
			return utils.Errorf("MCP server '%s' is not configured; please add it to 'mcpServers' in runtime config", server)
		}
		// Validate required environment variables (community style: env map)
		missingVars := []string{}
		for k := range info.Env {
			val := os.Getenv(k)
			utils.InfoCtx(ctx, "MCP server expects env", "server", server, "env_var", k)
			if val == "" {
				missingVars = append(missingVars, k)
			}
		}
		if len(missingVars) > 0 {
			return utils.Errorf("environment variable(s) %v required for MCP server %s but not set. Check your .env or shell environment.", missingVars, server)
		}
		if info.Command == "" {
			return utils.Errorf("MCP server '%s' config is missing 'command' (stdio only supported; HTTP fallback is disabled)", server)
		}
		utils.InfoCtx(ctx, "Spawning MCP server (stdio)", "server", server, "command", info.Command, "args", info.Args)
		cmd := NewMCPCommand(info)
		// If info.StdoutProtocol is true, do not redirect stdout (protocol communication)
		cmd.Stderr = &utils.LoggerWriter{Fn: utils.Error, Prefix: "[MCP " + server + " ERR] "}
		if err := cmd.Start(); err != nil {
			utils.ErrorCtx(ctx, "Failed to start MCP server", "server", server, "error", err)
			utils.ErrorCtx(ctx, "Command", "command", info.Command, "args", info.Args)
			utils.ErrorCtx(ctx, "Env", "env", cmd.Env)
			return utils.Errorf("failed to start MCP server %s: %v", server, err)
		}
		utils.DebugCtx(ctx, "MCP server (stdio) started", "server", server)
		// Wait for MCP server to be ready (HTTP only for now)
		if info.Endpoint != "" {
			if err := waitForMCP(ctx, info.Endpoint, timeout); err != nil {
				return utils.Errorf("MCP server '%s' did not become ready: %v", server, err)
			}
		}
	}
	return nil
}

// EnsureMCPServers uses a default timeout of 15s for backward compatibility.
func EnsureMCPServers(ctx context.Context, flow *pproto.Flow, cfg *config.Config) error {
	return EnsureMCPServersWithTimeout(ctx, flow, cfg, 15*time.Second)
}

// isPortOpen checks if a TCP port is open on localhost
func isPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// waitForMCP polls the MCP server until it responds to ListTools or timeout, with exponential backoff
func waitForMCP(ctx context.Context, baseURL string, timeout time.Duration) error {
	client := NewHTTPMCPClient(baseURL)
	deadline := time.Now().Add(timeout)
	interval := 500 * time.Millisecond
	maxInterval := 5 * time.Second
	for {
		_, err := client.ListTools(ctx, new(string))
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return utils.Errorf("timeout after %v waiting for MCP at %s: %w", timeout, baseURL, err)
		}
		time.Sleep(interval)
		if interval < maxInterval {
			interval *= 2
			if interval > maxInterval {
				interval = maxInterval
			}
		}
	}
}

// NewHTTPMCPClient creates an HTTP MCP client for manager readiness checks.
func NewHTTPMCPClient(baseURL string) *mcp.Client {
	transport := mcphttp.NewHTTPClientTransport("/mcp").WithBaseURL(baseURL)
	return mcp.NewClient(transport)
}
