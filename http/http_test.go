package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/utils"
	"github.com/google/uuid"
)

func TestMain(m *testing.M) {
	utils.WithCleanDir(m, config.DefaultConfigDir)
}

func TestHandlers_MethodsAndCodes(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		method  string
		want    int
		body    string
	}{
		{"runs GET", runsHandler, http.MethodGet, http.StatusMethodNotAllowed, ""},
		{"runs POST", runsHandler, http.MethodPost, http.StatusBadRequest, ""}, // missing body
		{"graph GET missing param", graphHandler, http.MethodGet, http.StatusBadRequest, ""},
		{"graph POST", graphHandler, http.MethodPost, http.StatusMethodNotAllowed, ""},
		{"validate GET", validateHandler, http.MethodGet, http.StatusMethodNotAllowed, ""},
		{"validate POST bad body", validateHandler, http.MethodPost, http.StatusBadRequest, "not-json"},
		{"test GET", testHandler, http.MethodGet, http.StatusNotImplemented, ""},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, "/", strings.NewReader(tt.body))
		w := httptest.NewRecorder()
		tt.handler(w, req)
		if w.Code != tt.want {
			t.Errorf("%s: expected status %d, got %d", tt.name, tt.want, w.Code)
		}
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
		cfg := &config.Config{HTTP: &config.HTTPConfig{Port: 18080}}
		_ = StartServer(cfg)
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

func TestAssistantChatHandler(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(assistantChatHandler))
	defer ts.Close()

	body := map[string]any{"messages": []string{"Draft a flow that echoes hello"}}
	b, _ := json.Marshal(body)
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 500 {
		t.Errorf("unexpected status: %d", resp.StatusCode)
	}
}

func TestRunsInlineHandler(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(runsInlineHandler))
	defer ts.Close()

	valid := map[string]any{
		"spec":  "name: test\non: cli.manual\nsteps:\n  - id: s1\n    use: core.echo\n    with:\n      text: hello\n",
		"event": map[string]any{"foo": "bar"},
	}
	b, _ := json.Marshal(valid)
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("unexpected status: %d", resp.StatusCode)
	}

	invalid := map[string]any{
		"spec":  "name: test\nsteps: [bad yaml",
		"event": map[string]any{},
	}
	b2, _ := json.Marshal(invalid)
	resp2, err := http.Post(ts.URL, "application/json", bytes.NewReader(b2))
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 400 && resp2.StatusCode != 500 {
		t.Errorf("unexpected status: %d", resp2.StatusCode)
	}
}
