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
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Kind        string         `json:"kind"`
	Parameters  map[string]any `json:"parameters"`
	Endpoint    string         `json:"endpoint,omitempty"`
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
