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
	"github.com/awantoch/beemflow/logger"
	"github.com/awantoch/beemflow/model"
	mcp "github.com/metoro-io/mcp-golang"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"
)

// FindMCPServersInFlow scans a Flow for MCP tool usage and returns a set of required MCP server addresses.
func FindMCPServersInFlow(flow *model.Flow) map[string]bool {
	servers := make(map[string]bool)
	for _, step := range flow.Steps {
		findMCPInStep(step, servers)
	}
	for _, step := range flow.Catch {
		findMCPInStep(step, servers)
	}
	return servers
}

func findMCPInStep(step model.Step, servers map[string]bool) {
	if strings.HasPrefix(step.Use, "mcp://") {
		// mcp://server/tool
		parts := strings.SplitN(strings.TrimPrefix(step.Use, "mcp://"), "/", 2)
		if len(parts) > 0 {
			servers[parts[0]] = true
		}
	}
	// Recursively check nested steps (foreach, do, etc.)
	for _, sub := range step.Do {
		findMCPInStep(sub, servers)
	}
}

// NewMCPCommand creates an *exec.Cmd for the given MCP server config, merging environment variables.
func NewMCPCommand(info config.MCPServerConfig) *exec.Cmd {
	cmd := exec.Command(info.Command, info.Args...)
	cmd.Env = os.Environ()
	for k, v := range info.Env {
		if v == "$env" {
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
func EnsureMCPServersWithTimeout(flow *model.Flow, cfg *config.Config, timeout time.Duration) error {
	servers := FindMCPServersInFlow(flow)
	for server := range servers {
		info, err := config.GetMergedMCPServerConfig(cfg, server)
		if err != nil {
			return logger.Errorf("MCP server '%s' is not configured; please add it to 'mcpServers' in runtime config", server)
		}
		// Validate required environment variables (community style: env map)
		missingVars := []string{}
		for k := range info.Env {
			val := os.Getenv(k)
			logger.Info("MCP server '%s' expects env %s", server, k)
			if val == "" {
				missingVars = append(missingVars, k)
			}
		}
		if len(missingVars) > 0 {
			return logger.Errorf("environment variable(s) %v required for MCP server %s but not set. Check your .env or shell environment.", missingVars, server)
		}
		if info.Command == "" {
			return logger.Errorf("MCP server '%s' config is missing 'command' (stdio only supported; HTTP fallback is disabled)", server)
		}
		logger.Info("Spawning MCP server '%s' (stdio) with command: %s %v", server, info.Command, info.Args)
		cmd := NewMCPCommand(info)
		// If info.StdoutProtocol is true, do not redirect stdout (protocol communication)
		cmd.Stderr = &logger.LoggerWriter{Fn: logger.Error, Prefix: "[MCP " + server + " ERR] "}
		if err := cmd.Start(); err != nil {
			logger.Error("Failed to start MCP server %s: %v", server, err)
			logger.Error("Command: %s %v", info.Command, info.Args)
			logger.Error("Env: %v", cmd.Env)
			return logger.Errorf("failed to start MCP server %s: %v", server, err)
		}
		logger.Debug("MCP server '%s' (stdio) started", server)
		// Wait for MCP server to be ready (HTTP only for now)
		if info.Endpoint != "" {
			if err := waitForMCP(info.Endpoint, timeout); err != nil {
				return logger.Errorf("MCP server '%s' did not become ready: %v", server, err)
			}
		}
	}
	return nil
}

// EnsureMCPServers uses a default timeout of 15s for backward compatibility.
func EnsureMCPServers(flow *model.Flow, cfg *config.Config) error {
	return EnsureMCPServersWithTimeout(flow, cfg, 15*time.Second)
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
func waitForMCP(baseURL string, timeout time.Duration) error {
	client := NewHTTPMCPClient(baseURL)
	deadline := time.Now().Add(timeout)
	interval := 500 * time.Millisecond
	maxInterval := 5 * time.Second
	for {
		_, err := client.ListTools(context.Background(), new(string))
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return logger.Errorf("timeout after %v waiting for MCP at %s: %w", timeout, baseURL, err)
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
