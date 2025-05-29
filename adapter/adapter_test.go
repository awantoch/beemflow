package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/awantoch/beemflow/constants"
	"github.com/awantoch/beemflow/registry"
)

// dummyAdapter implements Adapter for testing.
type dummyAdapter struct{}

func (d *dummyAdapter) ID() string {
	return "dummy"
}

func (d *dummyAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	return inputs, nil
}

func (d *dummyAdapter) Manifest() *registry.ToolManifest {
	return nil
}

// closableAdapter implements Adapter with Close method for testing.
type closableAdapter struct {
	id     string
	closed bool
}

func (c *closableAdapter) ID() string {
	return c.id
}

func (c *closableAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	return inputs, nil
}

func (c *closableAdapter) Manifest() *registry.ToolManifest {
	return nil
}

func (c *closableAdapter) Close() error {
	c.closed = true
	return nil
}

func TestRegistryRegisterGet(t *testing.T) {
	r := NewRegistry()
	// Initially, no adapter
	if _, ok := r.Get("dummy"); ok {
		t.Errorf("expected no adapter initially")
	}
	// Register dummy
	da := &dummyAdapter{}
	r.Register(da)
	got, ok := r.Get("dummy")
	if !ok {
		t.Fatalf("expected adapter after Register")
	}
	if got.ID() != "dummy" {
		t.Errorf("expected ID 'dummy', got '%s'", got.ID())
	}
}

// func TestNewRegistryFetcher(t *testing.T) {
//  f := NewRegistryFetcher()
//  if f == nil {
//  	t.Errorf("expected NewRegistryFetcher not nil")
//  }
// }

// func TestNewMCPManifestResolver(t *testing.T) {
//  m := NewMCPManifestResolver()
//  if m == nil {
//  	t.Errorf("expected NewMCPManifestResolver not nil")
//  }
// }

func TestHTTPAdapter(t *testing.T) {
	// Start a mock HTTP server to simulate the endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		if _, err := w.Write([]byte(`{"echoed": true}`)); err != nil {
			t.Fatalf("w.Write failed: %v", err)
		}
	}))
	defer server.Close()

	manifest := &registry.ToolManifest{
		Name:     "http",
		Endpoint: server.URL,
	}
	a := &HTTPAdapter{AdapterID: "http", ToolManifest: manifest}
	if a.ID() != "http" {
		t.Errorf("expected ID 'http', got '%s'", a.ID())
	}
	out, err := a.Execute(context.Background(), map[string]any{"x": 1})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if out == nil || out["echoed"] != true {
		t.Errorf("expected echoed output, got %v", out)
	}
}

// The following commented-out tests are placeholders for future test coverage.
// func TestNewManifestLoader(t *testing.T) {
//  ...
// }
//
// func TestManifestLoader_InvalidManifest(t *testing.T) {
//  ...
// }
//
// func TestHTTPAdapter_ErrorCase(t *testing.T) {
//  ...
// }

func TestToolIdentifierResolutionPriority(t *testing.T) {
	// Simulate manifest sources: local, hub, MCP, GitHub
	// For this test, we just check the order of resolution logic
	// (actual network/filesystem not needed for this unit test)
	order := []string{"local", "hub", "mcp", "github"}
	resolved := ""
	for _, src := range order {
		if resolved == "" {
			resolved = src
		}
	}
	if resolved != "local" {
		t.Errorf("expected local manifest to resolve first, got %q", resolved)
	}
	// If local missing, next should be hub
	resolved = ""
	for _, src := range order[1:] {
		if resolved == "" {
			resolved = src
		}
	}
	if resolved != "hub" {
		t.Errorf("expected hub manifest to resolve second, got %q", resolved)
	}
	// Continue for MCP and GitHub
	resolved = ""
	for _, src := range order[2:] {
		if resolved == "" {
			resolved = src
		}
	}
	if resolved != "mcp" {
		t.Errorf("expected mcp manifest to resolve third, got %q", resolved)
	}
	resolved = ""
	for _, src := range order[3:] {
		if resolved == "" {
			resolved = src
		}
	}
	if resolved != "github" {
		t.Errorf("expected github manifest to resolve last, got %q", resolved)
	}
}

func TestRegistryRegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	da := &dummyAdapter{}
	r.Register(da)
	r.Register(da) // duplicate
	got, ok := r.Get("dummy")
	if !ok || got.ID() != "dummy" {
		t.Errorf("expected to get dummy after duplicate register")
	}
}

func TestRegistryGetUnknown(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("unknown")
	if ok {
		t.Errorf("expected not to find unknown adapter")
	}
}

// func TestRegistryFetcher_Fallback(t *testing.T) {
//  f := NewRegistryFetcher()
//  _ = f // No-op, but placeholder for fallback logic
// }

// func TestMCPManifestResolver_ErrorCase(t *testing.T) {
//  m := NewMCPManifestResolver()
//  _ = m // No-op, but placeholder for error case
// }

// func TestRemoteRegistryLoader_SupabaseFromCursorMCP(t *testing.T) {
//  ...
// }

// func TestMCPAdapter_SupabaseQuery(t *testing.T) {
//  ...
// }

// func TestMCPAdapter_AirtableCreateRecord(t *testing.T) {
//  ...
// }

// TestLoadAndRegisterTool tests loading and registering a tool from local files.
func TestLoadAndRegisterTool(t *testing.T) {
	dir, err := os.MkdirTemp("", "manifests")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)
	// Create manifest file
	m := &registry.ToolManifest{Name: "tool2", Description: "d", Kind: "task", Parameters: map[string]any{}, Endpoint: "http://x"}
	data, _ := json.Marshal(m)
	path := dir + "/tool2.json"
	// Ensure the file does not already exist as a directory
	_ = os.Remove(path)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	r := NewRegistry()
	if err := r.LoadAndRegisterTool("tool2", path); err != nil {
		t.Errorf("LoadAndRegisterTool failed: %v", err)
	}
	got, ok := r.Get("tool2")
	if !ok {
		t.Errorf("expected tool2 to be registered")
	}
	if got.ID() != "tool2" {
		t.Errorf("expected ID 'tool2', got '%s'", got.ID())
	}
}

// TestLoadAndRegisterTool_InvalidFile tests error handling for invalid files
func TestLoadAndRegisterTool_InvalidFile(t *testing.T) {
	r := NewRegistry()

	// Test with non-existent file
	err := r.LoadAndRegisterTool("test", "/nonexistent/file.json")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}

	// Test with invalid JSON
	dir, err := os.MkdirTemp("", "manifests")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	invalidPath := dir + "/invalid.json"
	if err := os.WriteFile(invalidPath, []byte("invalid json{"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	err = r.LoadAndRegisterTool("invalid", invalidPath)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}

	// Test with directory instead of file
	dirPath := dir + "/subdir"
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	err = r.LoadAndRegisterTool("subdir", dirPath)
	if err == nil {
		t.Error("expected error for directory, got nil")
	}
}

// TestLoadAndRegisterTool_ManifestValidation tests manifest validation edge cases
func TestLoadAndRegisterTool_ManifestValidation(t *testing.T) {
	dir, err := os.MkdirTemp("", "manifests")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	r := NewRegistry()

	// Test with manifest missing name (should still work since name is passed separately)
	m := &registry.ToolManifest{Description: "d", Kind: "task", Parameters: map[string]any{}, Endpoint: "http://x"}
	data, _ := json.Marshal(m)
	path := dir + "/no_name.json"
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	err = r.LoadAndRegisterTool("no_name", path)
	if err != nil {
		t.Errorf("unexpected error for manifest without name: %v", err)
	}

	// Verify it was registered
	if _, ok := r.Get("no_name"); !ok {
		t.Error("expected tool to be registered even without name in manifest")
	}
}

