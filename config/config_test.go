package config

import (
	"io/ioutil"
	"os"
	"testing"
)

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
	err := ioutil.WriteFile(curatedPath, []byte(curated), 0644)
	if err != nil {
		t.Fatalf("failed to write curated: %v", err)
	}
	defer os.Remove(curatedPath)

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
