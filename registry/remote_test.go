package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRemoteRegistry_DefaultName(t *testing.T) {
	reg := NewRemoteRegistry("https://example.com", "")
	if reg.Registry != "remote" {
		t.Errorf("expected default registry name 'remote', got %s", reg.Registry)
	}
}

func TestRemoteRegistry_ListServers_Success(t *testing.T) {
	// Create test registry data
	testEntries := []RegistryEntry{
		{
			Type:        "tool",
			Name:        "test.tool",
			Description: "Test tool from remote registry",
			Kind:        "task",
			Endpoint:    "https://api.example.com/test",
		},
		{
			Type:        "mcp_server",
			Name:        "test.server",
			Description: "Test MCP server",
			Kind:        "server",
			Command:     "test-command",
		},
	}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept header 'application/json', got %s", r.Header.Get("Accept"))
		}
		if r.Header.Get("User-Agent") != "BeemFlow/1.0" {
			t.Errorf("expected User-Agent 'BeemFlow/1.0', got %s", r.Header.Get("User-Agent"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testEntries)
	}))
	defer server.Close()

	// Test the remote registry
	reg := NewRemoteRegistry(server.URL, "test-hub")
	entries, err := reg.ListServers(context.Background(), ListOptions{})

	if err != nil {
		t.Fatalf("ListServers failed: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	// Verify registry labeling
	for _, entry := range entries {
		if entry.Registry != "test-hub" {
			t.Errorf("expected registry 'test-hub', got %s", entry.Registry)
		}
	}

	// Verify specific entries
	if entries[0].Name != "test.tool" {
		t.Errorf("expected first entry name 'test.tool', got %s", entries[0].Name)
	}
	if entries[1].Name != "test.server" {
		t.Errorf("expected second entry name 'test.server', got %s", entries[1].Name)
	}
}

func TestRemoteRegistry_ListServers_HTTPError(t *testing.T) {
	// Create server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server error"))
	}))
	defer server.Close()

	reg := NewRemoteRegistry(server.URL, "test-hub")
	_, err := reg.ListServers(context.Background(), ListOptions{})

	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
	if err != nil && err.Error() != "remote registry returned status 500: 500 Internal Server Error" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRemoteRegistry_ListServers_InvalidJSON(t *testing.T) {
	// Create server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	reg := NewRemoteRegistry(server.URL, "test-hub")
	_, err := reg.ListServers(context.Background(), ListOptions{})

	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestRemoteRegistry_GetServer_Found(t *testing.T) {
	testEntries := []RegistryEntry{
		{Name: "tool1", Type: "tool", Description: "First tool"},
		{Name: "tool2", Type: "tool", Description: "Second tool"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testEntries)
	}))
	defer server.Close()

	reg := NewRemoteRegistry(server.URL, "test-hub")
	entry, err := reg.GetServer(context.Background(), "tool2")

	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.Name != "tool2" {
		t.Errorf("expected name 'tool2', got %s", entry.Name)
	}
	if entry.Description != "Second tool" {
		t.Errorf("expected description 'Second tool', got %s", entry.Description)
	}
}

func TestRemoteRegistry_GetServer_NotFound(t *testing.T) {
	testEntries := []RegistryEntry{
		{Name: "tool1", Type: "tool", Description: "First tool"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testEntries)
	}))
	defer server.Close()

	reg := NewRemoteRegistry(server.URL, "test-hub")
	entry, err := reg.GetServer(context.Background(), "nonexistent")

	if err != nil {
		t.Errorf("GetServer failed: %v", err)
	}
	if entry != nil {
		t.Errorf("expected nil for nonexistent tool, got %+v", entry)
	}
}

func TestRemoteRegistry_NetworkError(t *testing.T) {
	// Use invalid URL to trigger network error
	reg := NewRemoteRegistry("http://invalid-url-that-should-not-exist.local", "test-hub")
	_, err := reg.ListServers(context.Background(), ListOptions{})

	if err == nil {
		t.Error("expected network error, got nil")
	}
}

func TestRemoteRegistry_ContextCancellation(t *testing.T) {
	// Create server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This would normally cause a delay, but we'll cancel the context first
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]RegistryEntry{})
	}))
	defer server.Close()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	reg := NewRemoteRegistry(server.URL, "test-hub")
	_, err := reg.ListServers(ctx, ListOptions{})

	if err == nil {
		t.Error("expected context cancellation error, got nil")
	}
}