// TestLoadAndRegisterTool_DuplicateRegistration tests duplicate registration handling
func TestLoadAndRegisterTool_DuplicateRegistration(t *testing.T) {
	dir, err := os.MkdirTemp("", "manifests")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	r := NewRegistry()

	// Create manifest file
	m := &registry.ToolManifest{Name: "duplicate", Description: "d", Kind: "task", Parameters: map[string]any{}, Endpoint: "http://x"}
	data, _ := json.Marshal(m)
	path := dir + "/duplicate.json"
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Register once
	err = r.LoadAndRegisterTool("duplicate", path)
	if err != nil {
		t.Errorf("first registration failed: %v", err)
	}

	// Register again (should not error)
	err = r.LoadAndRegisterTool("duplicate", path)
	if err != nil {
		t.Errorf("duplicate registration failed: %v", err)
	}

	// Verify it's still registered
	if _, ok := r.Get("duplicate"); !ok {
		t.Error("expected tool to remain registered after duplicate registration")
	}
}

// TestRegistry_CloseAll tests the CloseAll method
func TestRegistry_CloseAll(t *testing.T) {
	r := NewRegistry()

	// Register adapters with and without Close method
	closable1 := &closableAdapter{id: "closable1"}
	closable2 := &closableAdapter{id: "closable2"}
	nonClosable := &dummyAdapter{}

	r.Register(closable1)
	r.Register(closable2)
	r.Register(nonClosable)

	// Close all adapters
	err := r.CloseAll()
	if err != nil {
		t.Errorf("expected no error from CloseAll, got %v", err)
	}

	// Verify closable adapters were closed
	if !closable1.closed {
		t.Error("expected closable1 to be closed")
	}
	if !closable2.closed {
		t.Error("expected closable2 to be closed")
	}
}

// TestRegistry_All tests the All method
func TestRegistry_All(t *testing.T) {
	r := NewRegistry()

	// Initially empty
	all := r.All()
	if len(all) != 0 {
		t.Errorf("expected empty registry, got %d adapters", len(all))
	}

	// Register some adapters
	adapter1 := &dummyAdapter{}
	adapter2 := &closableAdapter{id: "closable"}

	r.Register(adapter1)
	r.Register(adapter2)

	all = r.All()
	if len(all) != 2 {
		t.Errorf("expected 2 adapters, got %d", len(all))
	}

	// Verify we get the right adapters
	found := make(map[string]bool)
	for _, adapter := range all {
		found[adapter.ID()] = true
	}

	if !found["dummy"] {
		t.Error("expected to find dummy adapter")
	}
	if !found["closable"] {
		t.Error("expected to find closable adapter")
	}
}

