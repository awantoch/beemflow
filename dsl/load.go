package dsl

import (
	"os"

	"github.com/awantoch/beemflow/model"
)

// Load reads, templates, parses, and validates a flow file in one step.
func Load(path string, vars map[string]any) (*model.Flow, error) {
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
	if err := Validate(flow); err != nil {
		return nil, err
	}
	return flow, nil
}
