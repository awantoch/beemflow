package registry

import (
	"context"
	"encoding/json"
	"os"
)

// RegistryEntry is a unified representation of a server/tool in any registry.
type RegistryEntry struct {
	Registry    string            `json:"registry"` // e.g. "smithery", "local"
	Type        string            `json:"type"`
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
}

// ListOptions allows filtering and pagination for registry queries.
type ListOptions struct {
	Query    string
	Page     int
	PageSize int
}

// MCPRegistry is the interface for any MCP registry backend (Smithery, local, etc).
type MCPRegistry interface {
	ListServers(ctx context.Context, opts ListOptions) ([]RegistryEntry, error)
	GetServer(ctx context.Context, name string) (*RegistryEntry, error)
}

type LocalRegistry struct {
	Path string
}

func NewLocalRegistry(path string) *LocalRegistry {
	if path == "" {
		path = "registry/index.json"
	}
	return &LocalRegistry{Path: path}
}

func (l *LocalRegistry) ListServers(ctx context.Context, opts ListOptions) ([]RegistryEntry, error) {
	data, err := os.ReadFile(l.Path)
	if err != nil {
		return nil, err
	}
	var entries []RegistryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].Registry = "local"
	}
	return entries, nil
}

func (l *LocalRegistry) GetServer(ctx context.Context, name string) (*RegistryEntry, error) {
	servers, err := l.ListServers(ctx, ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, s := range servers {
		if s.Name == name {
			return &s, nil
		}
	}
	return nil, nil
}
