package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHTTPPostJSONAndGetRaw covers success and error cases for HTTPPostJSON and HTTPGetRaw.
func TestHTTPPostJSONAndGetRaw(t *testing.T) {
	// POST success
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		r.Body.Close()
		if body["x"] != float64(1) {
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(201)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	var out map[string]any
	err := HTTPPostJSON(context.Background(), server.URL, map[string]any{"x": 1}, map[string]string{"H": "V"}, &out)
	if err != nil || out["ok"] != true {
		t.Errorf("HTTPPostJSON success failed: %v %v", err, out)
	}

	// POST status error
	statusServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`error`))
	}))
	defer statusServer.Close()

	err = HTTPPostJSON(context.Background(), statusServer.URL, map[string]any{}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "unexpected status code") {
		t.Errorf("expected status code error, got %v", err)
	}

	// GET success
	getServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("hello"))
	}))
	defer getServer.Close()

	body, err := HTTPGetRaw(context.Background(), getServer.URL, map[string]string{"A": "B"})
	if err != nil || body != "hello" {
		t.Errorf("HTTPGetRaw success failed: %v %v", err, body)
	}

	// GET status error
	getErrorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("not found"))
	}))
	defer getErrorServer.Close()

	_, err = HTTPGetRaw(context.Background(), getErrorServer.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "unexpected status code") {
		t.Errorf("expected HTTPGetRaw status error, got %v", err)
	}
}

// TestHTTPFetchAdapter covers missing url error and successful fetch.
func TestHTTPFetchAdapter(t *testing.T) {
	a := &HTTPFetchAdapter{}
	_, err := a.Execute(context.Background(), map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "missing url") {
		t.Errorf("expected missing url error, got %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("data"))
	}))
	defer server.Close()

	res, err := a.Execute(context.Background(), map[string]any{"url": server.URL})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if res["body"] != "data" {
		t.Errorf("expected body=data, got %v", res)
	}
}

func TestHTTPAdapter_DefaultInjection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["foo"] != "bar" {
			t.Errorf("expected foo=bar in request body, got %v", body["foo"])
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	manifest := &ToolManifest{
		Name:     "test-defaults",
		Endpoint: server.URL,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"foo": map[string]any{"type": "string", "default": "bar"},
			},
		},
	}
	a := &HTTPAdapter{AdapterID: "test-defaults", ToolManifest: manifest}
	out, err := a.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out["ok"] != true {
		t.Errorf("expected ok=true in response, got %v", out)
	}
}
