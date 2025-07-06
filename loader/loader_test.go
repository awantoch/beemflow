package loader

import (
	"os"
	"path/filepath"
	"testing"
)

// Helper function to create test files
func createTestFile(t *testing.T, dir, filename, content string) string {
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	return path
}

func TestLoadYAMLSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	
	yamlContent := `
name: test_flow
description: Test flow
on:
  - http
steps:
  - id: test_step
    use: core.echo
    with:
      text: "Hello World"
`
	
	path := createTestFile(t, tmpDir, "test.yaml", yamlContent)
	
	flow, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	
	if flow.Name != "test_flow" {
		t.Errorf("Expected name 'test_flow', got '%s'", flow.Name)
	}
	
	if len(flow.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(flow.Steps))
	}
}

func TestLoadJsonnetSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	
	jsonnetContent := `
{
  name: "test_jsonnet_flow",
  description: "Test Jsonnet flow",
  on: ["http"],
  steps: [
    {
      id: "test_step",
      use: "core.echo",
      with: {
        text: "Hello from Jsonnet"
      }
    }
  ]
}
`
	
	path := createTestFile(t, tmpDir, "test.jsonnet", jsonnetContent)
	
	flow, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	
	if flow.Name != "test_jsonnet_flow" {
		t.Errorf("Expected name 'test_jsonnet_flow', got '%s'", flow.Name)
	}
}

func TestLoadJsonnetWithImport(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create helper library
	helperContent := `
{
  createEchoStep(id, text): {
    id: id,
    use: "core.echo",
    with: {
      text: text
    }
  }
}
`
	createTestFile(t, tmpDir, "helpers.libsonnet", helperContent)
	
	// Create main flow that imports the helper
	flowContent := `
local helpers = import 'helpers.libsonnet';

{
  name: "import_test_flow",
  description: "Test flow with imports",
  on: ["http"],
  steps: [
    helpers.createEchoStep("echo1", "Hello"),
    helpers.createEchoStep("echo2", "World")
  ]
}
`
	
	path := createTestFile(t, tmpDir, "test.jsonnet", flowContent)
	
	flow, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	
	if flow.Name != "import_test_flow" {
		t.Errorf("Expected name 'import_test_flow', got '%s'", flow.Name)
	}
	
	if len(flow.Steps) != 2 {
		t.Errorf("Expected 2 steps, got %d", len(flow.Steps))
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	
	invalidContent := `
name: test_flow
description: Test flow
# Missing required 'on' field
steps:
  - id: test_step
    use: core.echo
    with:
      text: "Hello"
`
	
	path := createTestFile(t, tmpDir, "invalid.yaml", invalidContent)
	
	_, err := Load(path, nil)
	if err == nil {
		t.Error("Expected validation error for missing 'on' field")
	}
}

func TestLoadInvalidJsonnet(t *testing.T) {
	tmpDir := t.TempDir()
	
	invalidContent := `
{
  name: "test_flow",
  description: "Test flow",
  // Invalid syntax - missing comma
  on: ["http"]
  steps: []
}
`
	
	path := createTestFile(t, tmpDir, "invalid.jsonnet", invalidContent)
	
	_, err := Load(path, nil)
	if err == nil {
		t.Error("Expected error for invalid Jsonnet syntax")
	}
}