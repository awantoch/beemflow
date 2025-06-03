package registry

import (
	"context"
	_ "embed"
	"encoding/json"
)

//go:embed default.json
var defaultRegistryData []byte

// DefaultRegistry provides default SaaS tools embedded in the binary
type DefaultRegistry struct {
	Registry string
}

// NewDefaultRegistry creates a new default registry
func NewDefaultRegistry() *DefaultRegistry {
	return &DefaultRegistry{
		Registry: "default",
	}
}

// ListServers returns all default registry entries
func (d *DefaultRegistry) ListServers(ctx context.Context, opts ListOptions) ([]RegistryEntry, error) {
	var entries []RegistryEntry
	if err := json.Unmarshal(defaultRegistryData, &entries); err != nil {
		return nil, err
	}

	// Label all entries with default registry
	for i := range entries {
		entries[i].Registry = d.Registry
	}

	return entries, nil
}

// GetServer finds a specific server/tool by name from the default registry
func (d *DefaultRegistry) GetServer(ctx context.Context, name string) (*RegistryEntry, error) {
	entries, err := d.ListServers(ctx, ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.Name == name {
			return &entry, nil
		}
	}

	return nil, nil // Not found
}
