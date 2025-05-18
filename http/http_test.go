package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/awantoch/beemflow/model"
	"github.com/google/uuid"
)

func TestHandlers_NotImplemented(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"runs", runsHandler},
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

func TestHandlers_NotImplemented_Methods(t *testing.T) {
	handlers := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"runs", runsHandler},
		{"graph", graphHandler},
		{"validate", validateHandler},
		{"test", testHandler},
	}
	methods := []string{"POST", "PUT", "DELETE"}
	for _, tt := range handlers {
		for _, method := range methods {
			req := httptest.NewRequest(method, "/", nil)
			w := httptest.NewRecorder()
			tt.handler(w, req)
			if w.Code != http.StatusNotImplemented {
				t.Errorf("%s %s: expected status %d, got %d", tt.name, method, http.StatusNotImplemented, w.Code)
			}
		}
	}
}

func TestStartServer_InvalidAddress(t *testing.T) {
	err := StartServer("invalid:address")
	if err == nil {
		t.Errorf("expected error for invalid address, got nil")
	}
}

func TestUpdateRunEvent(t *testing.T) {
	runsMu.Lock()
	runs = make(map[uuid.UUID]*model.Run) // reset for test
	runsMu.Unlock()

	runID := uuid.New()
	run := &model.Run{
		ID:    runID,
		Event: map[string]any{"foo": "bar"},
	}
	runsMu.Lock()
	runs[runID] = run
	runsMu.Unlock()

	newEvent := map[string]any{"hello": "world"}
	err := UpdateRunEvent(runID, newEvent)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if run.Event["hello"] != "world" {
		t.Errorf("event not updated: got %v", run.Event)
	}

	nonexistentID := uuid.New()
	err = UpdateRunEvent(nonexistentID, map[string]any{"x": 1})
	if err == nil {
		t.Errorf("expected error for nonexistent run, got nil")
	}
}

func TestResumeHandler_UpdatesEvent(t *testing.T) {
	runsMu.Lock()
	runs = make(map[uuid.UUID]*model.Run) // reset for test
	runsMu.Unlock()

	runID := uuid.New()
	run := &model.Run{
		ID:    runID,
		Event: map[string]any{"foo": "bar"},
	}
	runsMu.Lock()
	runs[runID] = run
	runsMu.Unlock()

	body := `{"hello": "world"}`
	req := httptest.NewRequest("POST", "/resume/"+runID.String(), strings.NewReader(body))
	w := httptest.NewRecorder()
	resumeHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
	if run.Event["hello"] != "world" {
		t.Errorf("event not updated: got %v", run.Event)
	}

	// Test invalid run ID
	req = httptest.NewRequest("POST", "/resume/not-a-uuid", strings.NewReader(body))
	w = httptest.NewRecorder()
	resumeHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid run ID, got %d", w.Code)
	}

	// Test run not found
	nonexistentID := uuid.New()
	req = httptest.NewRequest("POST", "/resume/"+nonexistentID.String(), strings.NewReader(body))
	w = httptest.NewRecorder()
	resumeHandler(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for run not found, got %d", w.Code)
	}

	// Test invalid JSON
	req = httptest.NewRequest("POST", "/resume/"+runID.String(), strings.NewReader("not-json"))
	w = httptest.NewRecorder()
	resumeHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestHTTPServer_ListRuns(t *testing.T) {
	go func() {
		_ = StartServer(":18080")
	}()
	time.Sleep(500 * time.Millisecond) // Give server time to start
	resp, err := http.Get("http://localhost:18080/runs")
	if err != nil {
		t.Fatalf("Failed to GET /runs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200 OK, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Expected application/json, got %s", ct)
	}
}
