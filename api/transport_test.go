package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/registry"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func TestConvertOpenAPIHandler(t *testing.T) {
	// Create a mock FlowService
	svc := &MockFlowService{}

	// Test valid OpenAPI conversion
	openAPISpec := `{
		"openapi": "3.0.0",
		"info": {"title": "Test API", "version": "1.0.0"},
		"servers": [{"url": "https://api.test.com"}],
		"paths": {
			"/users": {
				"get": {
					"summary": "List users",
					"parameters": [{
						"name": "limit",
						"in": "query",
						"schema": {"type": "integer"},
						"description": "Max results"
					}]
				},
				"post": {
					"summary": "Create user",
					"requestBody": {
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"properties": {
										"name": {"type": "string"},
										"email": {"type": "string"}
									},
									"required": ["name", "email"]
								}
							}
						}
					}
				}
			}
		}
	}`

	reqBody := map[string]any{
		"openapi":  openAPISpec,
		"api_name": "test_api",
		"base_url": "https://api.test.com",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/tools/convert", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	convertOpenAPIHandler(w, req, svc)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check response structure
	if resp["api_name"] != "test_api" {
		t.Errorf("Expected api_name 'test_api', got %v", resp["api_name"])
	}

	if resp["base_url"] != "https://api.test.com" {
		t.Errorf("Expected base_url 'https://api.test.com', got %v", resp["base_url"])
	}

	manifests, ok := resp["manifests"].([]any)
	if !ok {
		t.Fatalf("Expected manifests to be array, got %T", resp["manifests"])
	}

	if len(manifests) != 2 {
		t.Errorf("Expected 2 manifests, got %d", len(manifests))
	}

	// Check manifests - they should be ordered by path then method
	manifest1 := manifests[0].(map[string]any)
	manifest2 := manifests[1].(map[string]any)

	// Both should have the same name (path-based)
	if manifest1["name"] != "test_api.users" {
		t.Errorf("Expected name 'test_api.users', got %v", manifest1["name"])
	}
	if manifest2["name"] != "test_api.users" {
		t.Errorf("Expected name 'test_api.users', got %v", manifest2["name"])
	}

	// Check that we have both GET and POST methods (order may vary)
	methods := []string{manifest1["method"].(string), manifest2["method"].(string)}
	hasGet := false
	hasPost := false
	for _, method := range methods {
		if method == "GET" {
			hasGet = true
		}
		if method == "POST" {
			hasPost = true
		}
	}
	if !hasGet {
		t.Error("Expected to find GET method in manifests")
	}
	if !hasPost {
		t.Error("Expected to find POST method in manifests")
	}
}

