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

	loader := NewRemoteRegistryLoader(server.URL + "/index.json")
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
