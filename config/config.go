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
	// No legacy merging needed
	return &cfg, nil
}

// GetMergedMCPServerConfig returns the merged MCPServerConfig for a given host, merging the main config and curated config from mcp_servers/<host>.json if available.
func GetMergedMCPServerConfig(cfg *Config, host string) (MCPServerConfig, error) {
	info, ok := cfg.MCPServers[host]
	if !ok {
		return MCPServerConfig{}, fmt.Errorf("MCP server '%s' not found in config", host)
	}
	curatedPath := "mcp_servers/" + host + ".json"
	data, err := os.ReadFile(curatedPath)
	if err == nil {
		var m map[string]MCPServerConfig
		if err := json.Unmarshal(data, &m); err == nil {
			if ci, ok2 := m[host]; ok2 {
				if info.Command == "" {
					info = ci
				} else {
					if ci.Env == nil {
						ci.Env = map[string]string{}
					}
					if info.Env == nil {
						info.Env = map[string]string{}
					}
					if len(info.Args) > 0 {
						ci.Args = info.Args
					}
					if len(info.Env) > 0 {
						for k, v := range info.Env {
							ci.Env[k] = v
						}
					}
					if info.Port != 0 {
						ci.Port = info.Port
					}
					if info.Transport != "" {
						ci.Transport = info.Transport
					}
					if info.Endpoint != "" {
						ci.Endpoint = info.Endpoint
					}
					info = ci
				}
			}
		}
	}
	return info, nil
}
