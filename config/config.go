package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Storage    StorageConfig              `json:"storage"`
	Blob       BlobConfig                 `json:"blob"`
	Event      EventConfig                `json:"event"`
	Secrets    SecretsConfig              `json:"secrets"`
	Registries []string                   `json:"registries"`
	HTTP       HTTPConfig                 `json:"http"`
	Log        LogConfig                  `json:"log"`
	MCPServers map[string]MCPServerConfig `json:"mcp_servers,omitempty"`
}

type StorageConfig struct {
	Driver string `json:"driver"`
	DSN    string `json:"dsn"`
}

type BlobConfig struct {
	Driver string `json:"driver"`
	Bucket string `json:"bucket"`
}

type EventConfig struct {
	Driver string `json:"driver"`
	URL    string `json:"url"`
}

type SecretsConfig struct {
	Driver string `json:"driver"`
	Region string `json:"region,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

type HTTPConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type LogConfig struct {
	Level string `json:"level"`
}

// MCPServerConfig defines installation details for an MCP server
type MCPServerConfig struct {
	// InstallCmd is the command and args to install or start the server (e.g., ["npx","supabase-mcp-server"])
	InstallCmd []string `json:"install_cmd"`
	// RequiredEnv is a list of environment variables that must be set for the server
	RequiredEnv []string `json:"required_env,omitempty"`
	// Port is an optional port number to check if the server is already running
	Port int `json:"port,omitempty"`
}

// SecretsProvider resolves secrets for flows (env, AWS-SM, Vault, etc.)
type SecretsProvider interface {
	GetSecret(key string) (string, error)
}

func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}

	// Prepare MCPServers map and save user overrides
	userOverrides := make(map[string]MCPServerConfig)
	if cfg.MCPServers != nil {
		for k, v := range cfg.MCPServers {
			userOverrides[k] = v
		}
	}
	cfg.MCPServers = make(map[string]MCPServerConfig)

	// Load curated defaults from mcp_servers/*.json
	files, err := ioutil.ReadDir("mcp_servers")
	if err == nil {
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			filename := filepath.Join("mcp_servers", file.Name())
			if os.Getenv("BEEMFLOW_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[beemflow] Attempting to load curated MCP config: %s\n", filename)
			}
			data, err := ioutil.ReadFile(filename)
			if err != nil {
				if os.Getenv("BEEMFLOW_DEBUG") != "" {
					fmt.Fprintf(os.Stderr, "[beemflow] Could not read curated MCP config: %v\n", err)
				}
				continue
			}
			var curated map[string]MCPServerConfig
			if err := json.Unmarshal(data, &curated); err != nil {
				if os.Getenv("BEEMFLOW_DEBUG") != "" {
					fmt.Fprintf(os.Stderr, "[beemflow] Could not parse curated MCP config: %v\n", err)
				}
				continue
			}
			for k, v := range curated {
				cfg.MCPServers[k] = v
				if os.Getenv("BEEMFLOW_DEBUG") != "" {
					fmt.Fprintf(os.Stderr, "[beemflow] Loaded curated MCP config for '%s': %+v\n", k, v)
				}
			}
		}
	}

	// Apply user overrides on top of curated defaults
	for k, override := range userOverrides {
		existing, _ := cfg.MCPServers[k]
		if len(override.InstallCmd) > 0 {
			existing.InstallCmd = override.InstallCmd
		}
		if len(override.RequiredEnv) > 0 {
			existing.RequiredEnv = override.RequiredEnv
		}
		if override.Port != 0 {
			existing.Port = override.Port
		}
		cfg.MCPServers[k] = existing
	}

	return &cfg, nil
}