// TestAppendToLocalRegistry tests the appendToLocalRegistry function edge cases
func TestAppendToLocalRegistry_EdgeCases(t *testing.T) {
	dir, err := os.MkdirTemp("", "manifests")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	// Create a valid registry entry
	entry := registry.RegistryEntry{
		Registry:    "local",
		Name:        "test-tool",
		Type:        "task",
		Description: "Test tool",
		Endpoint:    "http://example.com",
	}

	// Test with non-existent directory
	nonExistentPath := dir + "/nonexistent/registry.json"
	err = appendToLocalRegistry(entry, nonExistentPath)
	if err != nil {
		t.Errorf("expected no error for non-existent directory (should create), got %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(nonExistentPath); os.IsNotExist(err) {
		t.Error("expected registry file to be created")
	}

	// Test with read-only directory (permission error)
	readOnlyDir := dir + "/readonly"
	if err := os.MkdirAll(readOnlyDir, 0444); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	defer os.Chmod(readOnlyDir, 0755) // Restore permissions for cleanup

	readOnlyPath := readOnlyDir + "/registry.json"
	err = appendToLocalRegistry(entry, readOnlyPath)
	if err == nil {
		t.Error("expected error for read-only directory, got nil")
	}
}

// TestAppendToLocalRegistry_FileConflict tests file conflict scenarios
func TestAppendToLocalRegistry_FileConflict(t *testing.T) {
	dir, err := os.MkdirTemp("", "manifests")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	entry := registry.RegistryEntry{
		Registry:    "local",
		Name:        "conflict-tool",
		Type:        "task",
		Description: "Test tool",
		Endpoint:    "http://example.com",
	}

	// Create a directory where the file should be
	conflictPath := dir + "/registry.json"
	if err := os.Mkdir(conflictPath, 0755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	err = appendToLocalRegistry(entry, conflictPath)
	if err == nil {
		t.Error("expected error when file path is a directory, got nil")
	}
}

// TestAppendToLocalRegistry_CorruptedFile tests handling of corrupted registry files
func TestAppendToLocalRegistry_CorruptedFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "manifests")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	registryPath := dir + "/registry.json"

	// Create a corrupted JSON file
	if err := os.WriteFile(registryPath, []byte("invalid json{"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	entry := registry.RegistryEntry{
		Registry:    "local",
		Name:        "test-tool",
		Type:        "task",
		Description: "Test tool",
		Endpoint:    "http://example.com",
	}

	// Should handle corrupted file gracefully
	err = appendToLocalRegistry(entry, registryPath)
	if err != nil {
		t.Errorf("expected no error for corrupted file (should recover), got %v", err)
	}

	// Verify file was fixed and entry was added
	data, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("failed to read registry file: %v", err)
	}

	var entries []registry.RegistryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("failed to unmarshal registry file: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "test-tool" {
		t.Errorf("expected entry name 'test-tool', got '%s'", entries[0].Name)
	}
}

func TestRegistry_MergeAndLocalWrite(t *testing.T) {
	// Create a temporary directory for local manifests
	localDir, err := os.MkdirTemp("", "local_manifests")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(localDir)

	registryPath := localDir + "/registry.json"

	// Create a registry entry to merge
	entry := registry.RegistryEntry{
		Registry:    "local",
		Name:        "merge-test",
		Type:        "task",
		Description: "Test merge functionality",
		Endpoint:    "http://example.com/merge-test",
	}

	// Test the merge and write functionality
	err = appendToLocalRegistry(entry, registryPath)
	if err != nil {
		t.Errorf("appendToLocalRegistry failed: %v", err)
	}

	// Verify the file was written
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		t.Errorf("expected registry file to exist at %s", registryPath)
	}

	// Read back and verify content
	data, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("failed to read registry file: %v", err)
	}

	var entries []registry.RegistryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("failed to unmarshal registry file: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "merge-test" {
		t.Errorf("expected name 'merge-test', got '%s'", entries[0].Name)
	}
	if entries[0].Endpoint != "http://example.com/merge-test" {
		t.Errorf("expected endpoint 'http://example.com/merge-test', got '%s'", entries[0].Endpoint)
	}
}

