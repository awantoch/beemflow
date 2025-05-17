package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlers_NotImplemented(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"runs", runsHandler},
		{"resume", resumeHandler},
		{"graph", graphHandler},
		{"validate", validateHandler},
		{"test", testHandler},
	}
	for _, tt := range tests {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		tt.handler(w, req)
		if w.Code != http.StatusNotImplemented {
			t.Errorf("%s: expected status %d, got %d", tt.name, http.StatusNotImplemented, w.Code)
		}
	}
}

func TestStartServer_InvalidAddress(t *testing.T) {
	err := StartServer("invalid:address")
	if err == nil {
		t.Errorf("expected error for invalid address, got nil")
	}
}

func TestResumeAndRunsEndpoints(t *testing.T) {
	// /resume/{token}
	req := httptest.NewRequest("POST", "/resume/abc123", nil)
	w := httptest.NewRecorder()
	resumeHandler(w, req)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("resumeHandler: expected status %d, got %d", http.StatusNotImplemented, w.Code)
	}
	// /runs
	req = httptest.NewRequest("GET", "/runs", nil)
	w = httptest.NewRecorder()
	runsHandler(w, req)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("runsHandler: expected status %d, got %d", http.StatusNotImplemented, w.Code)
	}
}
