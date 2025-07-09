package dsl

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/awantoch/beemflow/graph"
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

// FlowToVisual converts a Flow to React Flow format for the visual editor
func FlowToVisual(flow *model.Flow) (map[string]interface{}, error) {
	return graph.ExportReactFlow(flow)
}

// VisualToFlow converts React Flow visual data back to a Flow
func VisualToFlow(visualData map[string]interface{}) (*model.Flow, error) {
	nodesData, ok := visualData["nodes"]
	if !ok {
		return nil, fmt.Errorf("no nodes data provided")
	}
	
	nodes := extractNodes(nodesData)
	if nodes == nil {
		return nil, fmt.Errorf("invalid nodes data format")
	}
	
	steps := make([]model.Step, 0, len(nodes))
	for _, nodeData := range nodes {
		if step := extractStep(nodeData); step != nil {
			steps = append(steps, *step)
		}
	}
	
	return &model.Flow{
		Name:  "editor_flow",
		On:    "cli.manual",
		Steps: steps,
	}, nil
}

// Helper: Extract nodes from various formats
func extractNodes(nodesData interface{}) []interface{} {
	switch v := nodesData.(type) {
	case []interface{}:
		return v
	case []map[string]interface{}:
		nodes := make([]interface{}, len(v))
		for i, node := range v {
			nodes[i] = node
		}
		return nodes
	default:
		return nil
	}
}

// Helper: Extract step from node data
func extractStep(nodeData interface{}) *model.Step {
	node, ok := nodeData.(map[string]interface{})
	if !ok {
		return nil
	}
	
	data, ok := node["data"].(map[string]interface{})
	if !ok {
		return nil
	}
	
	step := &model.Step{
		ID:  getString(data, "id"),
		Use: getString(data, "use"),
	}
	
	if withData, exists := data["with"]; exists {
		if withMap, ok := withData.(map[string]interface{}); ok {
			step.With = withMap
		}
	}
	
	if ifCondition := getString(data, "if"); ifCondition != "" {
		step.If = ifCondition
	}
	
	return step
}

// Helper: Safely get string from map
func getString(data map[string]interface{}, key string) string {
	if value, exists := data[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}