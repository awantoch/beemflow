package http

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/utils"
	"github.com/google/uuid"
)

func TestMain(m *testing.M) {
	utils.WithCleanDirs(m, ".beemflow", config.DefaultConfigDir)
}

func TestUpdateRunEvent(t *testing.T) {
	// Create a temporary config file for the test
	tempConfig := &config.Config{
		Storage: config.StorageConfig{
			Driver: "sqlite",
			DSN:    ":memory:", // Use in-memory SQLite for testing
		},
	}

	// Ensure config directory exists
	configDir := filepath.Dir(config.DefaultConfigPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Write the config file
	configFile, err := os.Create(config.DefaultConfigPath)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	if err := json.NewEncoder(configFile).Encode(tempConfig); err != nil {
		configFile.Close()
		t.Fatalf("Failed to write config file: %v", err)
	}
	configFile.Close()

	// Clean up the config file after the test
	defer os.Remove(config.DefaultConfigPath)

	// We'll test the function directly rather than trying to use the API storage
	runID := uuid.New()
	newEvent := map[string]any{"hello": "world"}

	// We expect this to fail with a "run not found" error, which is the correct behavior
	// since we didn't actually save a run to storage
	err = UpdateRunEvent(runID, newEvent)
	if err == nil {
		t.Fatalf("expected 'run not found' error, got nil")
	}

	if !strings.Contains(err.Error(), "run not found") {
		t.Errorf("expected 'run not found' error, got: %v", err)
	}
}

func TestHTTPServer_ListRuns(t *testing.T) {
	// Create a temporary config file for the test
	tempConfig := &config.Config{
		Storage: config.StorageConfig{
			Driver: "sqlite",
			DSN:    ":memory:", // Use in-memory SQLite for testing
		},
		HTTP: &config.HTTPConfig{
			Port: 18080,
		},
	}

	// Ensure config directory exists
	configDir := filepath.Dir(config.DefaultConfigPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Write the config file
	configFile, err := os.Create(config.DefaultConfigPath)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	if err := json.NewEncoder(configFile).Encode(tempConfig); err != nil {
		configFile.Close()
		t.Fatalf("Failed to write config file: %v", err)
	}
	configFile.Close()

	// Clean up the config file after the test
	defer os.Remove(config.DefaultConfigPath)

	go func() {
		cfg := tempConfig
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
