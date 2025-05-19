package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/awantoch/beemflow/logger"
)

// RegistryConfig is the base type for all registry configs.
type RegistryConfig struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	// Add other common fields here
}

// SmitheryRegistryConfig is a type-safe config for Smithery registries.
type SmitheryRegistryConfig struct {
	RegistryConfig
	APIKey string `json:"apiKey"`
}

// ParseRegistryConfig parses a generic RegistryConfig into a specific type if needed.
func ParseRegistryConfig(reg RegistryConfig) (any, error) {
	switch reg.Type {
	case "smithery":
		// In real code, you would unmarshal the full object into SmitheryRegistryConfig
		// For now, just cast and return
		return SmitheryRegistryConfig{
			RegistryConfig: reg,
			// APIKey: ... (populate from JSON if needed)
		}, nil
	default:
		return reg, nil
	}
}

type Config struct {
	Storage    StorageConfig              `json:"storage"`
	Blob       BlobConfig                 `json:"blob"`
	Event      EventConfig                `json:"event"`
	Secrets    SecretsConfig              `json:"secrets"`
	Registries []RegistryConfig           `json:"registries"`
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

// readCuratedConfig loads a curated MCPServerConfig from mcp_servers/<host>.json at project root, supporting only the new format.
func readCuratedConfig(host string) (MCPServerConfig, bool) {
	// Try mcp_servers folder in current working directory first
	cwdPath := filepath.Join("mcp_servers", host+".json")
	data, err := os.ReadFile(cwdPath)
	if err != nil {
		// Fallback to project root
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			return MCPServerConfig{}, false
		}
		projectRoot := filepath.Dir(filepath.Dir(file))
		curatedPath := filepath.Join(projectRoot, "mcp_servers", host+".json")
		data, err = os.ReadFile(curatedPath)
		if err != nil {
			return MCPServerConfig{}, false
		}
	}
	// Only support new format
	var newMap map[string]MCPServerConfig
	if err := json.Unmarshal(data, &newMap); err == nil {
		if c, ok2 := newMap[host]; ok2 {
			if c.Command != "" || len(c.Args) > 0 || (c.Env != nil && len(c.Env) > 0) || c.Port != 0 || c.Transport != "" || c.Endpoint != "" {
				return c, true
			}
		}
	}
	return MCPServerConfig{}, false
}

func loadMCPServersFromRegistry(path string) (map[string]MCPServerConfig, error) {
	// Determine registry file path: override or absolute path from project root
	regPath := os.Getenv("BEEMFLOW_REGISTRY")
	if regPath == "" {
		// locate project root
		_, file, _, ok := runtime.Caller(0)
		if ok {
			projectRoot := filepath.Dir(filepath.Dir(file))
			regPath = filepath.Join(projectRoot, path)
		} else {
			regPath = path
		}
	}
	data, err := os.ReadFile(regPath)
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
	// 1. Load registry entries (ignore errors)
	regMap, _ := loadMCPServersFromRegistry("registry/index.json")
	// 2. Determine curated template
	curatedCfg, hasCurated := readCuratedConfig(host)
	// Start with base from registry or curated
	base, found := regMap[host]
	if hasCurated {
		base = curatedCfg
		found = true
	}
	// 3. Load override from config file
	var override MCPServerConfig
	overrideExists := false
	if cfg != nil && cfg.MCPServers != nil {
		if o, ok := cfg.MCPServers[host]; ok {
			override = o
			overrideExists = true
		}
	}
	// 4. Validate presence
	if !found && !overrideExists {
		return MCPServerConfig{}, logger.Errorf("MCP server '%s' not found in registry or config", host)
	}
	// 5. Merge: start from base, then overlay override fields
	merged := base
	if overrideExists {
		// Command: only override if no curated template
		if !hasCurated && override.Command != "" {
			merged.Command = override.Command
		}
		// Other fields override
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

// Regex to match GitHub shorthand owner/repo/path[@ref]
var githubShorthandRe = regexp.MustCompile(`^([^/]+)/([^/]+)/(.+?)(?:@([^/]+))?$`)

// UnmarshalJSON allows MCPServerConfig to be specified as either a JSON object, full URL string,
// or GitHub shorthand (owner/repo/path[@ref]).
func (m *MCPServerConfig) UnmarshalJSON(data []byte) error {
	// Try string first.
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		// Full URL?
		if strings.Contains(s, "://") {
			*m = MCPServerConfig{Transport: "http", Endpoint: s}
			return nil
		}
		// GitHub shorthand: owner/repo/path[@ref]
		if parts := githubShorthandRe.FindStringSubmatch(s); parts != nil {
			owner, repo, path, ref := parts[1], parts[2], parts[3], parts[4]
			if ref == "" {
				ref = "main"
			}
			rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, path)
			resp, err := http.Get(rawURL)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			var cfg MCPServerConfig
			if err := json.Unmarshal(body, &cfg); err != nil {
				return err
			}
			*m = cfg
			return nil
		}
		// Fallback: treat as HTTP endpoint.
		*m = MCPServerConfig{Transport: "http", Endpoint: s}
		return nil
	}
	// Unmarshal into struct normally.
	type alias MCPServerConfig
	var aux alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*m = MCPServerConfig(aux)
	return nil
}
