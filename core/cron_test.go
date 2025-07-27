package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/awantoch/beemflow/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShellQuote tests the shell quoting function
func TestShellQuote(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", `"simple"`},
		{"with spaces", `"with spaces"`},
		{"with'quote", `"with'quote"`},
		{`with"doublequote`, `"with\"doublequote"`},
		{"$(command)", `"$(command)"`},
		{"`backticks`", `"` + "`backticks`" + `"`},
		{"$variable", `"$variable"`},
		{";semicolon", `";semicolon"`},
		{"&ampersand", `"&ampersand"`},
		{"|pipe", `"|pipe"`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shellQuote(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestCronEndpoint tests the /cron endpoint functionality
func TestCron_GlobalEndpoint(t *testing.T) {
	// Create temp directory for test workflows
	tempDir, err := os.MkdirTemp("", "test-cron-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	oldDir := flowsDir
	SetFlowsDir(tempDir)
	defer SetFlowsDir(oldDir)

	// Test workflows
	testFlows := []struct {
		name          string
		yaml          string
		shouldTrigger bool
	}{
		{
			name: "scheduled_workflow",
			yaml: `name: scheduled_workflow
on: schedule.cron
steps:
  - id: test
    use: core.echo
    with:
      text: "Scheduled task"`,
			shouldTrigger: true,
		},
		{
			name: "http_workflow",
			yaml: `name: http_workflow
on: http.request
steps:
  - id: test
    use: core.echo
    with:
      text: "HTTP triggered"`,
			shouldTrigger: false,
		},
		{
			name: "multi_trigger_with_cron",
			yaml: `name: multi_trigger_with_cron
on:
  - schedule.cron
  - http.request
steps:
  - id: test
    use: core.echo
    with:
      text: "Multi-trigger"`,
			shouldTrigger: true,
		},
	}

	// Create test workflow files
	for _, tf := range testFlows {
		filePath := filepath.Join(tempDir, tf.name+".flow.yaml")
		if err := os.WriteFile(filePath, []byte(tf.yaml), 0644); err != nil {
			t.Fatalf("Failed to write test flow %s: %v", tf.name, err)
		}
	}

	// Get cron operation
	cronOp, exists := GetOperation("system_cron")
	if !exists || cronOp.HTTPHandler == nil {
		t.Fatal("system_cron operation not found or has no HTTPHandler")
	}

	// Test the endpoint
	req := httptest.NewRequest(http.MethodPost, "/cron", nil)
	// Add storage to request context
	store := storage.NewMemoryStorage()
	req = req.WithContext(WithStore(req.Context(), store))
	w := httptest.NewRecorder()

	cronOp.HTTPHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// The new architecture processes schedules asynchronously
	// Just verify the endpoint responds correctly
	if status, ok := response["status"].(string); !ok || status != "completed" {
		t.Error("Expected status 'completed' in response")
	}

	// Verify structure exists (even if empty for new architecture)
	if _, ok := response["triggered"]; !ok {
		t.Error("Missing triggered count in response")
	}

	if _, ok := response["results"]; !ok {
		t.Error("Missing results in response")
	}

	// Note: The new cron system uses storage-based scheduling and async events
	// It doesn't immediately trigger workflows in the HTTP response
	// Testing actual workflow triggering would require integration tests
	// with a full event bus and storage setup
}

func TestCron_TriggerWorkflow(t *testing.T) {
	// Create temp directory with test workflow
	tmpDir, err := os.MkdirTemp("", "cron_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a workflow with schedule.cron trigger
	testFlow := `name: test_cron_workflow
on: schedule.cron
cron: "0 9 * * *"

steps:
  - id: echo
    use: core.echo
    with:
      text: "Hello from cron!"
`
	flowPath := filepath.Join(tmpDir, "test_cron_workflow.flow.yaml")
	err = os.WriteFile(flowPath, []byte(testFlow), 0644)
	require.NoError(t, err)

	// Set flows directory
	SetFlowsDir(tmpDir)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/cron", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()

	// Get the operation handler
	op, exists := GetOperation("system_cron")
	require.True(t, exists)
	require.NotNil(t, op)
	require.NotNil(t, op.HTTPHandler)

	// Call handler
	op.HTTPHandler(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "completed", response["status"])
	assert.NotNil(t, response["triggered"])
	assert.NotNil(t, response["workflows"])
}

func TestCron_SpecificWorkflow(t *testing.T) {
	// Create temp directory with test workflow
	tmpDir, err := os.MkdirTemp("", "cron_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a workflow with schedule.cron trigger
	testFlow := `name: specific_workflow
on: schedule.cron
cron: "0 * * * *"

steps:
  - id: echo
    use: core.echo
    with:
      text: "Specific workflow triggered!"
`
	flowPath := filepath.Join(tmpDir, "specific_workflow.flow.yaml")
	err = os.WriteFile(flowPath, []byte(testFlow), 0644)
	require.NoError(t, err)

	// Set flows directory
	SetFlowsDir(tmpDir)

	// Create request for specific workflow
	req := httptest.NewRequest(http.MethodPost, "/cron/specific_workflow", nil)
	w := httptest.NewRecorder()

	// Get the operation handler
	op, exists := GetOperation("workflow_cron")
	require.True(t, exists)
	require.NotNil(t, op)
	require.NotNil(t, op.HTTPHandler)

	// Call handler
	op.HTTPHandler(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "triggered", response["status"])
	assert.Equal(t, "specific_workflow", response["workflow"])
	assert.NotEmpty(t, response["run_id"])
}

func TestCron_ValidationError(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "cron_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a workflow WITHOUT schedule.cron trigger
	testFlow := `name: non_cron_workflow
on: webhook

steps:
  - id: echo
    use: core.echo
    with:
      text: "Not a cron workflow"
`
	flowPath := filepath.Join(tmpDir, "non_cron_workflow.flow.yaml")
	err = os.WriteFile(flowPath, []byte(testFlow), 0644)
	require.NoError(t, err)

	// Set flows directory
	SetFlowsDir(tmpDir)

	// Try to trigger non-cron workflow
	req := httptest.NewRequest(http.MethodPost, "/cron/non_cron_workflow", nil)
	w := httptest.NewRecorder()

	op, exists := GetOperation("workflow_cron")
	require.True(t, exists)
	require.NotNil(t, op)

	op.HTTPHandler(w, req)

	// Should get bad request
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCron_Security(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "cron_security")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a workflow
	testFlow := `name: secure_workflow
on: schedule.cron
steps:
  - id: test
    use: core.echo`
	
	flowPath := filepath.Join(tmpDir, "secure_workflow.flow.yaml")
	os.WriteFile(flowPath, []byte(testFlow), 0644)
	SetFlowsDir(tmpDir)

	// Set CRON_SECRET
	os.Setenv("CRON_SECRET", "test-secret-123")
	defer os.Unsetenv("CRON_SECRET")

	op, _ := GetOperation("system_cron")

	t.Run("NoAuth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/cron", nil)
		w := httptest.NewRecorder()
		op.HTTPHandler(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("WrongAuth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/cron", nil)
		req.Header.Set("Authorization", "Bearer wrong-secret")
		w := httptest.NewRecorder()
		op.HTTPHandler(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("CorrectAuth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/cron", nil)
		req.Header.Set("Authorization", "Bearer test-secret-123")
		w := httptest.NewRecorder()
		op.HTTPHandler(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
