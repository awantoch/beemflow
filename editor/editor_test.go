package editor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awantoch/beemflow/dsl"
	"github.com/awantoch/beemflow/model"
)

// TestEditorFiles verifies that the editor build files exist
func TestEditorFiles(t *testing.T) {
	tests := []struct {
		name string
		path string
		desc string
	}{
		{"WASM Runtime", "wasm/wasm_exec.js", "WASM runtime file"},
		{"Web Config", "web/package.json", "Web package configuration"},
		{"Web Index", "web/index.html", "Web HTML template"},
		{"WASM Source", "wasm/main.go", "WASM source code"},
		{"React App", "web/src/App.tsx", "React main component"},
		{"React Hook", "web/src/hooks/useBeemFlow.ts", "BeemFlow WASM hook"},
		{"Step Node", "web/src/components/StepNode.tsx", "Step node component"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := os.Stat(tt.path); err != nil {
				t.Errorf("%s not found at %s: %v", tt.desc, tt.path, err)
			}
		})
	}
}

// TestBuildArtifacts verifies build artifacts are created correctly
func TestBuildArtifacts(t *testing.T) {
	// These files should exist after a build
	buildFiles := []struct {
		name string
		path string
		desc string
	}{
		{"WASM Binary", "wasm/main.wasm", "Compiled WASM binary"},
		{"Web Build", "web/dist/index.html", "Built web application"},
		{"Web Assets", "web/dist/assets", "Web asset directory"},
	}

	for _, bf := range buildFiles {
		t.Run(bf.name, func(t *testing.T) {
			if _, err := os.Stat(bf.path); err != nil {
				t.Skipf("%s not found at %s (run 'make editor-build' first): %v", bf.desc, bf.path, err)
			}
		})
	}
}

// TestWASMSize verifies the WASM file is reasonable size
func TestWASMSize(t *testing.T) {
	info, err := os.Stat("wasm/main.wasm")
	if err != nil {
		t.Skipf("WASM file not found, skipping size test: %v", err)
		return
	}

	size := info.Size()
	minSize := int64(1024 * 1024)     // 1MB minimum
	maxSize := int64(50 * 1024 * 1024) // 50MB maximum

	if size < minSize {
		t.Errorf("WASM file seems too small: %d bytes (min: %d)", size, minSize)
	}
	if size > maxSize {
		t.Errorf("WASM file seems too large: %d bytes (max: %d)", size, maxSize)
	}

	t.Logf("WASM file size: %.2f MB", float64(size)/(1024*1024))
}

// TestWASMFunctionality tests the WASM functions work correctly
func TestWASMFunctionality(t *testing.T) {
	// Test YAML parsing functionality
	testYAML := `name: test
on: cli.manual
steps:
  - id: hello
    use: core.echo
    with:
      text: "Hello, World!"
  - id: goodbye
    use: core.echo
    with:
      text: "Goodbye!"`

	// Test parsing
	flow, err := dsl.ParseFromString(testYAML)
	if err != nil {
		t.Fatalf("Failed to parse test YAML: %v", err)
	}

	// Verify flow structure
	if flow.Name != "test" {
		t.Errorf("Expected flow name 'test', got '%s'", flow.Name)
	}
	if len(flow.Steps) != 2 {
		t.Errorf("Expected 2 steps, got %d", len(flow.Steps))
	}

	// Test validation
	if err := dsl.Validate(flow); err != nil {
		t.Errorf("Flow validation failed: %v", err)
	}
}

// TestYAMLGeneration tests YAML generation from flow
func TestYAMLGeneration(t *testing.T) {
	flow := &model.Flow{
		Name: "test_flow",
		On:   "cli.manual",
		Steps: []model.Step{
			{
				ID:  "step1",
				Use: "core.echo",
				With: map[string]any{
					"text": "Test message",
				},
			},
		},
	}

	yamlBytes, err := dsl.FlowToYAML(flow)
	if err != nil {
		t.Fatalf("Failed to generate YAML: %v", err)
	}

	yamlStr := string(yamlBytes)
	if !strings.Contains(yamlStr, "name: test_flow") {
		t.Errorf("Generated YAML missing flow name")
	}
	if !strings.Contains(yamlStr, "use: core.echo") {
		t.Errorf("Generated YAML missing step use")
	}
}

// TestConfigFiles verifies configuration files are valid
func TestConfigFiles(t *testing.T) {
	configTests := []struct {
		name string
		path string
		contains []string
	}{
		{
			name: "Package.json",
			path: "web/package.json",
			contains: []string{
				"\"name\": \"beemflow-editor\"",
				"\"react\":",
				"\"reactflow\":",
				"\"@monaco-editor/react\":",
			},
		},
		{
			name: "Vite Config",
			path: "web/vite.config.ts",
			contains: []string{
				"import { defineConfig }",
				"base: '/editor/'",
				"assetsInclude: ['**/*.wasm']",
			},
		},
		{
			name: "TypeScript Config",
			path: "web/tsconfig.json",
			contains: []string{
				"\"target\": \"ES2020\"",
				"\"jsx\": \"react-jsx\"",
				"\"strict\": true",
			},
		},
	}

	for _, ct := range configTests {
		t.Run(ct.name, func(t *testing.T) {
			content, err := os.ReadFile(ct.path)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", ct.path, err)
			}

			contentStr := string(content)
			for _, expected := range ct.contains {
				if !strings.Contains(contentStr, expected) {
					t.Errorf("Config file %s missing expected content: %s", ct.path, expected)
				}
			}
		})
	}
}

