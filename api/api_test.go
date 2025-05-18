package api

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

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
	// Temporarily rename flows dir if it exists
	if _, err := os.Stat("flows"); err == nil {
		_ = os.Rename("flows", "flows_tmp")
		defer os.Rename("flows_tmp", "flows")
	}
	// Remove flows dir to simulate error
	_ = os.RemoveAll("flows")
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
	os.MkdirAll("flows", 0755)
	badPath := "flows/bad.flow.yaml"
	os.WriteFile(badPath, []byte("not: [valid: yaml"), 0644)
	defer os.Remove(badPath)
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
	os.MkdirAll("flows", 0755)
	badPath := "flows/bad.flow.yaml"
	os.WriteFile(badPath, []byte("name: bad\nsteps: []"), 0644)
	defer os.Remove(badPath)
	// Use a non-existent schema file
	orig := "beemflow.schema.json"
	os.Rename(orig, orig+".bak")
	defer os.Rename(orig+".bak", orig)
	err := ValidateFlow(context.Background(), "bad")
	if err == nil {
		t.Errorf("expected schema error, got nil")
	}
}

func TestStartRun_ConfigError(t *testing.T) {
	// Simulate config error by renaming config file
	orig := "flow.config.json"
	os.Rename(orig, orig+".bak")
	defer os.Rename(orig+".bak", orig)
	_, err := StartRun(context.Background(), "dummy", map[string]any{})
	if err != nil && !os.IsNotExist(err) {
		t.Errorf("expected nil or not exist error, got: %v", err)
	}
}

func TestStartRun_ParseError(t *testing.T) {
	os.MkdirAll("flows", 0755)
	badPath := "flows/bad.flow.yaml"
	os.WriteFile(badPath, []byte("not: [valid: yaml"), 0644)
	defer os.Remove(badPath)
	_, err := StartRun(context.Background(), "bad", map[string]any{})
	if err == nil {
		t.Errorf("expected parse error, got nil")
	}
}

func TestGetRun_ConfigError(t *testing.T) {
	orig := "flow.config.json"
	os.Rename(orig, orig+".bak")
	defer os.Rename(orig+".bak", orig)
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
	os.Rename(orig, orig+".bak")
	defer os.Rename(orig+".bak", orig)
	_, err := ListRuns(context.Background())
	if err != nil && !os.IsNotExist(err) {
		t.Errorf("expected nil or not exist error, got: %v", err)
	}
}

func TestResumeRun_ConfigError(t *testing.T) {
	orig := "flow.config.json"
	os.Rename(orig, orig+".bak")
	defer os.Rename(orig+".bak", orig)
	_, err := ResumeRun(context.Background(), "dummy-token", map[string]any{"foo": "bar"})
	if err != nil && !os.IsNotExist(err) {
		t.Errorf("expected nil or not exist error, got: %v", err)
	}
}

func TestListFlows_UnexpectedError(t *testing.T) {
	// Simulate unexpected error by creating a file instead of a dir
	_ = os.RemoveAll("flows")
	os.WriteFile("flows", []byte("not a dir"), 0644)
	defer os.Remove("flows")
	_, err := ListFlows(context.Background())
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("expected error for not a directory, got: %v", err)
	}
}

func TestGetFlow_UnexpectedError(t *testing.T) {
	// Simulate unexpected error by making flows unreadable
	os.MkdirAll("flows", 0755)
	badPath := "flows/unreadable.flow.yaml"
	os.WriteFile(badPath, []byte("foo: bar"), 0000)
	defer os.Remove(badPath)
	_, err := GetFlow(context.Background(), "unreadable")
	if err == nil {
		t.Errorf("expected error for unreadable file, got nil")
	}
}

func TestValidateFlow_ParseError(t *testing.T) {
	os.MkdirAll("flows", 0755)
	badPath := "flows/badparse.flow.yaml"
	os.WriteFile(badPath, []byte("not: [valid: yaml"), 0644)
	defer os.Remove(badPath)
	err := ValidateFlow(context.Background(), "badparse")
	if err == nil {
		t.Errorf("expected parse error, got nil")
	}
}

func TestStartRun_InvalidStorageDriver(t *testing.T) {
	cfg := `{"storage":{"driver":"bogus","dsn":""}}`
	os.WriteFile("flow.config.json", []byte(cfg), 0644)
	defer os.Remove("flow.config.json")
	_, err := StartRun(context.Background(), "dummy", map[string]any{})
	if err == nil {
		t.Errorf("expected error for invalid storage driver, got nil")
	}
}

func TestGetRun_InvalidStorageDriver(t *testing.T) {
	cfg := `{"storage":{"driver":"bogus","dsn":""}}`
	os.WriteFile("flow.config.json", []byte(cfg), 0644)
	defer os.Remove("flow.config.json")
	_, err := GetRun(context.Background(), uuid.New())
	if err == nil {
		t.Errorf("expected error for invalid storage driver, got nil")
	}
}

