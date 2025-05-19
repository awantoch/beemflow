package adapter

import (
	"context"
	"errors"

	"github.com/awantoch/beemflow/registry"
)

// Adapter is the interface for all BeemFlow adapters. Implement this to add new tool integrations.
type Adapter interface {
	ID() string
	Execute(ctx context.Context, inputs map[string]any) (map[string]any, error)
	Manifest() *registry.ToolManifest
}

// ClosableAdapter is an optional interface for adapters that need cleanup.
type ClosableAdapter interface {
	Adapter
	Close() error
}

// Registry holds registered adapters and provides lookup and registration methods.
type Registry struct {
	adapters map[string]Adapter
}

// NewRegistry creates a new adapter registry.
func NewRegistry() *Registry {
	return &Registry{adapters: make(map[string]Adapter)}
}

// Register registers an adapter with the registry.
func (r *Registry) Register(a Adapter) {
	r.adapters[a.ID()] = a
}

// Get retrieves a registered adapter by ID.
func (r *Registry) Get(id string) (Adapter, bool) {
	a, ok := r.adapters[id]
	return a, ok
}

// LoadAndRegisterTool loads a tool manifest from a local directory and registers an HTTPAdapter.
func (r *Registry) LoadAndRegisterTool(name, toolsDir string) error {
	if _, exists := r.adapters[name]; exists {
		return nil
	}
	localRegistry := registry.NewLocalRegistry(toolsDir)
	entry, err := localRegistry.GetServer(context.Background(), name)
	if err != nil {
		return err
	}
	if entry == nil {
		return errors.New("tool not found")
	}
	manifest := &registry.ToolManifest{
		Name:        entry.Name,
		Description: entry.Description,
		Kind:        entry.Kind,
		Parameters:  entry.Parameters,
		Endpoint:    entry.Endpoint,
		Headers:     entry.Headers,
	}
	r.Register(&HTTPAdapter{AdapterID: name, ToolManifest: manifest})
	return nil
}

// Add CloseAll to Registry to close all adapters that support it.
func (r *Registry) CloseAll() error {
	var firstErr error
	for _, a := range r.adapters {
		if ca, ok := a.(ClosableAdapter); ok {
			if err := ca.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// All returns every registered adapter.
func (r *Registry) All() []Adapter {
	out := make([]Adapter, 0, len(r.adapters))
	for _, a := range r.adapters {
		out = append(out, a)
	}
	return out
}
