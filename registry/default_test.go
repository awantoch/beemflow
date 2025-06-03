package registry

import (
	"context"
	"testing"
)

func TestDefaultRegistry_ListServers(t *testing.T) {
	reg := NewDefaultRegistry()

	entries, err := reg.ListServers(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("ListServers failed: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("Expected some default entries, got none")
	}

	// Verify all entries are labeled with default registry
	for _, entry := range entries {
		if entry.Registry != "default" {
			t.Errorf("Expected registry 'default', got %s", entry.Registry)
		}
	}

	// Check for expected default tools (only working ones)
	expectedTools := []string{
		"http.fetch",
		"openai.chat_completion",
		"anthropic.chat_completion",
	}

	foundTools := make(map[string]bool)
	foundServers := make(map[string]bool)
	for _, entry := range entries {
		switch entry.Type {
		case "tool":
			foundTools[entry.Name] = true
		case "mcp_server":
			foundServers[entry.Name] = true
		}
	}

	for _, expected := range expectedTools {
		if !foundTools[expected] {
			t.Errorf("Expected tool %s not found in default registry", expected)
		}
	}

	// Check for expected MCP server
	if !foundServers["airtable"] {
		t.Error("Expected airtable MCP server not found in default registry")
	}
}

func TestDefaultRegistry_GetServer(t *testing.T) {
	reg := NewDefaultRegistry()

	// Test getting an existing tool
	entry, err := reg.GetServer(context.Background(), "http.fetch")
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if entry == nil {
		t.Fatal("Expected to find http.fetch, got nil")
	}
	if entry.Name != "http.fetch" {
		t.Errorf("Expected name 'http.fetch', got %s", entry.Name)
	}
	if entry.Registry != "default" {
		t.Errorf("Expected registry 'default', got %s", entry.Registry)
	}

	// Test getting a non-existent tool
	entry, err = reg.GetServer(context.Background(), "non.existent.tool")
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if entry != nil {
		t.Error("Expected nil for non-existent tool, got entry")
	}
}

func TestDefaultRegistry_RegistryName(t *testing.T) {
	reg := NewDefaultRegistry()
	if reg.Registry != "default" {
		t.Errorf("Expected registry name 'default', got %s", reg.Registry)
	}
}
