package adapter

import (
	"context"
	"fmt"
)

// CoreEchoAdapter is the built-in echo adapter for debugging.
type CoreEchoAdapter struct{}

// ID returns the adapter ID.
func (a *CoreEchoAdapter) ID() string {
	return "core.echo"
}

// Execute prints the 'text' field to stdout and returns inputs unchanged.
func (a *CoreEchoAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	if t, ok := inputs["text"].(string); ok {
		fmt.Println(t)
	}
	return inputs, nil
}
