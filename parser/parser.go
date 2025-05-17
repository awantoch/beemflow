package parser

import (
	"encoding/json"
	"os"

	"github.com/awantoch/beemflow/model"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

// ParseFlow loads and parses a YAML flow file into a Flow struct.
func ParseFlow(path string) (*model.Flow, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var flow model.Flow
	if err := yaml.Unmarshal(f, &flow); err != nil {
		return nil, err
	}
	return &flow, nil
}

// ValidateFlow validates a flow against the BeemFlow JSON-Schema.
var ValidateFlow = validateFlow

func validateFlow(flow *model.Flow, schemaPath string) error {
	// Marshal the flow to JSON for validation
	jsonBytes, err := json.Marshal(flow)
	if err != nil {
		return err
	}
	// Load the schema (from file or embedded resource)
	schema, err := jsonschema.Compile(schemaPath)
	if err != nil {
		return err
	}
	// Unmarshal JSON into a generic interface for validation
	var doc interface{}
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		return err
	}
	// Validate the flow
	return schema.Validate(doc)
}
