package loader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAMLSuccess(t *testing.T) {
	path := filepath.Join("..", "flows", "examples", "http_request_example.flow.yaml")
	flow, err := Load(path, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error loading YAML flow: %v", err)
	}
	if flow == nil {
		t.Fatal("flow is nil")
	}
	if flow.Name != "http_request_example" {
		t.Fatalf("expected flow name 'http_request_example', got %q", flow.Name)
	}
}

func TestLoadJsonnetSuccess(t *testing.T) {
	path := filepath.Join("..", "flows", "examples", "http_request_example.flow.jsonnet")
	flow, err := Load(path, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error loading Jsonnet flow: %v", err)
	}
	if flow == nil {
		t.Fatal("flow is nil")
	}
	if flow.Name != "http_request_example" {
		t.Fatalf("expected flow name 'http_request_example', got %q", flow.Name)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "invalid.flow.yaml")
	os.WriteFile(p, []byte("name: invalid\n:\terror"), 0o644)
	if _, err := Load(p, map[string]any{}); err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadInvalidJsonnet(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "invalid.flow.jsonnet")
	os.WriteFile(p, []byte("{ name: \"bad"), 0o644)
	if _, err := Load(p, map[string]any{}); err == nil {
		t.Fatal("expected error for invalid Jsonnet, got nil")
	}
}

func TestLoadJsonnetWithImport(t *testing.T) {
	path := filepath.Join("..", "flows", "examples", "jsonnet_fanout.flow.jsonnet")
	flow, err := Load(path, map[string]any{"BASE": "https://example.com"})
	if err != nil {
		t.Fatalf("failed to load jsonnet with import: %v", err)
	}
	if flow == nil || flow.Name != "jsonnet_fanout" {
		t.Fatalf("unexpected flow name: %v", flow)
	}
	if len(flow.Steps) == 0 {
		t.Fatal("expected steps from Jsonnet fanout flow")
	}
}