package adapter

import (
	"context"
	"fmt"
)

// CoreAdapter is the built-in echo adapter for debugging (consolidated from core_echo_adapter.go).
type CoreAdapter struct{}

// ID returns the adapter ID.
func (a *CoreAdapter) ID() string {
	return "core.echo"
}

// Execute prints the 'text' field to stdout and returns inputs unchanged.
func (a *CoreAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	if t, ok := inputs["text"].(string); ok {
		fmt.Println(t)
	}
	return inputs, nil
}

func (a *CoreAdapter) Manifest() *ToolManifest {
	return nil
}
