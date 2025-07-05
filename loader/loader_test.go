package loader

import (
	"os"
	"path/filepath"
	"testing"
	
	"github.com/awantoch/beemflow/constants"
)

// Helper function to reduce test duplication
func loadExampleFlow(t *testing.T, filename string, vars map[string]any) {
	t.Helper()
	if vars == nil {
		vars = constants.EmptyStringMap
	}
	
	path := filepath.Join("..", "flows", "examples", filename)
	flow, err := Load(path, vars)
	if err != nil {
		t.Fatalf("unexpected error loading %s: %v", filename, err)
	}
	if flow == nil {
		t.Fatal("flow is nil")
	}
	if flow.Name == "" {
		t.Fatal("flow name is empty")
	}
}

func TestLoadYAMLSuccess(t *testing.T) {
	loadExampleFlow(t, "http_request_example.flow.yaml", nil)
}

func TestLoadJsonnetSuccess(t *testing.T) {
	loadExampleFlow(t, "http_request_example.flow.jsonnet", nil)
}

func TestLoadJsonnetWithImport(t *testing.T) {
	vars := map[string]any{"BASE": "https://example.com"}
	loadExampleFlow(t, "jsonnet_fanout.flow.jsonnet", vars)
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "invalid.flow.yaml")
	os.WriteFile(p, []byte("name: invalid\n:\terror"), 0o644)
	if _, err := Load(p, constants.EmptyStringMap); err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadInvalidJsonnet(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "invalid.flow.jsonnet")
	os.WriteFile(p, []byte("{ name: \"bad"), 0o644)
	if _, err := Load(p, constants.EmptyStringMap); err == nil {
		t.Fatal("expected error for invalid Jsonnet, got nil")
	}
}