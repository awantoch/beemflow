package api

import (
	"net/http"
	"testing"
)

func TestGenerateHTTPHandlers(t *testing.T) {
	mux := http.NewServeMux()
	GenerateHTTPHandlers(mux)
	// Test that handlers were registered - basic smoke test
}

func TestGenerateCLICommands(t *testing.T) {
	commands := GenerateCLICommands()
	if len(commands) == 0 {
		t.Error("Expected at least one CLI command")
	}
}

func TestGenerateMCPTools(t *testing.T) {
	tools := GenerateMCPTools()
	if len(tools) == 0 {
		t.Error("Expected at least one MCP tool")
	}
}
