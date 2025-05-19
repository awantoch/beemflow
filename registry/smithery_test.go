package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewSmitheryRegistry_DefaultBaseURL(t *testing.T) {
	reg := NewSmitheryRegistry("key", "")
	if reg.BaseURL != "https://registry.smithery.ai/servers" {
		t.Errorf("expected default baseURL, got %s", reg.BaseURL)
	}
}

func TestSmitheryRegistry_ListServersAndGetServer(t *testing.T) {
	// Mock Smithery API
	h := http.NewServeMux()
	h.HandleFunc("/servers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"servers":[{"qualifiedName":"foo","displayName":"Foo","description":"desc","homepage":"http://foo","isDeployed":true}]}`))
	})
	h.HandleFunc("/servers/foo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"qualifiedName":"foo","displayName":"Foo","description":"desc","homepage":"http://foo","connections":[{"type":"http","url":"http://foo","configSchema":{},"published":true} ]}`))
	})
	ts := httptest.NewServer(h)
	defer ts.Close()
	os.Setenv("SMITHERY_API_KEY", "testkey")
	reg := NewSmitheryRegistry("testkey", ts.URL+"/servers")
	servers, err := reg.ListServers(context.Background(), ListOptions{Query: "", Page: 0, PageSize: 0})
	if err != nil {
		t.Fatalf("ListServers failed: %v", err)
	}
	if len(servers) != 1 || servers[0].Name != "foo" {
		t.Errorf("expected foo, got %+v", servers)
	}
	entry, err := reg.GetServer(context.Background(), "foo")
	if err != nil || entry == nil || entry.Name != "foo" {
		t.Errorf("GetServer failed: %v, entry: %+v", err, entry)
	}
	// Error: no suitable connection
	h.HandleFunc("/servers/bar", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"qualifiedName":"bar","displayName":"Bar","description":"desc","homepage":"http://bar","connections":[]}`))
	})
	entry, err = reg.GetServer(context.Background(), "bar")
	if err == nil || entry != nil {
		t.Errorf("expected error for no suitable connection, got: %+v, err: %v", entry, err)
	}
	// Error: 404
	h.HandleFunc("/servers/404", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	entry, err = reg.GetServer(context.Background(), "404")
	if err == nil || entry != nil {
		t.Errorf("expected error for 404, got: %+v, err: %v", entry, err)
	}
	// Error: bad JSON
	h.HandleFunc("/servers/badjson", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	})
	entry, err = reg.GetServer(context.Background(), "badjson")
	if err == nil || entry != nil {
		t.Errorf("expected error for bad JSON, got: %+v, err: %v", entry, err)
	}
	// ListServers: bad JSON
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	reg2 := NewSmitheryRegistry("testkey", ts2.URL)
	_, err = reg2.ListServers(context.Background(), ListOptions{Query: "", Page: 0, PageSize: 0})
	if err == nil {
		t.Error("expected error for bad JSON in ListServers")
	}
	// ListServers: 401
	ts3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	reg3 := NewSmitheryRegistry("testkey", ts3.URL)
	_, err = reg3.ListServers(context.Background(), ListOptions{Query: "", Page: 0, PageSize: 0})
	if err == nil {
		t.Error("expected error for 401 in ListServers")
	}
}

// TestGetServerSpec parses a stdioFunction into command and args.
func TestGetServerSpec(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"connections": []map[string]any{
				{
					"type":          "stdio",
					"published":     true,
					"stdioFunction": "({command:'./foo',args:['-a','-b']})",
				},
			},
		})
	}))
	defer ts.Close()
	reg := NewSmitheryRegistry("key", ts.URL)
	spec, err := reg.GetServerSpec(context.Background(), "foo")
	if err != nil {
		t.Fatalf("GetServerSpec error: %v", err)
	}
	if spec.Command != "./foo" {
		t.Errorf("expected command './foo', got %s", spec.Command)
	}
	if len(spec.Args) != 2 || spec.Args[0] != "-a" || spec.Args[1] != "-b" {
		t.Errorf("unexpected args: %+v", spec.Args)
	}
}
