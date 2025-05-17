package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/awantoch/beemflow/internal/mcp"
)

type Adapter interface {
	ID() string
	Execute(ctx context.Context, inputs map[string]any) (map[string]any, error)
}

type Registry struct {
	adapters map[string]Adapter
}

func NewRegistry() *Registry {
	return &Registry{adapters: make(map[string]Adapter)}
}

func (r *Registry) Register(a Adapter) {
	r.adapters[a.ID()] = a
}

func (r *Registry) Get(id string) (Adapter, bool) {
	a, ok := r.adapters[id]
	return a, ok
}

// ManifestLoader loads tool manifests from various sources (stub).
type ManifestLoader interface {
	LoadManifest(name string) (*ToolManifest, error)
}

// RemoteRegistryLoader loads manifests from a remote registry index (e.g., Cursor MCP)
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
	// Download the registry index
	resp, err := http.Get(r.IndexURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var index map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, err
	}
	// Find the tool entry (assume index is map[string]any, tool name as key)
	entry, ok := index[name]
	if !ok {
		return nil, fmt.Errorf("tool %s not found in registry", name)
	}
	// Entry may be a map with an MCP endpoint or manifest URL
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

func fetchMCPManifest(mcpURL string) (*ToolManifest, error) {
	// Per spec, fetch /.well-known/beemflow.json from the MCP server
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

// ToolManifest represents a tool manifest loaded from JSON.
type ToolManifest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Kind        string         `json:"kind"`
	Parameters  map[string]any `json:"parameters"`
	Endpoint    string         `json:"endpoint,omitempty"`
}

// HTTPAdapter is a generic HTTP-backed tool adapter.
type HTTPAdapter struct {
	id       string
	manifest *ToolManifest
}

func (a *HTTPAdapter) ID() string { return a.id }

func (a *HTTPAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	if a.manifest == nil || a.manifest.Endpoint == "" {
		return nil, fmt.Errorf("no endpoint for tool %s", a.id)
	}
	body, _ := json.Marshal(inputs)
	req, err := http.NewRequestWithContext(ctx, "POST", a.manifest.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := ioutil.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Registry method to load and register a tool by name from local manifests
func (r *Registry) LoadAndRegisterTool(name, toolsDir string) error {
	loader := &LocalManifestLoader{Dir: toolsDir}
	manifest, err := loader.LoadManifest(name)
	if err != nil {
		return err
	}
	adapter := &HTTPAdapter{id: name, manifest: manifest}
	r.Register(adapter)
	return nil
}

// RegistryFetcher fetches and caches tool registries (hub, MCP, etc.)
type RegistryFetcher struct {
	// TODO: implement registry fetching and caching
}

func NewRegistryFetcher() *RegistryFetcher {
	return &RegistryFetcher{}
}

// MCPManifestResolver resolves MCP tool manifests.
type MCPManifestResolver struct {
	// TODO: implement MCP manifest resolution
}

func NewMCPManifestResolver() *MCPManifestResolver {
	return &MCPManifestResolver{}
}

// TODO: Manifest loading (local, hub, MCP, GitHub)

// Add core.echo adapter

type CoreEchoAdapter struct{}

func (a *CoreEchoAdapter) ID() string { return "core.echo" }

func (a *CoreEchoAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	// Just echo the input as output
	return inputs, nil
}

// Restore LocalManifestLoader

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

// MCPAdapter implements Adapter for mcp://server/tool
type MCPAdapter struct {
	clients map[string]mcp.MCPClient // key: host
}

func NewMCPAdapter() *MCPAdapter {
	return &MCPAdapter{clients: make(map[string]mcp.MCPClient)}
}

func (a *MCPAdapter) ID() string { return "mcp" }

// Parse mcp://host/tool
var mcpRe = regexp.MustCompile(`^mcp://([^/]+)/([\w.-]+)$`)

func (a *MCPAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	use, ok := inputs["__use"].(string)
	if !ok {
		return nil, fmt.Errorf("missing __use for MCPAdapter")
	}
	matches := mcpRe.FindStringSubmatch(use)
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid mcp:// identifier: %s", use)
	}
	host := matches[1]
	tool := matches[2]
	var baseURL string
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
		baseURL = "http://" + host
	} else {
		baseURL = "https://" + host
	}
	client, ok := a.clients[host]
	if !ok {
		client = mcp.NewHTTPMCPClient(baseURL)
		a.clients[host] = client
	}
	// List tools (cache per client)
	tools, err := client.ListTools()
	if err != nil {
		return nil, fmt.Errorf("MCP ListTools failed: %w", err)
	}
	found := false
	for _, t := range tools {
		if t.Name == tool {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("tool %s not found on MCP server %s", tool, host)
	}
	// Remove __use from inputs
	args := make(map[string]any)
	for k, v := range inputs {
		if k == "__use" {
			continue
		}
		args[k] = v
	}
	return client.CallTool(tool, args)
}
