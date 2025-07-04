package dsl

import (
	"gopkg.in/yaml.v3"

	"github.com/awantoch/beemflow/model"
)

// FlowToYAML converts a Flow struct to YAML bytes
func FlowToYAML(flow *model.Flow) ([]byte, error) {
	return yaml.Marshal(flow)
}

// FlowToYAMLString converts a Flow struct to a YAML string
func FlowToYAMLString(flow *model.Flow) (string, error) {
	bytes, err := FlowToYAML(flow)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}