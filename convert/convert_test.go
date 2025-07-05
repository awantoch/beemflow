package convert

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestYAMLToJsonnetRoundTrip(t *testing.T) {
	yamlPath := filepath.Join("..", "flows", "examples", "http_request_example.flow.yaml")
	yamlBytes, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("failed reading yaml: %v", err)
	}

	jsonnetStr, err := YAMLToJsonnet(yamlBytes)
	if err != nil {
		t.Fatalf("YAML to Jsonnet failed: %v", err)
	}

	// Convert back to YAML and ensure we get something non-empty.
	yamlOut, err := JsonnetToYAML([]byte(jsonnetStr))
	if err != nil {
		t.Fatalf("Jsonnet back to YAML failed: %v", err)
	}
	if len(bytes.TrimSpace([]byte(yamlOut))) == 0 {
		t.Fatal("round-tripped YAML is empty")
	}
}

func TestJsonnetToYAMLRoundTrip(t *testing.T) {
	jsonnetPath := filepath.Join("..", "flows", "examples", "http_request_example.flow.jsonnet")
	jsonnetBytes, err := os.ReadFile(jsonnetPath)
	if err != nil {
		t.Fatalf("failed reading jsonnet: %v", err)
	}

	yamlStr, err := JsonnetToYAML(jsonnetBytes)
	if err != nil {
		t.Fatalf("Jsonnet to YAML failed: %v", err)
	}

	jsonnetBack, err := YAMLToJsonnet([]byte(yamlStr))
	if err != nil {
		t.Fatalf("YAML back to Jsonnet failed: %v", err)
	}

	if len(bytes.TrimSpace([]byte(jsonnetBack))) == 0 {
		t.Fatal("round-tripped Jsonnet is empty")
	}
}