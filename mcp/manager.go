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

// EnsureMCPServersWithTimeout uses runtime configuration to check and run all MCP servers referenced in the flow, with a configurable timeout.
func EnsureMCPServersWithTimeout(flow *model.Flow, cfg *config.Config, timeout time.Duration) error {
	servers := FindMCPServersInFlow(flow)
	for server := range servers {
		info, ok := cfg.MCPServers[server]
		if !ok {
			return fmt.Errorf("MCP server '%s' is not configured; please add it to 'mcp_servers' in runtime config", server)
		}
		// Default to stdio transport if InstallCmd provided and no transport set
		if info.Transport == "" && len(info.InstallCmd) > 0 {
			info.Transport = "stdio"
		}
		// Validate required environment variables
		missingVars := []string{}
		for _, key := range info.RequiredEnv {
			val := os.Getenv(key)
			fmt.Fprintf(os.Stderr, "[beemflow] MCP server '%s' requires env %s=%q\n", server, key, val)
			if val == "" {
				missingVars = append(missingVars, key)
			}
		}
		if len(missingVars) > 0 {
			return fmt.Errorf("environment variable(s) %v required for MCP server %s but not set. Check your .env or shell environment.", missingVars, server)
		}
		// Ensure MCP server process is running and ready
		if info.Transport == "stdio" {
			// For stdio, just start the process if not already running (no port check)
			if len(info.InstallCmd) == 0 {
				return fmt.Errorf("MCP server '%s' config is missing 'install_cmd' (got: %+v). Check your config and curated files.", server, info)
			}
			fmt.Fprintf(os.Stderr, "[beemflow] Spawning MCP server '%s' (stdio) with command: %v\n", server, info.InstallCmd)
			cmd := exec.Command(info.InstallCmd[0], info.InstallCmd[1:]...)
			cmd.Env = os.Environ()
			for _, key := range info.RequiredEnv {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, os.Getenv(key)))
			}
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "[beemflow] ERROR: Failed to start MCP server %s: %v\n", server, err)
				fmt.Fprintf(os.Stderr, "[beemflow] Command: %v\n", info.InstallCmd)
				fmt.Fprintf(os.Stderr, "[beemflow] Env: %v\n", cmd.Env)
				return fmt.Errorf("failed to start MCP server %s: %v", server, err)
			}
			if os.Getenv("BEEMFLOW_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[beemflow] MCP server '%s' (stdio) started\n", server)
			}
			continue // skip HTTP readiness checks
		}
		// Default: HTTP transport (or unspecified)
		baseURL := info.Endpoint
		if baseURL == "" && info.Port > 0 {
			baseURL = fmt.Sprintf("http://localhost:%d", info.Port)
		}
		if baseURL == "" {
			baseURL = fmt.Sprintf("http://%s", server)
		}
		if os.Getenv("BEEMFLOW_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[beemflow] MCP config for '%s': %+v\n", server, info)
			fmt.Fprintf(os.Stderr, "[beemflow] Ensuring MCP server '%s' at %s is running...\n", server, baseURL)
		}
		// Check if port is open (server already running)
		if info.Port > 0 && isPortOpen(info.Port) {
			fmt.Fprintf(os.Stderr, "[beemflow] MCP server '%s' port %d is open. Checking if it responds to tools/list...\n", server, info.Port)
			if err := waitForMCP(baseURL, 3*time.Second); err != nil {
				fmt.Fprintf(os.Stderr, "[beemflow] WARNING: Port %d is open but MCP server did not respond as expected: %v\n", info.Port, err)
				fmt.Fprintf(os.Stderr, "[beemflow] This may mean another process is using the port, or the MCP server is misconfigured.\n")
				return fmt.Errorf("MCP server '%s' port %d is open but not responding as MCP. Please check for conflicting processes or restart the MCP server.", server, info.Port)
			}
			if os.Getenv("BEEMFLOW_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[beemflow] MCP server '%s' already listening and responding on port %d\n", server, info.Port)
			}
		} else {
			// Defensive check for InstallCmd
			if len(info.InstallCmd) == 0 {
				return fmt.Errorf("MCP server '%s' config is missing 'install_cmd' (got: %+v). Check your config and curated files.", server, info)
			}
			fmt.Fprintf(os.Stderr, "[beemflow] Spawning MCP server '%s' with command: %v\n", server, info.InstallCmd)
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
				fmt.Fprintf(os.Stderr, "[beemflow] ERROR: Failed to start MCP server %s: %v\n", server, err)
				fmt.Fprintf(os.Stderr, "[beemflow] Command: %v\n", info.InstallCmd)
				fmt.Fprintf(os.Stderr, "[beemflow] Env: %v\n", cmd.Env)
				return fmt.Errorf("failed to start MCP server %s: %v", server, err)
			}
			if os.Getenv("BEEMFLOW_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[beemflow] MCP server '%s' started\n", server)
			}
		}
		// Wait for readiness if a port is specified
		if info.Port > 0 {
			maxRetries := 3
			var lastErr error
			for attempt := 1; attempt <= maxRetries; attempt++ {
				fmt.Fprintf(os.Stderr, "[beemflow] Waiting for MCP server '%s' to become ready on port %d... (attempt %d/%d)\n", server, info.Port, attempt, maxRetries)
				lastErr = waitForMCP(baseURL, timeout)
				if lastErr == nil {
					break
				}
				if attempt < maxRetries {
					fmt.Fprintf(os.Stderr, "[beemflow] MCP server '%s' did not become ready, retrying...\n", server)
				}
			}
			if lastErr != nil {
				fmt.Fprintf(os.Stderr, "[beemflow] ERROR: MCP server '%s' did not become ready after %d attempts: %v\n", server, maxRetries, lastErr)
				return fmt.Errorf("MCP server '%s' did not become ready after %d attempts: %v", server, maxRetries, lastErr)
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
			return fmt.Errorf("timeout after %v waiting for MCP at %s: %w", timeout, baseURL, err)
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
