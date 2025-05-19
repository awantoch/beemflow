package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/awantoch/beemflow/pkg/logger"
)

type Config struct {
	Storage    StorageConfig              `json:"storage"`
	Blob       BlobConfig                 `json:"blob"`
	Event      EventConfig                `json:"event"`
	Secrets    SecretsConfig              `json:"secrets"`
	Registries []string                   `json:"registries"`
	HTTP       HTTPConfig                 `json:"http"`
	Log        LogConfig                  `json:"log"`
	FlowsDir   string                     `json:"flowsDir,omitempty"`
	MCPServers map[string]MCPServerConfig `json:"mcpServers,omitempty"`
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
// Community/Claude style only
// Only supports: command, args, env, port, transport, endpoint
// (No install_cmd, required_env, or snake_case)
type MCPServerConfig struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Port      int               `json:"port,omitempty"`
	Transport string            `json:"transport,omitempty"`
	Endpoint  string            `json:"endpoint,omitempty"`
}

// SecretsProvider resolves secrets for flows (env, AWS-SM, Vault, etc.)
type SecretsProvider interface {
	GetSecret(key string) (string, error)
}

// LoadConfig loads the JSON config from the given path.
func LoadConfig(path string) (*Config, error) {
	logger.Debug("Entered LoadConfig with path: %s", path)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	logger.Debug("Raw loaded config after decode: %+v", cfg)
	return &cfg, nil
}

// Minimal MCP server registry loader (no import cycle)
type registryEntry struct {
	Type      string            `json:"type"`
	Name      string            `json:"name"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Port      int               `json:"port,omitempty"`
	Transport string            `json:"transport,omitempty"`
	Endpoint2 string            `json:"endpoint2,omitempty"`
	Endpoint  string            `json:"endpoint,omitempty"`
}

func loadMCPServersFromRegistry(path string) (map[string]MCPServerConfig, error) {
	// Use BEEMFLOW_REGISTRY env var if set
	if envPath := os.Getenv("BEEMFLOW_REGISTRY"); envPath != "" {
		path = envPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []registryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	out := map[string]MCPServerConfig{}
	for _, e := range entries {
		if e.Type == "mcp_server" {
			out[e.Name] = MCPServerConfig{
				Command:   e.Command,
				Args:      e.Args,
				Env:       e.Env,
				Port:      e.Port,
				Transport: e.Transport,
				Endpoint:  e.Endpoint2,
			}
		}
	}
	return out, nil
}

// GetMergedMCPServerConfig returns the merged MCPServerConfig for a given host, merging the registry (registry/index.json) and config file (flow.config.json).
func GetMergedMCPServerConfig(cfg *Config, host string) (MCPServerConfig, error) {
	regMap, err := loadMCPServersFromRegistry("registry/index.json")
	if err != nil {
		return MCPServerConfig{}, err
	}
	// 2. Load override from config file
	var override MCPServerConfig
	ok := false
	if cfg != nil && cfg.MCPServers != nil {
		override, ok = cfg.MCPServers[host]
	}
	// 3. Merge: config wins, then registry
	base, found := regMap[host]
	if !found && !ok {
		return MCPServerConfig{}, fmt.Errorf("MCP server '%s' not found in registry or config", host)
	}
	merged := base
	if ok {
		if override.Command != "" {
			merged.Command = override.Command
		}
		if len(override.Args) > 0 {
			merged.Args = override.Args
		}
		if override.Env != nil {
			if merged.Env == nil {
				merged.Env = map[string]string{}
			}
			for k, v := range override.Env {
				merged.Env[k] = v
			}
		}
		if override.Port != 0 {
			merged.Port = override.Port
		}
		if override.Transport != "" {
			merged.Transport = override.Transport
		}
		if override.Endpoint != "" {
			merged.Endpoint = override.Endpoint
		}
	}
	return merged, nil
}
