package adapter

import "context"

// Adapter is the interface for all BeemFlow adapters.
type Adapter interface {
	ID() string
	Execute(ctx context.Context, inputs map[string]any) (map[string]any, error)
	Manifest() *ToolManifest
}

// Registry holds registered adapters.
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
	loader := &LocalManifestLoader{Dir: toolsDir}
	manifest, err := loader.LoadManifest(name)
	if err != nil {
		return err
	}
	r.Register(&HTTPAdapter{id: name, manifest: manifest})
	return nil
}

func (a *HTTPAdapter) Manifest() *ToolManifest {
	return a.manifest
}
