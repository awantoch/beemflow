package registry

import (
	"context"
)

type RegistryManager struct {
	registries []MCPRegistry
}

func NewRegistryManager(registries ...MCPRegistry) *RegistryManager {
	return &RegistryManager{registries: registries}
}

// ListAllServers aggregates servers from all registries.
func (m *RegistryManager) ListAllServers(ctx context.Context, opts ListOptions) ([]RegistryEntry, error) {
	var all []RegistryEntry
	for _, reg := range m.registries {
		servers, err := reg.ListServers(ctx, opts)
		if err != nil {
			return nil, err
		}
		all = append(all, servers...)
	}
	return all, nil
}

// GetServer finds a server by name from any registry.
func (m *RegistryManager) GetServer(ctx context.Context, name string) (*RegistryEntry, error) {
	for _, reg := range m.registries {
		entry, err := reg.GetServer(ctx, name)
		if err == nil && entry != nil {
			return entry, nil
		}
	}
	return nil, nil // or return an error if not found
}
