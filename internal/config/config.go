package config

import (
	"encoding/json"
	"os"
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
	return &cfg, nil
}
