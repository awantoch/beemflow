package loader

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"os"

	"github.com/awantoch/beemflow/dsl"
	"github.com/awantoch/beemflow/model"
	jsonnet "github.com/google/go-jsonnet"
)

// Load reads a flow definition from the given path. It supports YAML/YML, JSON,
// and Jsonnet (.jsonnet, .libsonnet) files. Variables are passed through to the
// YAML renderer and as Jsonnet external variables when possible.
func Load(path string, vars map[string]any) (*model.Flow, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".jsonnet", ".libsonnet":
		return loadJsonnet(path, vars)
	case ".json":
		return loadJSON(path)
	case ".yml", ".yaml":
		// Re-use existing DSL loader which handles templating & validation.
		return dsl.Load(path, vars)
	default:
		// Fallback to YAML for unknown extensions (maintains current behaviour).
		return dsl.Load(path, vars)
	}
}

// loadJSON reads a raw JSON file and unmarshals it into a Flow struct.
func loadJSON(path string) (*model.Flow, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var flow model.Flow
	if err := json.Unmarshal(b, &flow); err != nil {
		return nil, err
	}
	if err := dsl.Validate(&flow); err != nil {
		return nil, err
	}
	return &flow, nil
}

// loadJsonnet evaluates a Jsonnet file, optionally injecting vars as external
// variables, then unmarshals the resulting JSON into a Flow struct.
func loadJsonnet(path string, vars map[string]any) (*model.Flow, error) {
	vm := jsonnet.MakeVM()

	// Provide simple string vars as ext vars for convenience.
	for k, v := range vars {
		if s, ok := v.(string); ok {
			vm.ExtVar(k, s)
		}
	}

	jsonStr, err := vm.EvaluateFile(path)
	if err != nil {
		return nil, err
	}

	var flow model.Flow
	if err := json.Unmarshal([]byte(jsonStr), &flow); err != nil {
		return nil, err
	}
	if err := dsl.Validate(&flow); err != nil {
		return nil, err
	}
	return &flow, nil
}