// TestMakeTargets verifies Makefile targets exist
func TestMakeTargets(t *testing.T) {
	makefileContent, err := os.ReadFile("../Makefile")
	if err != nil {
		t.Fatalf("Failed to read Makefile: %v", err)
	}

	content := string(makefileContent)
	expectedTargets := []string{
		"editor:",
		"editor-build:",
		"editor-web:",
		"editor/wasm/main.wasm:",
	}

	for _, target := range expectedTargets {
		if !strings.Contains(content, target) {
			t.Errorf("Makefile missing target: %s", target)
		}
	}
}

// TestDirectoryStructure verifies the editor directory structure
func TestDirectoryStructure(t *testing.T) {
	expectedDirs := []string{
		"wasm",
		"web",
		"web/src",
		"web/src/components",
		"web/src/hooks",
	}

	for _, dir := range expectedDirs {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("Expected directory %s not found or not a directory", dir)
		}
	}
}

// TestSourceCodeQuality performs basic code quality checks
func TestSourceCodeQuality(t *testing.T) {
	// Check for common issues in source files
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip node_modules and dist directories
		if strings.Contains(path, "node_modules") || strings.Contains(path, "dist") {
			return nil
		}

		// Check source files
		if strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			contentStr := string(content)
			
			// Check for TODO/FIXME comments
			if strings.Contains(contentStr, "TODO") || strings.Contains(contentStr, "FIXME") {
				t.Logf("Found TODO/FIXME in %s - consider addressing", path)
			}

			// Check for console.log in production code (except test files)
			if !strings.Contains(path, "_test") && strings.Contains(contentStr, "console.log") {
				t.Errorf("Found console.log in production code: %s", path)
			}
		}

		return nil
	})

	if err != nil {
		t.Errorf("Error walking directory: %v", err)
	}
}

// TestIntegration performs basic integration testing
func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test the complete flow: YAML -> Parse -> Validate -> Generate
	testYAML := `name: integration_test
on: cli.manual
steps:
  - id: test_step
    use: core.echo
    with:
      text: "Integration test"`

	// Parse
	flow, err := dsl.ParseFromString(testYAML)
	if err != nil {
		t.Fatalf("Integration test failed at parsing: %v", err)
	}

	// Validate
	if err := dsl.Validate(flow); err != nil {
		t.Fatalf("Integration test failed at validation: %v", err)
	}

	// Generate YAML back
	yamlBytes, err := dsl.FlowToYAML(flow)
	if err != nil {
		t.Fatalf("Integration test failed at YAML generation: %v", err)
	}

	// Parse generated YAML again
	flow2, err := dsl.ParseFromString(string(yamlBytes))
	if err != nil {
		t.Fatalf("Integration test failed at re-parsing: %v", err)
	}

	// Verify round-trip consistency
	if flow.Name != flow2.Name {
		t.Errorf("Round-trip failed: name mismatch %s != %s", flow.Name, flow2.Name)
	}
	if len(flow.Steps) != len(flow2.Steps) {
		t.Errorf("Round-trip failed: steps count mismatch %d != %d", len(flow.Steps), len(flow2.Steps))
	}
}

// BenchmarkWASMFunctions benchmarks WASM function performance
func BenchmarkWASMFunctions(b *testing.B) {
	testYAML := `name: benchmark
on: cli.manual
steps:
  - id: step1
    use: core.echo
    with:
      text: "Benchmark test"`

	b.Run("ParseYAML", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := dsl.ParseFromString(testYAML)
			if err != nil {
				b.Fatalf("Parse failed: %v", err)
			}
		}
	})

	b.Run("ValidateYAML", func(b *testing.B) {
		flow, _ := dsl.ParseFromString(testYAML)
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			err := dsl.Validate(flow)
			if err != nil {
				b.Fatalf("Validate failed: %v", err)
			}
		}
	})
}

// TestConcurrentAccess tests concurrent access to WASM functions
func TestConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	testYAML := `name: concurrent_test
on: cli.manual
steps:
  - id: step1
    use: core.echo
    with:
      text: "Concurrent test"`

	// Test concurrent parsing
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			for j := 0; j < 10; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					_, err := dsl.ParseFromString(testYAML)
					if err != nil {
						t.Errorf("Concurrent parse failed in goroutine %d: %v", id, err)
						return
					}
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Success
		case <-ctx.Done():
			t.Fatal("Concurrent test timed out")
		}
	}
}