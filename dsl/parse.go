package dsl

import (
	"os"

	"gopkg.in/yaml.v3"

	"github.com/awantoch/beemflow/model"
)

// Parse reads a YAML flow file from the given path and unmarshals it into a Flow struct.
func Parse(path string) (*model.Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseFromString(string(data))
}

// ParseFromString unmarshals a YAML string into a Flow struct.
func ParseFromString(yamlStr string) (*model.Flow, error) {
	var flow model.Flow
	if err := yaml.Unmarshal([]byte(yamlStr), &flow); err != nil {
		return nil, err
	}
	return &flow, nil
}
