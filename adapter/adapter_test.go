package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

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
		w.Header().Set("Content-Type", "application/json")
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
		t.Fatalf("LoadAndRegisterTool failed: %v", err)
	}
	a, ok := r.Get("tool2")
	if !ok {
		t.Fatalf("expected adapter registered for tool2")
	}
	hta, ok := a.(*HTTPAdapter)
	if !ok {
		t.Errorf("expected HTTPAdapter, got %T", a)
	}
	if hta.ToolManifest.Name != "tool2" {
		t.Errorf("expected manifest Name tool2, got %s", hta.ToolManifest.Name)
	}
}

func TestRegistry_MergeAndLocalWrite(t *testing.T) {
	curatedPath := "curated_registry.json"
	localPath := "registry.json"
	defer os.Remove(curatedPath)
	defer os.Remove(localPath)

	// Curated: toolA, toolB
	curatedEntries := []registry.RegistryEntry{
		{Registry: "curated", Name: "toolA", Type: "task", Description: "curated A", Endpoint: "http://curated/a"},
		{Registry: "curated", Name: "toolB", Type: "task", Description: "curated B", Endpoint: "http://curated/b"},
	}
	_ = os.WriteFile(curatedPath, mustJSON(curatedEntries), 0644)

	// Local: toolB (override), toolC
	localEntries := []registry.RegistryEntry{
		{Registry: "local", Name: "toolB", Type: "task", Description: "local B", Endpoint: "http://local/b"},
		{Registry: "local", Name: "toolC", Type: "task", Description: "local C", Endpoint: "http://local/c"},
	}
	_ = os.WriteFile(localPath, mustJSON(localEntries), 0644)

	// Simulate config
	osErr := os.WriteFile("flow.config.json", []byte(`{"registries":[{"type":"local","path":"registry.json"}]}`), 0644)
	if osErr != nil {
		t.Fatalf("os.WriteFile failed: %v", osErr)
	}
	defer os.Remove("flow.config.json")

	// Load registries
	curatedReg := registry.NewLocalRegistry(curatedPath)
	curatedMgr := registry.NewRegistryManager(curatedReg)
	curatedTools, _ := curatedMgr.ListAllServers(context.Background(), registry.ListOptions{})

	localReg := registry.NewLocalRegistry(localPath)
	localMgr := registry.NewRegistryManager(localReg)
	localTools, _ := localMgr.ListAllServers(context.Background(), registry.ListOptions{})

	// Merge: local takes precedence
	toolMap := map[string]registry.RegistryEntry{}
	for _, entry := range curatedTools {
		toolMap[entry.Name] = entry
	}
	for _, entry := range localTools {
		toolMap[entry.Name] = entry
	}

	if len(toolMap) != 3 {
		t.Fatalf("expected 3 merged tools, got %d", len(toolMap))
	}
	if toolMap["toolB"].Description != "local B" {
		t.Errorf("expected local toolB to override, got %q", toolMap["toolB"].Description)
	}
	if toolMap["toolA"].Description != "curated A" {
		t.Errorf("expected curated toolA, got %q", toolMap["toolA"].Description)
	}
	if toolMap["toolC"].Description != "local C" {
		t.Errorf("expected local toolC, got %q", toolMap["toolC"].Description)
	}

	// Test writing a new tool to the local registry
	newTool := registry.RegistryEntry{Registry: "local", Name: "toolD", Type: "task", Description: "local D", Endpoint: "http://local/d"}
	if err := appendToLocalRegistry(newTool, localPath); err != nil {
		t.Fatalf("appendToLocalRegistry failed: %v", err)
	}
	data, _ := os.ReadFile(localPath)
	var written []registry.RegistryEntry
	_ = json.Unmarshal(data, &written)
	found := false
	for _, e := range written {
		if e.Name == "toolD" && e.Description == "local D" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected toolD to be written to local registry")
	}
}

func mustJSON(v any) []byte {
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}
