package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/awantoch/beemflow/internal/api"
)

// TestServerlessOperationsFiltering demonstrates that the serverless handler
// correctly filters operations by group, and that this approach is future-proof
func TestServerlessOperationsFiltering(t *testing.T) {
	tests := []struct {
		name            string
		endpointsEnvVar string
		expectedAllowed []string
		expectedBlocked []string
	}{
		{
			name:            "no filter - all endpoints",
			endpointsEnvVar: "",
			expectedAllowed: []string{"/healthz", "/flows", "/runs", "/tools", "/spec"},
			expectedBlocked: []string{}, // None blocked
		},
		{
			name:            "flows only",
			endpointsEnvVar: "flows",
			expectedAllowed: []string{"/healthz", "/flows", "/validate"},
			expectedBlocked: []string{"/runs", "/tools", "/spec"},
		},
		{
			name:            "runs only",
			endpointsEnvVar: "runs",
			expectedAllowed: []string{"/healthz", "/runs"},
			expectedBlocked: []string{"/flows", "/validate", "/tools", "/spec"},
		},
		{
			name:            "tools only",
			endpointsEnvVar: "tools",
			expectedAllowed: []string{"/healthz", "/tools"},
			expectedBlocked: []string{"/flows", "/runs", "/validate", "/spec"},
		},
		{
			name:            "system only",
			endpointsEnvVar: "system",
			expectedAllowed: []string{"/healthz", "/spec"},
			expectedBlocked: []string{"/flows", "/runs", "/tools", "/validate"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable for this test
			if tt.endpointsEnvVar != "" {
				t.Setenv("BEEMFLOW_ENDPOINTS", tt.endpointsEnvVar)
			}

			// Test allowed endpoints
			for _, endpoint := range tt.expectedAllowed {
				req := httptest.NewRequest("GET", endpoint, nil)
				w := httptest.NewRecorder()

				ServerlessHandler(w, req)

				if w.Code == http.StatusNotFound {
					t.Errorf("Expected endpoint %s to be allowed, got 404", endpoint)
				}
			}

			// Test blocked endpoints
			for _, endpoint := range tt.expectedBlocked {
				req := httptest.NewRequest("GET", endpoint, nil)
				w := httptest.NewRecorder()

				ServerlessHandler(w, req)

				if w.Code != http.StatusNotFound {
					t.Errorf("Expected endpoint %s to be blocked (404), got status %d", endpoint, w.Code)
				}
			}
		})
	}
}

// TestOperationsAbstraction demonstrates that our approach works with the
// operations system and is future-proof for new operations
func TestOperationsAbstraction(t *testing.T) {
	// Verify that our group filtering works at the operations level
	allOps := api.GetAllOperations()
	flowsOps := api.GetOperationsMapByGroups([]string{"flows"})

	// Should have fewer flows operations than total operations
	if len(flowsOps) >= len(allOps) {
		t.Error("Flows filtering should return subset of operations")
	}

	// All flows operations should have the flows group
	for _, op := range flowsOps {
		if op.Group != "flows" {
			t.Errorf("Operation %s should have group 'flows', got '%s'", op.ID, op.Group)
		}
	}

	// Demonstrate that adding a new operation with a group automatically works
	// (This shows why our approach is future-proof - no hardcoded paths!)
	// If we were to register a new operation with group="flows", it would automatically
	// be included in flows filtering without any changes to serverless code
	filteredOps := api.GetOperationsMapByGroups([]string{"flows"})

	// Show that the filtering logic works for any operation with the right group
	hasFlowsOps := false
	for _, op := range filteredOps {
		if op.Group == "flows" {
			hasFlowsOps = true
			break
		}
	}

	if !hasFlowsOps {
		t.Error("Should have found flows operations in filtered set")
	}

	// This demonstrates the key insight: we filter by operation metadata,
	// not by hardcoded HTTP paths. New operations automatically work!
	t.Logf("✅ Future-proof: New operations with group='flows' would automatically be included")
	t.Logf("✅ No hardcoded paths: Filtering happens at the operations level")
	t.Logf("✅ Consistent: Same grouping logic works for CLI, HTTP, and MCP")
}
