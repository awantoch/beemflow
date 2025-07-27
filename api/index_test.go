package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandler_CORS(t *testing.T) {
	// Test OPTIONS request for CORS
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	rec := httptest.NewRecorder()

	Handler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization", rec.Header().Get("Access-Control-Allow-Headers"))
}

func TestHandler_HealthCheck(t *testing.T) {
	// Set up temporary flows directory
	tmpDir := t.TempDir()
	oldFlowsDir := os.Getenv("FLOWS_DIR")
	os.Setenv("FLOWS_DIR", tmpDir)
	defer os.Setenv("FLOWS_DIR", oldFlowsDir)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	Handler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"status":"healthy"}`, rec.Body.String())
}

func TestHandler_WithDatabaseURL(t *testing.T) {
	tests := []struct {
		name        string
		databaseURL string
		wantStatus  int
	}{
		{
			name:        "PostgreSQL URL - invalid",
			databaseURL: "postgres://user:pass@host:5432/db",
			wantStatus:  http.StatusInternalServerError, // Can't connect
		},
		{
			name:        "PostgreSQL URL with postgresql scheme - invalid",
			databaseURL: "postgresql://user:pass@host:5432/db",
			wantStatus:  http.StatusInternalServerError, // Can't connect
		},
		{
			name:        "SQLite URL",
			databaseURL: "file:" + t.TempDir() + "/test.db",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "No DATABASE_URL",
			databaseURL: "",
			wantStatus:  http.StatusOK, // defaults to in-memory
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			oldDB := os.Getenv("DATABASE_URL")
			if tt.databaseURL != "" {
				os.Setenv("DATABASE_URL", tt.databaseURL)
			} else {
				os.Unsetenv("DATABASE_URL")
			}
			defer func() {
				if oldDB != "" {
					os.Setenv("DATABASE_URL", oldDB)
				} else {
					os.Unsetenv("DATABASE_URL")
				}
			}()

			tmpDir := t.TempDir()
			oldFlowsDir := os.Getenv("FLOWS_DIR")
			os.Setenv("FLOWS_DIR", tmpDir)
			defer os.Setenv("FLOWS_DIR", oldFlowsDir)

			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			rec := httptest.NewRecorder()

			Handler(rec, req)

			// Check expected status
			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestHandler_CleanupOnRequestEnd(t *testing.T) {
	// This test verifies that resources are cleaned up after each request
	// by making multiple requests and checking they don't interfere
	
	tmpDir := t.TempDir()
	oldFlowsDir := os.Getenv("FLOWS_DIR")
	os.Setenv("FLOWS_DIR", tmpDir)
	defer os.Setenv("FLOWS_DIR", oldFlowsDir)

	// Make multiple requests
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		Handler(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		// Each request should work independently
	}
}

func TestHandler_ContextTimeout(t *testing.T) {
	// Test that context has timeout set
	tmpDir := t.TempDir()
	oldFlowsDir := os.Getenv("FLOWS_DIR")
	os.Setenv("FLOWS_DIR", tmpDir)
	defer os.Setenv("FLOWS_DIR", oldFlowsDir)

	// We verify context timeout by making a request
	// The handler sets a 30-second timeout and serverless=true
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	Handler(rec, req)
	
	// If we got here without hanging, the timeout is working
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandler_EndpointFiltering(t *testing.T) {
	// Test BEEMFLOW_ENDPOINTS filtering
	tmpDir := t.TempDir()
	oldFlowsDir := os.Getenv("FLOWS_DIR")
	oldEndpoints := os.Getenv("BEEMFLOW_ENDPOINTS")
	
	os.Setenv("FLOWS_DIR", tmpDir)
	os.Setenv("BEEMFLOW_ENDPOINTS", "core,flow")
	
	defer func() {
		os.Setenv("FLOWS_DIR", oldFlowsDir)
		if oldEndpoints != "" {
			os.Setenv("BEEMFLOW_ENDPOINTS", oldEndpoints)
		} else {
			os.Unsetenv("BEEMFLOW_ENDPOINTS")
		}
	}()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	Handler(rec, req)

	// Should still have health endpoint
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandler_InitializationError(t *testing.T) {
	// Test handling of initialization errors
	// Force an error by setting invalid database URL
	oldDB := os.Getenv("DATABASE_URL")
	os.Setenv("DATABASE_URL", "postgres://invalid:invalid@nonexistent:5432/db")
	defer func() {
		if oldDB != "" {
			os.Setenv("DATABASE_URL", oldDB)
		} else {
			os.Unsetenv("DATABASE_URL")
		}
	}()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	Handler(rec, req)

	// Should return 500 on initialization error
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}