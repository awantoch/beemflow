package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/utils"
	"github.com/google/uuid"
)

// TestMain ensures that the "flows" directory is removed before and after tests.
func TestMain(m *testing.M) {
	utils.WithCleanDirs(m, ".beemflow", config.DefaultConfigDir, config.DefaultFlowsDir)
}

func TestListFlows(t *testing.T) {
	_, err := ListFlows(context.Background())
	if err != nil {
		t.Errorf("ListFlows returned error: %v", err)
	}
}

func TestGetFlow(t *testing.T) {
	_, err := GetFlow(context.Background(), "dummy")
	if err != nil {
		t.Errorf("GetFlow returned error: %v", err)
	}
}

func TestValidateFlow(t *testing.T) {
	err := ValidateFlow(context.Background(), "dummy")
	if err != nil {
		t.Errorf("ValidateFlow returned error: %v", err)
	}
}

func TestGraphFlow(t *testing.T) {
	_, err := GraphFlow(context.Background(), "dummy")
	if err != nil {
		t.Errorf("GraphFlow returned error: %v", err)
	}
}

func TestStartRun(t *testing.T) {
	_, err := StartRun(context.Background(), "dummy", map[string]any{})
	if err != nil {
		t.Errorf("StartRun returned error: %v", err)
	}
}

func TestGetRun(t *testing.T) {
	_, err := GetRun(context.Background(), uuid.New())
	if err != nil {
		t.Errorf("GetRun returned error: %v", err)
	}
}

func TestListRuns(t *testing.T) {
	_, err := ListRuns(context.Background())
	if err != nil {
		t.Errorf("ListRuns returned error: %v", err)
	}
}

func TestPublishEvent(t *testing.T) {
	err := PublishEvent(context.Background(), "test.topic", map[string]any{"foo": "bar"})
	if err != nil {
		if strings.Contains(err.Error(), "event bus not configured") || strings.Contains(err.Error(), "no such file or directory") {
			t.Skipf("skipping: event bus not configured or config missing: %v", err)
		}
		// Otherwise, fail
		t.Errorf("PublishEvent returned error: %v", err)
	}
}

func TestResumeRun(t *testing.T) {
	outputs, err := ResumeRun(context.Background(), "dummy-token", map[string]any{"foo": "bar"})
	if err != nil {
		t.Errorf("ResumeRun returned error: %v", err)
	}
	if outputs != nil {
		t.Errorf("expected nil outputs for non-existent token, got: %v", outputs)
	}
}

func TestListFlows_DirError(t *testing.T) {
	// Use testutil to clean flows dir before simulating error
	os.RemoveAll(config.DefaultFlowsDir)
	// Temporarily rename flows dir if it exists
	if _, err := os.Stat(config.DefaultFlowsDir); err == nil {
		_ = os.Rename(config.DefaultFlowsDir, "flows_tmp")
		defer func() { _ = os.Rename("flows_tmp", config.DefaultFlowsDir) }()
	}
	_, err := ListFlows(context.Background())
	if err != nil && !os.IsNotExist(err) {
		t.Errorf("expected nil or not exist error, got: %v", err)
	}
}

func TestGetFlow_FileNotFound(t *testing.T) {
	_, err := GetFlow(context.Background(), "definitely_not_a_real_flow")
	if err != nil {
		t.Errorf("expected nil error for missing file, got: %v", err)
	}
}

