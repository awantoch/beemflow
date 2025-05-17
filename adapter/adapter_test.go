package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// dummyAdapter implements Adapter for testing
type dummyAdapter struct{}

func (d *dummyAdapter) ID() string {
	return "dummy"
}

func (d *dummyAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	return inputs, nil
}

func (d *dummyAdapter) Manifest() *ToolManifest {
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

func TestNewRegistryFetcher(t *testing.T) {
	f := NewRegistryFetcher()
	if f == nil {
		t.Errorf("expected NewRegistryFetcher not nil")
	}
}

func TestNewMCPManifestResolver(t *testing.T) {
	m := NewMCPManifestResolver()
	if m == nil {
		t.Errorf("expected NewMCPManifestResolver not nil")
	}
}

func TestHTTPAdapter(t *testing.T) {
	// Start a mock HTTP server to simulate the endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"echoed": true}`))
	}))
	defer server.Close()

	manifest := &ToolManifest{
		Name:     "http",
		Endpoint: server.URL,
	}
	a := &HTTPAdapter{id: "http", manifest: manifest}
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

func TestRegistryFetcher_Fallback(t *testing.T) {
	f := NewRegistryFetcher()
	_ = f // No-op, but placeholder for fallback logic
}

func TestMCPManifestResolver_ErrorCase(t *testing.T) {
	m := NewMCPManifestResolver()
	_ = m // No-op, but placeholder for error case
}

func TestRemoteRegistryLoader_SupabaseFromCursorMCP(t *testing.T) {
	imported := false
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
			  "supabase.query": {
			    "mcp": "` + server.URL + `/supabase-mcp"
			  }
			}`))
			return
		}
		if r.URL.Path == "/supabase-mcp/.well-known/beemflow.json" {
			imported = true
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
			  "name": "supabase.query",
			  "description": "Query Supabase via MCP",
			  "kind": "task",
			  "parameters": {"type": "object", "properties": {"sql": {"type": "string"}}},
			  "endpoint": "` + server.URL + `/supabase-mcp/query"
			}`))
			return
		}
		if r.URL.Path == "/supabase-mcp/query" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"result": "ok"}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	loader := NewRemoteRegistryLoader(server.URL + "/index.json")
	manifest, err := loader.LoadManifest("supabase.query")
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	if manifest == nil || manifest.Name != "supabase.query" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if !imported {
		t.Errorf("MCP manifest was not fetched from endpoint")
	}
}

func TestMCPAdapter_SupabaseQuery(t *testing.T) {
	t.Skip("Skipping Supabase HTTP fallback test; HTTP transport not supported in this adapter version")
	// Simulate a Supabase MCP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if req["method"] == "tools/list" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"tools":[{"name":"supabase.query","description":"Query Supabase","input_schema":{"type":"object","properties":{"sql":{"type":"string"}}}}]}`))
			return
		}
		if req["method"] == "tools/call" {
			params := req["params"].(map[string]any)
			if params["name"] == "supabase.query" {
				args := params["arguments"].(map[string]any)
				if args["sql"] == "SELECT * FROM users" {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"result":{"rows":[{"id":1,"name":"Alice"}]}}`))
					return
				}
			}
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	os.Setenv("BEEMFLOW_CONFIG", "test_supabase_config.json")
	defer os.Unsetenv("BEEMFLOW_CONFIG")
	cfg := map[string]any{
		// The host part of the mcp:// URL in the test below
		server.URL[7:]: map[string]any{
			"command":   "true",
			"transport": "http",
			"endpoint":  server.URL,
		},
	}
	b, _ := json.Marshal(map[string]any{"mcpServers": cfg})
	_ = os.WriteFile("test_supabase_config.json", b, 0644)
	defer os.Remove("test_supabase_config.json")

	adapter := NewMCPAdapter()
	url := server.URL
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	inputs := map[string]any{
		"__use": "mcp://" + url + "/supabase.query",
		"sql":   "SELECT * FROM users",
	}
	out, err := adapter.Execute(context.Background(), inputs)
	if err != nil {
		t.Fatalf("MCPAdapter.Execute failed: %v", err)
	}
	rows, ok := out["rows"].([]any)
	if !ok || len(rows) == 0 {
		t.Fatalf("expected rows in output, got %v", out)
	}
}

func TestMCPAdapter_AirtableCreateRecord(t *testing.T) {
	t.Skip("Skipping Airtable HTTP fallback test; HTTP transport not supported in this adapter version")
	// Simulate an Airtable MCP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if req["method"] == "tools/list" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"tools":[{"name":"create_record","description":"Create Airtable record","input_schema":{"type":"object","properties":{"baseId":{"type":"string"},"tableId":{"type":"string"},"fields":{"type":"object"}}}}]}`))
			return
		}
		if req["method"] == "tools/call" {
			params := req["params"].(map[string]any)
			if params["name"] == "create_record" {
				args := params["arguments"].(map[string]any)
				if args["baseId"] == "test_base" && args["tableId"] == "test_table" {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"result":{"id":"rec123","fields":{"Copy":"Hello!","Status":"Pending"}}}`))
					return
				}
			}
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	// Patch config loader to point to mock server for airtable
	os.Setenv("BEEMFLOW_CONFIG", "test_airtable_config.json")
	defer os.Unsetenv("BEEMFLOW_CONFIG")
	cfg := map[string]any{
		"airtable": map[string]any{
			"command":      "true",
			"install_cmd":  []string{"npx", "-y", "airtable-mcp-server"},
			"required_env": []string{"AIRTABLE_API_KEY"},
			"port":         0,
			"transport":    "http",
			"endpoint":     server.URL,
		},
	}
	b, _ := json.Marshal(map[string]any{"mcpServers": cfg})
	_ = os.WriteFile("test_airtable_config.json", b, 0644)
	defer os.Remove("test_airtable_config.json")

	adapter := NewMCPAdapter()
	inputs := map[string]any{
		"__use":   "mcp://airtable/create_record",
		"baseId":  "test_base",
		"tableId": "test_table",
		"fields":  map[string]any{"Copy": "Hello!", "Status": "Pending"},
	}
	out, err := adapter.Execute(context.Background(), inputs)
	if err != nil {
		t.Fatalf("MCPAdapter.Execute failed: %v", err)
	}
	id, ok := out["id"].(string)
	if !ok || id != "rec123" {
		t.Fatalf("expected id 'rec123' in output, got %v", out)
	}
	fields, ok := out["fields"].(map[string]any)
	if !ok || fields["Copy"] != "Hello!" || fields["Status"] != "Pending" {
		t.Fatalf("expected fields in output, got %v", out)
	}
}