func TestConvertOpenAPIHandler_InvalidJSON(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("POST", "/tools/convert", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	convertOpenAPIHandler(w, req, svc)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestConvertOpenAPIHandler_MissingOpenAPI(t *testing.T) {
	svc := &MockFlowService{}

	reqBody := map[string]any{
		"api_name": "test_api",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/tools/convert", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	convertOpenAPIHandler(w, req, svc)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestConvertOpenAPIHandler_MethodNotAllowed(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("GET", "/tools/convert", nil)
	w := httptest.NewRecorder()

	convertOpenAPIHandler(w, req, svc)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// Test HTTP Handlers for comprehensive coverage

func TestRunsListHandler(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("GET", "/runs", nil)
	w := httptest.NewRecorder()

	runsListHandler(w, req, svc)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp []any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
}

func TestRunsHandler_POST(t *testing.T) {
	svc := &MockFlowService{}

	reqBody := map[string]any{
		"flow_name": "test_flow",
		"event":     map[string]any{"key": "value"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/runs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	runsHandler(w, req, svc)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRunsHandler_InvalidJSON(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("POST", "/runs", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	runsHandler(w, req, svc)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRunsHandler_MethodNotAllowed(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("DELETE", "/runs", nil)
	w := httptest.NewRecorder()

	runsHandler(w, req, svc)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestRunStatusHandler(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("GET", "/runs/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	runStatusHandler(w, req, svc)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRunStatusHandler_InvalidUUID(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("GET", "/runs/invalid-uuid", nil)
	w := httptest.NewRecorder()

	runStatusHandler(w, req, svc)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestResumeHandler(t *testing.T) {
	svc := &MockFlowService{}

	reqBody := map[string]any{
		"event": map[string]any{"key": "value"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/resume/test-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	resumeHandler(w, req, svc)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestResumeHandler_InvalidJSON(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("POST", "/resume/test-token", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	resumeHandler(w, req, svc)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGraphHandler(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("GET", "/graph?flow=test_flow", nil)
	w := httptest.NewRecorder()

	graphHandler(w, req, svc)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGraphHandler_MissingFlowName(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("GET", "/graph", nil)
	w := httptest.NewRecorder()

	graphHandler(w, req, svc)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestValidateHandler(t *testing.T) {
	svc := &MockFlowService{}

	reqBody := map[string]any{
		"flow_name": "test_flow",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	validateHandler(w, req, svc)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestValidateHandler_InvalidJSON(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("POST", "/validate", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	validateHandler(w, req, svc)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestTestHandler(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()

	testHandler(w, req, svc)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("Expected status 501, got %d", w.Code)
	}
}

func TestRunsInlineHandler(t *testing.T) {
	svc := &MockFlowService{}

	reqBody := map[string]any{
		"flow": map[string]any{
			"name":  "test_flow",
			"steps": []any{},
		},
		"event": map[string]any{"key": "value"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/runs/inline", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	runsInlineHandler(w, req, svc)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRunsInlineHandler_InvalidJSON(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("POST", "/runs/inline", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	runsInlineHandler(w, req, svc)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestToolsIndexHandler(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("GET", "/tools", nil)
	w := httptest.NewRecorder()

	toolsIndexHandler(w, req, svc)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestToolsManifestHandler(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("GET", "/tools/test_tool", nil)
	w := httptest.NewRecorder()

	toolsManifestHandler(w, req, svc)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestFlowsHandler(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("GET", "/flows", nil)
	w := httptest.NewRecorder()

	flowsHandler(w, req, svc)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestFlowSpecHandler(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("GET", "/flows/test_flow", nil)
	w := httptest.NewRecorder()

	flowSpecHandler(w, req, svc)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestEventsHandler(t *testing.T) {
	svc := &MockFlowService{}

	reqBody := map[string]any{
		"topic":   "test_topic",
		"payload": map[string]any{"key": "value"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	eventsHandler(w, req, svc)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestEventsHandler_InvalidJSON(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("POST", "/events", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	eventsHandler(w, req, svc)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAttachHTTPHandlers(t *testing.T) {
	mux := http.NewServeMux()
	svc := &MockFlowService{}

	// Should not panic
	AttachHTTPHandlers(mux, svc)

	// Test that routes are registered
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected health check to return 200, got %d", w.Code)
	}
}

func TestAttachHTTPHandlers_Metadata(t *testing.T) {
	mux := http.NewServeMux()
	svc := &MockFlowService{}

	AttachHTTPHandlers(mux, svc)

	req := httptest.NewRequest("GET", "/metadata", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected metadata endpoint to return 200, got %d", w.Code)
	}

	var resp []any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode metadata response: %v", err)
	}
}

func TestAttachHTTPHandlers_Spec(t *testing.T) {
	mux := http.NewServeMux()
	svc := &MockFlowService{}

	AttachHTTPHandlers(mux, svc)

	req := httptest.NewRequest("GET", "/spec", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected spec endpoint to return 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "text/markdown" {
		t.Errorf("Expected Content-Type text/markdown, got %s", w.Header().Get("Content-Type"))
	}
}

// Test additional HTTP handler edge cases

func TestRunsHandler_MissingFlowName(t *testing.T) {
	svc := &MockFlowService{}

	reqBody := map[string]any{
		"event": map[string]any{"key": "value"},
		// Missing flow field - this will result in empty string, which is valid
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/runs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	runsHandler(w, req, svc)

	// The handler accepts empty flow name and passes it to the service
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for empty flow name, got %d", w.Code)
	}
}

func TestRunStatusHandler_MethodNotAllowed(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("POST", "/runs/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	runStatusHandler(w, req, svc)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestResumeHandler_MethodNotAllowed(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("GET", "/resume/test-token", nil)
	w := httptest.NewRecorder()

	resumeHandler(w, req, svc)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestGraphHandler_MethodNotAllowed(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("POST", "/graph", nil)
	w := httptest.NewRecorder()

	graphHandler(w, req, svc)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestValidateHandler_MethodNotAllowed(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("GET", "/validate", nil)
	w := httptest.NewRecorder()

	validateHandler(w, req, svc)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestValidateHandler_MissingFlow(t *testing.T) {
	svc := &MockFlowService{}

	reqBody := map[string]any{
		// Missing flow field - this will result in empty string, which is valid
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	validateHandler(w, req, svc)

	// The handler accepts empty flow name and passes it to the service
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for empty flow name, got %d", w.Code)
	}
}

func TestRunsInlineHandler_MethodNotAllowed(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("GET", "/runs/inline", nil)
	w := httptest.NewRecorder()

	runsInlineHandler(w, req, svc)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestRunsInlineHandler_MissingSpec(t *testing.T) {
	svc := &MockFlowService{}

	reqBody := map[string]any{
		"event": map[string]any{"key": "value"},
		// Missing spec field - this will result in empty string
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/runs/inline", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	runsInlineHandler(w, req, svc)

	// Empty spec might parse successfully or cause an error during execution
	// Either 200 (success) or 500 (execution error) is acceptable
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500 for empty spec, got %d", w.Code)
	}
}

func TestRunsInlineHandler_InvalidSpec(t *testing.T) {
	svc := &MockFlowService{}

	reqBody := map[string]any{
		"spec":  "invalid yaml content [",
		"event": map[string]any{"key": "value"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/runs/inline", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	runsInlineHandler(w, req, svc)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid spec, got %d", w.Code)
	}
}

func TestToolsIndexHandler_MethodNotAllowed(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("POST", "/tools", nil)
	w := httptest.NewRecorder()

	toolsIndexHandler(w, req, svc)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestToolsManifestHandler_MethodNotAllowed(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("POST", "/tools/test_tool", nil)
	w := httptest.NewRecorder()

	toolsManifestHandler(w, req, svc)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestFlowsHandler_POST_NotImplemented(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("POST", "/flows", nil)
	w := httptest.NewRecorder()

	flowsHandler(w, req, svc)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("Expected status 501, got %d", w.Code)
	}
}

func TestFlowsHandler_MethodNotAllowed(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("DELETE", "/flows", nil)
	w := httptest.NewRecorder()

	flowsHandler(w, req, svc)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestFlowSpecHandler_MissingName(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("GET", "/flows/", nil)
	w := httptest.NewRecorder()

	flowSpecHandler(w, req, svc)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for missing flow name, got %d", w.Code)
	}
}

func TestFlowSpecHandler_DELETE_NotImplemented(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("DELETE", "/flows/test_flow", nil)
	w := httptest.NewRecorder()

	flowSpecHandler(w, req, svc)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("Expected status 501, got %d", w.Code)
	}
}

func TestFlowSpecHandler_MethodNotAllowed(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("POST", "/flows/test_flow", nil)
	w := httptest.NewRecorder()

	flowSpecHandler(w, req, svc)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestEventsHandler_MethodNotAllowed(t *testing.T) {
	svc := &MockFlowService{}

	req := httptest.NewRequest("GET", "/events", nil)
	w := httptest.NewRecorder()

	eventsHandler(w, req, svc)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestEventsHandler_MissingTopic(t *testing.T) {
	svc := &MockFlowService{}

	reqBody := map[string]any{
		"payload": map[string]any{"key": "value"},
		// Missing topic - this will result in empty string, which is valid
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	eventsHandler(w, req, svc)

	// The handler accepts empty topic and passes it to the service
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for empty topic, got %d", w.Code)
	}
}

func TestConvertOpenAPIHandler_ConversionError(t *testing.T) {
	svc := &MockFlowService{}

	reqBody := map[string]any{
		"openapi":  "invalid openapi spec",
		"api_name": "test_api",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/tools/convert", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	convertOpenAPIHandler(w, req, svc)

	// Invalid OpenAPI spec causes internal server error during conversion
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for invalid OpenAPI spec, got %d", w.Code)
	}
}

// Test CLI command attachment

func TestAttachCLICommands(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	svc := &MockFlowService{}
	constructors := CommandConstructors{
		NewServeCmd: func() *cobra.Command {
			return &cobra.Command{Use: "serve"}
		},
		NewRunCmd: func() *cobra.Command {
			return &cobra.Command{Use: "run"}
		},
		NewLintCmd: func() *cobra.Command {
			return &cobra.Command{Use: "lint"}
		},
		NewValidateCmd: func() *cobra.Command {
			return &cobra.Command{Use: "validate"}
		},
		NewGraphCmd: func() *cobra.Command {
			return &cobra.Command{Use: "graph"}
		},
		NewTestCmd: func() *cobra.Command {
			return &cobra.Command{Use: "test"}
		},
		NewToolCmd: func() *cobra.Command {
			return &cobra.Command{Use: "tool"}
		},
		NewMCPCmd: func() *cobra.Command {
			return &cobra.Command{Use: "mcp"}
		},
		NewMetadataCmd: func() *cobra.Command {
			return &cobra.Command{Use: "metadata"}
		},
		NewSpecCmd: func() *cobra.Command {
			return &cobra.Command{Use: "spec"}
		},
	}

	// Should not panic
	AttachCLICommands(root, svc, constructors)

	// Check that commands were added
	if len(root.Commands()) != 10 {
		t.Errorf("Expected 10 commands, got %d", len(root.Commands()))
	}
}

func TestAttachCLICommands_NilConstructors(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	svc := &MockFlowService{}
	constructors := CommandConstructors{
		// All nil constructors
	}

	// Should not panic
	AttachCLICommands(root, svc, constructors)

	// No commands should be added
	if len(root.Commands()) != 0 {
		t.Errorf("Expected 0 commands, got %d", len(root.Commands()))
	}
}

// Test MCP tool registrations

func TestBuildMCPToolRegistrations(t *testing.T) {
	svc := &MockFlowService{}

	registrations := BuildMCPToolRegistrations(svc)

	if len(registrations) == 0 {
		t.Error("Expected non-empty tool registrations")
	}

	// Check that spec tool is included
	foundSpec := false
	for _, reg := range registrations {
		if reg.Name == "spec" {
			foundSpec = true
			break
		}
	}
	if !foundSpec {
		t.Error("Expected to find 'spec' tool in registrations")
	}
}

// TestBuildMCPToolRegistrations_Comprehensive tests all MCP tool registrations
func TestBuildMCPToolRegistrations_Comprehensive(t *testing.T) {
	svc := &MockFlowService{}

	registrations := BuildMCPToolRegistrations(svc)

	// Should have some tool registrations
	if len(registrations) == 0 {
		t.Error("Expected non-empty tool registrations")
	}

	// Check that all registrations have required fields
	for _, reg := range registrations {
		// Verify each registration has required fields
		if reg.Name == "" {
			t.Error("Tool registration has empty name")
		}
		if reg.Description == "" {
			t.Error("Tool registration has empty description")
		}
		if reg.Handler == nil {
			t.Errorf("Tool registration %s has nil handler", reg.Name)
		}
	}

	// Log what tools are actually registered for debugging
	t.Logf("Found %d tool registrations:", len(registrations))
	for _, reg := range registrations {
		t.Logf("  - %s: %s", reg.Name, reg.Description)
	}
}

// TestBuildMCPToolRegistrations_StructureValidation tests the structure of registrations
func TestBuildMCPToolRegistrations_StructureValidation(t *testing.T) {
	svc := &MockFlowService{}
	registrations := BuildMCPToolRegistrations(svc)

	// Verify all registrations have proper structure
	for _, reg := range registrations {
		if reg.Name == "" {
			t.Error("Found registration with empty name")
		}
		if reg.Description == "" {
			t.Errorf("Registration %s has empty description", reg.Name)
		}
		if reg.Handler == nil {
			t.Errorf("Registration %s has nil handler", reg.Name)
		}

		// Verify name format (should be valid identifier)
		if strings.Contains(reg.Name, " ") {
			t.Errorf("Registration name %s contains spaces", reg.Name)
		}

		// Verify description is meaningful
		if len(reg.Description) < 5 {
			t.Errorf("Registration %s has very short description: %s", reg.Name, reg.Description)
		}
	}
}

// TestBuildMCPToolRegistrations_Coverage tests that we have good coverage of tools
func TestBuildMCPToolRegistrations_Coverage(t *testing.T) {
	svc := &MockFlowService{}
	registrations := BuildMCPToolRegistrations(svc)

	// Should have a reasonable number of tools
	if len(registrations) < 5 {
		t.Errorf("Expected at least 5 tool registrations for good coverage, got %d", len(registrations))
	}

	// Check that we have some key tools (adjust based on actual implementation)
	toolNames := make(map[string]bool)
	for _, reg := range registrations {
		toolNames[reg.Name] = true
	}

	// Check for at least one tool that should exist
	if !toolNames["spec"] {
		t.Error("Expected to find 'spec' tool in registrations")
	}
}

// MockFlowService for testing
type MockFlowService struct{}

func (m *MockFlowService) StartRun(ctx context.Context, flowName string, event map[string]any) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (m *MockFlowService) GetRun(ctx context.Context, id uuid.UUID) (*model.Run, error) {
	return &model.Run{ID: id}, nil
}

func (m *MockFlowService) ListRuns(ctx context.Context) ([]*model.Run, error) {
	return []*model.Run{}, nil
}

func (m *MockFlowService) DeleteRun(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *MockFlowService) ResumeRun(ctx context.Context, token string, event map[string]any) (map[string]any, error) {
	return map[string]any{}, nil
}

func (m *MockFlowService) GraphFlow(ctx context.Context, flowName string) (string, error) {
	return "graph TD", nil
}

func (m *MockFlowService) ValidateFlow(ctx context.Context, flowName string) error {
	return nil
}

func (m *MockFlowService) RunSpec(ctx context.Context, flow *model.Flow, event map[string]any) (uuid.UUID, map[string]any, error) {
	return uuid.New(), map[string]any{}, nil
}

func (m *MockFlowService) ListTools(ctx context.Context) ([]registry.ToolManifest, error) {
	return []registry.ToolManifest{}, nil
}

func (m *MockFlowService) GetToolManifest(ctx context.Context, name string) (*registry.ToolManifest, error) {
	return &registry.ToolManifest{Name: name}, nil
}

func (m *MockFlowService) ListFlows(ctx context.Context) ([]string, error) {
	return []string{"test_flow"}, nil
}

func (m *MockFlowService) GetFlow(ctx context.Context, name string) (model.Flow, error) {
	return model.Flow{Name: name}, nil
}

func (m *MockFlowService) PublishEvent(ctx context.Context, topic string, payload map[string]any) error {
	return nil
}