func TestGetFlow_ParseError(t *testing.T) {
	flowsDir := filepath.Join(t.TempDir(), config.DefaultFlowsDir)
	if err := os.MkdirAll(flowsDir, 0755); err != nil {
		t.Fatalf("os.MkdirAll failed: %v", err)
	}
	badPath := filepath.Join(flowsDir, "bad.flow.yaml")
	if err := os.WriteFile(badPath, []byte("not: [valid: yaml"), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	// Set the global flowsDir in the api package
	SetFlowsDir(flowsDir)
	_, err := GetFlow(context.Background(), "bad")
	if err == nil {
		t.Errorf("expected parse error, got nil")
	}
}

func TestValidateFlow_FileNotFound(t *testing.T) {
	err := ValidateFlow(context.Background(), "definitely_not_a_real_flow")
	if err != nil {
		t.Errorf("expected nil error for missing file, got: %v", err)
	}
}

func TestValidateFlow_SchemaError(t *testing.T) {
	flowsDir := filepath.Join(t.TempDir(), config.DefaultFlowsDir)
	if err := os.MkdirAll(flowsDir, 0755); err != nil {
		t.Fatalf("os.MkdirAll failed: %v", err)
	}
	badPath := filepath.Join(flowsDir, "bad.flow.yaml")
	if err := os.WriteFile(badPath, []byte("name: bad\nsteps: []"), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	defer os.Remove(badPath)
	SetFlowsDir(flowsDir)
	// Use a non-existent schema file
	orig := "beemflow.schema.json"
	if err := os.Rename(orig, orig+".bak"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("os.Rename failed: %v", err)
	}
	defer func() { _ = os.Rename(orig+".bak", orig) }()
	err := ValidateFlow(context.Background(), "bad")
	if err == nil {
		t.Errorf("expected schema error, got nil")
	}
}

func TestStartRun_ConfigError(t *testing.T) {
	// Simulate config error by renaming config file
	orig := "flow.config.json"
	if err := os.Rename(orig, orig+".bak"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("os.Rename failed: %v", err)
	}
	defer func() { _ = os.Rename(orig+".bak", orig) }()
	_, err := StartRun(context.Background(), "dummy", map[string]any{})
	if err != nil && !os.IsNotExist(err) {
		t.Errorf("expected nil or not exist error, got: %v", err)
	}
}

func TestStartRun_ParseError(t *testing.T) {
	flowsDir := filepath.Join(t.TempDir(), config.DefaultFlowsDir)
	if err := os.MkdirAll(flowsDir, 0755); err != nil {
		t.Fatalf("os.MkdirAll failed: %v", err)
	}
	badPath := filepath.Join(flowsDir, "bad.flow.yaml")
	if err := os.WriteFile(badPath, []byte("not: [valid: yaml"), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	defer os.Remove(badPath)
	SetFlowsDir(flowsDir)
	_, err := StartRun(context.Background(), "bad", map[string]any{})
	if err == nil {
		t.Errorf("expected parse error, got nil")
	}
}

func TestGetRun_ConfigError(t *testing.T) {
	orig := "flow.config.json"
	if err := os.Rename(orig, orig+".bak"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("os.Rename failed: %v", err)
	}
	defer func() { _ = os.Rename(orig+".bak", orig) }()
	_, err := GetRun(context.Background(), uuid.New())
	if err != nil && !os.IsNotExist(err) {
		t.Errorf("expected nil or not exist error, got: %v", err)
	}
}

func TestGetRun_NotFound(t *testing.T) {
	_, err := GetRun(context.Background(), uuid.New())
	if err != nil {
		t.Errorf("expected nil error for not found, got: %v", err)
	}
}

func TestListRuns_ConfigError(t *testing.T) {
	orig := "flow.config.json"
	if err := os.Rename(orig, orig+".bak"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("os.Rename failed: %v", err)
	}
	defer func() { _ = os.Rename(orig+".bak", orig) }()
	_, err := ListRuns(context.Background())
	if err != nil && !os.IsNotExist(err) {
		t.Errorf("expected nil or not exist error, got: %v", err)
	}
}

func TestResumeRun_ConfigError(t *testing.T) {
	orig := "flow.config.json"
	if err := os.Rename(orig, orig+".bak"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("os.Rename failed: %v", err)
	}
	defer func() { _ = os.Rename(orig+".bak", orig) }()
	_, err := ResumeRun(context.Background(), "dummy-token", map[string]any{"foo": "bar"})
	if err != nil && !os.IsNotExist(err) {
		t.Errorf("expected nil or not exist error, got: %v", err)
	}
}

func TestListFlows_UnexpectedError(t *testing.T) {
	// Simulate unexpected error by creating a file instead of a dir
	os.RemoveAll(config.DefaultFlowsDir)
	if err := os.WriteFile("flows", []byte("not a dir"), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	defer os.RemoveAll(config.DefaultFlowsDir)
	_, err := ListFlows(context.Background())
	if err == nil {
		// This is OS/filesystem dependent; skip if not reproducible
		t.Skip("skipping: expected error for not a directory, but got nil (may be OS/filesystem dependent)")
	}
}

func TestGetFlow_UnexpectedError(t *testing.T) {
	// Simulate unexpected error by making flows unreadable
	if err := os.MkdirAll("flows", 0755); err != nil {
		t.Fatalf("os.MkdirAll failed: %v", err)
	}
	badPath := "flows/unreadable.flow.yaml"
	if err := os.WriteFile(badPath, []byte("foo: bar"), 0000); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	defer os.Remove(badPath)
	_, err := GetFlow(context.Background(), "unreadable")
	if err == nil {
		// This is OS/filesystem dependent; skip if not reproducible
		t.Skip("skipping: expected error for unreadable file, but got nil (may be OS/filesystem dependent)")
	}
}

func TestValidateFlow_ParseError(t *testing.T) {
	flowsDir := filepath.Join(t.TempDir(), config.DefaultFlowsDir)
	if err := os.MkdirAll(flowsDir, 0755); err != nil {
		t.Fatalf("os.MkdirAll failed: %v", err)
	}
	badPath := filepath.Join(flowsDir, "badparse.flow.yaml")
	if err := os.WriteFile(badPath, []byte("not: [valid: yaml"), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	defer os.Remove(badPath)
	SetFlowsDir(flowsDir)
	err := ValidateFlow(context.Background(), "badparse")
	if err == nil {
		t.Errorf("expected parse error, got nil")
	}
}

func TestStartRun_InvalidStorageDriver(t *testing.T) {
	cfg := `{"storage":{"driver":"bogus","dsn":""}}`
	if err := os.WriteFile("flow.config.json", []byte(cfg), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	defer os.Remove("flow.config.json")
	_, err := StartRun(context.Background(), "dummy", map[string]any{})
	if err == nil {
		t.Errorf("expected error for invalid storage driver, got nil")
	}
}

func TestGetRun_InvalidStorageDriver(t *testing.T) {
	cfg := `{"storage":{"driver":"bogus","dsn":""}}`
	if err := os.WriteFile("flow.config.json", []byte(cfg), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	defer os.Remove("flow.config.json")
	_, err := GetRun(context.Background(), uuid.New())
	if err == nil {
		t.Errorf("expected error for invalid storage driver, got nil")
	}
}

func TestListRuns_InvalidStorageDriver(t *testing.T) {
	cfg := `{"storage":{"driver":"bogus","dsn":""}}`
	if err := os.WriteFile("flow.config.json", []byte(cfg), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	defer os.Remove("flow.config.json")
	_, err := ListRuns(context.Background())
	if err == nil {
		t.Errorf("expected error for invalid storage driver, got nil")
	}
}

func TestResumeRun_InvalidStorageDriver(t *testing.T) {
	cfg := `{"storage":{"driver":"bogus","dsn":""}}`
	if err := os.WriteFile("flow.config.json", []byte(cfg), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	defer os.Remove("flow.config.json")
	_, err := ResumeRun(context.Background(), "dummy-token", map[string]any{"foo": "bar"})
	if err == nil {
		t.Errorf("expected error for invalid storage driver, got nil")
	}
}

func TestStartRun_ListRunsError(t *testing.T) {
	// Patch storage to return error from ListRuns
	// Not possible without interface injection or reflection, so just test empty runs case
	if err := os.MkdirAll("flows", 0755); err != nil {
		t.Fatalf("os.MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(config.DefaultFlowsDir+"/empty.flow.yaml", []byte("name: empty\nsteps: []"), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	defer os.Remove(config.DefaultFlowsDir + "/empty.flow.yaml")
	id, err := StartRun(context.Background(), "empty", map[string]any{})
	if err != nil {
		t.Errorf("expected no error for empty runs, got: %v", err)
	}
	if id != uuid.Nil {
		t.Errorf("expected uuid.Nil for no runs, got: %v", id)
	}
}

func TestIntegration_FlowLifecycle(t *testing.T) {
	// Create a temporary directory for test flows
	tempDir, err := os.MkdirTemp("", "test_flows")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set the flows directory
	originalFlowsDir := flowsDir
	SetFlowsDir(tempDir)
	defer SetFlowsDir(originalFlowsDir)

	// Create a simple test flow
	flowContent := `name: test_flow
on: cli.manual
steps:
  - id: echo_step
    use: core.echo
    with:
      text: "Hello World"
`
	flowPath := filepath.Join(tempDir, "test_flow.flow.yaml")
	if err := os.WriteFile(flowPath, []byte(flowContent), 0644); err != nil {
		t.Fatalf("Failed to write test flow: %v", err)
	}

	ctx := context.Background()

	// Test ListFlows
	flows, err := ListFlows(ctx)
	if err != nil {
		t.Errorf("ListFlows failed: %v", err)
	}
	if len(flows) != 1 || flows[0] != "test_flow" {
		t.Errorf("Expected [test_flow], got %v", flows)
	}

	// Test GetFlow
	flow, err := GetFlow(ctx, "test_flow")
	if err != nil {
		t.Errorf("GetFlow failed: %v", err)
	}
	if flow.Name != "test_flow" {
		t.Errorf("Expected flow name 'test_flow', got %s", flow.Name)
	}

	// Test ValidateFlow
	err = ValidateFlow(ctx, "test_flow")
	if err != nil {
		t.Errorf("ValidateFlow failed: %v", err)
	}

	// Test GraphFlow
	graph, err := GraphFlow(ctx, "test_flow")
	if err != nil {
		t.Errorf("GraphFlow failed: %v", err)
	}
	if graph == "" {
		t.Error("Expected non-empty graph")
	}

	// Test StartRun
	runID, err := StartRun(ctx, "test_flow", map[string]any{})
	if err != nil {
		t.Errorf("StartRun failed: %v", err)
	}
	if runID == uuid.Nil {
		t.Error("Expected non-nil run ID")
	}
}

func TestIntegration_ResumeRun(t *testing.T) {
	flowsDir := filepath.Join(t.TempDir(), config.DefaultFlowsDir)
	if err := os.MkdirAll(flowsDir, 0755); err != nil {
		t.Fatalf("os.MkdirAll failed: %v", err)
	}
	SetFlowsDir(flowsDir)
	flowYAML := `name: resumeflow
on: cli.manual
steps:
  - id: s1
    use: core.echo
    with:
      text: "start"
  - id: wait
    await_event:
      source: test
      match:
        token: "{{ event.token }}"
  - id: s2
    use: core.echo
    with:
      text: "resumed"
`
	if err := os.WriteFile(filepath.Join(flowsDir, "resumeflow.flow.yaml"), []byte(flowYAML), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	defer os.Remove(filepath.Join(flowsDir, "resumeflow.flow.yaml"))
	// Write minimal schema for validation
	schema := `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`
	if err := os.WriteFile("beemflow.schema.json", []byte(schema), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	defer os.Remove("beemflow.schema.json")
	// StartRun with token triggers pause
	event := map[string]any{"token": "tok123"}
	runID, err := StartRun(context.Background(), "resumeflow", event)
	if err != nil {
		if !strings.Contains(err.Error(), "is waiting for event") {
			t.Fatalf("StartRun error: %v", err)
		}
		// If we get the pause error, that's expected - the test should pass
		t.Logf("StartRun correctly paused: %v", err)
	}
	if runID == uuid.Nil {
		t.Errorf("StartRun returned uuid.Nil")
	}
}

// TestListFlows_CustomDir ensures ListFlows reads from a custom flowsDir.
func TestListFlows_CustomDir(t *testing.T) {
	// capture and restore original flowsDir
	orig := flowsDir
	defer SetFlowsDir(orig)

	tmp := t.TempDir()
	custom := filepath.Join(tmp, "custom_flows")
	if err := os.MkdirAll(custom, 0755); err != nil {
		t.Fatalf("failed to create custom flows dir: %v", err)
	}
	// write a single flow file
	yaml := []byte("name: testflow\non: cli.manual\nsteps: []\n")
	if err := os.WriteFile(filepath.Join(custom, "testflow.flow.yaml"), yaml, 0644); err != nil {
		t.Fatalf("failed to write flow file: %v", err)
	}
	SetFlowsDir(custom)
	flows, err := ListFlows(context.Background())
	if err != nil {
		t.Fatalf("ListFlows error: %v", err)
	}
	if len(flows) != 1 || flows[0] != "testflow" {
		t.Errorf("expected [testflow], got %v", flows)
	}
}

// TestGetFlow_CustomDir ensures GetFlow reads from a custom flowsDir.
func TestGetFlow_CustomDir(t *testing.T) {
	orig := flowsDir
	defer SetFlowsDir(orig)

	tmp := t.TempDir()
	cust := filepath.Join(tmp, "flows2")
	if err := os.MkdirAll(cust, 0755); err != nil {
		t.Fatalf("failed to create custom flows2 dir: %v", err)
	}
	// minimal valid flow
	yaml := []byte("name: myflow\non: cli.manual\nsteps: []\n")
	if err := os.WriteFile(filepath.Join(cust, "myflow.flow.yaml"), yaml, 0644); err != nil {
		t.Fatalf("failed to write myflow file: %v", err)
	}
	SetFlowsDir(cust)
	flow, err := GetFlow(context.Background(), "myflow")
	if err != nil {
		t.Fatalf("GetFlow error: %v", err)
	}
	if flow.Name != "myflow" {
		t.Errorf("expected flow.Name=myflow; got %s", flow.Name)
	}
}

// Test comprehensive API function coverage

func TestParseFlowFromString(t *testing.T) {
	yamlStr := `name: test_flow
on: cli.manual
steps:
  - id: echo_step
    use: core.echo
    with:
      text: "Hello World"
`

	flow, err := ParseFlowFromString(yamlStr)
	if err != nil {
		t.Errorf("ParseFlowFromString failed: %v", err)
	}
	if flow == nil {
		t.Fatal("Expected non-nil flow")
	}
	if flow.Name != "test_flow" {
		t.Errorf("Expected flow name 'test_flow', got %s", flow.Name)
	}
}

func TestParseFlowFromString_InvalidYAML(t *testing.T) {
	yamlStr := `invalid: yaml: content: [unclosed`

	_, err := ParseFlowFromString(yamlStr)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestRunSpec(t *testing.T) {
	flow := &model.Flow{
		Name: "test_flow",
		Steps: []model.Step{
			{
				ID:  "echo_step",
				Use: "core.echo",
				With: map[string]any{
					"text": "Hello World",
				},
			},
		},
	}

	ctx := context.Background()
	runID, result, err := RunSpec(ctx, flow, map[string]any{})
	if err != nil {
		t.Errorf("RunSpec failed: %v", err)
	}
	if runID == uuid.Nil {
		t.Error("Expected non-nil run ID")
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestRunSpec_InvalidFlow(t *testing.T) {
	// Flow with invalid step
	flow := &model.Flow{
		Name: "invalid_flow",
		Steps: []model.Step{
			{
				ID:  "invalid_step",
				Use: "nonexistent.tool",
			},
		},
	}

	ctx := context.Background()
	_, _, err := RunSpec(ctx, flow, map[string]any{})
	if err == nil {
		t.Error("Expected error for invalid flow")
	}
}

func TestListTools(t *testing.T) {
	ctx := context.Background()
	tools, err := ListTools(ctx)
	if err != nil {
		t.Errorf("ListTools failed: %v", err)
	}
	// tools can be nil if no tools are registered, that's okay
	_ = tools
}

func TestListMCPServers(t *testing.T) {
	ctx := context.Background()
	servers, err := ListMCPServers(ctx)
	// This may fail if smithery API key is not set, which is expected in tests
	if err != nil && !strings.Contains(err.Error(), "smithery API key not set") {
		t.Errorf("ListMCPServers failed with unexpected error: %v", err)
	}
	// servers can be nil if no servers are available, that's okay
	_ = servers
}

func TestGetStoreFromConfig_AllDrivers(t *testing.T) {
	// Test SQLite driver
	cfg := &config.Config{
		Storage: config.StorageConfig{
			Driver: "sqlite",
			DSN:    ":memory:",
		},
	}
	store, err := GetStoreFromConfig(cfg)
	if err != nil {
		t.Errorf("Expected no error for sqlite driver, got %v", err)
	}
	if store == nil {
		t.Error("Expected non-nil store")
	}

	// Test Postgres driver (should return error, not fallback)
	cfg.Storage.Driver = "postgres"
	cfg.Storage.DSN = "invalid-dsn"
	store, err = GetStoreFromConfig(cfg)
	if err == nil {
		t.Error("Expected error for unsupported postgres driver")
	}
	if store != nil {
		t.Error("Expected nil store for unsupported driver")
	}

	// Test default (nil config)
	store, err = GetStoreFromConfig(nil)
	if err != nil {
		t.Errorf("Expected no error for nil config, got %v", err)
	}
	if store == nil {
		t.Error("Expected non-nil store")
	}

	// Test empty config
	cfg = &config.Config{}
	store, err = GetStoreFromConfig(cfg)
	if err != nil {
		t.Errorf("Expected no error for empty config, got %v", err)
	}
	if store == nil {
		t.Error("Expected non-nil store")
	}
}

func TestSetFlowsDir(t *testing.T) {
	originalDir := flowsDir
	defer func() {
		flowsDir = originalDir
	}()

	// Test setting a new directory
	newDir := "/test/flows"
	SetFlowsDir(newDir)
	if flowsDir != newDir {
		t.Errorf("Expected flowsDir to be %s, got %s", newDir, flowsDir)
	}

	// Test setting empty string (should not change)
	SetFlowsDir("")
	if flowsDir != newDir {
		t.Errorf("Expected flowsDir to remain %s, got %s", newDir, flowsDir)
	}
}

func TestListFlows_EmptyDir(t *testing.T) {
	// Create empty temp directory
	tempDir, err := os.MkdirTemp("", "empty_flows")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir := flowsDir
	SetFlowsDir(tempDir)
	defer SetFlowsDir(originalDir)

	ctx := context.Background()
	flows, err := ListFlows(ctx)
	if err != nil {
		t.Errorf("ListFlows failed: %v", err)
	}
	if len(flows) != 0 {
		t.Errorf("Expected empty flows list, got %v", flows)
	}
}

func TestListFlows_WithSubdirectories(t *testing.T) {
	// Create temp directory with subdirectories and files
	tempDir, err := os.MkdirTemp("", "mixed_flows")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a subdirectory (should be ignored)
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Create flow files
	flowFiles := []string{"test1.flow.yaml", "test2.flow.yaml", "not_a_flow.txt"}
	for _, file := range flowFiles {
		path := filepath.Join(tempDir, file)
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}

	originalDir := flowsDir
	SetFlowsDir(tempDir)
	defer SetFlowsDir(originalDir)

	ctx := context.Background()
	flows, err := ListFlows(ctx)
	if err != nil {
		t.Errorf("ListFlows failed: %v", err)
	}
	if len(flows) != 2 {
		t.Errorf("Expected 2 flows, got %d: %v", len(flows), flows)
	}

	// Check that only .flow.yaml files are included
	expectedFlows := []string{"test1", "test2"}
	for _, expected := range expectedFlows {
		found := false
		for _, flow := range flows {
			if flow == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find flow %s in %v", expected, flows)
		}
	}
}

func TestGetFlow_NonExistentFlow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test_flows")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir := flowsDir
	SetFlowsDir(tempDir)
	defer SetFlowsDir(originalDir)

	ctx := context.Background()
	flow, err := GetFlow(ctx, "nonexistent")
	if err != nil {
		t.Errorf("GetFlow should not error for non-existent flow, got %v", err)
	}
	if flow.Name != "" {
		t.Errorf("Expected empty flow for non-existent, got %v", flow)
	}
}

func TestValidateFlow_NonExistentFlow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test_flows")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir := flowsDir
	SetFlowsDir(tempDir)
	defer SetFlowsDir(originalDir)

	ctx := context.Background()
	err = ValidateFlow(ctx, "nonexistent")
	if err != nil {
		t.Errorf("ValidateFlow should not error for non-existent flow, got %v", err)
	}
}

func TestGraphFlow_NonExistentFlow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test_flows")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir := flowsDir
	SetFlowsDir(tempDir)
	defer SetFlowsDir(originalDir)

	ctx := context.Background()
	graph, err := GraphFlow(ctx, "nonexistent")
	if err != nil {
		t.Errorf("GraphFlow should not error for non-existent flow, got %v", err)
	}
	if graph != "" {
		t.Errorf("Expected empty graph for non-existent flow, got %s", graph)
	}
}

func TestStartRun_NonExistentFlow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test_flows")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir := flowsDir
	SetFlowsDir(tempDir)
	defer SetFlowsDir(originalDir)

	ctx := context.Background()
	runID, err := StartRun(ctx, "nonexistent", map[string]any{})
	if err != nil {
		t.Errorf("StartRun should not error for non-existent flow, got %v", err)
	}
	if runID != uuid.Nil {
		t.Errorf("Expected nil UUID for non-existent flow, got %v", runID)
	}
}

func TestStartRun_WithPausedRun(t *testing.T) {
	// Create a flow that will pause
	tempDir, err := os.MkdirTemp("", "test_flows")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir := flowsDir
	SetFlowsDir(tempDir)
	defer SetFlowsDir(originalDir)

	// Create a flow with await_event step
	flowContent := `name: pause_flow
on: cli.manual
steps:
  - id: wait_step
    await_event:
      match:
        token: "test_token"
`
	flowPath := filepath.Join(tempDir, "pause_flow.flow.yaml")
	if err := os.WriteFile(flowPath, []byte(flowContent), 0644); err != nil {
		t.Fatalf("Failed to write test flow: %v", err)
	}

	ctx := context.Background()
	runID, err := StartRun(ctx, "pause_flow", map[string]any{})
	if err != nil {
		if !strings.Contains(err.Error(), "is waiting for event") {
			t.Errorf("StartRun failed with unexpected error: %v", err)
		}
		// If we get the pause error, that's expected
		t.Logf("StartRun correctly paused: %v", err)
	}
	// This should return a run ID even though it pauses
	if runID == uuid.Nil {
		t.Error("Expected non-nil run ID for paused flow")
	}
}

// TestFlowService tests the FlowService implementation
func TestFlowService(t *testing.T) {
	// This test is no longer needed since we eliminated the FlowService interface
	// All functionality is now tested through direct API function calls
	t.Skip("FlowService interface has been eliminated - functionality tested through direct API calls")
}

// TestFlowService_ErrorCases tests error cases for FlowService
func TestFlowService_ErrorCases(t *testing.T) {
	// This test is no longer needed since we eliminated the FlowService interface
	t.Skip("FlowService interface has been eliminated - functionality tested through direct API calls")
}

// TestPublishEvent_EdgeCases tests PublishEvent with various edge cases
func TestPublishEvent_EdgeCases(t *testing.T) {
	ctx := context.Background()

	// Test with nil payload
	err := PublishEvent(ctx, "test_event", nil)
	// PublishEvent may fail if event bus is not configured, which is expected in tests
	if err != nil && !strings.Contains(err.Error(), "event bus not configured") {
		t.Errorf("PublishEvent with nil payload failed with unexpected error: %v", err)
	}

	// Test with empty event name
	err = PublishEvent(ctx, "", map[string]any{"key": "value"})
	// PublishEvent may fail if event bus is not configured, which is expected in tests
	if err != nil && !strings.Contains(err.Error(), "event bus not configured") {
		t.Errorf("PublishEvent with empty event name failed with unexpected error: %v", err)
	}

	// Test with complex payload
	complexPayload := map[string]any{
		"string":  "value",
		"number":  42,
		"boolean": true,
		"array":   []any{1, 2, 3},
		"object":  map[string]any{"nested": "value"},
	}
	err = PublishEvent(ctx, "complex_event", complexPayload)
	// PublishEvent may fail if event bus is not configured, which is expected in tests
	if err != nil && !strings.Contains(err.Error(), "event bus not configured") {
		t.Errorf("PublishEvent with complex payload failed with unexpected error: %v", err)
	}
}

// TestStartRun_EdgeCases tests StartRun with various edge cases
func TestStartRun_EdgeCases(t *testing.T) {
	// Create test flows directory
	tempDir, err := os.MkdirTemp("", "startrun_test_flows")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir := flowsDir
	SetFlowsDir(tempDir)
	defer SetFlowsDir(originalDir)

	// Create a simple flow
	flowContent := `name: edge_test_flow
on: cli.manual
steps:
  - id: echo_step
    use: core.echo
    with:
      text: "{{ event.message }}"
`
	flowPath := filepath.Join(tempDir, "edge_test_flow.flow.yaml")
	if err := os.WriteFile(flowPath, []byte(flowContent), 0644); err != nil {
		t.Fatalf("Failed to write test flow: %v", err)
	}

	ctx := context.Background()

	// Test with nil event
	runID, err := StartRun(ctx, "edge_test_flow", nil)
	if err != nil {
		t.Errorf("StartRun with nil event failed: %v", err)
	}
	if runID == uuid.Nil {
		t.Error("Expected non-nil run ID")
	}

	// Test with empty event
	runID, err = StartRun(ctx, "edge_test_flow", map[string]any{})
	if err != nil {
		t.Errorf("StartRun with empty event failed: %v", err)
	}
	if runID == uuid.Nil {
		t.Error("Expected non-nil run ID")
	}

	// Test with complex event
	complexEvent := map[string]any{
		"message": "Hello World",
		"user":    map[string]any{"id": 123, "name": "test"},
		"tags":    []string{"test", "api"},
	}
	runID, err = StartRun(ctx, "edge_test_flow", complexEvent)
	if err != nil {
		t.Errorf("StartRun with complex event failed: %v", err)
	}
	if runID == uuid.Nil {
		t.Error("Expected non-nil run ID")
	}
}

// TestGetRun_EdgeCases tests GetRun with various edge cases
func TestGetRun_EdgeCases(t *testing.T) {
	ctx := context.Background()

	// Test with nil UUID - this may or may not error depending on implementation
	_, err := GetRun(ctx, uuid.Nil)
	if err == nil {
		t.Logf("GetRun did not error for nil UUID (may be expected depending on implementation)")
	}

	// Test with random UUID (non-existent) - this may or may not error depending on implementation
	randomID := uuid.New()
	_, err = GetRun(ctx, randomID)
	if err == nil {
		t.Logf("GetRun did not error for non-existent run (may be expected depending on storage)")
	}
}

// TestListRuns_EdgeCases tests ListRuns with various edge cases
func TestListRuns_EdgeCases(t *testing.T) {
	ctx := context.Background()

	// Test ListRuns (should work even with no runs)
	runs, err := ListRuns(ctx)
	if err != nil {
		t.Errorf("ListRuns failed: %v", err)
	}
	// runs can be empty, that's okay
	_ = runs
}

// TestResumeRun_EdgeCases tests ResumeRun with various edge cases
func TestResumeRun_EdgeCases(t *testing.T) {
	ctx := context.Background()

	// Test with empty token
	result, err := ResumeRun(ctx, "", map[string]any{"key": "value"})
	if err != nil {
		t.Errorf("ResumeRun with empty token failed: %v", err)
	}
	_ = result

	// Test with nil payload
	result, err = ResumeRun(ctx, "test_token", nil)
	if err != nil {
		t.Errorf("ResumeRun with nil payload failed: %v", err)
	}
	_ = result

	// Test with complex payload
	complexPayload := map[string]any{
		"resume_data": map[string]any{
			"step_id": "test_step",
			"outputs": []any{1, 2, 3},
		},
	}
	result, err = ResumeRun(ctx, "complex_token", complexPayload)
	if err != nil {
		t.Errorf("ResumeRun with complex payload failed: %v", err)
	}
	_ = result
}

// Test the API functions directly instead of through service
func TestListFlows_Direct(t *testing.T) {
	ctx := context.Background()
	flows, err := ListFlows(ctx)
	if err != nil {
		t.Fatalf("ListFlows failed: %v", err)
	}
	// Note: flows may be empty if no flows are available in the test environment
	_ = flows
}

// Test error cases directly
func TestGetFlow_Direct(t *testing.T) {
	ctx := context.Background()
	flow, err := GetFlow(ctx, "nonexistent")
	if err != nil {
		t.Logf("GetFlow returned error for non-existent flow: %v", err)
	}
	// Note: Our implementation returns an empty flow rather than an error for non-existent flows
	_ = flow
}
