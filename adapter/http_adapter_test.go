package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/awantoch/beemflow/registry"
)

// TestHTTPAdapter_Generic covers both manifest-based and generic HTTP requests.
func TestHTTPAdapter_Generic(t *testing.T) {
	// Test generic HTTP GET
	getServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("hello"))
	}))
	defer getServer.Close()

	adapter := &HTTPAdapter{AdapterID: "http"}
	result, err := adapter.Execute(context.Background(), map[string]any{
		"url":    getServer.URL,
		"method": "GET",
	})
	if err != nil {
		t.Errorf("GET request failed: %v", err)
	}
	if result["body"] != "hello" {
		t.Errorf("expected body=hello, got %v", result["body"])
	}

	// Test generic HTTP POST
	postServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["test"] != "data" {
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"success": true}`))
	}))
	defer postServer.Close()

	result, err = adapter.Execute(context.Background(), map[string]any{
		"url":    postServer.URL,
		"method": "POST",
		"body":   map[string]any{"test": "data"},
	})
	if err != nil {
		t.Errorf("POST request failed: %v", err)
	}
	if result["success"] != true {
		t.Errorf("expected success=true, got %v", result["success"])
	}

	// Test missing URL error
	_, err = adapter.Execute(context.Background(), map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "missing or invalid url") {
		t.Errorf("expected missing or invalid url error, got %v", err)
	}

	// Test HTTP error status
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("server error"))
	}))
	defer errorServer.Close()

	_, err = adapter.Execute(context.Background(), map[string]any{
		"url": errorServer.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Errorf("expected status 500 error, got %v", err)
	}
}

// TestHTTPAdapter_ManifestBased tests manifest-based HTTP requests.
func TestHTTPAdapter_ManifestBased(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("json.Decode failed: %v", err)
		}
		if body["foo"] != "bar" {
			t.Errorf("expected foo=bar in request body, got %v", body["foo"])
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	manifest := &registry.ToolManifest{
		Name:     "test-defaults",
		Endpoint: server.URL,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"foo": map[string]any{"type": "string", "default": "bar"},
			},
		},
	}

	adapter := &HTTPAdapter{AdapterID: "test-defaults", ToolManifest: manifest}
	result, err := adapter.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result["ok"] != true {
		t.Errorf("expected ok=true in response, got %v", result)
	}
}

// TestHTTPAdapter_EnvironmentVariableExpansion tests environment variable expansion in headers and defaults
func TestHTTPAdapter_EnvironmentVariableExpansion(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_API_KEY", "secret-key-123")
	defer func() {
		os.Unsetenv("TEST_API_KEY")
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that the Authorization header was expanded correctly
		auth := r.Header.Get("Authorization")
		if auth != "Bearer secret-key-123" {
			t.Errorf("expected Authorization header 'Bearer secret-key-123', got '%s'", auth)
		}

		w.WriteHeader(200)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	manifest := &registry.ToolManifest{
		Name:     "test-env-expansion",
		Endpoint: server.URL,
		Headers: map[string]string{
			"Authorization": "Bearer $env:TEST_API_KEY",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"api_key": map[string]any{
					"type":    "string",
					"default": "$env:TEST_API_KEY",
				},
			},
		},
	}

	adapter := &HTTPAdapter{AdapterID: "test-env-expansion", ToolManifest: manifest}
	result, err := adapter.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result["success"] != true {
		t.Errorf("expected success=true in response, got %v", result)
	}
}
