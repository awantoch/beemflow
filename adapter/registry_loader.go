package adapter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/awantoch/beemflow/logger"
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

// IsTool returns true if the entry is a tool (by type or by presence of Kind).
func (e RegistryEntry) IsTool() bool {
	return e.Type == "tool" || (e.Type == "" && e.Kind != "")
}

// IsMCPServer returns true if the entry is an MCP server (by type or by presence of Command).
func (e RegistryEntry) IsMCPServer() bool {
	return e.Type == "mcp_server" || (e.Type == "" && e.Command != "")
}

// LoadUnifiedRegistryFromEntries loads registry entries and returns all ToolManifests and MCPServerConfigs.
func LoadUnifiedRegistryFromEntries(entries []RegistryEntry) ([]*ToolManifest, []*MCPServerConfig, error) {
	var tools []*ToolManifest
	var mcps []*MCPServerConfig
	for _, entry := range entries {
		if entry.IsTool() {
			tools = append(tools, &ToolManifest{
				Name:        entry.Name,
				Description: entry.Description,
				Kind:        entry.Kind,
				Parameters:  entry.Parameters,
				Endpoint:    entry.Endpoint,
				Headers:     entry.Headers,
			})
		}
		if entry.IsMCPServer() {
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

// LoadUnifiedRegistry loads registry/index.json and returns all ToolManifests and MCPServerConfigs.
func LoadUnifiedRegistry(path string) ([]*ToolManifest, []*MCPServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, logger.Errorf("failed to read registry: %w", err)
	}
	var entries []RegistryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, nil, logger.Errorf("failed to parse registry: %w", err)
	}
	return LoadUnifiedRegistryFromEntries(entries)
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
	Type     string // e.g. "smithery"
	APIKey   string // for smithery
}

func NewRemoteRegistryLoader(indexURL string) *RemoteRegistryLoader {
	if indexURL == "" {
		indexURL = GetRegistryIndexURL()
	}
	return &RemoteRegistryLoader{IndexURL: indexURL, cache: make(map[string]*ToolManifest)}
}

// NewSmitheryRegistryLoader creates a loader for Smithery registry with auth
func NewSmitheryRegistryLoader(url, apiKey string) *RemoteRegistryLoader {
	return &RemoteRegistryLoader{IndexURL: url, cache: make(map[string]*ToolManifest), Type: "smithery", APIKey: apiKey}
}

// Helper to fetch all Smithery MCP servers as RegistryEntry objects
func fetchSmitheryMCPServers(url, apiKey string) ([]RegistryEntry, error) {
	var entries []RegistryEntry
	// Fetch all servers
	req, err := http.NewRequest("GET", url+"?pageSize=1000", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+resolveEnvVar(apiKey))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data struct {
		Servers []struct {
			QualifiedName string `json:"qualifiedName"`
			DisplayName   string `json:"displayName"`
			Description   string `json:"description"`
			Homepage      string `json:"homepage"`
			IsDeployed    bool   `json:"isDeployed"`
		} `json:"servers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	for _, s := range data.Servers {
		// Fetch server details
		detailReq, err := http.NewRequest("GET", strings.TrimSuffix(url, "/servers")+"/servers/"+s.QualifiedName, nil)
		if err != nil {
			continue
		}
		detailReq.Header.Set("Authorization", "Bearer "+resolveEnvVar(apiKey))
		detailResp, err := http.DefaultClient.Do(detailReq)
		if err != nil {
			continue
		}
		var detail struct {
			QualifiedName string `json:"qualifiedName"`
			DisplayName   string `json:"displayName"`
			Description   string `json:"description"`
			DeploymentUrl string `json:"deploymentUrl"`
			Connections   []struct {
				Type         string         `json:"type"`
				Url          string         `json:"url"`
				ConfigSchema map[string]any `json:"configSchema"`
			} `json:"connections"`
		}
		if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
			detailResp.Body.Close()
			continue
		}
		detailResp.Body.Close()
		// Find HTTP connection if available
		var endpoint string
		var transport string
		var configSchema map[string]any
		for _, conn := range detail.Connections {
			if conn.Type == "http" && conn.Url != "" {
				endpoint = conn.Url
				transport = "http"
				configSchema = conn.ConfigSchema
				break
			}
		}
		if endpoint == "" {
			continue // skip if no HTTP endpoint
		}
		entries = append(entries, RegistryEntry{
			Type:        "mcp_server",
			Name:        detail.QualifiedName,
			Description: detail.Description,
			Endpoint2:   endpoint,
			Transport:   transport,
			Parameters:  configSchema,
		})
	}
	return entries, nil
}

func (r *RemoteRegistryLoader) LoadManifest(name string) (*ToolManifest, error) {
	if m, ok := r.cache[name]; ok {
		return m, nil
	}
	var resp *http.Response
	var err error
	if r.Type == "smithery" {
		entries, err := fetchSmitheryMCPServers(r.IndexURL, r.APIKey)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			manifest := &ToolManifest{
				Name:        entry.Name,
				Description: entry.Description,
				Kind:        entry.Kind,
				Parameters:  entry.Parameters,
				Endpoint:    entry.Endpoint2,
				Headers:     map[string]string{"Authorization": "Bearer " + resolveEnvVar(r.APIKey)},
			}
			r.cache[manifest.Name] = manifest
			if manifest.Name == name {
				return manifest, nil
			}
		}
		return nil, logger.Errorf("tool %s not found in Smithery registry", name)
	}
	if r.Type == "smithery" {
		// Smithery: fetch server list with auth header
		req, err := http.NewRequest("GET", r.IndexURL+"?pageSize=1000", nil)
		if err != nil {
			return nil, err
		}
		apiKey := resolveEnvVar(r.APIKey)
		if apiKey == "" {
			return nil, logger.Errorf("Smithery API key not set")
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
	} else {
		resp, err = http.Get(r.IndexURL)
		if err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()
	if r.Type == "smithery" {
		// Smithery returns { servers: [...] }
		var data struct {
			Servers []struct {
				QualifiedName string `json:"qualifiedName"`
				DisplayName   string `json:"displayName"`
				Description   string `json:"description"`
				Homepage      string `json:"homepage"`
				IsDeployed    bool   `json:"isDeployed"`
			} `json:"servers"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return nil, err
		}
		for _, s := range data.Servers {
			if s.QualifiedName == name {
				// Fetch server details
				detailReq, err := http.NewRequest("GET", strings.TrimSuffix(r.IndexURL, "/servers")+"/servers/"+name, nil)
				if err != nil {
					return nil, err
				}
				detailReq.Header.Set("Authorization", "Bearer "+resolveEnvVar(r.APIKey))
				detailResp, err := http.DefaultClient.Do(detailReq)
				if err != nil {
					return nil, err
				}
				defer detailResp.Body.Close()
				var detail struct {
					QualifiedName string `json:"qualifiedName"`
					DisplayName   string `json:"displayName"`
					Description   string `json:"description"`
					DeploymentUrl string `json:"deploymentUrl"`
					Connections   []struct {
						Type         string         `json:"type"`
						Url          string         `json:"url"`
						ConfigSchema map[string]any `json:"configSchema"`
					} `json:"connections"`
				}
				if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
					return nil, err
				}
				// For each connection, register as ToolManifest
				for _, conn := range detail.Connections {
					manifest := &ToolManifest{
						Name:        detail.QualifiedName + "." + conn.Type,
						Description: detail.Description,
						Kind:        conn.Type,
						Parameters:  conn.ConfigSchema,
						Endpoint:    detail.DeploymentUrl,
						Headers:     map[string]string{"Authorization": "Bearer " + resolveEnvVar(r.APIKey)},
					}
					r.cache[manifest.Name] = manifest
					if manifest.Name == name {
						return manifest, nil
					}
				}
			}
		}
		return nil, logger.Errorf("tool %s not found in Smithery registry", name)
	}
	var index map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, err
	}
	entry, ok := index[name]
	if !ok {
		return nil, logger.Errorf("tool %s not found in registry", name)
	}
	entryMap, ok := entry.(map[string]any)
	if !ok {
		return nil, logger.Errorf("invalid registry entry for %s", name)
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
	return nil, logger.Errorf("no MCP or manifest URL for tool %s", name)
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
	return nil, logger.Errorf("tool %s not found in registry", name)
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

// Helper to resolve $env:VARNAME to os.Getenv(VARNAME)
func resolveEnvVar(val string) string {
	if strings.HasPrefix(val, "$env:") {
		return os.Getenv(strings.TrimPrefix(val, "$env:"))
	}
	return val
}

// LoadUnifiedRegistryFromConfig loads registry entries from a mix of sources:
// - registries: array of strings (file/url) or inline RegistryEntry objects
// - mcpServers: map[string]MCPServerConfig overrides
func LoadUnifiedRegistryFromConfig(registries []any, mcpServers map[string]MCPServerConfig) ([]*ToolManifest, []*MCPServerConfig, error) {
	var entries []RegistryEntry
	for _, reg := range registries {
		switch v := reg.(type) {
		case string:
			// Load from file or URL
			data, err := os.ReadFile(v)
			if err != nil {
				// Try as URL
				resp, err2 := http.Get(v)
				if err2 != nil {
					return nil, nil, logger.Errorf("failed to read registry %s: %v, %v", v, err, err2)
				}
				defer resp.Body.Close()
				data, err = io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, logger.Errorf("failed to read registry body %s: %v", v, err)
				}
			}
			var fileEntries []RegistryEntry
			if err := json.Unmarshal(data, &fileEntries); err != nil {
				return nil, nil, logger.Errorf("failed to parse registry %s: %v", v, err)
			}
			entries = append(entries, fileEntries...)
		case map[string]any:
			// Inline object
			b, _ := json.Marshal(v)
			var e RegistryEntry
			if err := json.Unmarshal(b, &e); err == nil {
				if e.Type == "smithery" {
					// Instead of RegistryEntry, create a loader and fetch all tools
					url, _ := v["url"].(string)
					apiKey, _ := v["apiKey"].(string)
					// Fetch all tools (pageSize=1000)
					req, err := http.NewRequest("GET", url+"?pageSize=1000", nil)
					if err != nil {
						continue
					}
					req.Header.Set("Authorization", "Bearer "+resolveEnvVar(apiKey))
					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						continue
					}
					var data struct {
						Servers []struct {
							QualifiedName string `json:"qualifiedName"`
							DisplayName   string `json:"displayName"`
							Description   string `json:"description"`
							Homepage      string `json:"homepage"`
							IsDeployed    bool   `json:"isDeployed"`
						} `json:"servers"`
					}
					if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
						resp.Body.Close()
						continue
					}
					resp.Body.Close()
					for _, s := range data.Servers {
						// Fetch server details
						detailReq, err := http.NewRequest("GET", strings.TrimSuffix(url, "/servers")+"/servers/"+s.QualifiedName, nil)
						if err != nil {
							continue
						}
						detailReq.Header.Set("Authorization", "Bearer "+resolveEnvVar(apiKey))
						detailResp, err := http.DefaultClient.Do(detailReq)
						if err != nil {
							continue
						}
						var detail struct {
							QualifiedName string `json:"qualifiedName"`
							DisplayName   string `json:"displayName"`
							Description   string `json:"description"`
							DeploymentUrl string `json:"deploymentUrl"`
							Connections   []struct {
								Type         string         `json:"type"`
								Url          string         `json:"url"`
								ConfigSchema map[string]any `json:"configSchema"`
							} `json:"connections"`
						}
						if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
							detailResp.Body.Close()
							continue
						}
						detailResp.Body.Close()
						for _, conn := range detail.Connections {
							entries = append(entries, RegistryEntry{
								Type:        "mcp_server",
								Name:        detail.QualifiedName + "." + conn.Type,
								Description: detail.Description,
								Kind:        conn.Type,
								Parameters:  conn.ConfigSchema,
								Endpoint2:   conn.Url,
								Transport:   conn.Type,
							})
						}
					}
				} else {
					entries = append(entries, e)
				}
			}
		}
	}
	// Merge mcpServers map as RegistryEntry objects
	for name, cfg := range mcpServers {
		entries = append(entries, RegistryEntry{
			Type:      "mcp_server",
			Name:      name,
			Command:   cfg.Command,
			Args:      cfg.Args,
			Env:       cfg.Env,
			Port:      cfg.Port,
			Transport: cfg.Transport,
			Endpoint2: cfg.Endpoint,
		})
	}
	return LoadUnifiedRegistryFromEntries(entries)
}
