// mcp/manager_test.go
package mcp

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/model"
)

func TestFindMCPServersInFlow(t *testing.T) {
	flow := &model.Flow{
		Steps: []model.Step{
			{Use: "mcp://foo/tool1"},
			{Use: "other://bar"},
			{
				Do: []model.Step{
					{Use: "mcp://bar/tool2"},
				},
			},
		},
		Catch: []model.Step{
			{Use: "mcp://baz/tool3"},
		},
	}
	servers := FindMCPServersInFlow(flow)
	want := map[string]bool{"foo": true, "bar": true, "baz": true}
	for k := range want {
		if !servers[k] {
			t.Errorf("expected server %s in flow, got %v", k, servers)
		}
	}
	if len(servers) != len(want) {
		t.Errorf("expected servers %v, got %v", want, servers)
	}
}

func TestEnsureMCPServers_MissingConfig(t *testing.T) {
	flow := &model.Flow{
		Steps: []model.Step{{Use: "mcp://unknown/tool"}},
	}
	cfg := &config.Config{MCPServers: map[string]config.MCPServerConfig{}}
	err := EnsureMCPServers(context.Background(), flow, cfg)
	if err == nil {
		t.Errorf("expected error when MCP server config missing")
	}
}

func TestEnsureMCPServers_MissingEnv(t *testing.T) {
	flow := &model.Flow{
		Steps: []model.Step{{Use: "mcp://foo/tool"}},
	}
	cfg := &config.Config{
		MCPServers: map[string]config.MCPServerConfig{
			"foo": {Command: "true", Env: map[string]string{"FOO": "bar"}},
		},
	}
	err := EnsureMCPServers(context.Background(), flow, cfg)
	if err == nil {
		t.Errorf("expected error when env vars missing")
	}
}

func TestEnsureMCPServers_Success(t *testing.T) {
	// Use "true" command which should exist on system
	flow := &model.Flow{
		Steps: []model.Step{{Use: "mcp://foo/tool"}},
	}
	cfg := &config.Config{
		MCPServers: map[string]config.MCPServerConfig{
			"foo": {Command: "true"},
		},
	}
	// Ensure no relevant env vars are set
	os.Unsetenv("FOO")
	err := EnsureMCPServers(context.Background(), flow, cfg)
	if err != nil {
		t.Errorf("expected no error when starting MCP server, got %v", err)
	}
}

func TestIsPortOpen(t *testing.T) {
	// Test closed port (unlikely to be open)
	if isPortOpen(65535) {
		t.Errorf("expected port 65535 to be closed")
	}
	// Test open port by listening
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on port: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	if !isPortOpen(port) {
		t.Errorf("expected port %d to be open", port)
	}
}

func TestNewHTTPMCPClient(t *testing.T) {
	client := NewHTTPMCPClient("http://example.com")
	if client == nil {
		t.Errorf("expected non-nil MCP client")
	}
}

// TestWaitForMCP_Success tests that waitForMCP returns nil when server responds quickly.
func TestWaitForMCP_Success(t *testing.T) {
	t.Skip("skipping waitForMCP success path until backoff logic refactored")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`[]`)); err != nil {
			t.Fatalf("w.Write failed: %v", err)
		}
	}))
	defer server.Close()
	err := waitForMCP(context.Background(), server.URL, 1*time.Second)
	if err != nil {
		t.Errorf("expected waitForMCP to succeed, got %v", err)
	}
}

// TestWaitForMCP_Error tests that waitForMCP returns an error immediately on negative timeout.
func TestWaitForMCP_Error(t *testing.T) {
	err := waitForMCP(context.Background(), "http://127.0.0.1:0", -1*time.Second)
	if err == nil {
		t.Errorf("expected error for negative timeout")
	}
}

// TestEnsureMCPServers_MissingCommand checks that missing Command yields an error
func TestEnsureMCPServers_MissingCommand(t *testing.T) {
	flow := &model.Flow{Steps: []model.Step{{Use: "mcp://foo/tool"}}}
	cfg := &config.Config{MCPServers: map[string]config.MCPServerConfig{"foo": {Command: ""}}}
	err := EnsureMCPServers(context.Background(), flow, cfg)
	if err == nil || !strings.Contains(err.Error(), "config is missing 'command'") {
		t.Errorf("expected missing command error, got %v", err)
	}
}

// TestEnsureMCPServers_DebugLogging ensures the debug logging branch is exercised
func TestEnsureMCPServers_DebugLogging(t *testing.T) {
	flow := &model.Flow{Steps: []model.Step{{Use: "mcp://foo/tool"}}}
	cfg := &config.Config{MCPServers: map[string]config.MCPServerConfig{"foo": {Command: "true"}}}
	os.Setenv("BEEMFLOW_DEBUG", "1")
	defer os.Unsetenv("BEEMFLOW_DEBUG")
	err := EnsureMCPServers(context.Background(), flow, cfg)
	if err != nil {
		t.Errorf("expected no error in debug mode, got %v", err)
	}
}

// TestEnsureMCPServers_CommandStartError ensures errors starting the command are handled
func TestEnsureMCPServers_CommandStartError(t *testing.T) {
	flow := &model.Flow{Steps: []model.Step{{Use: "mcp://foo/tool"}}}
	cfg := &config.Config{MCPServers: map[string]config.MCPServerConfig{"foo": {Command: "nonexistent_binary"}}}
	err := EnsureMCPServers(context.Background(), flow, cfg)
	if err == nil || !strings.Contains(err.Error(), "failed to start MCP server foo") {
		t.Errorf("expected start error for nonexistent command, got %v", err)
	}
}

// TestEnsureMCPServers_EnvMapping exercises the env mapping logic for both literal and $env values
func TestEnsureMCPServers_EnvMapping(t *testing.T) {
	flow := &model.Flow{Steps: []model.Step{{Use: "mcp://foo/tool"}}}
	cfg := &config.Config{MCPServers: map[string]config.MCPServerConfig{"foo": {
		Command: "true",
		Env:     map[string]string{"FOO_LIT": "val1", "FOO_SHELL": "$env"},
	}}}
	// Set both literal and $env variables
	os.Setenv("FOO_LIT", "val1")
	os.Setenv("FOO_SHELL", "shellval")
	defer os.Unsetenv("FOO_LIT")
	defer os.Unsetenv("FOO_SHELL")
	err := EnsureMCPServers(context.Background(), flow, cfg)
	if err != nil {
		t.Errorf("expected no error with env mapping, got %v", err)
	}
}
