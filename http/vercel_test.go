package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVercelHandler(t *testing.T) {
	// Create a test request
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	// Call the Vercel handler
	Handler(w, req)

	// Check that we get a response (the serverless handler should respond)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check that we got JSON content
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected JSON content type, got %s", contentType)
	}

	// Check that the response body contains health check data
	body := w.Body.String()
	if body == "" {
		t.Error("Expected non-empty response body")
	}
}

func TestVercelHandlerWithEndpointFilter(t *testing.T) {
	// Set environment variable for endpoint filtering
	t.Setenv("BEEMFLOW_ENDPOINTS", "system")

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	// This should work (system endpoints allowed)
	Handler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected system endpoint to be allowed, got status %d", w.Code)
	}

	// Test blocked endpoint
	req2 := httptest.NewRequest("GET", "/flows", nil)
	w2 := httptest.NewRecorder()

	Handler(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Errorf("Expected flows endpoint to be blocked, got status %d", w2.Code)
	}
}