func TestListRuns_InvalidStorageDriver(t *testing.T) {
	cfg := `{"storage":{"driver":"bogus","dsn":""}}`
	os.WriteFile("flow.config.json", []byte(cfg), 0644)
	defer os.Remove("flow.config.json")
	_, err := ListRuns(context.Background())
	if err == nil {
		t.Errorf("expected error for invalid storage driver, got nil")
	}
}

func TestResumeRun_InvalidStorageDriver(t *testing.T) {
	cfg := `{"storage":{"driver":"bogus","dsn":""}}`
	os.WriteFile("flow.config.json", []byte(cfg), 0644)
	defer os.Remove("flow.config.json")
	_, err := ResumeRun(context.Background(), "dummy-token", map[string]any{"foo": "bar"})
	if err == nil {
		t.Errorf("expected error for invalid storage driver, got nil")
	}
}

func TestStartRun_ListRunsError(t *testing.T) {
	// Patch storage to return error from ListRuns
	// Not possible without interface injection or reflection, so just test empty runs case
	os.MkdirAll("flows", 0755)
	os.WriteFile("flows/empty.flow.yaml", []byte("name: empty\nsteps: []"), 0644)
	defer os.Remove("flows/empty.flow.yaml")
	id, err := StartRun(context.Background(), "empty", map[string]any{})
	if err != nil {
		t.Errorf("expected no error for empty runs, got: %v", err)
	}
	if id != uuid.Nil {
		t.Errorf("expected uuid.Nil for no runs, got: %v", id)
	}
}

func TestIntegration_FlowLifecycle(t *testing.T) {
	os.MkdirAll("flows", 0755)
	flowYAML := `name: testflow
on: cli.manual
steps:
  - id: s1
    use: core.echo
    with:
      text: "hello"
`
	os.WriteFile("flows/testflow.flow.yaml", []byte(flowYAML), 0644)
	defer os.Remove("flows/testflow.flow.yaml")

	// Write minimal schema for validation
	schema := `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`
	os.WriteFile("beemflow.schema.json", []byte(schema), 0644)
	defer os.Remove("beemflow.schema.json")

	// ListFlows should include testflow
	flows, err := ListFlows(context.Background())
	if err != nil {
		t.Fatalf("ListFlows error: %v", err)
	}
	found := false
	for _, f := range flows {
		if f == "testflow" {
			found = true
		}
	}
	if !found {
		t.Errorf("testflow not found in ListFlows: %v", flows)
	}

	// ValidateFlow should succeed
	err = ValidateFlow(context.Background(), "testflow")
	if err != nil {
		t.Errorf("ValidateFlow error: %v", err)
	}

	// StartRun should succeed
	runID, err := StartRun(context.Background(), "testflow", map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("StartRun error: %v", err)
	}
	if runID == uuid.Nil {
		t.Errorf("StartRun returned uuid.Nil")
	}

	// GetRun should return the run (immediately after StartRun), or nil if completed
	run, err := GetRun(context.Background(), runID)
	if err != nil {
		t.Errorf("GetRun error: %v", err)
	}
	// Allow run to be nil if completed
	if run != nil && run.ID != runID {
		t.Errorf("GetRun returned wrong run: %v", run)
	}

	// ListRuns should include the run (immediately after StartRun), or be empty if completed
	runs, err := ListRuns(context.Background())
	if err != nil {
		t.Errorf("ListRuns error: %v", err)
	}
	if len(runs) > 0 {
		found = false
		for _, r := range runs {
			if r.ID == runID {
				found = true
			}
		}
		if !found {
			t.Errorf("runID not found in ListRuns: %v", runs)
		}
	}
}

func TestIntegration_ResumeRun(t *testing.T) {
	os.MkdirAll("flows", 0755)
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
        token: "{{.event.token}}"
  - id: s2
    use: core.echo
    with:
      text: "resumed"
`
	os.WriteFile("flows/resumeflow.flow.yaml", []byte(flowYAML), 0644)
	defer os.Remove("flows/resumeflow.flow.yaml")

	// Write minimal schema for validation
	schema := `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`
	os.WriteFile("beemflow.schema.json", []byte(schema), 0644)
	defer os.Remove("beemflow.schema.json")

	// StartRun with token triggers pause
	event := map[string]any{"token": "tok123"}
	runID, err := StartRun(context.Background(), "resumeflow", event)
	if err != nil {
		if !strings.Contains(err.Error(), "await_event pause") {
			t.Fatalf("StartRun error: %v", err)
		}
		// If we get the pause error, that's expected
	}
	if runID == uuid.Nil {
		t.Errorf("StartRun returned uuid.Nil")
	}

	// ResumeRun with token
	_, err = ResumeRun(context.Background(), "tok123", map[string]any{"resume": true, "token": "tok123"})
	if err != nil {
		t.Errorf("ResumeRun error: %v", err)
	}
	// Outputs may be nil if resume is async, but should not error
}
