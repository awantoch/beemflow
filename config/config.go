package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/awantoch/beemflow/docs"
	"github.com/awantoch/beemflow/utils"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Sample config for BeemFlow registry system:
//
// {
//   "storage": { "driver": "sqlite", "dsn": ".beemflow/flow.db" },
//   "blob": { "driver": "filesystem", "bucket": "", "directory": ".beemflow/files" },
//   "registries": [
//     { "type": "local", "path": ".beemflow/registry.json" }
//   ],
//   "http": { "host": "localhost", "port": 8080 },
//   "log": { "level": "info" }
// }
//
// - The curated registry (repo-managed, read-only) is always loaded from registry/index.json.
// - The local registry (user-writable) is loaded from the path in registries[].path, defaulting to .beemflow/registry.json.
// - When listing/using tools, local entries take precedence over curated ones.
// - Any tool installed via the CLI is written to the local registry file.
// - All config roots are under .beemflow/ by default.
//
// This system is future-proofed for remote/community registries.
//
// See docs for more details.

// RegistryConfig is the base type for all registry configs.
// For local registries, set type: "local" and path: ".beemflow/registry.json" (default).
// For Smithery, set type: "smithery" and url: "https://registry.smithery.ai/servers" (default).
// For other remote registries, set type: "remote" and url: the base URL of the registry.
type RegistryConfig struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	Path string `json:"path,omitempty"` // For local registries, path to the registry file (default: .beemflow/registry.json)
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

// TracingConfig controls OpenTelemetry tracing exporter and options.
type TracingConfig struct {
	Exporter    string `json:"exporter,omitempty"`    // "stdout", "otlp"
	Endpoint    string `json:"endpoint,omitempty"`    // OTLP endpoint URL
	ServiceName string `json:"serviceName,omitempty"` // Service name for traces
	// Add more fields as needed (sampling, etc.)
}

type Config struct {
	Storage    StorageConfig              `json:"storage"`
	Blob       *BlobConfig                `json:"blob,omitempty"`
	Event      *EventConfig               `json:"event,omitempty"`
	Secrets    *SecretsConfig             `json:"secrets,omitempty"`
	Registries []RegistryConfig           `json:"registries,omitempty"`
	HTTP       *HTTPConfig                `json:"http,omitempty"`
	Log        *LogConfig                 `json:"log,omitempty"`
	FlowsDir   string                     `json:"flowsDir,omitempty"`
	MCPServers map[string]MCPServerConfig `json:"mcpServers,omitempty"`
	Tracing    *TracingConfig             `json:"tracing,omitempty"`
}

type StorageConfig struct {
	Driver string `json:"driver"`
	DSN    string `json:"dsn"`
}

type BlobConfig struct {
	Driver string `json:"driver,omitempty"`
	Bucket string `json:"bucket,omitempty"`
}

// EventConfig configures the event bus.
//
// Supported drivers:
//   - "memory" (default, in-process event bus)
//   - "nats" (requires URL)
//
// Unknown drivers will error out at startup.
//
// Future: Extend with fields like ClusterID, ClientID, TLS options as needed.
type EventConfig struct {
	Driver string `json:"driver,omitempty"`
	URL    string `json:"url,omitempty"`
}

type SecretsConfig struct {
	Driver string `json:"driver,omitempty"`
	Region string `json:"region,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

type HTTPConfig struct {
	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
}

type LogConfig struct {
	Level string `json:"level,omitempty"`
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

// DefaultConfigPath is the default path for the main config file.
const DefaultConfigPath = "flow.config.json"

// ValidateConfig validates the config JSON against the embedded schema.
func ValidateConfig(raw []byte) error {
	schema, err := jsonschema.CompileString("flow.config.schema.json", docs.FlowConfigSchema)
	if err != nil {
		return err
	}
	var doc interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	return schema.Validate(doc)
}

// LoadConfig loads the JSON config from the given path.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var raw []byte
	raw, err = io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if err := ValidateConfig(raw); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
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
			if c.Command != "" || len(c.Args) > 0 || len(c.Env) > 0 || c.Port != 0 || c.Transport != "" || c.Endpoint != "" {
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
		return MCPServerConfig{}, utils.Errorf("MCP server '%s' not found in registry or config", host)
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
			body, err := io.ReadAll(resp.Body)
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

// InjectEnvVarsIntoRegistry walks a registry config and replaces any string field set to "$env:VARNAME" or missing required fields with the value from the environment.
// It works generically for any registry type and any field.
func InjectEnvVarsIntoRegistry(reg map[string]any) {
	for k, v := range reg {
		str, ok := v.(string)
		if ok && strings.HasPrefix(str, "$env:") {
			envVar := strings.TrimPrefix(str, "$env:")
			if val := os.Getenv(envVar); val != "" {
				reg[k] = val
			}
		} else if v == nil {
			// If the field is nil, check for a matching env var by convention
			envVar := strings.ToUpper(k)
			if val := os.Getenv(envVar); val != "" {
				reg[k] = val
			}
		}
	}
}

// LoadAndInjectRegistries loads config, auto-enables Smithery if needed, and injects env vars into all registries.
func LoadAndInjectRegistries(path string) (*Config, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		cfg = &Config{}
	}
	// Auto-enable Smithery if not present and env var is set
	apiKey := os.Getenv("SMITHERY_API_KEY")
	foundSmithery := false
	for _, reg := range cfg.Registries {
		if reg.Type == "smithery" {
			foundSmithery = true
			break
		}
	}
	if !foundSmithery && apiKey != "" {
		cfg.Registries = append(cfg.Registries, RegistryConfig{
			Type: "smithery",
			URL:  "https://registry.smithery.ai/servers",
		})
	}
	// Inject env vars for all registries
	for i := range cfg.Registries {
		regMap := map[string]any{
			"type": cfg.Registries[i].Type,
			"url":  cfg.Registries[i].URL,
			"path": cfg.Registries[i].Path,
		}
		InjectEnvVarsIntoRegistry(regMap)
		cfg.Registries[i].Type = regMap["type"].(string)
		cfg.Registries[i].URL = regMap["url"].(string)
		cfg.Registries[i].Path, _ = regMap["path"].(string)
	}
	return cfg, nil
}

// SaveConfig writes the config to the given path.
func SaveConfig(path string, cfg *Config) error {
	bytesOut, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytesOut, 0644)
}

// UpsertMCPServer adds or updates an MCP server entry in the config.
func UpsertMCPServer(cfg *Config, name string, spec MCPServerConfig) {
	if cfg.MCPServers == nil {
		cfg.MCPServers = map[string]MCPServerConfig{}
	}
	cfg.MCPServers[name] = spec
}

// Validate checks the config for required fields and sensible values.
func (c *Config) Validate() error {
	if c.Storage.Driver == "" {
		return fmt.Errorf("config: storage.driver is required")
	}
	if c.Storage.DSN == "" {
		return fmt.Errorf("config: storage.dsn is required")
	}
	if c.HTTP != nil && c.HTTP.Port == 0 {
		return fmt.Errorf("config: http.port must be set and nonzero")
	}
	// Add more validation as needed (blob, event, etc.)
	return nil
}
