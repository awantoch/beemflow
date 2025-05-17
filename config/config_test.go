package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

// TestMain ensures the mcp_servers folder is cleaned up before and after tests
func TestMain(m *testing.M) {
	// Remove any existing mcp_servers directory before tests
	os.RemoveAll("mcp_servers")
	// Run tests
	code := m.Run()
	// Clean up after tests
	os.RemoveAll("mcp_servers")
	os.Exit(code)
}

func TestLoadConfig(t *testing.T) {
	cfgJSON := `{"storage":{"driver":"d","dsn":"u"},"blob":{"driver":"b","bucket":"c"},"event":{"driver":"e","url":"u"},"secrets":{"driver":"s","region":"r","prefix":"p"},"registries":["r1","r2"],"http":{"host":"h","port":8080},"log":{"level":"l"}}`
	tmp, err := os.CreateTemp("", "config.json")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write([]byte(cfgJSON)); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp.Close()

	c, err := LoadConfig(tmp.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if c.Storage.Driver != "d" || c.Storage.DSN != "u" {
		t.Errorf("unexpected Storage: %+v", c.Storage)
	}
	if c.Blob.Driver != "b" || c.Blob.Bucket != "c" {
		t.Errorf("unexpected Blob: %+v", c.Blob)
	}
	if c.HTTP.Host != "h" || c.HTTP.Port != 8080 {
		t.Errorf("unexpected HTTP: %+v", c.HTTP)
	}
	if len(c.Registries) != 2 {
		t.Errorf("unexpected Registries: %+v", c.Registries)
	}
}

func TestLoadConfig_Partial(t *testing.T) {
	cfgJSON := `{"storage":{"driver":"d","dsn":"u"}}`
	tmp, err := os.CreateTemp("", "config_partial.json")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write([]byte(cfgJSON)); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp.Close()

	c, err := LoadConfig(tmp.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if c.Storage.Driver != "d" || c.Storage.DSN != "u" {
		t.Errorf("unexpected Storage: %+v", c.Storage)
	}
	// Other fields should be zero-valued
	if c.Blob.Driver != "" || c.Blob.Bucket != "" {
		t.Errorf("expected zero Blob, got %+v", c.Blob)
	}
	if c.HTTP.Host != "" || c.HTTP.Port != 0 {
		t.Errorf("expected zero HTTP, got %+v", c.HTTP)
	}
}

func TestLoadConfig_FileNotExist(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.json")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmp, err := os.CreateTemp("", "bad.json")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write([]byte("not a json")); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp.Close()
	_, err = LoadConfig(tmp.Name())
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestLoadConfig_MCPAutoInclude(t *testing.T) {
	// Write a curated config file for airtable
	curated := `{"airtable": {"install_cmd": ["npx", "-y", "airtable-mcp-server"], "required_env": ["AIRTABLE_API_KEY"], "port": 3030}}`
	curatedPath := "mcp_servers/airtable.json"
	_ = os.MkdirAll("mcp_servers", 0755)
	defer os.Remove(curatedPath)
	err := ioutil.WriteFile(curatedPath, []byte(curated), 0644)
	if err != nil {
		t.Fatalf("failed to write curated: %v", err)
	}

	cfgJSON := `{"mcpServers": {"airtable": {}}}`
	tmp, err := os.CreateTemp("", "config.json")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write([]byte(cfgJSON)); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp.Close()

	c, err := LoadConfig(tmp.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	merged, err := GetMergedMCPServerConfig(c, "airtable")
	if err != nil {
		t.Fatalf("airtable not found in merged config: %v", err)
	}
	if merged.Port != 3030 {
		t.Errorf("expected port 3030 from curated, got %d", merged.Port)
	}
}

// TestGetMergedMCPServerConfig_NoCuratedFile ensures that without a curated file, the original config is returned.
func TestGetMergedMCPServerConfig_NoCuratedFile(t *testing.T) {
	orig := MCPServerConfig{
		Command:   "cmd",
		Args:      []string{"a", "b"},
		Env:       map[string]string{"E": "V"},
		Port:      1,
		Transport: "t",
		Endpoint:  "e",
	}
	cfg := &Config{MCPServers: map[string]MCPServerConfig{"foo": orig}}
	info, err := GetMergedMCPServerConfig(cfg, "foo")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !reflect.DeepEqual(info, orig) {
		t.Errorf("expected %+v, got %+v", orig, info)
	}
}

// TestGetMergedMCPServerConfig_CuratedMissingCommand uses a curated file to supply a missing Command.
func TestGetMergedMCPServerConfig_CuratedMissingCommand(t *testing.T) {
	curated := map[string]MCPServerConfig{"foo": {
		Command:   "cmd2",
		Args:      []string{"x"},
		Env:       map[string]string{"A": "B"},
		Port:      2,
		Transport: "tr",
		Endpoint:  "end",
	}}
	data, _ := json.Marshal(curated)
	path := "mcp_servers/foo.json"
	_ = os.MkdirAll("mcp_servers", 0755)
	os.WriteFile(path, data, 0644)
	defer os.Remove(path)

	cfg := &Config{MCPServers: map[string]MCPServerConfig{"foo": {}}}
	info, err := GetMergedMCPServerConfig(cfg, "foo")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	want := curated["foo"]
	if !reflect.DeepEqual(info, want) {
		t.Errorf("expected %+v, got %+v", want, info)
	}
}

// TestGetMergedMCPServerConfig_CuratedMergeOriginal merges original fields into the curated template.
func TestGetMergedMCPServerConfig_CuratedMergeOriginal(t *testing.T) {
	curated := map[string]MCPServerConfig{"foo": {
		Args:      []string{"y"},
		Env:       map[string]string{"A": "ci", "C": "ci"},
		Port:      3,
		Transport: "tci",
		Endpoint:  "eci",
	}}
	data, _ := json.Marshal(curated)
	path := "mcp_servers/foo.json"
	_ = os.MkdirAll("mcp_servers", 0755)
	os.WriteFile(path, data, 0644)
	defer os.Remove(path)

	orig := MCPServerConfig{
		Command:   "co",
		Args:      []string{"x"},
		Env:       map[string]string{"A": "orig"},
		Port:      5,
		Transport: "tor",
		Endpoint:  "eor",
	}
	cfg := &Config{MCPServers: map[string]MCPServerConfig{"foo": orig}}
	info, err := GetMergedMCPServerConfig(cfg, "foo")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Expect original Command, Args; curated Env merged with original override; original Port, Transport, Endpoint
	want := MCPServerConfig{
		Command:   "",
		Args:      []string{"x"},
		Env:       map[string]string{"A": "orig", "C": "ci"},
		Port:      5,
		Transport: "tor",
		Endpoint:  "eor",
	}
	if !reflect.DeepEqual(info, want) {
		t.Errorf("expected %+v, got %+v", want, info)
	}
}

// TestGetMergedMCPServerConfig_MalformedCuratedIgnored ensures that malformed JSON is ignored and original returned.
func TestGetMergedMCPServerConfig_MalformedCuratedIgnored(t *testing.T) {
	path := "mcp_servers/foo.json"
	_ = os.MkdirAll("mcp_servers", 0755)
	os.WriteFile(path, []byte("not json"), 0644)
	defer os.Remove(path)

	orig := MCPServerConfig{Command: "co", Args: []string{"x"}, Env: map[string]string{"A": "orig"}}
	cfg := &Config{MCPServers: map[string]MCPServerConfig{"foo": orig}}
	info, err := GetMergedMCPServerConfig(cfg, "foo")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !reflect.DeepEqual(info, orig) {
		t.Errorf("expected %+v, got %+v", orig, info)
	}
}

// TestGetMergedMCPServerConfig_MissingHostError ensures an error when the host is not in the main config.
func TestGetMergedMCPServerConfig_MissingHostError(t *testing.T) {
	cfg := &Config{MCPServers: map[string]MCPServerConfig{}}
	_, err := GetMergedMCPServerConfig(cfg, "unknown")
	if err == nil {
		t.Fatalf("expected error for unknown host, got nil")
	}
}
