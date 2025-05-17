package mcp

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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
		// TODO: Check if port is open (server running)
		if os.Getenv("BEEMFLOW_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[beemflow] Ensuring MCP server '%s' is running...\n", server)
		}
		cmd := exec.Command(info.InstallCmd[0], info.InstallCmd[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start MCP server %s: %v", server, err)
		}
		if os.Getenv("BEEMFLOW_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[beemflow] MCP server '%s' started.\n", server)
		}
	}
	return nil
}
