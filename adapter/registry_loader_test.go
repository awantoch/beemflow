// adapter/registry_loader_test.go
package adapter

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestLocalManifestLoader tests loading from a local directory.
func TestLocalManifestLoader(t *testing.T) {
	dir, err := ioutil.TempDir("", "manifests")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	defer os.RemoveAll(dir)
	m := &ToolManifest{Name: "tool1", Description: "desc", Kind: "task", Parameters: map[string]any{"p": map[string]any{"type": "string"}}, Endpoint: "/endpoint"}
	data, _ := json.Marshal(m)
	path := filepath.Join(dir, "tool1.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	loader := &LocalManifestLoader{Dir: dir}
	got, err := loader.LoadManifest("tool1")
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}
	if !reflect.DeepEqual(m, got) {
		t.Errorf("expected %+v, got %+v", m, got)
	}
}

// TestLocalManifestLoader_MissingFile tests error on missing file.
func TestLocalManifestLoader_MissingFile(t *testing.T) {
	loader := &LocalManifestLoader{Dir: "nonexistent"}
	_, err := loader.LoadManifest("nope")
	if err == nil {
		t.Error("expected error for missing manifest file")
	}
}

// TestRemoteRegistryLoader_DirectManifest tests fetching via direct manifest URL from the registry index.
func TestRemoteRegistryLoader_DirectManifest(t *testing.T) {
	var manifestFetched bool
	var tmpl = ToolManifest{Name: "tool3"}
	// HTTP server with index and direct manifest
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"tool3":{"manifest":"` + server.URL + `/tool3.json"}}`))
			return
		}
		if r.URL.Path == "/tool3.json" {
			manifestFetched = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(tmpl)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	os.Setenv("BEEMFLOW_REGISTRY", server.URL+"/index.json")
	defer os.Unsetenv("BEEMFLOW_REGISTRY")

	loader := NewRemoteRegistryLoader("")
	got, err := loader.LoadManifest("tool3")
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}
	if !manifestFetched {
		t.Error("expected direct manifest to be fetched")
	}
	if got.Name != tmpl.Name {
		t.Errorf("expected name %s, got %s", tmpl.Name, got.Name)
	}
}

func TestLoadUnifiedRegistry(t *testing.T) {
	entries := []map[string]any{
		{
			"type":        "tool",
			"name":        "http.fetch",
			"description": "Fetches a URL via HTTP GET and returns the response body as text.",
			"kind":        "task",
			"parameters": map[string]any{
				"type":     "object",
				"required": []string{"url"},
				"properties": map[string]any{
					"url": map[string]any{"type": "string", "description": "The URL to fetch."},
				},
			},
			"endpoint": "https://api.beemflow.com/http/fetch",
		},
		{
			"type":    "mcp_server",
			"name":    "airtable",
			"command": "npx",
			"args":    []string{"-y", "airtable-mcp-server"},
			"env":     map[string]any{"AIRTABLE_API_KEY": "$env"},
		},
	}
	data, _ := json.Marshal(entries)
	tmp, err := os.CreateTemp("", "index.json")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	tmp.Close()

	os.Setenv("BEEMFLOW_REGISTRY", tmp.Name())
	defer os.Unsetenv("BEEMFLOW_REGISTRY")

	tools, mcps, err := LoadUnifiedRegistry(tmp.Name())
	if err != nil {
		t.Fatalf("LoadUnifiedRegistry failed: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if len(mcps) != 1 {
		t.Fatalf("expected 1 mcp_server, got %d", len(mcps))
	}
	if tools[0].Name != "http.fetch" {
		t.Errorf("expected tool name http.fetch, got %s", tools[0].Name)
	}
	if mcps[0].Name != "airtable" {
		t.Errorf("expected mcp_server name airtable, got %s", mcps[0].Name)
	}
	if mcps[0].Command != "npx" {
		t.Errorf("expected mcp_server command npx, got %s", mcps[0].Command)
	}
	if !reflect.DeepEqual(mcps[0].Args, []string{"-y", "airtable-mcp-server"}) {
		t.Errorf("expected args ['-y', 'airtable-mcp-server'], got %+v", mcps[0].Args)
	}
}

func TestGetRegistryIndexURL(t *testing.T) {
	os.Setenv("BEEMFLOW_REGISTRY", "test-registry.json")
	defer os.Unsetenv("BEEMFLOW_REGISTRY")
	if got := GetRegistryIndexURL(); got != "test-registry.json" {
		t.Errorf("expected test-registry.json, got %s", got)
	}
	os.Unsetenv("BEEMFLOW_REGISTRY")
	if got := GetRegistryIndexURL(); got != "https://hub.beemflow.com/index.json" {
		t.Errorf("expected default URL, got %s", got)
	}
}
