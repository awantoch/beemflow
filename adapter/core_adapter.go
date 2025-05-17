package adapter

import (
	"context"
	"os"

	"github.com/awantoch/beemflow/pkg/logger"
)

// CoreAdapter is the built-in echo adapter for debugging (consolidated from core_echo_adapter.go).
type CoreAdapter struct{}

// ID returns the adapter ID.
func (a *CoreAdapter) ID() string {
	return "core.echo"
}

// Execute prints the 'text' field to stdout and returns inputs unchanged.
func (a *CoreAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	if text, ok := inputs["text"].(string); ok {
		if os.Getenv("BEEMFLOW_DEBUG") != "" {
			logger.Info("%s", text)
		}
	}
	return inputs, nil
}

func (a *CoreAdapter) Manifest() *ToolManifest {
	return nil
}
