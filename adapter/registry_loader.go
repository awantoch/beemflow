package adapter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// ToolManifest represents a tool manifest loaded from JSON.
type ToolManifest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Kind        string            `json:"kind"`
	Parameters  map[string]any    `json:"parameters"`
	Endpoint    string            `json:"endpoint,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// MCPServerConfig represents a MCP server configuration loaded from JSON.
type MCPServerConfig struct {
	Name      string            `json:"name"`
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Port      int               `json:"port,omitempty"`
	Transport string            `json:"transport,omitempty"`
	Endpoint  string            `json:"endpoint,omitempty"`
}

// RegistryEntry represents an entry in the unified registry.
type RegistryEntry struct {
	Type string `json:"type"`
	// Tool fields
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Kind        string            `json:"kind,omitempty"`
	Parameters  map[string]any    `json:"parameters,omitempty"`
	Endpoint    string            `json:"endpoint,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	// MCP server fields
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Port      int               `json:"port,omitempty"`
	Transport string            `json:"transport,omitempty"`
	Endpoint2 string            `json:"endpoint2,omitempty"`
}

// LoadUnifiedRegistry loads registry/index.json and returns all ToolManifests and MCPServerConfigs.
func LoadUnifiedRegistry(path string) ([]*ToolManifest, []*MCPServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read registry: %w", err)
	}
	var entries []RegistryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, nil, fmt.Errorf("failed to parse registry: %w", err)
	}
	var tools []*ToolManifest
	var mcps []*MCPServerConfig
	for _, entry := range entries {
		switch entry.Type {
		case "tool":
			tools = append(tools, &ToolManifest{
				Name:        entry.Name,
				Description: entry.Description,
				Kind:        entry.Kind,
				Parameters:  entry.Parameters,
				Endpoint:    entry.Endpoint,
				Headers:     entry.Headers,
			})
		case "mcp_server":
			mcps = append(mcps, &MCPServerConfig{
				Name:      entry.Name,
				Command:   entry.Command,
				Args:      entry.Args,
				Env:       entry.Env,
				Port:      entry.Port,
				Transport: entry.Transport,
				Endpoint:  entry.Endpoint2, // fallback for alternate field name
			})
		}
	}
	return tools, mcps, nil
}

// ManifestLoader loads tool manifests from various sources.
type ManifestLoader interface {
	LoadManifest(name string) (*ToolManifest, error)
}

// LocalManifestLoader loads manifests from a local directory.
type LocalManifestLoader struct {
	Dir string
}

func (l *LocalManifestLoader) LoadManifest(name string) (*ToolManifest, error) {
	path := filepath.Join(l.Dir, name+".json")
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m ToolManifest
	if err := json.Unmarshal(f, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// RemoteRegistryLoader loads manifests from a remote registry index.
type RemoteRegistryLoader struct {
	IndexURL string
	cache    map[string]*ToolManifest
}

func NewRemoteRegistryLoader(indexURL string) *RemoteRegistryLoader {
	if indexURL == "" {
		indexURL = GetRegistryIndexURL()
	}
	return &RemoteRegistryLoader{IndexURL: indexURL, cache: make(map[string]*ToolManifest)}
}

func (r *RemoteRegistryLoader) LoadManifest(name string) (*ToolManifest, error) {
	if m, ok := r.cache[name]; ok {
		return m, nil
	}
	resp, err := http.Get(r.IndexURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var index map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, err
	}
	entry, ok := index[name]
	if !ok {
		return nil, fmt.Errorf("tool %s not found in registry", name)
	}
	entryMap, ok := entry.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid registry entry for %s", name)
	}
	// Try MCP endpoint first
	if mcpURL, ok := entryMap["mcp"].(string); ok && mcpURL != "" {
		manifest, err := fetchMCPManifest(mcpURL)
		if err != nil {
			return nil, err
		}
		r.cache[name] = manifest
		return manifest, nil
	}
	// Or direct manifest URL
	if manifestURL, ok := entryMap["manifest"].(string); ok && manifestURL != "" {
		manifest, err := fetchManifestFromURL(manifestURL)
		if err != nil {
			return nil, err
		}
		r.cache[name] = manifest
		return manifest, nil
	}
	return nil, fmt.Errorf("no MCP or manifest URL for tool %s", name)
}

// fetchMCPManifest fetches a tool manifest from the MCP well-known endpoint.
func fetchMCPManifest(mcpURL string) (*ToolManifest, error) {
	url := fmt.Sprintf("%s/.well-known/beemflow.json", mcpURL)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var m ToolManifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// fetchManifestFromURL fetches a tool manifest directly from a URL.
func fetchManifestFromURL(url string) (*ToolManifest, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var m ToolManifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// NewRegistryFetcher creates a ManifestLoader for the default community hub registry.
func NewRegistryFetcher() ManifestLoader {
	return NewRemoteRegistryLoader("https://hub.beemflow.com/index.json")
}

// NewMCPManifestResolver creates a ManifestLoader for MCP manifest resolution.
func NewMCPManifestResolver() ManifestLoader {
	return &LocalManifestLoader{Dir: ""}
}

// CompositeManifestLoader loads manifests from a remote index.json (array of entries) and optionally local disk.
type CompositeManifestLoader struct {
	IndexURL string
	LocalDir string
	cache    map[string]*ToolManifest
}

func NewCompositeManifestLoader(indexURL, localDir string) *CompositeManifestLoader {
	return &CompositeManifestLoader{IndexURL: indexURL, LocalDir: localDir, cache: make(map[string]*ToolManifest)}
}

func (c *CompositeManifestLoader) LoadManifest(name string) (*ToolManifest, error) {
	// 1. Check cache
	if m, ok := c.cache[name]; ok {
		return m, nil
	}
	// 2. Try local disk first (override)
	if c.LocalDir != "" {
		path := filepath.Join(c.LocalDir, name+".json")
		if f, err := os.ReadFile(path); err == nil {
			var m ToolManifest
			if err := json.Unmarshal(f, &m); err == nil {
				c.cache[name] = &m
				return &m, nil
			}
		}
	}
	// 3. Fetch remote index.json (array of entries)
	resp, err := http.Get(c.IndexURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var entries []ToolManifest
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		c.cache[entry.Name] = &entry
		if entry.Name == name {
			return &entry, nil
		}
	}
	return nil, fmt.Errorf("tool %s not found in registry", name)
}

// GetRegistryIndexURL returns the registry index URL from BEEMFLOW_REGISTRY or the default.
func GetRegistryIndexURL() string {
	if env := os.Getenv("BEEMFLOW_REGISTRY"); env != "" {
		return env
	}
	// Prefer local registry if it exists
	if _, err := os.Stat("registry/index.json"); err == nil {
		return "registry/index.json"
	}
	// Fallback to remote hub
	return "https://hub.beemflow.com/index.json"
}
