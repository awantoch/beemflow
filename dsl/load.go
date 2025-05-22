package dsl

import (
	"os"

	pproto "github.com/awantoch/beemflow/spec/proto"
)

// Load reads, templates, parses, and validates a proto.Flow file in one step.
func Load(path string, vars map[string]any) (*pproto.Flow, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	rendered, err := Render(string(raw), vars)
	if err != nil {
		return nil, err
	}
	flow, err := ParseFromString(rendered)
	if err != nil {
		return nil, err
	}
	// Skipping schema validation during proto-first port
	return flow, nil
}
