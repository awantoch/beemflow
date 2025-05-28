package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/registry"
	"github.com/google/uuid"
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

	// Check first manifest (GET /users)
	manifest1 := manifests[0].(map[string]any)
	if manifest1["name"] != "test_api.users" {
		t.Errorf("Expected name 'test_api.users', got %v", manifest1["name"])
	}
	if manifest1["method"] != "GET" {
		t.Errorf("Expected method 'GET', got %v", manifest1["method"])
	}

	// Check second manifest (POST /users)
	manifest2 := manifests[1].(map[string]any)
	if manifest2["name"] != "test_api.users" {
		t.Errorf("Expected name 'test_api.users', got %v", manifest2["name"])
	}
	if manifest2["method"] != "POST" {
		t.Errorf("Expected method 'POST', got %v", manifest2["method"])
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
	return &registry.ToolManifest{}, nil
}

func (m *MockFlowService) ListFlows(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (m *MockFlowService) GetFlow(ctx context.Context, name string) (model.Flow, error) {
	return model.Flow{Name: name}, nil
}

func (m *MockFlowService) PublishEvent(ctx context.Context, topic string, payload map[string]any) error {
	return nil
}
