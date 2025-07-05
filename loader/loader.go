package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/awantoch/beemflow/docs"
	"github.com/awantoch/beemflow/model"
	"github.com/google/go-jsonnet"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

// Load reads, parses, and validates a flow file based on its extension
func Load(path string, vars map[string]any) (*model.Flow, error) {
	ext := strings.ToLower(filepath.Ext(path))
	
	switch ext {
	case ".yaml", ".yml":
		// For YAML files, parse directly without templating
		return loadYAMLFlow(path, vars)
	case ".json":
		// For JSON files, parse directly
		return loadJSONFlow(path)
	case ".jsonnet", ".libsonnet":
		// For Jsonnet files, evaluate and parse
		return loadJsonnetFlow(path, vars)
	default:
		// Default to YAML for unknown extensions
		return loadYAMLFlow(path, vars)
	}
}

// loadYAMLFlow loads a YAML flow file
func loadYAMLFlow(path string, vars map[string]any) (*model.Flow, error) {
	// For pure YAML (no templating), we ignore vars
	flow, err := parseYAMLFile(path)
	if err != nil {
		return nil, err
	}
	
	if err := Validate(flow); err != nil {
		return nil, err
	}
	
	return flow, nil
}

// loadJSONFlow loads a JSON flow file
func loadJSONFlow(path string) (*model.Flow, error) {
	flow, err := parseJSONFile(path)
	if err != nil {
		return nil, err
	}
	
	if err := Validate(flow); err != nil {
		return nil, err
	}
	
	return flow, nil
}

// loadJsonnetFlow loads and evaluates a Jsonnet flow file
func loadJsonnetFlow(path string, vars map[string]any) (*model.Flow, error) {
	vm := jsonnet.MakeVM()
	
	// Add variables to Jsonnet VM
	for k, v := range vars {
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		vm.ExtVar(k, string(jsonBytes))
	}
	
	// Evaluate Jsonnet to JSON
	jsonStr, err := vm.EvaluateFile(path)
	if err != nil {
		return nil, err
	}
	
	// Parse the resulting JSON
	var flow model.Flow
	if err := json.Unmarshal([]byte(jsonStr), &flow); err != nil {
		return nil, err
	}
	
	if err := Validate(&flow); err != nil {
		return nil, err
	}
	
	return &flow, nil
}

// parseYAMLFile reads and parses a YAML file
func parseYAMLFile(path string) (*model.Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseFromString(string(data))
}

// parseJSONFile reads and parses a JSON file
func parseJSONFile(path string) (*model.Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var flow model.Flow
	if err := json.Unmarshal(data, &flow); err != nil {
		return nil, err
	}
	return &flow, nil
}

// ParseFromString unmarshals a YAML string into a Flow struct.
func ParseFromString(yamlStr string) (*model.Flow, error) {
	var flow model.Flow
	if err := yaml.Unmarshal([]byte(yamlStr), &flow); err != nil {
		return nil, err
	}
	return &flow, nil
}

// Validate runs JSON-Schema validation against the embedded BeemFlow schema.
func Validate(flow *model.Flow) error {
	// Marshal the flow to JSON for validation
	jsonBytes, err := json.Marshal(flow)
	if err != nil {
		return err
	}
	// Compile the embedded schema
	schema, err := jsonschema.CompileString("beemflow.schema.json", docs.BeemflowSchema)
	if err != nil {
		return err
	}
	// Unmarshal JSON into a generic interface for validation
	var doc any
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		return err
	}
	// Validate the flow
	return schema.Validate(doc)
}