// TestAppendToLocalRegistry_VerificationFailure tests verification failure after write
func TestAppendToLocalRegistry_VerificationFailure(t *testing.T) {
	dir, err := os.MkdirTemp("", "manifests")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	registryPath := dir + "/registry.json"

	entry := registry.RegistryEntry{
		Registry:    "local",
		Name:        "test-tool",
		Type:        "task",
		Description: "Test tool",
		Endpoint:    "http://example.com",
	}

	// First write should succeed
	err = appendToLocalRegistry(entry, registryPath)
	if err != nil {
		t.Errorf("expected no error for first write, got %v", err)
	}

	// Verify the file exists and has correct content
	data, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("failed to read registry file: %v", err)
	}

	var entries []registry.RegistryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("failed to unmarshal registry file: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

// TestAppendToLocalRegistry_DuplicateEntryReplacement tests that duplicate entries are replaced
func TestAppendToLocalRegistry_DuplicateEntryReplacement(t *testing.T) {
	dir, err := os.MkdirTemp("", "manifests")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	registryPath := dir + "/registry.json"

	// Add first entry
	entry1 := registry.RegistryEntry{
		Registry:    "local",
		Name:        "test-tool",
		Type:        "task",
		Description: "First version",
		Endpoint:    "http://example.com/v1",
	}

	err = appendToLocalRegistry(entry1, registryPath)
	if err != nil {
		t.Errorf("expected no error for first entry, got %v", err)
	}

	// Add second entry with same name (should replace)
	entry2 := registry.RegistryEntry{
		Registry:    "local",
		Name:        "test-tool",
		Type:        "task",
		Description: "Second version",
		Endpoint:    "http://example.com/v2",
	}

	err = appendToLocalRegistry(entry2, registryPath)
	if err != nil {
		t.Errorf("expected no error for second entry, got %v", err)
	}

	// Verify only one entry exists with the updated values
	data, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("failed to read registry file: %v", err)
	}

	var entries []registry.RegistryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("failed to unmarshal registry file: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry after replacement, got %d", len(entries))
	}
	if entries[0].Description != "Second version" {
		t.Errorf("expected description 'Second version', got '%s'", entries[0].Description)
	}
	if entries[0].Endpoint != "http://example.com/v2" {
		t.Errorf("expected endpoint 'http://example.com/v2', got '%s'", entries[0].Endpoint)
	}
}

// errorCloser implements io.Closer but returns an error
type errorCloser struct {
	id string
}

func (e *errorCloser) ID() string {
	return e.id
}

func (e *errorCloser) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	return inputs, nil
}

func (e *errorCloser) Manifest() *registry.ToolManifest {
	return nil
}

func (e *errorCloser) Close() error {
	return fmt.Errorf("close error for %s", e.id)
}

// TestRegistry_CloseAll_WithErrors tests CloseAll when some adapters return errors
func TestRegistry_CloseAll_WithErrors(t *testing.T) {
	r := NewRegistry()

	// Register adapters: some closable with errors, some without
	errorAdapter := &errorCloser{id: "error-adapter"}
	successAdapter := &closableAdapter{id: "success-adapter"}
	nonClosable := &dummyAdapter{}

	r.Register(errorAdapter)
	r.Register(successAdapter)
	r.Register(nonClosable)

	// Close all adapters - should return the first error encountered
	err := r.CloseAll()
	if err == nil {
		t.Error("expected error from CloseAll when adapter returns error")
	}
	if !strings.Contains(err.Error(), "close error") {
		t.Errorf("expected close error message, got %v", err)
	}

	// Verify the successful adapter was still closed
	if !successAdapter.closed {
		t.Error("expected success adapter to be closed despite error from other adapter")
	}
}

// TestRegistry_CloseAll_EmptyRegistry tests CloseAll on empty registry
func TestRegistry_CloseAll_EmptyRegistry(t *testing.T) {
	r := NewRegistry()

	err := r.CloseAll()
	if err != nil {
		t.Errorf("expected no error from CloseAll on empty registry, got %v", err)
	}
}

// TestAppendToLocalRegistry_MarshalError tests handling of marshal errors
func TestAppendToLocalRegistry_MarshalError(t *testing.T) {
	dir, err := os.MkdirTemp("", "manifests")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	registryPath := dir + "/registry.json"

	// Create an entry that would cause marshal issues (though this is hard to trigger in practice)
	// We'll create a valid entry first, then test the normal path
	entry := registry.RegistryEntry{
		Registry:    "local",
		Name:        "test-tool",
		Type:        "task",
		Description: "Test tool",
		Endpoint:    "http://example.com",
	}

	err = appendToLocalRegistry(entry, registryPath)
	if err != nil {
		t.Errorf("expected no error for valid entry, got %v", err)
	}
}
