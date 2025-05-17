package mcp

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/awantoch/beemflow/internal/config"
	"github.com/awantoch/beemflow/internal/model"
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

// EnsureMCPServers uses runtime configuration to check and run all MCP servers referenced in the flow.
func EnsureMCPServers(flow *model.Flow, cfg *config.Config) error {
	servers := FindMCPServersInFlow(flow)
	for server := range servers {
		info, ok := cfg.MCPServers[server]
		if !ok {
			return fmt.Errorf("MCP server '%s' is not configured; please add it to 'mcp_servers' in runtime config", server)
		}
		// Validate required environment variables
		for _, key := range info.RequiredEnv {
			if os.Getenv(key) == "" {
				return fmt.Errorf("environment variable %s is required for MCP server %s", key, server)
			}
		}
		// Ensure MCP server process is running and ready
		baseURL := fmt.Sprintf("http://localhost:%d", info.Port)
		if os.Getenv("BEEMFLOW_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[beemflow] Ensuring MCP server '%s' at %s is running...\n", server, baseURL)
		}
		// Check if port is open (server already running)
		if info.Port > 0 && isPortOpen(info.Port) {
			if os.Getenv("BEEMFLOW_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[beemflow] MCP server '%s' already listening on port %d\n", server, info.Port)
			}
		} else {
			// Start MCP server process
			cmd := exec.Command(info.InstallCmd[0], info.InstallCmd[1:]...)
			// Inherit current environment and inject required vars
			cmd.Env = os.Environ()
			for _, key := range info.RequiredEnv {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, os.Getenv(key)))
			}
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("failed to start MCP server %s: %v", server, err)
			}
			if os.Getenv("BEEMFLOW_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[beemflow] MCP server '%s' started\n", server)
			}
		}
		// Wait for readiness if a port is specified
		if info.Port > 0 {
			if err := waitForMCP(baseURL, 15*time.Second); err != nil {
				return fmt.Errorf("MCP server '%s' did not become ready: %v", server, err)
			}
		}
	}
	return nil
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

// waitForMCP polls the MCP server until it responds to ListTools or timeout
func waitForMCP(baseURL string, timeout time.Duration) error {
	client := NewHTTPMCPClient(baseURL)
	deadline := time.Now().Add(timeout)
	for {
		_, err := client.ListTools()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout after %v waiting for MCP at %s: %w", timeout, baseURL, err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
